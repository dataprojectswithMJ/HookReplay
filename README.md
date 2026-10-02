# HookReplay — Vertical Slice

A working end-to-end slice of the HookReplay SaaS: fire a **provider-accurate,
correctly-signed** webhook through a tunnel at localhost, and see it in a
dashboard. Built against `HookReplay_Build_Spec.md` (Go 1.23+, Next.js 14,
PostgreSQL 16, Redis 7).

> **Status:** vertical slice. This proves the core promise — *"first valid
> webhook in under 2 minutes"* — end to end: **signing → dispatch → tunnel →
> CLI `run` → dashboard log**. Multi-step chains, retry schedules, domain
> verification, billing, and recorder/replay are deferred (see [Scope](#scope)).

---

## Architecture

```
hookreplay.yml ──► CLI (Go) ──► POST /v1/dispatch ──► API (Go)
                                                       │  sign at dispatch time
                                                       ▼
                                              Dispatcher (Go) ──► Tunnel (WS) ──► localhost
                                                       │
                                                       └──► PostgreSQL (executions, byte-exact)

Dashboard (Next.js) ──► /v1/executions (same API, same data)
```

| Component | Path | Notes |
|---|---|---|
| Signing engine | `internal/signing` | Stripe, GitHub, Paystack, custom HMAC; sign + independent verify |
| Dispatcher | `internal/dispatch` | sign-at-dispatch, timeout, breaker, rate limiter, `deliver` failure sim |
| API | `cmd/api` | `/v1/*` REST, API-key auth, secrets (write-only, AES-256-GCM) |
| Tunnel | `internal/tunnel` | WebSocket relay, subdomain registry, reconnect client |
| CLI | `cmd/cli` | `login`, `init`, `fmt`, `validate`, `tunnel`, `run`, `templates`, `secrets`, `api-keys`, `workspaces`, `keys generate`, `chains push/pull/list` |
| Templates | `templates/providers/*` | embedded fixtures: payload + `sign.yml` + `delivery.yml` |
| Dashboard | `web/` | Next.js 14 + Tailwind (dark-first) |
| Schema | `pkg/chainschema` | frozen `hookreplay.yml` JSON-schema |

---

## Prerequisites

- Go 1.25+ (to build the CLI/API)
- Node 18+ (for the dashboard)
- Podman (or Docker with the Compose plugin) for Postgres + Redis

## Install the CLI

**Recommended — one line (any OS with a shell):**

```bash
curl -fsSL https://raw.githubusercontent.com/dataprojectswithMJ/HookReplay/main/install.sh | sh
```

This downloads the right binary for your OS/arch from GitHub Releases, verifies
its checksum, and installs it to `/usr/local/bin` (or `~/.local/bin`).

Alternatives:

```bash
brew install hookreplay      # Homebrew tap (see .goreleaser.yml)
go install ./cmd/cli         # from source (dev)
```

Verify:

```bash
hookreplay --version
hookreplay templates list
```

## Run it

> **Owner run book** (Podman/Docker boot + operations): see
> [`RUNNING.md`](./RUNNING.md). **In-app getting-started**: the **Docs** page at
> `/docs` in the dashboard.

### Option A — Podman (one command)

```bash
podman compose up --build -d
# API   → http://localhost:8080  (seeds a dev workspace + API key)
# Web   → http://localhost:3000  (dashboard)
# DB    → postgres:16 + redis:7
```

> Docker also works (`docker compose up --build -d`) **only if the Compose plugin
> is installed**. If you see `unknown flag: --build`, use `podman compose` instead
> (see `RUNNING.md` for details).

Open http://localhost:3000, enter `hrk_dev_local_dev_only_key` as the API key,
pick a template, set a target + secret, and hit **Fire**.

> The tunnel (`hookreplay tunnel`) is the CLI's job — run it from the host
> against the API: `go run ./cmd/cli tunnel --port 3000`.

### Option B — Local (dev)

### 1. Start infrastructure

```bash
docker compose up -d          # postgres:16 + redis:7  (or: podman compose up -d)
```

### 2. Resolve Go dependencies (first time only)

```bash
go mod tidy
```

### 3. Start the API

```bash
cp .env.example .env          # adjust HOOKREPLAY_DEV_API_KEY if you like
set -a; source .env; set +a
go run ./cmd/api
```

The API seeds a dev workspace + user + API key from `HOOKREPLAY_DEV_API_KEY`
(default `hrk_dev_local_dev_only_key`).

### 4. Start the tunnel (terminal A)

```bash
export HOOKREPLAY_API_KEY=hrk_dev_local_dev_only_key
go run ./cmd/cli tunnel --port 3000
# → Tunnel ready: http://localhost:8080/t/hrk-xxxxxxxx/
```

Point it at your own webhook handler running on port 3000 (e.g. a local Node or
Go server that verifies the signature).

### 5. Fire a webhook (terminal B)

```bash
export HOOKREPLAY_API_KEY=hrk_dev_local_dev_only_key
export TUNNEL_URL=http://localhost:8080/t/hrk-xxxxxxxx/   # copy from step 4
export STRIPE_WEBHOOK_SECRET=whsec_your_test_secret

go run ./cmd/cli init
go run ./cmd/cli run
```

`run` resolves `env:STRIPE_WEBHOOK_SECRET` from the shell, dispatches through the
API, and streams the result. The payload is signed at dispatch time over the raw
bytes — your local handler's Stripe signature check will pass.

### 6. Dashboard (terminal C)

```bash
cd web
npm install
npm run dev
# → http://localhost:3000
```

On the dashboard, sign in with the API key (or GitHub), pick a template, set a
target + secret, and hit **Fire**. The **Log** page lists executions; each
execution shows the raw body, signature header, **SHA-256**, and **byte length**
(trust signals).

---

## Authenticate (CLI + dashboard)

The CLI needs an API key. Two ways:

```bash
# 1. Local dev key (named "dev") — no GitHub needed
hookreplay login --name dev

# 2. Web login — email/Google/GitHub (OAuth needs client IDs/secrets on the server)
hookreplay login
```

`login` stores the key in `~/.hookreplay/config`; you can also use the
`HOOKREPLAY_API_KEY` / `HOOKREPLAY_API_BASE` env vars. The dashboard's
**Sign in** page accepts the same key, or GitHub OAuth.

### Setting up GitHub OAuth (optional)

1. GitHub → **Settings → Developer settings → OAuth Apps → New OAuth App**.
2. Homepage URL: `http://localhost:8080` · Callback URL:
   `http://localhost:8080/v1/auth/oauth/callback`.
3. Put the Client ID + Client Secret in `.env` / `docker-compose.yml` as
   `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`.
4. Rebuild: `podman compose up --build -d`.

---

## Verification (the trust guarantee)

A reference verifier (`internal/signing.Verify`) is a *separate* implementation
from `Sign`. Tests assert:

- every scheme round-trips (sign → verify passes);
- a wrong key **fails** verification (signatures aren't cosmetic);
- Paystack is hex(HMAC-SHA512) — 128 hex chars, never base64;
- the dispatcher sends **byte-exact** bodies (`hash sent == hash stored`).

```bash
go test ./...
```

---

## Scope

**Implemented:** signing (Stripe/GitHub/Paystack/custom), dispatch with
byte-exact storage + breaker + rate limiter + idempotency + `deliver` failure
simulation + async retry scheduling, WebSocket tunnel, API
(auth/secrets/templates/dispatch/executions/chains/usage/domains), the chain
engine (`webhook`/`api`/`custom` steps, `when`, `{{steps.*}}` extraction,
`expect` incl. `status`/`latency_under`/`retries`/`delays`/`body`), the CLI
(`run`/`tunnel`/`validate`/`fmt`/`templates`/`secrets`/`api-keys`/`workspaces`/
`keys`/`chains`), and the dashboard (fire → log → detail → chains → keys →
workspace → secrets → usage → domains).

**Deferred to later workstreams (interfaces are in place):**
- The other 19 provider templates + §7.4 live sandbox-gate vectors (the
  deterministic HMAC/Ed25519 vectors are covered; live provider sandboxes are a
  manual step requiring real accounts).
- Redis-Streams worker split (dispatch is synchronous in this slice), DNS-TXT
  domain verification, full tier enforcement/metering/billing, browser OAuth,
  and recorder/replay (WS-9, interface-only).
