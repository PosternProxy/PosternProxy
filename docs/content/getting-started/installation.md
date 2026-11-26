---
title: Installation
weight: 1
---

# Installation

Native installation uses systemd on Debian/Ubuntu or
[Windows services]({{< relref "/getting-started/windows" >}}), and
[Docker images]({{< relref "/getting-started/docker" >}}) are published for
amd64/arm64. Release targets are native Linux amd64 and arm64, Windows amd64,
and Docker amd64/arm64.

## Prerequisites

- A fresh Debian or Ubuntu server with root access
- Ports 80, 443, and 81 open in your firewall
- A domain or IP for the management UI

## Automated install

The controller includes its own installer. It can use the Caddy and agent from
a supplied archive, or download them when installing an individual published
controller binary. The build must contain the `install` subcommand; older
releases do not gain the command automatically.

### Install from a bundle (`.tar.gz`)

{{% hint info %}}
**Which architecture?**
Both native Linux architectures have release bundles. Use
`posternproxy-linux-amd64.tar.gz` on x86-64 servers or
`posternproxy-linux-arm64.tar.gz` on ARM64 servers. Run `uname -m` if unsure:
`x86_64` means amd64; `aarch64` means arm64. The commands below use amd64.
{{% /hint %}}

Copy the matching bundle to your server.
Extract it in a dedicated directory and install:

```bash
mkdir -p posternproxy-install
tar -xzf posternproxy-linux-amd64.tar.gz -C posternproxy-install
cd posternproxy-install
chmod +x posternproxy
sudo ./posternproxy install --assets-dir .
```

The archive contains these four files:

| File | Purpose |
|---|---|
| `posternproxy` | Controller with the built-in installer |
| `posternproxy-agent` | Matching remote agent |
| `caddy` | Caddy with the required plugins |
| `checksums.txt` | SHA-256 checksums for the binaries |

The extracted executable is named **`posternproxy`**, not
`posternproxy-linux-amd64` or `posternproxy-linux-arm64`. Keep `--assets-dir .` in the command: it uses the
extracted assets and works with unreleased source builds that have no matching
published release. No additional binary downloads occur; OS packages still use APT.

The installer also places the included agent at
`/usr/local/bin/posternproxy-agent` for uploading to remote nodes. It does not
start an agent service on the controller. Remote nodes need this agent and Caddy
for their architecture; see [Provisioning]({{< relref "/multi-server/provisioning" >}})
for the separate remote setup and its current beta limitations.

{{% hint info %}}
**Prefer to build it yourself?**
See [Building from source]({{< relref "/getting-started/building-from-source" >}})
for building the binaries on Linux or Windows.
{{% /hint %}}

### Install an individual release binary

If you downloaded an individual controller instead of an archive from the
[GitHub releases page](https://github.com/PosternProxy/PosternProxy/releases/),
choose `posternproxy-linux-amd64` or `posternproxy-linux-arm64` for your server.
Verify against `checksums.txt`, then rename it to the
same command name used in the examples:

```bash
mv posternproxy-linux-amd64 posternproxy
chmod +x posternproxy
sudo ./posternproxy install
```

Use the `arm64` filename in the rename command on ARM servers. This path downloads
Caddy and the agent from the controller's exact release tag over HTTPS. The
matching assets and checksum manifest must already be published; use the bundle
path above for the supplied unreleased build.

### What installation does

1. Stages bundled assets, or downloads the matching published assets.
2. Checks SHA-256 checksums, Linux architecture, and Caddy's pinned version/plugins before changing the system.
3. Installs OS packages, dedicated users, protected configuration, and hardened systemd services.
4. Applies sysctl hardening and configures fail2ban and unattended security updates.
5. Starts Caddy and the controller, then waits for running services and reachable local APIs.

No Go compiler, repository checkout, install script, or `caddy-build.sh` is needed
on the server. Download failures and checksum/plugin mismatches stop installation.
Checksums verify artifact integrity against the release manifest; they are not
independent release signatures.

### Options and reruns

```bash
./posternproxy install --help

# Use a different management port with the extracted bundle:
sudo ./posternproxy install --assets-dir . --port 8081

# Add firewall rules, allowing the actual SSH port before enabling UFW:
sudo ./posternproxy install --assets-dir . --configure-firewall --ssh-port 2222
```

For an individual published controller using automatic downloads, omit
`--assets-dir .`.

Firewall setup is opt-in. It adds SSH, HTTP, HTTPS (TCP/UDP), and the management
port without resetting existing rules or changing default policies. Without the
flag, open the required ports yourself in the host and cloud firewalls. SSH
configuration is never modified.

Reruns preserve `/etc/posternproxy/config.env`, secrets, data, and existing
hardening configuration. `--port` applies only when creating a new configuration.
Managed service units and binaries are replaced and services restart. For an
existing installation, follow [Beta upgrades and recovery]({{< relref
"/administration/upgrading" >}}) first; configuration preservation does not
qualify an older database for upgrade.

Existing storage paths outside `/var/lib/posternproxy` (or explicitly configured
relative storage paths) are rejected before installation changes. Such deployments
need a deliberate storage migration or their existing custom service setup; the
installer does not replace a custom layout with incompatible sandbox permissions.

Development builds need `--assets-dir` or an explicit `--release vX.Y.Z` selecting
compatible published assets. There is no automatic fallback to the latest release.

### Local assets and offline installation

Source builds produce `posternproxy`, `posternproxy-agent`, `caddy`, and
`checksums.txt`. Copy these four files together and run:

```bash
chmod +x posternproxy
sudo ./posternproxy install --assets-dir .
```

Release assets with platform suffixes are also accepted in this directory.
Use binaries and checksums from the same build. The local files are copied into
private staging and checked before installation. No release downloads occur with
`--assets-dir`; OS packages still come from APT. For a disconnected host, preinstall
`ca-certificates`, `fail2ban`, `iptables`, `unattended-upgrades`, and `apt-listchanges` (plus
`ufw` if requested), then add `--skip-packages`. A running systemd host and Bash
for the embedded Caddy verifier are required.

## Post-install

Once complete, the UI is available at:

```
http://<your-server-ip>:81
```

On a fresh installation, `posternproxy install` prints the initial administrator
email and password directly in your terminal after the services pass readiness:

```text
Initial administrator login
Email: admin@posternproxy.local
Password: <generated password>
Change this password after your first login.
```

The installer uses a private temporary file under `/run` for the first service
start, then removes it. The generated password is not saved in the persistent
service configuration or printed in service logs. Reinstalling an existing
database preserves its credentials and does not display or reset its password.

Log in with the displayed credentials, then change the password. Direct runs and
containers still use the startup banner when no initial password is configured.

{{% hint info %}}
**Setting a fixed admin password**
Set `POSTERNPROXY_ADMIN_PASSWORD` in `/etc/posternproxy/config.env` before the first boot to use a specific password instead of a generated one.
{{% /hint %}}

{{% hint warning %}}
**Lost the generated password?**
If you missed the banner, use the `reset-password` subcommand to recover access without running the server. Run it as the `posternproxy` user with the config loaded, so it uses the correct database path and does not create root-owned database files:

```bash
sudo -u posternproxy bash -c 'set -a; . /etc/posternproxy/config.env; set +a; \
  /usr/local/bin/posternproxy reset-password'

# or for a specific account, append: --email admin@posternproxy.local
```

A new random password is printed to the terminal.
{{% /hint %}}

## Environment variables

PosternProxy is configured via `/etc/posternproxy/config.env`. All variables are optional — sensible defaults are used if not set.

| Variable | Default | Description |
|---|---|---|
| `POSTERNPROXY_LISTEN` | `:81` | Management UI listen address |
| `POSTERNPROXY_DATA_DIR` | `data` | Data directory; the installer sets `/var/lib/posternproxy` |
| `POSTERNPROXY_LOG_FORMAT` | Console | Set `json` for structured logs; the installed service uses JSON for fail2ban |
| `POSTERNPROXY_DB_PATH` | `data/posternproxy.db` | SQLite path; the installer sets `/var/lib/posternproxy/posternproxy.db` |
| `POSTERNPROXY_JWT_SECRET` | *(persisted in `<DataDir>/secret.key`)* | Secret for signing JWT tokens and encrypting SSH credentials and custom certificates — stable across restarts |
| `POSTERNPROXY_CADDY_API` | `http://localhost:2019` | Caddy admin API URL |
| `POSTERNPROXY_CERT_DIR` | `data/certs` | Custom certificate storage; the installer sets `/var/lib/posternproxy/certs` |
| `POSTERNPROXY_CERT_RUNTIME_DIR` | `data/caddy-certs` | Group-restricted runtime PEM; native install uses `/run/posternproxy-certs` |
| `POSTERNPROXY_CADDY_BIN` | `/usr/bin/caddy` | Pinned Caddy artifact for SSH provisioning |
| `POSTERNPROXY_CONTROLLER_URL` | Empty | Public URL agents use to connect; configure for remote-node provisioning |
| `POSTERNPROXY_TRUSTED_PROXIES` | Empty | Comma-separated proxy CIDRs trusted to supply `X-Forwarded-For`. See [Security Hardening]({{< relref "/administration/security-hardening" >}}). |
| `POSTERNPROXY_CADDY_TRUSTED_PROXIES` | Empty | JSON object mapping managed server IDs to Caddy trusted-proxy CIDRs. No trust is inherited between servers. See [Security Hardening]({{< relref "/administration/security-hardening" >}}). |
| `POSTERNPROXY_ALLOWED_ORIGINS` | Empty | Comma-separated browser origins allowed by CORS; use `http://localhost:3000` for Vite development |
| `POSTERNPROXY_AGENT_BIN` | `/usr/local/bin/posternproxy-agent` | Agent binary for provisioning; the installer sets `/usr/local/bin/posternproxy-agent` |
| `POSTERNPROXY_ADMIN_EMAIL` | `admin@posternproxy.local` | Initial admin email (first boot only) |
| `POSTERNPROXY_ADMIN_PASSWORD` | *(random generated)* | Initial admin password (first boot only; native install displays it in the terminal) |
| `POSTERNPROXY_BEHIND_TLS_PROXY` | `true` | Marks session/CSRF cookies `Secure`. The installer writes `false` because the UI is served over HTTP on port 81 — browsers drop `Secure` cookies over HTTP, causing "missing CSRF token" errors. Set to `true` only when the UI is fronted by TLS and accessed over HTTPS. |

## Systemd management

```bash
# Check status
systemctl status posternproxy
systemctl status caddy

# View logs
sudo tail -f /var/log/posternproxy/posternproxy.log
journalctl -u caddy -f

# Restart
systemctl restart posternproxy
```

## Caddy plugins included

The installer downloads a pinned Caddy build with two plugins beyond the standard distribution:

| Plugin | Purpose |
|---|---|
| `github.com/mholt/caddy-l4` | Layer 4 routing (stream hosts, TLS passthrough) |
| `github.com/mholt/caddy-ratelimit` | Per-host request rate limiting |
