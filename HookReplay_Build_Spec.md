# HookReplay — Build Specification (for swarm execution)

**Version:** 1.0 · 2026-09-26
**Companion doc:** `HookReplay_Project_Brief_v2.md` — strategy, pricing, competitive positioning. Read it first; this spec is the engineering contract derived from it.
**Status:** MVP scope (Phases 1–2 of the brief). Phase 3 (recorder/replay) and Phase 4 (enterprise) are specified at interface level only.

---

## 0. How a swarm should use this document

- **Contracts first, implementation second.** Sections 4 (data model), 5 (API), 6 (chain YAML schema), and 7 (signing spec) are normative. Workstreams may be built in parallel against these contracts; do not change a contract without updating this file and notifying dependent workstreams.
- **Each workstream (§9) is self-contained** with its own definition of done. A workstream may be assigned to one agent; cross-workstream integration happens at the contract boundaries.
- **Test vectors are part of the spec.** §7.4 defines the required sandbox verification procedure; payment templates are not "done" without it.
- **Conventions (§2) are mandatory** across all workstreams.

---

## 1. Product summary (one paragraph)

HookReplay lets developers fire provider-accurate, correctly signed webhooks at localhost or staging; define multi-step webhook sequences ("chains") as YAML in their repos and run them from a CLI or CI; and (Phase 3) record production webhook traffic verbatim and replay it to staging with freshly generated signatures. Core promise: first valid webhook in under 2 minutes.

---

## 2. Mandatory conventions

| Convention | Rule |
|---|---|
| Language, services | Go 1.23+ for API, CLI, tunnel, chain runner. Next.js 14 + Tailwind for dashboard. PostgreSQL 16 + Redis 7. |
| YAML | Block style only. Ship `hookreplay fmt` (normalizes to block style) with `hookreplay init`. |
| Secrets | Never in YAML, never in logs, never in CLI output. YAML references `env:NAME` only. Server-side secrets: AES-256-GCM at rest, decrypted only at dispatch, write-only API (no read endpoint). |
| Signing | Always at dispatch time, over raw body bytes, per scheme spec (§7). Never pre-sign at chain-parse time. |
| Timeouts | Dispatcher per-attempt timeout 30s default; per-target circuit breaker (§5.4). |
| IDs | `hrk_` API keys, `evt_` events/executions, `chn_` chains, `tmpl_` templates, `sec_` stored secrets (names only in API responses). |
| Errors | JSON: `{ "error": { "code": "snake_case", "message": "human readable", "request_id": "..." } }`. |
| Versioning | All API routes under `/v1/`. CLI and API versioned independently. |

---

## 3. Repository structure (monorepo)

```
hookreplay/
├── cmd/
│   ├── api/            # API server (WS-2)
│   ├── cli/            # hookreplay binary: run, tunnel, secrets, chains, templates (WS-3)
│   └── worker/         # dispatch worker (WS-2)
├── internal/
│   ├── signing/        # signing engine + provider schemes (WS-1)
│   ├── templates/      # template loading, versioning, overrides (WS-5)
│   ├── chains/         # YAML parser, validator, executor (WS-4)
│   ├── dispatch/       # dispatcher, circuit breaker, rate limiter (WS-2)
│   ├── tunnel/         # websocket tunnel server + client (WS-7)
│   ├── auth/           # API keys, OAuth, workspaces (WS-8)
│   ├── billing/        # tiers, metering, limits (WS-8)
│   ├── recorder/       # Phase 3: capture tap (interface-level now)
│   └── replay/         # Phase 3: re-signing replay (interface-level now)
├── templates/          # 22 provider templates as YAML+JSON fixtures (WS-5)
│   └── providers/stripe/ ...
├── pkg/
│   ├── chainschema/    # chain YAML JSON-schema (shared CLI/API)
│   └── apiclient/      # thin typed client for dashboard/CLI
├── web/                # Next.js dashboard (WS-6)
├── .github/workflows/  # CI: unit, integration, sandbox soak gate, e2e
└── docs/               # generated API docs, chain schema docs
```

---

## 4. Data model (PostgreSQL)

```sql
workspaces(id, name, tier free|pro|team|enterprise, created_at)
users(id, email, oauth_provider, created_at)
workspace_members(workspace_id, user_id, role owner|admin|member, seat_count basis)
api_keys(id, user_id, workspace_id, prefix, hashed_key, scopes, last_used_at)
domains(id, workspace_id, domain, verification_token, verified_at, method dns_txt|served_token)
secrets(id, workspace_id, env local|staging|production, name, ciphertext, key_version, created_at)
  -- write-only: no SELECT of plaintext outside dispatch path; no API returns ciphertext
templates(id, provider, event, version, schema jsonb, verified_at, verified_against, status)
custom_templates(id, workspace_id, name, payload jsonb, signing_config jsonb, created_by)
chains(id, workspace_id, source repo|dashboard, name, yaml_text, version, created_by, updated_at)
executions(id, workspace_id, chain_id NULL, step_name, target_kind tunnel|url,
           target, provider NULL, template_id NULL, request_body BYTEA,   -- raw bytes
           request_headers jsonb, signature_header jsonb, deliver jsonb,   -- result/reason
           response_status int, response_body BYTEA, latency_ms int,
           attempt int, parent_execution_id NULL, idempotency_key, created_at)
  -- request_body stored as raw bytes verbatim; this is load-bearing for replay (brief §3.5)
tunnels(id, workspace_id, subdomain, connected_at, last_seen_at, cli_version)
rate_counters(key, window_start, count)            -- Redis-backed; PG table as backup
metering(workspace_id, period, executions_count, replays_count)  -- monthly rollup

-- Phase 3: capture (v2.2 contract — no raw persistence)
recorded_events(id, workspace_id, source, source_event_id, received_at,
                body_sanitized BYTEA,          -- sanitized-at-capture, encrypted
                redactions jsonb,              -- paths touched + actions + schema version
                schema_name, schema_version,
                forward_status int, forward_latency_ms int,
                response_status int, response_body_sanitized BYTEA, response_retention_until)
  -- NO raw body column. NO original signature headers. Raw exists only in memory in flight.
capture_routes(id, workspace_id, source, inbound_path, forward_target, created_at)
```

Indexes: `executions(workspace_id, created_at)`, `executions(idempotency_key)` unique per workspace, `secrets(workspace_id, env, name)` unique, `recorded_events(workspace_id, received_at)`.

**Design invariants:**
- `executions.request_body` is byte-exact. Any code path that parses-then-reserializes before storage is a bug.
- `recorded_events` has **no raw-persistence path by construction**: the table has no column that can hold a raw body. Sanitization runs before INSERT, in the async post-response stage (§7.6).
- Secrets table has no application-level read path except the dispatch worker, which decrypts in-memory, uses, and zeroizes.

---

## 5. API surface (Go, REST, JSON)

### 5.1 Auth & workspaces
```
POST   /v1/auth/oauth/callback        # browser OAuth (GitHub first)
POST   /v1/api-keys                   # create personal/workspace key
GET    /v1/me
GET    /v1/workspaces                 # list memberships
POST   /v1/workspaces/:id/members     # invite (Team tier)
```

### 5.2 Domains (endpoint verification — MVP)
```
POST   /v1/domains                    # {domain} → {verification_token, method}
POST   /v1/domains/:id/verify         # checks DNS TXT or served token path /.well-known/hookreplay-verify
GET    /v1/domains                    # list, verified status
```
Rule: dispatch to a public URL requires a verified domain match (exact host or wildcard registered by workspace). Tunnel subdomains are pre-authorized.

### 5.3 Secrets
```
PUT    /v1/secrets/:env/:name         # write-only (create/update)
DELETE /v1/secrets/:env/:name
GET    /v1/secrets                    # names + envs + updated_at only — NEVER values
```

### 5.4 Dispatch
```
POST   /v1/dispatch                   # single fire: {template|custom, target, secret_env_ref, overrides, deliver}
POST   /v1/chains/execute             # {yaml_text | chain_id, target, env} → {run_id}
GET    /v1/runs/:id                   # run status, per-step executions, expect results
POST   /v1/dispatch/:id/retry         # re-fire identical execution (idempotency-key aware)
```
Server behaviors (all mandatory):
- **Rate limit:** 10 req/s per workspace, burst 30 (token bucket, Redis). Running chains: max 5 concurrent per workspace.
- **Circuit breaker:** per target host: 5 consecutive connection errors or 5xx → open 60s, half-open probe. Record breaker state in executions metadata.
- **Timeout:** 30s per attempt; timeout recorded as `response_status = 0`, `error_code = "target_timeout"`.
- **Idempotency:** client-supplied `Idempotency-Key` header; duplicate within 24h returns the original execution result.
- **Retry observation (for `expect.retries`):** when an execution's response is non-2xx, the dispatcher schedules re-deliveries per the provider's documented backoff schedule ( Stripe: see §7.1; Paystack: 5m/30m/2h; etc. — table in §7.5), up to the provider's documented max attempts, recording each attempt as a separate `executions` row sharing `parent_execution_id`. Backoff tables are data, not code — `templates/providers/*/delivery.yml`.

### 5.5 Templates & chains
```
GET    /v1/templates                  # library (public)
GET    /v1/templates/:provider/:event
GET/POST/PUT/DELETE /v1/custom-templates      # Pro+
GET/POST/PUT /v1/chains               # dashboard-authored chains (source=dashboard)
POST   /v1/chains/validate            # lint YAML against schema (used by CLI fmt --check and dashboard)
```

### 5.6 Usage & limits (for dashboard + CLI)
```
GET    /v1/usage?period=2026-09       # executions, replays, chain runs vs tier limits
```

### 5.7 Phase 3 interfaces (specify now, implement later)
```
POST   /v1/recorder-endpoints         # provisions hooks.hookreplay.dev/{ws}/capture/{source} + forward target
POST   /v1/recorded-events/:id/replay # {target_env} → re-signs per §7.5 over sanitized body, dispatches
GET    /v1/recorded-events            # sanitized views only; every response carries redactions manifest
PUT    /v1/sanitization-schemas/:name # Team tier; built-in "default" readable by all, not modifiable
```
The capture ingress implements the §7.6 streaming-proxy contract — it is a delivery-critical path, not a background job.

### 5.8 Tier enforcement (server-side, non-negotiable)
- **Free:** dispatch targets restricted to workspace-owned tunnels. Public URLs → `403 tier_required: pro` with upgrade link. Chain runs allowed (3 concurrent).
- **Pro:** verified-domain targets allowed; replay metered as add-on packs.
- **Team:** GitHub Action runs (identified by `X-Client: hookreplay-action` on the API key request — analytics signal only, never the security boundary) require tier ≥ team for `--env staging` chain executions; the 403 response body includes a 14-day trial start link.
- Enforcement point: `POST /v1/chains/execute` and `POST /v1/dispatch`, resolved from `workspaces.tier` at request time. Never trust client-reported tier.

---

## 6. Chain YAML schema (normative)

File: `hookreplay.yml` at repo root (or `--file`). JSON-schema shipped in `pkg/chainschema`.

```yaml
name: payment-onboarding-flow          # required, [a-z0-9-], ≤64
target: ${TUNNEL_URL}                  # optional; --target flag wins; ${ENV} interpolated by CLI pre-run
environment: staging                   # optional; selects verified domain + env-scoped secrets
concurrency: 3                         # optional, 1..5 (default 3); max concurrent *chains* per run is server-capped

steps:                                 # required, 1..50
  - name: payment-received             # required, unique within chain, [a-z0-9-]
    webhook: stripe/checkout.session.completed   # either webhook: or custom: or api:
    secret: env:STRIPE_WEBHOOK_SECRET            # required for webhook/custom with sign
    override:                            # optional, dotted-path patches applied to template
      data.object.amount_total: 20000
    delay: 3s                            # optional, before this step
    deliver:                             # optional; default {result: success}
      result: fail
      reason: invalid_signature          # enum: invalid_signature|missing_signature|stale_signature
                                           #      |malformed_payload|connection_refused
    expect:                              # optional; unmet → step fails
      status: 200                        # int or range "2xx"
      latency_under: 3s                  # optional
      retries: 3                         # optional; assertions on re-delivery count
      delays: ["5m", "30m", "2h"]        # optional; exact backoff sequence observed
      body:                              # optional
        json_path: "$.status"
        equals: paid                     # or: contains, exists: true
  - name: order-must-exist
    api:
      method: GET
      url: "{{target}}/api/orders/{{steps.payment-received.body.data.object.id}}"
      headers: {X-Test: "true"}          # optional
      timeout: 10s                       # optional
    expect: {status: 200}
  - name: internal-invoice
    custom:
      payload: ./fixtures/invoice.paid.json        # file path (≤256KB) or inline object
      headers: {X-Event-Name: invoice.paid}
      sign:
        algorithm: hmac-sha256           # hmac-sha1|hmac-sha256|hmac-sha512|ed25519|none
        secret: env:INTERNAL_WEBHOOK_SECRET
        header: X-Signature
        format: "{{timestamp}}.{{hmac}}" # hmac over: raw|{{timestamp}}.body|body.{{timestamp}}|custom
        encoding: hex                    # hex|base64 (default hex)
        timestamp_header: X-Timestamp    # optional; required when format uses {{timestamp}}
```

**Placeholder rules (CLI validates):**
- `${VAR}` — environment interpolation, resolved once before the run. Unknown var → hard error listing the name.
- `{{target}}`, `{{steps.<name>.body...}}`, `{{steps.<name>.status}}`, `{{timestamp}}` (within sign.format) — runtime context, resolved per step. Referencing a step defined later → validation error. Referencing a path that doesn't exist at runtime → step fails with `extraction_error` naming the path.

**Execution semantics:**
- Sequential unless `concurrency` on parallel `name:` groups — MVP: sequential only; concurrency field reserved.
- Each `webhook`/`custom` step: render payload → apply overrides → sign raw bytes at dispatch → POST → record. `deliver.result: fail` mutates the outgoing request per `reason` (§7.7).
- Each `api` step: HTTP call from HookReplay to the user's app (the only direction where we call *them*); response feeds context.
- `expect` evaluated after the step's response and after any observed retries. `--fail-on expect` (CLI) → non-zero exit; dashboard always reports.

---

## 7. Signing specification

### 7.1 Scheme implementations (WS-1)

| Provider | Scheme | Verify note |
|---|---|---|
| Stripe | `t={unix}.{v1}` where `v1 = HMAC-SHA256("{t}.{body}", whsec)` hex; header `Stripe-Signature`; sign with **current** timestamp at dispatch; tolerance 180s | Test both fresh and stale rejection |
| GitHub | `sha256={hex(HMAC-SHA256(body, secret))}` header `X-Hub-Signature-256` | hex, sha256= prefix |
| Slack | `v0:{hmac-sha256("v0:{ts}:{body}")}` header `X-Slack-Signature`; `X-Slack-Request-Timestamp` fresh at dispatch | body prefix `v0:` mandatory |
| Meta | `sha256={hex}` header `X-Hub-Signature-256` | app-secret proof documented in template |
| Paystack | `hex(HMAC-SHA512(body, secret))` header `x-paystack-signature` | **hex, not base64** — CI vector mandatory |
| Razorpay | `hex(HMAC-SHA256(body, secret))` header `x-razorpay-signature` | |
| Flutterwave | header `verif-hash = hex(sha256(secret))` — static, does not bind payload | Template docs MUST state: verify-only scheme; recommend IP allowlisting. Failure-sim semantics: `deliver.reason` values that imply payload tamper-detection are N/A for Flutterwave; reject at validation time |
| Resend | Ed25519(secret_key) over body; header `Resend-Signature`; includes timestamp header `Resend-Signature-Timestamp` | keypair management in CLI (`hookreplay keys generate` for custom ed25519) |
| Twilio | base64(HMAC-SHA1(url+params+body concatenation, auth_token)) header `X-Twilio-Signature` | exact concatenation order from docs; CI vector mandatory |
| Klarna/Adyen | base64(HMAC-SHA256(body, secret)) | base64 padding exact |
| Paytm | AES-256-CBC(sha256hex(canonical_param_string), merchant_key) base64 — **Phase 3/Batch 3**, interface reserved | canonical ordering table required before implementation |
| Custom | per chain YAML `sign:` block | format/encoding/timestamp_header per §6 |

All HMAC over **raw request body bytes** as they will be transmitted. The signer receives the final byte slice; transport does not touch it.

### 7.2 `deliver.reason` semantics (WS-2)
| reason | Effect on outgoing request |
|---|---|
| `invalid_signature` | Sign with a different random key of the same scheme |
| `missing_signature` | Omit signature header entirely |
| `stale_signature` | Sign with timestamp now−(tolerance+60s) |
| `malformed_payload` | Corrupt body bytes (flip bytes in random JSON string value; keep content-type) |
| `connection_refused` | Dispatcher attempts TCP connect to target and records refusal; no request sent |

### 7.3 Retry backoff schedules (`delivery.yml` per provider)
Ship as data: `attempt_intervals: ["5m","30m","2h","6h","24h"]`, `max_attempts`, `timeout_s`. Dispatcher reads this to schedule re-deliveries when the user's endpoint returns non-2xx (enables `expect.retries` / `expect.delays`).

### 7.4 Sandbox verification gate (payment templates — CI-enforced)
For each payment provider template before `status: live`:
1. Create sandbox webhook endpoint in provider dashboard pointing at `hooks.hookreplay.dev/ci-verify/{provider}`.
2. Trigger a real sandbox event (test payment).
3. Capture the real request: extract body bytes + signature header + secret.
4. Independently verify signature matches body per §7.1 using a separate verifier implementation.
5. Generate a synthetic payload with our signer; verify it with the same independent verifier; then confirm the *provider's sandbox endpoint accepts our generated signature* where the provider offers a verify endpoint (Stripe: `stripe trigger`-equivalent validation; others: docs-published algorithm cross-check).
6. Freeze inputs/outputs as CI test vectors: `templates/providers/{p}/testdata/vector_*.json` — `{secret, body_b64, expected_signature}`.
7. Stamp template `verified_at` + `verified_against: sandbox|docs`.

CI blocks merge if any payment template lacks current vectors.

### 7.5 Replay re-signing (Phase 3 interface)
Replay = stored **sanitized** `recorded_events.body_sanitized` + fresh signature for target env's configured secret, scheme per §7.1. Timestamped schemes always re-stamp. Flutterwave: regenerate static hash from target env's secret. Paytm: regenerate if key configured, else `signature_mode: passthrough` with warning. Never mutate stored bytes. Replay is byte-identical to the sanitized event — this is deliberate (staging must not receive real PII); hash-pseudonyms preserve correlation.

### 7.6 Capture pipeline contract (Phase 3 — normative)

**Streaming proxy, never store-and-forward:**
1. On provider request arrival: parse headers, resolve route (capture endpoint → forwarding target), open the downstream connection **immediately**.
2. Stream request body downstream chunk-by-chunk; tee each chunk into an in-memory, encrypted sanitizer buffer (bounded; oversized → abort capture, never delivery).
3. Stream the customer's response back to the provider. Added latency budget: **≤ 100ms p99 end-to-end** beyond the direct path; measured per event as `forward_latency_ms`.
4. Only after the provider's connection closes: run sanitizer → encrypt → persist `recorded_events` (async, retryable). DB failure = capture gap logged via `capture_failed` metric; delivery unaffected.
5. Raw material exists only in memory and is zeroized at request end. Sole exception: if *our* forward failed and we must re-deliver, hold the body in an **encrypted retry buffer with minutes-level TTL** — never write raw to a durable store.
6. Do not store original signature headers (secret-derived in some schemes; regenerated at replay anyway).

### 7.7 Sanitization schemas

Applied **at capture time** (pipeline §7.6.4), never at display time, and never mutating anything the customer received — forwarding is verbatim.

```yaml
# built-in schema (all tiers): schema_id "default", versioned
rules:
  - match: { json_path: "$..email" }            action: pseudonymize   # usr_<sha256[:12]>, stable per workspace+salt
  - match: { regex: '\b\d{13,19}\b' }           action: mask_keep_last4
  - match: { regex: '(?i)authorization|x-api-key' , scope: headers }  action: redact
  - match: { json_path: "$..phone" }            action: redact

# custom schema (Team): per-path rules + exceptions
rules:
  - match: { json_path: "$.data.customer.email" }  action: pseudonymize
  - match: { json_path: "$.data.currency" }        action: keep_verbatim   # exception
```
Actions: `redact` (→ "***"), `mask_keep_last4`, `pseudonymize` (HMAC-SHA256 with workspace-scoped salt → stable pseudonym; salt never leaves secrets store), `hash` (one-way, no correlation), `keep_verbatim`.
Stored alongside: `redactions` jsonb (paths touched + action + schema version) so every view can show "sanitized per default-v3".

**Coverage risk (accepted, monitored):** a field the schema misses persists sanitized-but-raw. Mitigations: schema editor UI with "test against sample event", regex lint warnings, and an optional deny-list rule (`action: drop_event` when matched). Open question 9 in the brief tracks the deny-list decision.

---

## 8. CLI specification (WS-3)

Go, single static binary (linux/darwin/windows, amd64+arm64). Cobra. All commands support `--json` (machine-readable stdout, stable schema) and respect `HOOKREPLAY_API_KEY` / `~/.hookreplay/config`.

| Command | Behavior | Exit codes |
|---|---|---|
| `login` | Browser OAuth; stores API key | 0 ok, 1 denied |
| `init` | Scaffolds `hookreplay.yml`, `.hookreplayignore`, installs git pre-push hook (optional) | |
| `fmt [file]` | Normalize YAML to block style; `--check` for CI | 1 on diff |
| `validate [file]` | Schema lint incl. placeholder references | 1 with errors |
| `tunnel --port N [--subdomain X]` | Opens WebSocket tunnel, prints URL, exports `TUNNEL_URL` to child env | reconnect w/ backoff |
| `run [--file F] [--chain NAME] [--env E] [--target URL] [--fail-on expect] [--json]` | Interpolate `${}` from env; resolve secrets (shell → workspace store); execute sequentially; stream step results; `--fail-on expect` → non-zero on any unmet expect | 0 pass, 1 expect/step failure, 2 config/auth error |
| `chains push/pull/list` | Workspace sync; `push` is versioned server-side (git-sha tagged) | |
| `templates list/show P/E` | Library from terminal | |
| `secrets set NAME --env E [--value V]` | Write-only; prompts if `--value` omitted | |
| `keys generate` | Ed25519 keypair for custom schemes (Phase 2) | |
| `replay ls/run` | Phase 3 | |

CLI never prints secret values; `run` resolves `env:` refs in-memory.

---

## 9. Workstreams

### WS-1 — Signing engine (internal/signing)
- Implement all §7.1 MVP schemes (Stripe, GitHub, Slack, Meta, Paystack, Razorpay, Flutterwave, Resend, Twilio, Klarna/Adyen, custom) behind one interface: `Sign(scheme Scheme, body []byte, key Material, ts time.Time) (http.Header, error)`.
- Pure functions, table-driven tests from §7.4 vectors.
- **DoD:** every scheme passes its CI vector; negative tests (wrong key → verify fails) for each; benchmark: ≥50k signs/sec/core.

### WS-2 — Dispatcher + API (cmd/api, cmd/worker, internal/dispatch)
- REST API per §5; worker consumes dispatch jobs from Redis Streams; enforces rate limits, circuit breakers, timeouts, idempotency, retry schedules (§5.4, §7.3).
- **DoD:** k6 load test — 10k executions/min sustained, p99 dispatch enqueue <50ms; breaker and limiter behavior e2e-tested; raw-body byte-exactness asserted in storage tests (hash before send == hash stored).

### WS-3 — CLI (cmd/cli)
- §8 surface. `run` executes chains locally *through the API* (dispatch calls) so dashboard and CLI share one execution path.
- **DoD:** golden-file tests for `fmt`/`validate`; scripted e2e against a local stack (WS-2 + WS-7) covering the three landing-page scenarios: single fire, forged-webhook rejection, retry rehearsal with `expect.retries`.

### WS-4 — Chain engine (internal/chains)
- Parser/validator per §6 JSON-schema; runtime context resolution (`{{}}`); `when` evaluator (== != contains exists); expect evaluator (status/latency/retries/delays/body json_path).
- **DoD:** schema fuzzing (no panics on 10k mutated inputs); extraction-failure errors name the exact path; full example suite from `templates/examples/` passes.

### WS-5 — Template library (templates/)
- 14 launch + 8 fast-follow templates per brief §3.2: payload fixtures + `delivery.yml` + `sign.yml` + docs note (Flutterwave caveat, Meta handshake).
- **DoD:** §7.4 sandbox gate green for Paystack, Flutterwave, Razorpay, Stripe, Adyen, Klarna; every template has `verified_at` stamp and override examples.

### WS-6 — Dashboard (web/)
- Next.js + Tailwind. MVP pages: auth, workspace, template picker + fire (single executions), execution log (raw body view, headers, signature header, latency, attempts), chain editor (visual over YAML, reads/writes same schema), domains verification UI, usage/limits page, secrets names page.
- **DoD:** time-to-first-fire from signup < 2 min in scripted Playwright flow; dark-mode-first per brand; execution detail view shows byte-length and SHA-256 of request body (trust signal).

### WS-7 — Tunnel (internal/tunnel)
- WebSocket server (Go, gorilla/websocket or coder/websocket): subdomain allocation, connection registry, request relay with proper hop-by-hop header handling, 30s per-request relay timeout, auto-reconnect client.
- **DoD:** 1k concurrent tunnels per node; relay adds <10ms p99 latency over direct localhost; header fidelity test (content-length, transfer-encoding, authorization preserved).

### WS-8 — Auth, workspaces, billing/limits
- OAuth (GitHub first), API keys, workspace membership, tier enforcement per **§5.8** (Free: 1,000 executions/mo, 3 chains, tunnel-only targets; Pro: verified domains + replay packs; Team: GitHub Action CI runs; Pro/Team: fair-use + rate limits per §5.4), metering rollups.
- **DoD:** limit transitions tested at boundaries (999/1000/1001); workspace isolation tests (no cross-workspace reads by ID enumeration); fair-use threshold alerting job; §5.8 enforcement e2e-tested from CLI *and* raw HTTP (forged tier headers rejected).

### WS-9 — Recorder & replay (Phase 3, interface now)
- Implement §5.7 + §7.5–§7.7 contracts only: capture-route provisioning, the §7.6 streaming-proxy ingress (forward-first, async sanitize-and-persist), sanitization schemas, re-signing replay over sanitized bodies.
- **DoD:**
  - Forward-open time measured: downstream connection established before request body fully received; added latency ≤100ms p99 in load test.
  - Storage failure injection (kill DB mid-stream): delivery unaffected, `capture_failed` metric increments, runbook entry exists.
  - **No-raw audit:** static analysis + runtime assertion that no code path writes a raw body or original signature header to any durable store (table has no column for it — §4); pen-test checklist item.
  - Replayed recorded event against staging with real verification middleware passes; pseudonyms stable across replays of the same customer event; `keep_verbatim` exceptions honored.
  - Sanitizer unit tests: built-in schema catches email/phone/card/token shapes in 50-fixture corpus; redactions manifest complete and displayed in dashboard.

### WS-10 — QA & test infrastructure
- Unit (per WS), integration (docker-compose: api+worker+postgres+redis+tunnel), e2e (Playwright: dashboard; scripted CLI), load (k6), chaos (kill worker mid-chain → run resumes or fails cleanly with run status `interrupted`), security (secret-scanning in CI, dependency vuln gate).
- **DoD:** CI green required for merge; sandbox gate (§7.4) blocks payment template changes; coverage floor 80% on internal/signing and internal/chains.

---

## 10. Milestones

| Milestone | Contents | Gate to proceed |
|---|---|---|
| **M1 (wk 2)** | WS-1 schemes + CI vectors for Stripe/Paystack/Razorpay from sandboxes; chain schema frozen (`pkg/chainschema`) | Vectors green in CI |
| **M2 (wk 4)** | WS-2 dispatcher (rate limit, breaker, idempotency, raw-byte storage) + WS-7 tunnel | k6 + relay latency DoDs |
| **M3 (wk 6)** | WS-3 CLI (`run`, `tunnel`, `validate`), WS-5 launch templates, WS-8 free/pro enforcement | Landing-page demo scenarios pass e2e; sandbox gate green for all payment templates |
| **M4 (wk 9)** | WS-4 full chain engine (`when`, extraction, expect incl. retries/delays), `chains push/pull`, GitHub Action, WS-6 dashboard execution log + chain editor | Retry-rehearsal e2e passes against real staging URL with `--fail-on expect` |
| **M5 (wk 12)** | Team workspaces, private templates, usage analytics, fast-follow templates, hardening + launch | Load + chaos DoDs; §12 kill-criteria instrumentation live |

Phase 3 (WS-9 full) starts only after M5 + 60-day instrumentation review per brief §6.

---

## 11. Non-functional requirements

- **Security:** AES-256-GCM secrets; write-only secret API; raw-body integrity (SHA-256 recorded per execution); dependency vuln gate; no PII in logs; security review checklist before public launch (brief §9 Phase 3 security posture pulled forward for the recorder only — simulation path has no PII by design except user-provided payload overrides, which are workspace-scoped).
- **Reliability:** dispatcher stateless (Redis Streams); worker crash → at-least-once execution with idempotency keys; tunnel reconnect with jittered backoff.
- **Observability:** structured JSON logs with request_id; Prometheus metrics: dispatch_duration, executions_total{result}, breaker_open, tunnel_connections, signer_failures; per-workspace usage events.
- **Performance:** §9 DoDs; dashboard TTI < 2s on 4G.

---

## 12. Explicit non-goals (do not build)

- General REST API testing clients, load testing, GraphQL tooling (brief §7.4).
- Arbitrary scripting/loops in chain YAML (SDK/CI glue is the escape hatch).
- Webhook delivery infrastructure (gateways, queues for the user) — we simulate and replay; we are not Hookdeck.
- `secrets get`, secret value echo anywhere.
- Multi-step conditional branching trees — `when` skip only.
- **Durable raw-payload storage, anywhere, for any reason.** Raw capture material is in-memory-only (§7.6); if a requirement seems to need raw at rest, that is a design-error signal — escalate, don't implement.
- Store-and-forward capture (receive-all → then forward) — the streaming proxy exists precisely to forbid this.

---

## Errata

- **§6 example (fixed 2026-09-26):** the example previously had `when: "{{steps.order-must-exist.status}} == 200"` on `payment-received`, which references a step defined later — a validation error per the spec's own placeholder rules. The `when` line was removed. (`welcome-email` in brief §3.4 shows the same pattern with correct ordering.)

---

*This spec is a contract. Deviations require a decision-log entry in the brief.*
