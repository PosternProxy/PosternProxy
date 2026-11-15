# PosternProxy

> **Status: Beta** — Feature-complete for core use cases, under active testing. APIs and configuration may change before 1.0.

**Read the Docs @  [docs.posternproxy.com](https://docs.posternproxy.com)**

A self-hosted reverse proxy manager built on [Caddy](https://caddyserver.com). PosternProxy gives you a web UI to manage proxy hosts, redirections, stream hosts, SSL certificates, access lists, and port forwarding rules — all backed by Caddy's automatic HTTPS and distributed as a single binary with no Docker required.

## Features

- **Proxy hosts** — reverse proxy with load balancing, health checks, rate limiting, custom headers, and per-path locations
- **Redirection hosts** — HTTP 301/302/307/308 redirects
- **Stream hosts** — TCP/UDP stream proxy and TLS passthrough via Caddy L4
- **Dead hosts** — return 404 for parked domains
- **SSL certificates** — automatic Let's Encrypt (HTTP-01) or upload your own PEM files
- **Access lists** — IP allowlist/denylist per host
- **Traffic forwarding** — raw TCP/UDP forwarding to any backend via Caddy Layer 4
- **Multi-server** — manage remote servers via a lightweight agent connected over WebSocket
- **Audit log** — every change recorded with actor, IP, and before/after snapshot
- **Config history** — roll back any proxy host to a previous configuration
- **Nginx import** — migrate an existing nginx config

## Architecture

```
Browser
  └── PosternProxy UI (:81)
        ├── REST API
        ├── SQLite database
        └── Caddy Admin API (:2019)
              └── Caddy (ports 80 / 443)
```

For multi-server setups, remote servers run `posternproxy-agent`, which maintains an outbound WebSocket connection to the controller and applies Caddy config updates as they arrive.

## Security model

PosternProxy delegates TLS termination and routing to Caddy — it does not include a WAF. For threat filtering, put PosternProxy behind:

- **Cloudflare** (free tier) — WAF, bot protection, DDoS mitigation
- **CrowdSec** with the [Caddy bouncer](https://github.com/crowdsecurity/cs-caddy-bouncer) — crowd-sourced IP reputation
- **Coraza WAF** (`corazawaf/coraza-caddy`) — OWASP Core Rule Set in Go

For internal / homelab use where hosts are not directly internet-exposed, no additional layer is typically needed.

See [SECURITY.md](SECURITY.md) for the vulnerability reporting process and a full list of hardening measures applied by the install script.

---

## Quick start

### Deploy with Docker

```bash
# Controller
cd docker
cp .env.example .env   # edit .env with your JWT secret
docker compose up -d
```

```bash
# Agent (on a remote server)
cd docker
docker compose -f docker-compose.agent.yml up -d
```

The UI is at `http://<server-ip>:81`. On first boot a random admin password is printed to the container logs — check `docker logs posternproxy`.

### Deploy from release binaries

```bash
# 1. Download posternproxy, posternproxy-agent, and caddy for your architecture
#    from https://github.com/PosternProxy/PosternProxy/releases/
#    (rename to the bare names if the assets carry a platform suffix)

# 2. Copy the binaries + install script + shared build contract
scp posternproxy posternproxy-agent caddy install.sh root@your-server:/tmp/posternproxy/
scp internal/caddybuild/build.sh root@your-server:/tmp/posternproxy/caddy-build.sh

# 3. Install (as root)
ssh root@your-server
bash /tmp/posternproxy/install.sh
```

The install script installs the shipped Caddy, creates system users, generates secrets, installs hardened systemd services, and configures UFW / fail2ban / unattended upgrades. After install the UI is at `http://<server-ip>:81`.

Prefer to build it yourself? See [Full production build](#full-production-build-with-embedded-frontend) below, or the [Building from Source](docs/content/getting-started/building-from-source.md) guide.

---

## Local development

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.25+ |
| Node.js | 20.19+ or 22.12+ |
| npm | included with Node |
| Caddy | built with `caddy-l4` and `caddy-ratelimit` plugins |

### 1. Install Caddy (with required plugins)

```bash
go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
xcaddy build \
    --with github.com/mholt/caddy-l4 \
    --with github.com/mholt/caddy-ratelimit \
    --output ./caddy
./caddy run
```

> On Linux you may need `sudo setcap cap_net_bind_service=+ep ./caddy` so Caddy can bind ports 80 and 443 without root. On Windows, run as Administrator or change Caddy's listen address for local testing.

### 2. Install frontend dependencies

```bash
cd frontend && npm install && cd ..
```

### 3. Start the dev servers

```bash
# Terminal 1 — backend
make dev-backend

# Terminal 2 — frontend (Vite HMR)
make dev-frontend
```

Or without make: `go run ./cmd/posternproxy` and `cd frontend && npm run dev`.

### 4. Open the UI

| Service | URL |
|---|---|
| Frontend (Vite dev server) | http://localhost:3000 |
| Backend API | http://localhost:81 |

The Vite dev server proxies `/api` and `/ws` requests to the backend automatically (configured in `frontend/vite.config.ts`).

On first boot a random admin password is printed to the console. Set `POSTERNPROXY_ADMIN_EMAIL` and `POSTERNPROXY_ADMIN_PASSWORD` environment variables to override.

### Environment variables

The backend is configured via environment variables. For local dev you can export these in your shell or create a `.env` file and source it:

```bash
export POSTERNPROXY_LISTEN=:81
export POSTERNPROXY_DB_PATH=data/posternproxy.db
export POSTERNPROXY_CADDY_API=http://localhost:2019
export POSTERNPROXY_CERT_DIR=data/certs
# Required for dev: allow the Vite dev server origin so CORS works correctly
export POSTERNPROXY_ALLOWED_ORIGINS=http://localhost:3000
# Set a stable secret so sessions survive backend restarts
export POSTERNPROXY_JWT_SECRET=dev-secret-change-me
```

The `data/` directory is created automatically on first run.

---

## Building

### Quick build (Go binaries only)

```bash
make build
```

Builds `posternproxy` and `posternproxy-agent` for the current platform with version info embedded. Re-uses whatever frontend is already in `internal/ui/dist` — will fail if no frontend has been built yet.

### Full production build (with embedded frontend)

**Linux / macOS:**

```bash
bash scripts/build.sh
```

**Windows** (cross-compiles to linux/amd64 by default):

```powershell
.\scripts\build.ps1
.\scripts\build.ps1 -GoOS windows -GoArch amd64 # Windows .exe files
.\scripts\build.ps1 -GoOS linux -GoArch arm64   # override target
```

Both scripts build the Vue frontend, embed it via `//go:embed`, and compile both binaries with the version string from `git describe --tags`.

Run `make help` for additional developer targets.

---

## Configuration reference

All settings are environment variables. In production they live in `/etc/posternproxy/config.env`.

| Variable | Default | Description |
|---|---|---|
| `POSTERNPROXY_LISTEN` | `:81` | Management UI listen address |
| `POSTERNPROXY_DB_PATH` | `data/posternproxy.db` | SQLite database path |
| `POSTERNPROXY_JWT_SECRET` | *(generated)* | JWT signing secret — set a stable value in production |
| `POSTERNPROXY_CADDY_API` | `http://localhost:2019` | Caddy Admin API URL |
| `POSTERNPROXY_CERT_DIR` | `data/certs` | Directory for uploaded custom certificates |
| `POSTERNPROXY_CONTROLLER_URL` | *(empty)* | WebSocket URL advertised to agents (e.g. `http://10.0.0.1:81`) |
| `POSTERNPROXY_AGENT_BIN` | *(empty)* | Path to `posternproxy-agent` binary used during server provisioning |
| `POSTERNPROXY_ADMIN_EMAIL` | `admin@posternproxy.local` | Initial admin email (first boot only) |
| `POSTERNPROXY_ADMIN_PASSWORD` | *(generated)* | Initial admin password (first boot only; printed to logs) |

> **Important:** `POSTERNPROXY_JWT_SECRET` must be stable across restarts. If unset, a random secret is persisted to `data/secret.key` on first boot. The JWT secret also derives the AES-256 key used to encrypt stored SSH credentials — changing it invalidates all encrypted data.

---

## Multi-server (agent)

Remote servers run `posternproxy-agent`, which makes an outbound WebSocket connection to the controller. No inbound ports beyond 80 and 443 are needed on the remote server.

The easiest way to add a remote server is via the UI: go to **Servers → Add Server**, fill in SSH credentials, and click **Provision**. PosternProxy will SSH in and set everything up automatically.

To run the agent manually:

```bash
export POSTERNPROXY_CONTROLLER_URL=http://your-controller:81
export POSTERNPROXY_AGENT_TOKEN=<token-from-ui>
export POSTERNPROXY_SERVER_ID=<id-from-ui>
./posternproxy-agent
```

---

## Tech stack

| Layer | Technology |
|---|---|
| Proxy engine | [Caddy](https://caddyserver.com) with caddy-l4 and caddy-ratelimit |
| Backend | Go, chi router, zerolog, modernc SQLite |
| Frontend | Vue 3, Vite, TypeScript, Tailwind CSS, Pinia |
| Database | SQLite (embedded, no external DB) |
| Auth | JWT (golang-jwt/jwt) + bcrypt passwords |

---

## Versioning

PosternProxy embeds a version string at build time via `-ldflags`. When built with `make build` or the build scripts, the version comes from `git describe --tags --always --dirty`. Plain `go build` (no ldflags) falls back to Go's VCS build info, or `"dev"`.

The running version is logged at startup and available via `GET /api/version`.

## Continuous integration

Pull requests and pushes to master run the [CI workflow](.github/workflows/ci.yml):
- `go vet`, `go test`, `go build` for all Go packages
- `npm run type-check`, `npm run lint:check`, `npm run build` for the frontend
- `npm audit` for dependency vulnerabilities

PRs are expected to keep CI green.

---

## Contributing

Before opening a pull request, ensure CI checks pass locally:

```bash
make test      # Go tests
make lint      # go vet + frontend lint
```

Contributors must complete the project's CLA before a contribution can be accepted. See [LICENSING.md](LICENSING.md) for details.

Report suspected vulnerabilities privately as described in [SECURITY.md](SECURITY.md), not in a public issue.

## Development

This project uses AI assistance for tasks such as architecture planning, code review, documentation, and implementation support. All final code, design decisions, and project direction are by human maintainers. See [AGENTS.md](AGENTS.md) and [Claude.md](Claude.md) for how AI coding agents are configured for this codebase.

## License

PosternProxy is licensed under the **GNU Affero General Public License v3 (AGPLv3)**. Commercial licenses are also available from Cyclone Software Solutions, LLC for use cases where AGPLv3 is not suitable.

See [LICENSE](LICENSE) for the full license text and [LICENSING.md](LICENSING.md) for commercial options.

**Find out More @ [www.posternproxy.com](https://www.posternproxy.com**
