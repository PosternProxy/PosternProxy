---
title: Docker
weight: 2
---

# Docker installation

The release workflow publishes images for amd64 and arm64 after the owner
publishes a qualified GitHub release:

| Image | Purpose |
|---|---|
| `ghcr.io/posternproxy/posternproxy` | Controller with the built-in Caddy build and embedded UI |
| `ghcr.io/posternproxy/posternproxy-agent` | Remote agent for multi-server setups |

Both images bundle the pinned Caddy build with the `caddy-l4` and
`caddy-ratelimit` plugins, so no separate Caddy install is required. The manifests
are multi-architecture; Docker pulls the variant matching your host.

Use `:beta` for a published prerelease, `:latest` for a stable release, or pin an
exact version tag. The supplied Compose files use `:latest`; change their image
tags when testing a beta. Local qualification does not publish registry images.

## Prerequisites

- Docker Engine with the Compose plugin (`docker compose`)
- Ports 80, 443, and 81 available on the host
- A JWT signing secret (`openssl rand -hex 32`)

## Controller

The Compose files live in the `docker/` directory of the repository. Copy the
sample environment file and set your secret:

```bash
cd docker
cp .env.example .env   # set POSTERNPROXY_JWT_SECRET
```

When the UI is reached directly over HTTP on port 81, browsers drop `Secure`
cookies and sign-in fails with a missing-CSRF error. Disable secure cookies with
a Compose override for that case:

```bash
cat > docker-compose.http.yml <<'EOF'
services:
  posternproxy:
    environment:
      POSTERNPROXY_BEHIND_TLS_PROXY: "false"
EOF
docker compose -f docker-compose.yml -f docker-compose.http.yml up -d
```

Only set `POSTERNPROXY_BEHIND_TLS_PROXY=true` when management is actually
accessed through HTTPS behind a TLS proxy. The default `docker-compose.yml` does
not forward this variable from `.env`, so always set it through an override.

The container exposes 80, 443 (TCP and UDP for HTTP/3), and 81, and persists
state in the `posternproxy_data` volume mounted at `/data`.

## Post-install

The UI is available at `http://<server-ip>:81`. On first boot a random
administrator password is printed to the container logs:

```bash
docker logs posternproxy
```

Log in as `admin@posternproxy.local` (or `POSTERNPROXY_ADMIN_EMAIL`) with that
password, then change it. Set `POSTERNPROXY_ADMIN_PASSWORD` in `.env` before the
first boot to use a fixed password instead. Reinstalling over an existing
`/data` volume preserves credentials and the encryption identity.

If you missed the banner, reset the password without restarting the service:

```bash
docker exec posternproxy posternproxy reset-password
```

## Remote agent

Remote nodes run the agent image and connect outbound to the controller. Create a
manual server enrollment in the controller UI to obtain its token and server ID,
then set these in the agent host's environment before starting the service:

| Variable | Description |
|---|---|
| `POSTERNPROXY_CONTROLLER_URL` | Public URL the agent connects back to (also set on the controller) |
| `POSTERNPROXY_AGENT_TOKEN` | Enrollment token from the manual server registration |
| `POSTERNPROXY_SERVER_ID` | Registered server ID the agent applies config for |

```bash
cd docker
docker compose -f docker-compose.agent.yml up -d
```

The agent container drops all Linux capabilities except `NET_BIND_SERVICE` and
publishes ports 80 and 443 (TCP/UDP). After it starts, confirm the server shows
**Online** in the controller and that the expected revision has applied — a
running container alone does not prove configuration reached Caddy.

## Unraid

Unraid runs the same controller image through its Docker manager. There is no
Community Applications template yet, so add the container by hand.

{{% hint warning %}}
**Port conflict with the Unraid webUI**
Unraid's own management UI listens on ports 80 and 443. The controller needs
both for Caddy, so either move the Unraid webUI (Settings → Management Access)
to other ports first, or run the controller on a dedicated/custom Docker network
with its own IP. Do not map the host's 80/443 to the container while the Unraid
webUI still owns them.
{{% /hint %}}

In **Docker → Add Container**, switch to advanced view and set:

| Field | Value |
|---|---|
| Repository | `ghcr.io/posternproxy/posternproxy:latest` |
| Network Type | `Custom` (recommended, gives the container its own IP) or `Bridge` |
| WebUI | `http://[IP]:[PORT:81]/` |

Add these port mappings (container → host; only needed on `Bridge`):

| Container port | Protocol | Purpose |
|---|---|---|
| `80` | TCP | HTTP / ACME challenges |
| `443` | TCP | HTTPS |
| `443` | UDP | HTTP/3 |
| `81` | TCP | Management UI |

Add one path mapping and the environment variables:

| Kind | Container path | Host path / value |
|---|---|---|
| Path | `/data` | `/mnt/user/appdata/posternproxy` |
| Variable | `POSTERNPROXY_JWT_SECRET` | output of `openssl rand -hex 32` (required) |
| Variable | `POSTERNPROXY_BEHIND_TLS_PROXY` | `false` for direct HTTP access to the UI on port 81 |
| Variable | `POSTERNPROXY_ADMIN_EMAIL` | optional, defaults to `admin@posternproxy.local` |
| Variable | `POSTERNPROXY_ADMIN_PASSWORD` | optional; leave unset to use the generated first-boot password |

Apply to pull and start the container. The first-boot admin password appears in
the container log (the Unraid icon → **Logs**), the same banner `docker logs`
shows. Open the UI from the container's **WebUI** link, then change the password.
Keep `POSTERNPROXY_BEHIND_TLS_PROXY=false` unless you front the UI with HTTPS.
The `/data` appdata share holds the database, encryption identity and
certificates — back it up and reuse it to preserve credentials across upgrades.

For a remote agent on Unraid, use `ghcr.io/posternproxy/posternproxy-agent:latest`
with the same port mappings (80, 443 TCP/UDP) and the `POSTERNPROXY_CONTROLLER_URL`,
`POSTERNPROXY_AGENT_TOKEN` and `POSTERNPROXY_SERVER_ID` variables from the manual
enrollment above.

## Environment variables

See the [installation guide]({{< relref "/getting-started/installation" >}}) for
the full list of `POSTERNPROXY_*` variables. The entrypoint sets container
defaults for `POSTERNPROXY_DATA_DIR`, `POSTERNPROXY_DB_PATH`,
`POSTERNPROXY_CERT_DIR`, and the Caddy admin API; override them only if you
change the volume layout.

{{% hint info %}}
**Prefer native services?**
Use the [Linux installer]({{< relref "/getting-started/installation" >}}) for
systemd or the [Windows service guide]({{< relref "/getting-started/windows" >}})
for a native Windows deployment.
{{% /hint %}}
