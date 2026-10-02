# HookReplay — Owner Run Book

How to boot HookReplay on **Podman** or **Docker**, verify it's healthy, and run
it day-to-day. Assumes you're at the repo root.

## One command

```bash
# Podman — use this (it's what's installed)
podman compose up --build -d

# Docker — only if the Compose plugin is installed
docker compose up --build -d
```

> **"unknown flag: --build" / "'compose' is not a docker command"?** Your Docker
> install has no Compose plugin. Use `podman compose up --build -d` instead
> (Podman is the default here), or install the Docker Compose plugin / legacy
> `docker-compose`.

After boot:

| Thing | URL |
|---|---|
| Dashboard | http://localhost:3000 |
| API | http://localhost:8080 |
| Health | http://localhost:8080/v1/healthz → `{"status":"ok"}` |
| Templates | http://localhost:8080/v1/templates |

The API seeds a dev workspace + user + API key from `HOOKREPLAY_DEV_API_KEY`
(default `hrk_dev_local_dev_only_key`). Sign in on the dashboard with that key
(or GitHub).

---

## Prerequisites

- **Podman** (or Docker) with the Compose plugin.
- For local (non-container) dev: Go 1.25+ and Node 18+.
- Optional: a GitHub OAuth app (for `hookreplay login`) — only needed for
  browser sign-in; everything else works with API keys.

## 1. Configure the environment

The Compose file bakes sensible defaults. For **local** runs (not compose),
copy and adjust:

```bash
cp .env.example .env
```

Key variables (full reference below):

| Variable | Required | Notes |
|---|---|---|
| `HOOKREPLAY_MASTER_KEY` | yes | 32-byte key (hex or base64). `openssl rand -hex 32` |
| `HOOKREPLAY_DEV_API_KEY` | dev only | Seeds the bootstrap workspace + key |
| `HOOKREPLAY_DATABASE_URL` | yes | Postgres DSN |
| `HOOKREPLAY_PUBLIC_BASE_URL` | no | Base URL printed by the tunnel |
| `GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET` | OAuth only | For `hookreplay login` |

### GitHub OAuth (optional)

To enable `hookreplay login` / the dashboard's GitHub button:

1. GitHub → **Settings → Developer settings → OAuth Apps → New OAuth App**.
2. Homepage URL: `http://localhost:8080` · Callback URL:
   `http://localhost:8080/v1/auth/oauth/callback`.
3. Set `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` in `docker-compose.yml`
   (the `api` service) or `.env` for local runs.
4. Reboot: `podman compose up --build -d`.

Skip OAuth entirely with `hookreplay login --key <key>`.

## 2. Boot

```bash
# Podman
podman compose up --build -d

# Docker
docker compose up --build -d
```

First build takes a couple of minutes (Go image + Next.js `npm run build`).
Subsequent boots are fast (layers cached).

## 3. Verify

```bash
curl -s http://localhost:8080/v1/healthz
# → {"status":"ok"}

curl -s http://localhost:8080/v1/templates | head -c 400
# → JSON list of provider templates
```

Then open http://localhost:3000 and confirm the **Fire** page lists templates.

## 4. Logs, restart, teardown

```bash
podman compose logs -f api          # follow API logs
podman compose logs -f web          # follow dashboard logs
podman compose restart api web      # restart app containers
podman compose stop                 # stop (keep containers + volumes)
podman compose down                 # stop + remove containers (keeps DB volume)
podman compose down -v              # also wipe the DB → fresh state next boot
```

## 5. Dev mode (no containers for api/web)

Run only Postgres + Redis in containers, the rest on the host:

```bash
podman compose up -d postgres redis
set -a; source .env; set +a          # export env vars
go run ./cmd/api                     # API → :8080

# in another terminal
cd web && npm install && npm run dev # dashboard → :3000
```

## 6. Install the CLI (any machine)

**Recommended — one line:**

```bash
curl -fsSL https://raw.githubusercontent.com/hookreplay/hookreplay/main/install.sh | sh
```

Downloads the right binary for your OS/arch from GitHub Releases, verifies the
checksum, and installs to `/usr/local/bin` (or `~/.local/bin`).

From source (needs Go 1.25+):

```bash
go install ./cmd/cli                 # → $GOPATH/bin/hookreplay
```

Verify: `hookreplay --version`, `hookreplay templates list`.

### Authenticate the CLI

```bash
hookreplay login --key <your-api-key>   # no browser needed
hookreplay login                        # opens the web login page (email/Google/GitHub)
```

`login` stores the key in `~/.hookreplay/config`; the `HOOKREPLAY_API_KEY` and
`HOOKREPLAY_API_BASE` env vars also work, and can point at any server.

---

## Troubleshooting

### `bind: address already in use` (ports 5432 / 6379 / 8080 / 3000)

Something else already owns a published port (most commonly a **host Postgres
on 5432**). The stack publishes 5432, 6379, 8080, and 3000.

Options:

1. Stop the conflicting service, e.g. `sudo systemctl stop postgresql`.
2. Remap the host port in `docker-compose.yml` (e.g. `"5433:5432"`), then
   update `HOOKREPLAY_DATABASE_URL` for any host-side runs.

Also run `podman compose down` before re-`up` to clear stale containers.

### API exits at startup with a "master key" error

`HOOKREPLAY_MASTER_KEY` must be **exactly 32 bytes** — 64 hex chars or 44
base64 chars. Generate one: `openssl rand -hex 32`.

### Dashboard can't reach the API (proxy 404 / ECONNREFUSED)

The Next.js rewrite target is `HOOKREPLAY_API_PROXY`, baked at **build time**
(`web/Dockerfile` ARG). If you change it, rebuild the web image:

```bash
podman compose up --build web
```

### `hookreplay login` times out

GitHub OAuth isn't configured. Either set `GITHUB_CLIENT_ID`/`SECRET` and
reboot, or skip OAuth and store a key directly:

```bash
hookreplay login --key hrk_dev_local_dev_only_key
```

### Reset to a clean slate

```bash
podman compose down -v
podman compose up --build -d
```

This drops the Postgres volume (all executions, secrets, keys) and reseeds.

---

## Environment variable reference

| Variable | Default | Notes |
|---|---|---|
| `HOOKREPLAY_HTTP_ADDR` | `:8080` | API listen address |
| `HOOKREPLAY_DATABASE_URL` | `postgres://hookreplay:hookreplay@localhost:5432/hookreplay?sslmode=disable` | Postgres DSN |
| `HOOKREPLAY_MASTER_KEY` | — | 32-byte AES-256-GCM key (required) |
| `HOOKREPLAY_DEV_API_KEY` | — | Seeds the dev workspace/key |
| `HOOKREPLAY_PUBLIC_BASE_URL` | `http://localhost:8080` | Base URL for tunnel + OAuth callback |
| `GITHUB_CLIENT_ID` | — | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | — | GitHub OAuth app client secret |
| `HOOKREPLAY_API_PROXY` (web build ARG) | `http://api:8080` | API host the dashboard proxies to |
| `HOOKREPLAY_API_KEY` (CLI) | dev key | CLI API key |
| `HOOKREPLAY_API_BASE` (CLI) | `http://localhost:8080` | CLI API base URL |
