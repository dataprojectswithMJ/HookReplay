# HookReplay — Project Brief (v2.1)

**Status:** Concept validated → MVP scoping
**Last updated:** 2026-09-26
**Owner:** [You]
**Category:** Developer Tools / SaaS

> **v2.0 changelog (2026-09-26):** Competitive landscape rebuilt around Hookdeck Console, ngrok, and CatchHook. Pricing restructured (meter replays, not executions; Team repriced to $59/mo). Custom webhooks added as a core feature. Security model added (endpoint verification, rate limits, fair use). Free tier made generous (1,000 executions). Success metrics recalibrated. Kill criteria added.
>
> **v2.1 changelog (2026-09-26):** Template strategy formalized — staged by crypto complexity, never by geography; Africa/Asia providers in launch tier; sandbox verification as the quality gate. Chains-as-Code added: YAML schema (block style), `deliver`/`expect` model, CLI surface. Capture & replay lifecycle documented (raw-byte storage, per-scheme re-signing). SDK deferred. Decisions log and open questions synced.
>
> **v2.2 changelog (2026-09-26):** Recorder architecture corrected per two directives: (1) production path is a **streaming proxy** — forwarding opens immediately, storage is an async side effect after the provider's connection closes, so capture adds ~one network hop and never sits on the provider's timeout clock; (2) **sanitize at capture, never persist raw** — raw bytes exist only in memory in flight; what is stored is the sanitized body + redactions manifest; original signature headers are not stored (regenerated at replay); replay replays the sanitized event, with hash-pseudonyms and per-path keep-verbatim exceptions preserving debugging fidelity. Free tier clarified: CLI included (adoption engine); the gate is target type (tunnel free, verified staging domains Pro+). GitHub Action enforcement specified: server-side tier check, Action is a thin CLI wrapper, error message is the upsell.

---

## 1. Elevator Pitch

HookReplay is a webhook testing and simulation platform for developers. It lets you simulate real webhooks from Stripe, GitHub, Resend, Meta, Slack, Paystack, Flutterwave, Razorpay, and others locally — with cryptographically valid signatures, request chaining, failure simulation, and production traffic replay. Works with any provider template **and any custom or in-house webhook**.

**The core promise:** Test your webhook handlers in under 60 seconds without touching production. Single webhook? Use the provider's CLI. Anything involving sequences, failures, cross-provider flows, or providers without a CLI — use HookReplay.

---

## 2. Problem Statement

Developers building webhook integrations currently:
- Hit real APIs in test mode (expensive, side effects)
- Build brittle mocks manually — especially for **in-house or niche-provider webhooks with no template and no CLI**
- Use ngrok + curl to inspect payloads but can't replay complex sequences
- Have no way to test failure scenarios (forged signatures, malformed payloads, retry storms)
- Can't simulate multi-step async workflows (payment → email → Slack)
- **Can't easily test providers that don't serve their market** — e.g., Stripe is unavailable to African developers; African/Asian payment rails are unsupported by every existing webhook tool

Postman owns REST API testing. Webhook testing is contested but unsolved: incumbents own pieces (inspection, tunnels, delivery reliability) — no one owns **chained, failure-injected, signature-valid simulation and replay**.

---

## 3. Solution

### 3.1 Core Product
- **Template Library:** Pre-built webhook payloads for 22+ providers, staged by signing complexity (§3.2). Built **long-tail first**: prioritize providers with no official CLI and underserved markets.
- **Custom Webhooks:** Raw fire mode — paste any payload, set any headers, choose signing mode (none / shared-secret HMAC with configurable algorithm, header, and payload convention via `{{timestamp}}` + `{{hmac}}` placeholders / manual headers). One custom-signing config object reused across raw fire, private templates, and replay re-signing.
- **Private Templates (Pro):** Save custom payloads + signing configs as reusable personal templates; workspace-published templates in Team; community templates Phase 3 (§3.2).
- **Signature Generation:** Cryptographically valid signatures per provider, computed **at dispatch time** (fresh timestamps — Stripe rejects events older than ~180s; per-provider tolerance windows documented). Secrets encrypted at rest (AES-256-GCM), decrypted only at dispatch.
- **CLI Tunnel:** One-command tunnel (`hookreplay tunnel --port 3000`) — public URL routed to localhost. A convenience feature, **always free**, never marketed as a differentiator. (Tunnel-only firing on Free is also an abuse control — see §7.)
- **Request Chaining (Chains as Code, §3.4):** Link webhooks and API calls into YAML-defined sequences with extraction, conditionals, delays, and assertions. Chain engine is provider-agnostic — it synthesizes and signs every step itself; provider CLIs are never invoked inside chains.
- **Failure Simulation (`deliver`/`expect` model, §3.4):** Simulate forged signatures, malformed payloads, and rehearse retry/idempotency behavior against real provider backoff schedules.
- **Production Replay (§3.5):** Record real webhook traffic (sanitized), replay to staging with freshly generated signatures.

### 3.2 Template Strategy

**Principle: geography decides *whether* a provider ships; crypto complexity decides *when*; sandbox verification is the gate — never calendar time.**

**Launch batch (14) — weeks 1–6:**

| # | Provider | Event(s) | Signing | Note |
|---|----------|----------|---------|------|
| 1 | Stripe | `checkout.session.completed` | HMAC-SHA256 + timestamp window | Most-integrated event among SaaS builders |
| 2 | Stripe | `invoice.payment_succeeded` | HMAC-SHA256 + timestamp window | Recurring-revenue flow |
| 3 | GitHub | `push` | HMAC-SHA256 | Highest-volume dev webhook |
| 4 | GitHub | `pull_request` | HMAC-SHA256 | CI/CD use case |
| 5 | Slack | `block_actions` | HMAC-SHA256 | Modern interactive pattern |
| 6 | Slack | `event_callback` (app_mention) | HMAC-SHA256 | Covers Events API |
| 7 | **Paystack** | `charge.success` | HMAC-SHA512, hex | Africa anchor #1; encoding test vector required |
| 8 | **Razorpay** | `payment.captured` | HMAC-SHA256, hex | Asia anchor; commodity scheme |
| 9 | **Flutterwave** | `charge.completed` | Static hash of secret | Signature doesn't bind payload — template must document that IP allowlisting is the real production control |
| 10 | Resend | `email.delivered` | Ed25519 | Absorbs asymmetric-crypto path early |
| 11 | Resend | `email.bounced` | Ed25519 | Pairs with failure sim |
| 12 | Meta | `leadgen` | HMAC-SHA256 | Ships **with `hub.challenge` handshake documented in template** |
| 13 | Twilio | `message.status` | HMAC-SHA1 | Legacy scheme, trivial |
| 14 | Clerk | `user.created` | HMAC-SHA256 (Svix) | Doubles as proof we handle the Svix ecosystem |

**Fast-follow (8) — weeks 7–12:** Adyen (`AUTHORISATION`, Europe anchor), Klarna (`order.created`), Paddle (`subscription.activated`), Shopify (`orders/create`), Linear (`Issue`), Segment (`track`), Supabase (`database.changes`), Slack slash-commands (form-encoded template example).

**Batch 3 — demand-driven, not calendar-driven:** Paytm (AES-256 checksum — genuinely different subsystem), Discord interactions (Ed25519), plus whatever usage data shows. Instrument template-picker searches and custom-webhook saves per provider — **saved custom payloads are the demand signal.**

**Deliberate cuts:** Zapier `hook` (user-defined payload — nothing canonical to template; custom webhook option is strictly better).

**Quality gate (non-negotiable for payment templates):** before public launch, point Paystack/Flutterwave/Razorpay **sandbox** webhooks at our own test endpoint, verify our generated signatures pass, and freeze as CI test vectors ("every payment template verified against the provider's sandbox" — a credibility line no competitor has written). Every template carries a "verified against provider docs on [date]" stamp; schema-versioned; drift-alert process until Phase 3 capture provides canonical refresh.

### 3.3 Phase 2 (Team Tier)
- **Production Replay** — see §3.5 for the full lifecycle
- **Custom templates from capture:** save a captured event as a private template — for in-house systems, captured real traffic is a better fixture than anything hand-written
- **Sanitization schemas** for PII redaction (one built-in schema available at all tiers — it's a trust feature)
- **Usage analytics** dashboard

### 3.4 Chains as Code

**Design decision:** a chain is data, not UI state. YAML is the canonical format; the dashboard builder reads/writes the same schema; the CLI runs it. YAML lives in the user's repo (versioned, code-reviewed); the UI is the runtime debugger; the CLI/CI is a first-class runner.

**Block style everywhere** (better diffs, commentable, consistent with Compose/Actions/K8s). `hookreplay init` ships a formatter/linter that normalizes to block style.

**Execution semantics:**
- Steps run in order; each fires a webhook (signed at dispatch time) or makes an API call against the user's app
- Steps extract values from previous steps' HTTP responses via `{{steps.<name>.body...}}` — that's what makes it a chain
- Conditionals (`when`), delays, and concurrency caps
- Two placeholder syntaxes, cleanly separated: `${VAR}` = env interpolation by the CLI before the run (e.g. `${TUNNEL_URL}`); `{{...}}` = runtime values from the chain context while steps execute

**The `deliver`/`expect` model** (revised 2026-09-26 — supersedes the earlier `fail`/`idempotency` design):

```yaml
# hookreplay.yml
name: payment-onboarding-flow
target: ${TUNNEL_URL}            # env interpolation; or --target flag, or literal URL

steps:
  - name: payment-received
    webhook: stripe/checkout.session.completed
    secret: env:STRIPE_WEBHOOK_SECRET
    override:
      data.object.customer_email: test@example.com

  - name: order-must-exist
    api:
      method: GET
      url: "{{target}}/api/orders/{{steps.payment-received.body.data.object.id}}"
    expect:
      status: 200
      body:
        json_path: "$.status"
        equals: paid

  - name: welcome-email
    webhook: resend/email.delivered
    secret: env:RESEND_WEBHOOK_SECRET
    delay: 3s
    when: "{{steps.order-must-exist.status}} == 200"
```

- **`deliver`** — how the webhook is sent. `result: success` (default) or `result: fail` with a `reason` enum: `invalid_signature | missing_signature | stale_signature | malformed_payload | connection_refused`. **Boundary rule: `reason` may only contain failures HookReplay controls** (what we send). Endpoint-side failures are rehearsed with a debug flag in the user's app (e.g. `?force_fail=1`); HookReplay plays the provider's half of the conversation.
- **`expect`** — assertions on what the endpoint did: `status`, `body` (json_path), `retries` (re-delivery count), `delays` (observed backoff), `latency_under`. Retry is an *observation*, not a delivery mode.

```yaml
  # Reject a forged webhook
  - name: forged-webhook
    webhook: stripe/checkout.session.completed
    deliver:
      result: fail
      reason: invalid_signature
    expect:
      status: 400
      retries: 0

  # Idempotency rehearsal — app returns 500 once, then 200
  - name: payment-retried
    webhook: paystack/charge.success
    secret: env:PAYSTACK_SECRET_KEY
    expect:
      status: 200
      retries: 3
      delays: ["5m", "30m", "2h"]   # must match Paystack's real schedule
```

**Custom webhooks in chains** use the same signing config:

```yaml
  - name: internal-invoice-event
    custom:
      payload: ./fixtures/invoice.paid.json
      headers:
        X-Event-Name: invoice.paid
      sign:
        algorithm: hmac-sha256
        secret: env:INTERNAL_WEBHOOK_SECRET
        header: X-Signature
        format: "{{timestamp}}.{{hmac}}"
```

**Secrets rule:** YAML contains secret *names* only (`env:FOO`). Resolution precedence: shell environment → workspace encrypted store (set via `hookreplay secrets set NAME --env staging`; write-only, no `get`). The same YAML works for local dev and CI with no edits.

**CLI surface:**

```bash
hookreplay login                        # browser OAuth → API key in ~/.hookreplay
hookreplay init                         # scaffold hookreplay.yml + formatter
hookreplay tunnel --port 3000           # exposes $TUNNEL_URL to child processes
hookreplay run                          # run hookreplay.yml; injects tunnel URL
hookreplay run --env staging --fail-on expect --json
                                        # strict mode for CI; --json = machine-readable
hookreplay chains push | pull | list    # sync repo ↔ workspace (repo is source of truth)
hookreplay templates list | show <name> # browse/dump library from terminal
hookreplay secrets set NAME --env staging   # prompts; never stored in shell history
hookreplay replay ls --source paystack  # Phase 3
hookreplay replay run <event-id> --env staging
```

`--json` on every command = the "poor man's SDK" (shell out from any test script). No `secrets get` — write-only, by design.

**Deliberately out of scope for chain YAML:** scripting/loops/arbitrary expressions. If chains need real logic, that's test code with the signing-helper library (Phase 2+ micro-package; full SDK deferred — demand-driven, build when users wrap the CLI in scripts).

### 3.5 Capture & Replay Lifecycle

**Dev loop (Phases 1–2):** HookReplay is a **stand-in for the producer**. Dispatcher renders template, signs raw payload bytes, delivers via tunnel or verified URL. Handler receives byte-for-byte realistic traffic; verification code can't tell the difference.

**Production (Phase 3):** the merchant points their provider dashboard at HookReplay's recorder endpoint — HookReplay becomes an **inline streaming proxy**:

```
Paystack ──► hooks.hookreplay.dev/acme/capture/paystack
                 │
                 ├─► forward to myapp.com opened IMMEDIATELY (streaming, chunk-by-chunk)
                 │      body tee'd in-memory into sanitizer buffer (parallel, off hot path)
                 ├─► myapp's response streamed back to Paystack
                 └─► AFTER provider connection closes (async, off the timeout clock):
                        sanitize buffer → encrypt → persist sanitized copy + redactions manifest
```

**Two hard contracts (v2.2):**
1. **Latency:** delivery is a streaming proxy — the downstream connection opens before the request body has fully arrived, so capture adds roughly one network hop (~10–50ms), never a store-then-forward delay. The database write happens *after* the provider's connection closes: storage problems degrade capture completeness, never delivery.
2. **No raw persistence:** raw bytes exist only in memory, in flight, and are zeroized when the request ends. What is stored is the **sanitized body + a redactions manifest** (which paths were touched, schema name/version). Original signature headers are **not stored** — some are secret-derived (Flutterwave's `verif-hash` is a hash of the secret), and replay regenerates signatures anyway. Endpoint response bodies: same treatment, short retention. The only raw that may linger is an encrypted retry buffer when *we* must re-deliver after a forward failure — TTL in minutes, never a durable raw store.

**Replay rule: signing is always live over the stored (sanitized) body.** Fresh signature computed for the target environment at dispatch time:

- **Commodity HMAC (Paystack, Razorpay, GitHub):** recompute over stored sanitized body with target env's secret
- **Timestamped schemes (Stripe, Slack):** fresh timestamp at dispatch (the ~180s tolerance window makes any stored timestamp useless anyway)
- **Flutterwave:** static hash regenerated from the target env's secret; template docs must note the scheme provides zero tamper evidence
- **Paytm:** regenerate AES-256 checksum with target env's merchant key if configured; otherwise passthrough mode with a clear warning

**Sanitization defaults (built-in schema, all tiers):** emails → hash-pseudonym (`usr_<sha256[:12]>` — stable, so a customer's events stay correlated across replays); phone/card-shaped numbers → masked or redacted; auth tokens in headers → redacted. Custom schemas (Team) add per-path rules, including **keep-verbatim exceptions** for fields where exact bytes matter (rare; e.g. `$.data.currency`).

**The one honest trade-off:** replay is byte-identical to the *sanitized* event, not to production. Accepted deliberately — you don't want real customer emails firing your staging email system — and documented, with hash-pseudonyms covering correlation. Edge case that genuinely breaks: a bug caused by exact byte content (encoding, unusual unicode) may be masked by sanitization; `keep-verbatim` exceptions are the escape hatch.

**Why this matters commercially:** transient processing with pseudonymized data at rest is a far easier SOC 2 / GDPR conversation than raw PII storage — and "we never keep your raw payloads" is a stronger trust line than any encryption badge. It also feeds the Enterprise wedge: the on-prem/VPC recorder agent runs inside the customer's network where *their* policy, not ours, governs.

**Commercial note:** Phase 3 touches production traffic → compliance teams ask "where do payloads live?" → the on-prem/VPC recorder agent is the Enterprise wedge (tap runs inside their network; HookReplay sees only metadata + sanitized copies).

---

## 4. Target Market

| Segment | Description | Pain Level | Budget |
|---------|-------------|------------|--------|
| **Primary:** Indie devs / small teams, incl. Africa/Asia SaaS builders | Building SaaS with webhook integrations; underserved by US/EU-centric tools | High | $19/mo |
| **Secondary:** Startups & SaaS builders (10-100 eng) | Real users, production debugging, CI against staging | High | $59/mo flat |
| **Tertiary:** Mid-market tech (50-500 eng) | Multiple providers, CI/CD, in-house integrations | High | $59/mo flat → Enterprise |
| **Quaternary:** Enterprise | Compliance, unlimited replay, SSO, VPC recording | Medium | Custom (~$499/mo anchor) |

**Total Addressable Market:** API testing ~$1B+. Webhook-specific niche contested in 2026 (Hookdeck, ngrok, CatchHook all touch it) but the chained-simulation + signature-valid replay combination is unclaimed. Africa/Asia templates are a moat-widener and community unlock — note: expect word-of-mouth and conversion value there, not SEO traffic (thin search volume).

---

## 5. Competitive Landscape

| Competitor | What they do | Where they stand vs. us |
|------------|-------------|------------------------|
| **Hookdeck Console** | Free inspector, ~120 provider samples, CLI replay-to-localhost; Growth $75/mo bundles replay, retries, DLQ | Closest competitor. Owns "record/replay received traffic." Weak on: chaining, failure injection, cross-provider sequences, in-house/custom webhooks |
| **ngrok** | Tunnels; editable traffic replay; Traffic Policy verify-webhook (signature + timestamp validation, encrypted secrets); $8–20/mo | Owns the local dev loop. Replay does **not** recreate signing info — signature-verification code breaks on replay. No chaining, no templates, no failure sim |
| **CatchHook** | Provider-aware capture, signature re-generation on replay, replay cases with assertions; $10–49/mo | Overlaps our signature-valid replay pillar directly. Small; watch closely. Differentiator: chaining + failure sim + custom webhooks |
| **Provider CLIs** (`stripe trigger`, `gh webhook forward`) | Free, authentic signed events for the biggest providers | Beat us for single-webhook testing on covered providers. We win on: chaining, failure sim, no-CLI providers, cross-provider. Position around them, don't fight them |
| **webhook.site / HookCap / Requex / Webhook Bin / Beeceptor** | Inspection, response simulation, replay variants | Long tail of point solutions; most cover one pillar, none the combination |
| **Postman** | General REST API testing | Not async/webhook-native; not the fight we pick |

**Moat:** NOT "templates + signatures + tunnel" (all replicable or free elsewhere). The defensible combination:
1. **Signature-valid replay of arbitrary/recorded events** (ngrok explicitly doesn't; CatchHook is the only one that does)
2. **Chained multi-step sequences as code** with failure injection and assertions (weakly covered everywhere)
3. **Custom/in-house webhook support** with reusable private templates

---

## 6. Business Model

**Meter replays and storage, not executions. Simulation is the PLG engine — never make a dev hesitate before hitting "Fire."**

| Tier | Price | Who | Features |
|------|-------|-----|----------|
| **Free** | $0 | Solo devs exploring | 1,000 simulation executions/mo (~30/day), 3 concurrent chains, 5 provider templates, **CLI included** (tunnel + chains at localhost — the CLI is the adoption engine, never a paywall), tunnel targets only (verified staging domains gate below), built-in PII redaction schema, 7-day logs, 1 user |
| **Pro** | $19/seat/mo | Individual devs & small teams | **Unlimited executions** (fair use, §7), unlimited chains (concurrency-capped), all provider templates, custom webhooks + private templates, custom signing schemes, **`--env staging`: fire at verified domains**, failure sim, 30-day logs. Replay add-on: $10 per 500 replays, metered |
| **Team** | **$59/mo flat** (first 5 seats, ~$10/seat after) | Startups & SaaS builders | Everything in Pro + **2,000 replays/mo included**, sanitization schemas, shared workspaces + workspace templates, **GitHub Action** (server-side tier enforcement; the Action is a thin CLI wrapper, and its 403 error message is the upsell — 14-day trial triggered from it), usage analytics, 90-day logs |
| **Enterprise** | Custom (anchor ~$499/mo) | Big orgs with compliance needs | Unlimited replay, SSO/SAML, audit logs, SLA, SOC 2 (roadmap), dedicated support, **on-prem/VPC recorder agent** |

**Go-to-market:** Product-led growth. Free tier drives adoption; Pro converts on chains + custom webhooks; Team converts on replay + shared workspaces. Annual billing: 2 months free at paid tiers; grandfather launch users permanently.

**Validation plan:** ship limits as above; instrument for 60 days — median executions/mo per free user (sets free ceiling), replays/mo per Team trial (validates 2,000), free→Pro conversion (validates chain gate). Adjust **limits, not sticker prices**.

---

## 7. Technical Architecture

### 7.1 High-Level Flow
```
DEV:      Dashboard/CLI → HookReplay API → Dispatcher → [Tunnel/Verified URL] → User Endpoint
                                    (stand-in for the provider)

PROD (P3): Provider → Recorder (streaming proxy; forward opens immediately,
           body tee'd to in-memory sanitizer) → User App
           └── AFTER provider connection closes (async): sanitized copy +
               redactions manifest persisted. Raw bytes never touch disk.
```

### 7.2 Key Components
- **API:** **Go** (decided 2026-09-26 — dispatcher is I/O-bound; Rust's perf is irrelevant)
- **Dashboard:** Next.js + Tailwind
- **Database:** PostgreSQL + Redis
- **Queue:** Redis Streams or NATS
- **CLI:** Go (single binary, cross-platform) — also the chain runner and tunnel client
- **Tunnel:** WebSocket-based, auto-generated subdomains

### 7.3 Signature Algorithms by Provider
| Provider | Algorithm | Header | Complexity class |
|----------|-----------|--------|------------------|
| Stripe | HMAC-SHA256 + timestamp window | `Stripe-Signature` | Commodity+ (freshness) |
| GitHub | HMAC-SHA256 | `X-Hub-Signature-256` | Commodity |
| Slack | HMAC-SHA256 + timestamp | `X-Slack-Signature` | Commodity |
| Meta | HMAC-SHA256 | `X-Hub-Signature-256` | Commodity + setup docs |
| Paystack | HMAC-SHA512, hex | `x-paystack-signature` | Commodity + encoding vector |
| Razorpay | HMAC-SHA256, hex | `x-razorpay-signature` | Commodity |
| Flutterwave | Static hash of secret | `verif-hash` | Special case — doesn't bind payload |
| Klarna / Adyen | HMAC-SHA256, base64 | per docs | Commodity + encoding |
| Resend | Ed25519 | `Resend-Signature` | Asymmetric |
| Twilio | HMAC-SHA1 | `X-Twilio-Signature` | Legacy, trivial |
| **Custom** | Configurable (HMAC-SHA1/256, none, manual) | User-defined, `{{timestamp}}`/`{{hmac}}` conventions | Config |

**Signing happens at dispatch time, per step, inside chains** — critical for delayed steps longer than provider timestamp tolerance.

**Security:**
- Secrets encrypted at rest (AES-256-GCM), decrypted only at dispatch; write-only from CLI (no `secrets get`)
- **Endpoint verification (MVP):** tunnel targets fire freely (self-limiting); public URLs (staging/CI) require one-time domain verification (DNS TXT or served token, ACME-style). Closes the attack-relay/SSRF hole.
- **Rate limits:** ~10 executions/s per workspace, burst 30, ~5 concurrent running chains
- **Per-target circuit breaker:** automatic backoff on repeated connection-refused/5xx
- **Fair use** on unlimited Pro/Team executions: documented threshold (~10× plan median), email before enforcement

### 7.4 Scope Boundary
**In scope:** Webhook simulation (templated + custom), chains-as-code, failure simulation, capture/replay. **Out of scope:** General REST API testing, load testing, GraphQL playgrounds, webhook *delivery/infrastructure* (Hookdeck's domain — we simulate and replay; we are not a delivery gateway).

---

## 8. User Journey

1. Sign up → detect stack → suggest template (or "Custom webhook")
2. Paste webhook secret from provider dashboard (or configure custom signing)
3. Run `hookreplay tunnel --port 3000`
4. `hookreplay run` — tunnel URL injected automatically
5. Local server receives indistinguishable-from-production webhook
6. See response, latency, retries, and logs in dashboard or `--json` output

**Target time to first successful test: < 2 minutes.**

---

## 9. Product Roadmap

### Phase 1: MVP (Weeks 1–6)
- [ ] 14 launch templates (§3.2) — **after sandbox soak: verify generated signatures against Paystack/Flutterwave/Razorpay sandbox webhooks; freeze CI test vectors**
- [ ] Signature generation, dispatch-time signing
- [ ] Custom webhooks: raw fire + custom signing schemes
- [ ] Endpoint verification (DNS TXT / served token) + rate limits
- [ ] CLI tunnel + `hookreplay run` (single-webhook YAML)
- [ ] Free + Pro tiers

### Phase 2: Chaining + Growth (Weeks 7–12)
- [ ] Chains-as-code engine: extraction, conditionals, `deliver`/`expect`, concurrency caps
- [ ] Fast-follow templates (8)
- [ ] Private templates (save custom payloads + signing configs)
- [ ] GitHub Action (`hookreplay run --env staging --fail-on expect`)
- [ ] Signing-helper micro-package (language TBD — standalone value: offline signature unit tests)
- [ ] Team workspaces
- [ ] Public launch (Hacker News, Dev.to + Paystack/Flutterwave/Razorpay dev communities)

### Phase 3: Team Tier + Production Replay (Months 4–6)
- [ ] Recorder agent (sidecar / Lambda / Worker — architecture decision still open)
- [ ] Sanitization schemas (built-in free; custom Team)
- [ ] Custom templates from capture
- [ ] Replay dashboard; `hookreplay replay` CLI
- [ ] Team tier launch at $59/mo flat
- [ ] Usage analytics
- [ ] **Security posture work (pulled forward):** encryption audit, retention policies, data-handling docs, SOC 2 Type I scoping

### Phase 4: Enterprise (Months 7–12)
- [ ] Unlimited replay, SSO/SAML, audit logs, on-prem/VPC recorder agent, SOC 2 Type II, dedicated support
- [ ] *Contingent on Team ARR trajectory (§12 kill criteria)*

---

## 10. Key Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-08-01 | Selected webhook testing as #1 idea; named product; positioned as "event-driven workflow testing" | PLG fit, avoids Postman head-on |
| 2026-08-10 | API calls within chains only; tunnel auto-generates URLs; replay = Team tier | Focus; zero-config; startups need replay without enterprise tags |
| 2026-08-10 | Team tier = $129/mo flat | *Superseded 2026-09-26* |
| 2026-09-26 | Competitive landscape rebuilt: Hookdeck Console, ngrok, CatchHook primary; moat = signature-valid replay + chains-as-code + custom webhooks | Prior analysis missed 2024–26 launches |
| 2026-09-26 | Team repriced **$59/mo flat**; Enterprise ~$499 anchor | $129 priced above Hookdeck Growth with less capability |
| 2026-09-26 | Free tier **1,000 executions**; Pro/Team unlimited + fair use + rate limits; meter replays not executions | Simulation is PLG engine |
| 2026-09-26 | Custom webhooks are core; one signing-config object across fire/templates/replay | In-house endpoints were an implied gap |
| 2026-09-26 | **Go** for API/CLI | I/O-bound; Rust marketing-only |
| 2026-09-26 | Chains never invoke provider CLIs; chain engine signs every step at dispatch | Mixed chains architecturally trivial |
| 2026-09-26 | Endpoint verification + rate limits are MVP scope | Closes attack-relay hole |
| 2026-09-26 | **Template staging principle: geography decides whether, crypto complexity decides when, sandbox verification is the gate** | Africa/Asia providers must be launch-tier or the differentiation doesn't exist at launch; Paytm's AES scheme is the only genuine "later" |
| 2026-09-26 | Flutterwave template must document that its signature doesn't bind payload; IP allowlisting is the real control | Honest security model or we teach a wrong one |
| 2026-09-26 | **Chains as Code: YAML canonical, UI editor + debugger, CLI runner; block style** | Versioned, reviewable, CI-native; devs live in repos |
| 2026-09-26 | **`deliver`/`expect` model replaces `fail`/`idempotency`** | Two things worth asserting: what we sent, what endpoint did; retry is an observation, not a delivery mode. `reason` enum limited to failures HookReplay controls |
| 2026-09-26 | Placeholder separation: `${}` env interpolation (CLI, pre-run) vs `{{}}` runtime context (engine, per-step) | No collision; teachable rule |
| 2026-09-26 | Secrets: names only in YAML; shell env → workspace store precedence; write-only CLI | Repo-safe chains; no leakage class |
| 2026-09-26 | SDK deferred; `--json` CLI as poor-man's SDK; signing-helper micro-package Phase 2 | Demand-driven; CLI flag covers 90% |
| 2026-09-26 | Replay: store raw bytes verbatim, sign live at dispatch per scheme | Parsed-JSON storage breaks HMAC replay for every scheme |
| 2026-09-26 | Community templates: Phase 3 | Quality/moderation burden before PMF |
| 2026-09-26 | Recorder = **streaming proxy**: downstream forward opens immediately, body streams chunk-by-chunk, response relays back, and only after the provider's connection closes does the async sanitize→encrypt→persist run | Capture must add ~one network hop (~10–50ms), never sit on the provider's 5–30s timeout clock. Storage failures degrade capture completeness, never delivery |
| 2026-09-26 | **Sanitize at capture; never persist raw.** Stored = sanitized body + redactions manifest only; original signature headers not stored (secret-derived in some schemes); endpoint responses short-retention; sole exception = encrypted forward-retry buffer with minutes-level TTL | Data liability is structural, not contractual — "we never keep your raw payloads" beats any encryption badge for SOC 2/GDPR. Accepted trade-off: replay is byte-identical to the *sanitized* event; hash-pseudonyms preserve correlation; `keep-verbatim` per-path exceptions are the escape hatch |
| 2026-09-26 | Free tier includes the full CLI (tunnel, single-fire, 3 chains at localhost). The upgrade gate is **target type**: verified staging domains = Pro+, CI/staging chain runs = Team | CLI is the adoption engine, not a paywall; target-type gating is enforceable server-side via domain verification, unlike client feature flags |
| 2026-09-26 | GitHub Action = thin CLI wrapper in Marketplace; tier enforcement server-side on the API key's workspace; 403 error message carries the trial upsell | Never trust client-side tier checks; the CI error log is the highest-converting upsell surface we own |

---

## 11. Open Questions

1. ~~Go vs Rust?~~ **Resolved: Go.**
2. **Free tier limits:** 1,000 executions + 3 chains — validate at 60 days.
3. **Community template marketplace mechanics** (attribution, review bar): decide month 3–4, informed by private-template usage patterns.
4. **Recorder agent architecture:** Sidecar, Lambda, or Cloudflare Worker? Streaming-proxy contract (v2.2) applies to all three. Also keep the **mirror/sidecar capture fallback** on the table — customer endpoint receives directly and mirrors to us; zero inline risk, less completeness. Decide in Phase 3 scoping with 3–5 design partners.
5. **Team replay limit:** 2,000/mo — validate against trial usage.
6. ~~SOC 2 timing?~~ **Resolved in direction:** Type I scoping Phase 3, Type II Phase 4, revenue-contingent. No-raw-persistence architecture (v2.2) substantially reduces the scope.
7. **Kill criteria thresholds** (§12) — agree exact numbers before launch.
8. **Signing-helper package: language choice** — match primary segment (TypeScript likely) or Go-first for dogfooding?
9. **Sanitization schema coverage risk:** built-in schema may miss customer-specific PII fields → they persist sanitized-but-raw. Mitigation: schema editor with test-against-sample-event, plus a deny-list mode ("don't store events matching rule X")? Needs a Phase 3 scoping decision.

---

## 12. Success Metrics

| Metric | Target (6 months) |
|--------|-------------------|
| Signups | 5,000 |
| Weekly active users | 1,500 |
| Free → Pro conversion | 4% (typical PLG dev-tool 2–5%) |
| Pro → Team conversion | 3% |
| Time to first test | < 2 min |
| NPS | > 50 |
| Templates in library | 22+ (launch 14 + fast-follow 8) |
| Sandbox-verified payment templates | 100% of payment templates at launch |

**Kill criteria (agree thresholds pre-launch):**
- Free→Pro < 1.5% after 1,500 signups → chain/custom-webhook gate isn't converting; fix packaging before growth spend
- < 10 Team trials by month 6 → replay isn't pulling; defer Phase 3 recorder, double down on simulation
- Median free executions/mo > 3,000 → free tier too generous; lower ceiling (silent limit change, not price change)

---

## 13. Brand & Voice

- **Tone:** Developer-native, no corporate fluff, slightly irreverent
- **Tagline:** "Test webhooks without the chaos"
- **Colors:** Dark mode first, emerald/cyan accent (#0a0a0f base)
- **Landing page:** Interactive demo as hero (template picker + fire button); landing-page diagram candidate: the §3.5 three-phase lifecycle (dev loop → production tap → replay)

---

*This document is a living brief. Update as decisions are made and the product evolves.*
