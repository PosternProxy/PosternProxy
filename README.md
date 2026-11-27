# PosternProxy

> **Status: Beta** — Under active development and testing. Installation, provisioning, certificate access, and recovery qualification remain open. APIs and configuration may change before 1.0.

**Read the Docs @  [docs.posternproxy.com](https://docs.posternproxy.com)**

A self-hosted reverse proxy manager built on [Caddy](https://caddyserver.com). PosternProxy gives you a web UI to manage proxy hosts, redirections, stream hosts, SSL certificates, access lists, and port forwarding rules — all backed by Caddy's automatic HTTPS and distributed as a single binary with no Docker required.

## Features

- **Proxy hosts** — reverse proxy with load balancing, health checks, rate limiting, custom headers, and per-path locations
- **NTLM passthrough** — legacy, optional per-host forwarding of Windows authentication to one IIS upstream. The control ships off. IIS behavior is not verified.
- **Redirection hosts** — HTTP 301/302/307/308 redirects
- **Stream hosts** — TCP/UDP stream proxy and TLS passthrough via Caddy L4
- **Dead hosts** — return 404 for parked domains
- **SSL certificates** — automatic Let's Encrypt (HTTP-01) or upload your own PEM files
- **Access lists** — IP allowlist/denylist per host
- **Traffic forwarding** — raw TCP/UDP forwarding to any backend via Caddy Layer 4
- **Multi-server** — manage remote servers via a lightweight agent connected over WebSocket
- **Audit log** — recorded actions with actor and IP; host changes include safe before/after fields
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

See [SECURITY.md](SECURITY.md) for the vulnerability reporting process and installer hardening measures.

---

## Quick start

### Deploy with Docker

```bash
# Controller
cd docker
cp .env.example .env   # edit .env with your JWT secret
# For direct HTTP access to port 81, explicitly disable Secure cookies:
cat > docker-compose.http.yml <<'EOF'
services:
  posternproxy:
    environment:
      POSTERNPROXY_BEHIND_TLS_PROXY: "false"
EOF
docker compose -f docker-compose.yml -f docker-compose.http.yml up -d
```

```bash
# Agent (on a remote server)
cd docker
docker compose -f docker-compose.agent.yml up -d
```

The UI is at `http://<server-ip>:81`. On first boot a random admin password is printed to the container logs — check `docker logs posternproxy`.

For management access over HTTPS behind a TLS proxy, use
`POSTERNPROXY_BEHIND_TLS_PROXY=true` instead. The default Compose file does not
forward this variable from `.env`, so set it through a Compose override. Configure
the agent's controller URL, token and registered server ID in its environment
before starting the agent Compose service.

### Deploy from release binaries

```bash
# For Linux amd64 (use the arm64 archive on ARM servers):
tar -xzf posternproxy-linux-amd64.tar.gz
chmod +x posternproxy
sudo ./posternproxy install --assets-dir .
```

The archive contains `posternproxy`, `posternproxy-agent`, `caddy`, and
`checksums.txt`. The extracted controller is named `posternproxy`, without a
platform suffix. `--assets-dir .` uses the bundled Caddy and agent rather than
downloading release assets, so it also works with unreleased builds.

Windows amd64 uses a zip bundle and native service installation. See the
[Windows service guide](docs/content/getting-started/windows.md). Release targets
are native Linux amd64 and arm64, Windows amd64, and Docker amd64/arm64.

The installer verifies checksums, architecture, and Caddy plugins, then installs
hardened systemd services, fail2ban, and unattended security updates. No scripts or
Go toolchain are needed on the server. The UI is at `http://<server-ip>:81`.

Individual controller binaries from the [release page](https://github.com/PosternProxy/PosternProxy/releases/)
also support downloading their matching dependencies automatically; see the
[installation guide](docs/content/getting-started/installation.md) for that path.

Existing configuration and firewall rules are preserved. To add UFW rules and
enable it, explicitly pass `--configure-firewall --ssh-port 22` with your actual
SSH port. See the [installation guide](docs/content/getting-started/installation.md)
for offline installation and other options. This command is available in builds
containing the new installer; older releases still require their original setup.

Prefer to build it yourself? See [Full production build](#full-production-build-with-embedded-frontend) below, or the [Building from Source](docs/content/getting-started/building-from-source.md) guide.

---

## Local development

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.26+ |
| Node.js | 20.19+ or 22.12+ |
| npm | included with Node |
| Caddy | built with `caddy-l4`, `caddy-ratelimit`, and the Apache-2.0 `http_ntlm` transport (`github.com/PosternProxy/ntlm-transport`, a fork of `caddyserver/ntlm-transport`) |

### 1. Install Caddy (with required plugins)

```bash
. internal/caddybuild/build.sh
build_caddy ./caddy
verify_caddy ./caddy
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
go run ./cmd/posternproxy

# Terminal 2 — frontend (Vite HMR)
cd frontend && npm run dev
```

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
# The Vite development UI uses HTTP:
export POSTERNPROXY_BEHIND_TLS_PROXY=false
# Set a stable secret so sessions survive backend restarts
export POSTERNPROXY_JWT_SECRET=dev-secret-change-me
```

The `data/` directory is created automatically on first run.

---

## Building

### Quick build (Go binaries only)

```bash
go build -o posternproxy ./cmd/posternproxy
go build -o posternproxy-agent ./cmd/posternproxy-agent
```

Builds `posternproxy` and `posternproxy-agent` for the current platform. It reuses
`internal/ui/dist`, including any placeholder or stale UI files; use the full build
for a deployable frontend. It does not build Caddy or create a checksum manifest.
Plain `go build` falls back to Go's VCS build info for the version (or `"dev"`); the
production scripts below embed a `git describe` version via `-ldflags`.

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

Both scripts build and embed the Vue frontend, compile the controller and agent,
build the pinned custom Caddy, and write `checksums.txt`. Linux/macOS outputs are
in the repository root; PowerShell outputs are in `bin/`. The supplied archive
contains all four files. The agent is a separate executable, included in the
same build and bundle; it is not embedded inside the controller.

---

## Configuration reference

All settings are environment variables. In production they live in `/etc/posternproxy/config.env`.

| Variable | Default | Description |
|---|---|---|
| `POSTERNPROXY_LISTEN` | `:81` | Management UI listen address |
| `POSTERNPROXY_DATA_DIR` | `data` | Data/secret directory; the installer sets `/var/lib/posternproxy` |
| `POSTERNPROXY_DB_PATH` | `data/posternproxy.db` | SQLite database path |
| `POSTERNPROXY_JWT_SECRET` | *(generated)* | JWT signing secret — set a stable value in production |
| `POSTERNPROXY_CADDY_API` | `http://localhost:2019` | Caddy Admin API URL |
| `POSTERNPROXY_CERT_DIR` | `data/certs` | Directory for uploaded custom certificates |
| `POSTERNPROXY_CONTROLLER_URL` | *(empty)* | WebSocket URL advertised to agents (e.g. `http://10.0.0.1:81`) |
| `POSTERNPROXY_AGENT_BIN` | *(empty)* | Path to `posternproxy-agent` binary used during server provisioning |
| `POSTERNPROXY_BEHIND_TLS_PROXY` | `true` | Secure cookies; set `false` for an HTTP-only management UI |
| `POSTERNPROXY_ALLOWED_ORIGINS` | *(empty)* | Comma-separated allowed browser origins |
| `POSTERNPROXY_TRUSTED_PROXIES` | *(empty)* | Trusted management forwarding-proxy CIDRs |
| `POSTERNPROXY_CADDY_TRUSTED_PROXIES` | *(empty)* | JSON object mapping managed server IDs to explicit trusted proxy CIDRs; separate from management trust |
| `POSTERNPROXY_LOG_FORMAT` | Console | `json` for structured output; enabled in the installed service |
| `POSTERNPROXY_ADMIN_EMAIL` | `admin@posternproxy.local` | Initial admin email (first boot only) |
| `POSTERNPROXY_ADMIN_PASSWORD` | *(generated)* | Initial admin password (first boot only; native install prints it in the terminal) |

> **Important:** `POSTERNPROXY_JWT_SECRET` must be stable across restarts. If unset, a random secret is persisted to `data/secret.key` on first boot. The JWT secret also derives the AES-256 key used to encrypt stored SSH credentials — changing it invalidates all encrypted data.

---

## Multi-server (agent)

Remote servers run the included `posternproxy-agent`, which makes an outbound
WebSocket connection to the controller. The controller installer places it at
`/usr/local/bin/posternproxy-agent` for SSH uploads; it does not start a local
agent service. Each remote node needs the agent and Caddy built for that node's
OS/CPU, but no controller process. Open the node's actual proxy/forwarding ports
and SSH port as needed; the agent connection needs no inbound listener.

The **Servers -> Add Server** wizard supports automatic SSH installation and
manual enrollment. SSH installation uploads prebuilt agent/Caddy artifacts,
preserves the firewall, and waits for authenticated configuration application.
Manual enrollment displays one-time JSON credentials. From a remote node's
extracted matching archive:

```bash
chmod 600 agent.json
chmod +x posternproxy-agent
sudo ./posternproxy-agent install --config-file agent.json --assets-dir .
rm agent.json
```

Set `POSTERNPROXY_CONTROLLER_URL` on the controller first. The agent is included
in the archive as a separate executable; no separate package is needed. See the
[provisioning guide](docs/content/multi-server/provisioning.md) for prerequisites,
credential rotation, cross-architecture assets and readiness checks.

---

## Tech stack

| Layer | Technology |
|---|---|
| Proxy engine | [Caddy](https://caddyserver.com) with caddy-l4, caddy-ratelimit, and http_ntlm |
| Backend | Go, chi router, zerolog, modernc SQLite |
| Frontend | Vue 3, Vite, TypeScript, Tailwind CSS, Pinia |
| Database | SQLite (embedded, no external DB) |
| Auth | JWT (golang-jwt/jwt) + bcrypt passwords |

---

## Versioning

PosternProxy embeds a version string at build time via `-ldflags`. When built with the build scripts, the version comes from `git describe --tags --always --dirty`. Plain `go build` (no ldflags) falls back to Go's VCS build info, or `"dev"`.

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
go run ./scripts/check    # format, vet, lint, tests, security, frontend
```

Contributors must complete the project's CLA before a contribution can be accepted. See [LICENSING.md](LICENSING.md) for details.

Report suspected vulnerabilities privately as described in [SECURITY.md](SECURITY.md), not in a public issue.

## Development

This project uses AI assistance for tasks such as architecture planning, code review, documentation, and implementation support. All final code, design decisions, and project direction are by human maintainers. See [AGENTS.md](AGENTS.md) and [Claude.md](Claude.md) for how AI coding agents are configured for this codebase.

## License

PosternProxy is licensed under the **GNU Affero General Public License v3 (AGPLv3)**. Commercial licenses are also available from Cyclone Software Solutions, LLC for use cases where AGPLv3 is not suitable.

See [LICENSE](LICENSE) for the full license text and [LICENSING.md](LICENSING.md) for commercial options.

**Find out more at [www.posternproxy.com](https://www.posternproxy.com).**
