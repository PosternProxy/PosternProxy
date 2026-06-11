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

The install script handles everything: building Caddy with required plugins, creating system users, configuring systemd, UFW, fail2ban, and unattended-upgrades.

**Step 1 — Build the binary**

On your development machine (or CI), run:

```bash
bash scripts/build.sh
```

This produces `./posternproxy` (controller) and `./posternproxy-agent` (remote agent).

**Step 2 — Copy the binary to the server**

```bash
scp posternproxy user@your-server:/tmp/
```

**Step 3 — Run the install script**

```bash
ssh root@your-server
cd /tmp
bash /path/to/scripts/install.sh
```

The script will:

1. Install Go (if needed) and build Caddy with the L4 and rate-limit plugins
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

On first boot, PosternProxy creates an admin account and **prints a generated password to the console** (stderr). Check the service logs immediately after startup:

```bash
journalctl -u posternproxy --no-pager | grep -A5 "INITIAL SETUP"
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
If you missed the banner, use the `reset-password` subcommand to recover access without running the server:

```bash
posternproxy reset-password
# or for a specific account:
posternproxy reset-password --email admin@posternproxy.local
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
