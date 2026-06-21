---
title: Installation
weight: 1
---

# Installation

PosternProxy installs directly on Debian 11/12 or Ubuntu 22.04/24.04. No Docker required.

## Prerequisites

- A fresh Debian or Ubuntu server with root access
- Ports 80, 443, and 81 open in your firewall
- A domain or IP for the management UI

## Automated install

The install script handles everything: installing the bundled Caddy, creating system users, configuring systemd, UFW, fail2ban, and unattended-upgrades.

**Step 1 — Download the release binaries**

Download the latest release from the
[GitHub releases page](https://github.com/PosternProxy/PosternProxy/releases/).
Each release provides three binaries for your server's architecture (`amd64` for
most servers, `arm64` for ARM):

| Binary | Purpose |
|---|---|
| `posternproxy` | The controller (management UI + API) |
| `posternproxy-agent` | The remote agent — required to manage remote servers |
| `caddy` | A custom Caddy build with the L4 and rate-limit plugins |

The installer looks for the binaries named exactly `posternproxy`,
`posternproxy-agent`, and `caddy`. If the release assets carry a platform suffix
(e.g. `posternproxy-linux-amd64`), rename them to the bare names above.

{{< hint info >}}
**Prefer to build it yourself?**
See [Building from source]({{< relref "/getting-started/building-from-source" >}})
for building the binaries on Linux or Windows.
{{< /hint >}}

**Step 2 — Copy the binaries and the install script to the server**

Put the three binaries and `install.sh` in one directory — the installer finds the
binaries in the directory it runs from. The install script ships with the release;
it is also available at `scripts/install.sh` in the repository.

```bash
ssh root@your-server 'mkdir -p /tmp/posternproxy'
scp posternproxy posternproxy-agent caddy install.sh root@your-server:/tmp/posternproxy/
```

Notes:
- The `caddy` binary is the custom build from Step 1. Shipping it means the
  installer skips compiling Caddy on the server, so **no Go toolchain is
  installed on production**. If you omit it, the installer falls back to
  building Caddy from source (and installs Go to do so).
- The `posternproxy-agent` binary is optional for a controller-only install, but
  it is required for provisioning remote nodes — copy it now to avoid a second trip.

**Step 3 — Run the install script**

```bash
ssh root@your-server
bash /tmp/posternproxy/install.sh
```

The script will:

1. Install the custom Caddy binary you shipped (or, if none was shipped, install Go and build Caddy with the L4 and rate-limit plugins on the server)
2. Create `caddy` and `posternproxy` system users (no login shell)
3. Generate a random JWT secret and write `/etc/posternproxy/config.env`
4. Install hardened systemd services for both Caddy and PosternProxy
5. Apply sysctl hardening (IP forwarding, rp_filter, TCP SYN cookies)
6. Configure UFW (allow ports 80, 443, 81)
7. Install and configure fail2ban with a PosternProxy login-failure jail
8. Enable unattended security upgrades
9. Optionally harden SSH (disable root login, enforce key-only auth)

{{< hint warning >}}
**SSH hardening prompt**
The installer asks whether to harden SSH. If you choose yes, ensure your SSH key is in `~/.ssh/authorized_keys` before logging out — password auth will be disabled.
{{< /hint >}}

## Post-install

Once complete, the UI is available at:

```
http://<your-server-ip>:81
```

On first boot, PosternProxy creates an admin account and **prints a generated password to stderr**, which the systemd service writes to its log file (not the journal). Check it immediately after startup:

```bash
grep -A6 "INITIAL SETUP" /var/log/posternproxy/posternproxy.log
```

The banner looks like:

```
╔══════════════════════════════════════════════════════╗
║           POSTERNPROXY — INITIAL SETUP               ║
╟──────────────────────────────────────────────────────╢
║  Email:    admin@posternproxy.local                  ║
║  Password: <random 20-character password>            ║
╟──────────────────────────────────────────────────────╢
║  Change this password immediately after first login. ║
║  Set POSTERNPROXY_ADMIN_PASSWORD to silence this.    ║
╚══════════════════════════════════════════════════════╝
```

Log in with the email and generated password shown, then change it immediately.

{{< hint info >}}
**Setting a fixed admin password**
Set `POSTERNPROXY_ADMIN_PASSWORD` in `/etc/posternproxy/config.env` before the first boot to use a specific password instead of a generated one.
{{< /hint >}}

{{< hint warning >}}
**Lost the generated password?**
If you missed the banner, use the `reset-password` subcommand to recover access without running the server. Run it as the `posternproxy` user with the config loaded, so it uses the correct database path and does not create root-owned database files:

```bash
sudo -u posternproxy bash -c 'set -a; . /etc/posternproxy/config.env; set +a; \
  /usr/local/bin/posternproxy reset-password'

# or for a specific account, append: --email admin@posternproxy.local
```

A new random password is printed to the terminal.
{{< /hint >}}

## Environment variables

PosternProxy is configured via `/etc/posternproxy/config.env`. All variables are optional — sensible defaults are used if not set.

| Variable | Default | Description |
|---|---|---|
| `POSTERNPROXY_LISTEN_ADDR` | `:81` | Management UI listen address |
| `POSTERNPROXY_DB_PATH` | `/var/lib/posternproxy/posternproxy.db` | SQLite database path |
| `POSTERNPROXY_JWT_SECRET` | *(persisted in `<DataDir>/secret.key`)* | Secret for signing JWT tokens and encrypting SSH credentials — stable across restarts |
| `POSTERNPROXY_CADDY_API` | `http://localhost:2019` | Caddy admin API URL |
| `POSTERNPROXY_CERT_DIR` | `/var/lib/posternproxy/certs` | Custom certificate storage |
| `POSTERNPROXY_CONTROLLER_URL` | `http://<host-ip>:81` | URL agents use to connect |
| `POSTERNPROXY_AGENT_BIN_PATH` | `/usr/local/bin/posternproxy-agent` | Agent binary for provisioning |
| `POSTERNPROXY_ADMIN_EMAIL` | `admin@posternproxy.local` | Initial admin email (first boot only) |
| `POSTERNPROXY_ADMIN_PASSWORD` | *(random generated)* | Initial admin password (first boot only — printed to stderr if not set) |
| `POSTERNPROXY_BEHIND_TLS_PROXY` | `true` | Marks session/CSRF cookies `Secure`. The installer writes `false` because the UI is served over HTTP on port 81 — browsers drop `Secure` cookies over HTTP, causing "missing CSRF token" errors. Set to `true` only when the UI is fronted by TLS and accessed over HTTPS. |

## Systemd management

```bash
# Check status
systemctl status posternproxy
systemctl status caddy

# View logs
journalctl -u posternproxy -f
journalctl -u caddy -f

# Restart
systemctl restart posternproxy
```

## Caddy plugins included

The install script builds Caddy with two plugins beyond the standard distribution:

| Plugin | Purpose |
|---|---|
| `github.com/mholt/caddy-l4` | Layer 4 routing (stream hosts, TLS passthrough) |
| `github.com/mholt/caddy-ratelimit` | Per-host request rate limiting |
