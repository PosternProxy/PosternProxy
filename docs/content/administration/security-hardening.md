---
title: Security Hardening
weight: 1
---

# Security Hardening

`posternproxy install` configures service sandboxing, fail2ban, sysctl hardening,
and unattended security updates. Firewall configuration is opt-in. This page
documents the defaults and verification commands.

## Application-level hardening

### Authentication

- JWT access tokens with 15-minute TTL
- Refresh tokens with 7-day TTL stored in HttpOnly / SameSite=Strict cookies;
  Secure follows `POSTERNPROXY_BEHIND_TLS_PROXY`
- bcrypt password hashing at cost factor 12
- Login rate limit: 5 attempts/min per IP (enforced by the API and by fail2ban)
- Account lockout is not time-based — use fail2ban to block IPs

### API security

- **Rate limiting:** 120 requests/min per IP on authenticated management API routes; 5 login attempts/min per IP; 30 agent WebSocket connection attempts/min per IP
- **CSRF protection:** Double-submit cookie/header checks on management mutations,
  including API-token requests; the authentication routes have separate handling
- **Secure headers on all responses:**
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains`
  - Content Security Policy restricts sources to the app, with allowances for inline styles and data images
- **Input validation:** Resources have validation for their supported inputs, but
  validation coverage is not fully qualified across every API. Stream listener
  collisions with known managed listeners are rejected before saving.

### Encrypted storage

Stored SSH credentials and configuration-history snapshot contents use
AES-256-GCM with a key derived from the stable JWT secret. Custom certificate
private keys are encrypted in immutable source files. Caddy reads separate
runtime copies through the restricted certificate group. Retain the original
controller secret with offline backups; an empty or unreadable persisted secret
stops startup instead of silently replacing the encryption key. DNS-provider
integration is not implemented.

### Management UI behind a reverse proxy

Set `POSTERNPROXY_TRUSTED_PROXIES` to the comma-separated IP CIDRs of the proxies
that forward requests to the controller. Use only networks or addresses you
control, and configure each trusted proxy to append its immediate peer to
`X-Forwarded-For`. The controller accepts that header only from a trusted peer
and walks the chain from right to left to identify the client for rate limits
and logs. With the setting empty, the controller uses the direct peer address.

### Visitor identity for proxied websites

Management proxy trust does not configure Caddy's public listeners. In the
controller's `/etc/posternproxy/config.env`, configure a separate JSON policy
keyed by the managed **server ID**. For example, with a proxy at `192.0.2.10`
in front of local server 1 and a proxy at `2001:db8::10` in front of server 4:

```ini
POSTERNPROXY_CADDY_TRUSTED_PROXIES={"1":["192.0.2.10/32"],"4":["2001:db8::10/128"]}
```

These documentation addresses must be replaced with your actual proxy addresses.
Restart the controller after editing. It applies the selected policy locally or
pushes it over the existing outbound agent connection. Omitted server IDs trust
no proxies. Neither private networks nor other servers inherit trust. Invalid
CIDRs, duplicate IDs and all-address `/0` ranges reject startup.

Each trusted proxy must append the verified immediate peer to
`X-Forwarded-For`, or replace incoming values with its authenticated client
address. Caddy walks the chain from right to left, skipping trusted hops and
invalid tokens, and uses the first valid untrusted address. Untrusted socket
peers cannot supply visitor identity. Host IP rules and rate limits use that
identity; upstream `X-Real-IP` and `X-Forwarded-For` are replaced with the single
verified address, including location routes. Caddy access logging, when enabled
by an operator, retains separate `remote_ip` and `client_ip` fields. The beta
does not expose an access-log viewer or enable per-host access logs by default.

Ranges are static and never fetched automatically. Review proxy-provider range
changes before updating the policy and any origin firewall. A failed download
must leave the last reviewed list in place; never fall back to trusting everyone.
Use the [Caddy trust model](https://caddyserver.com/docs/caddyfile/options#trusted_proxies)
when configuring intermediate proxies. This controls identity, not origin access.

[NTLM authentication passthrough]({{< relref "/proxy-hosts" >}}#ntlm-authentication-passthrough)
does not use these forwarded headers as the connection identity. It follows the
immediate client connection. An intermediary that shares one connection across
users can break per-user isolation. Leave the control off behind that kind of
proxy. The control is legacy, and IIS behavior is not verified.

### HTTP resource safeguards

Generated Caddy listeners allow at most 1 MiB of request headers, allow 10 seconds
to read headers and close idle keepalive connections after two minutes. Upstream
connections retain Caddy's three-second dial timeout; HTTPS upstreams have a
ten-second handshake timeout and certificate verification remains enabled.
There is no blanket website request-body limit or whole-response deadline:
large uploads, SSE, WebSockets and streamed responses remain supported. Choose
application-specific upload/concurrency limits at the upstream or an explicitly
reviewed advanced configuration. These defaults do not provide volumetric DDoS
protection; use an upstream provider when that protection is needed.
Management certificate multipart uploads are capped at 5 MiB in total, with a
separate 2 MiB limit for each PEM certificate/key input.

### Restrict administration and origin access

Keep Caddy's unauthenticated admin API on loopback port 2019. Keep the management
port private using a host/cloud firewall, VPN or SSH tunnel; do not expose port
81 merely because proxy traffic uses public ports 80/443. For a local-only UI,
set `POSTERNPROXY_LISTEN=127.0.0.1:81` and access it through an authenticated TLS
proxy or an SSH tunnel. Remote agents still need an explicitly reachable,
TLS-protected controller endpoint. Retain application authentication and CSRF.

Cloudflare-origin deployment is **not yet an end-to-end qualified beta path**.
Correct forwarding headers alone do not prevent direct-origin bypass. If you
evaluate it, use verified origin TLS and enforce origin access; prefer a Tunnel
or customer-specific authenticated-origin credentials. A Cloudflare network IP
allowlist alone does not identify your account. Follow
[Cloudflare's origin guidance](https://developers.cloudflare.com/fundamentals/security/protect-your-origin-server/)
and validate direct-origin rejection before relying on the deployment.

## OS-level hardening

### Systemd sandboxing

The controller service runs with:

```ini
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=/var/lib/posternproxy /var/log/posternproxy
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
SystemCallFilter=@system-service
MemoryDenyWriteExecute=true
RestrictRealtime=true
RestrictSUIDSGID=true
PrivateDevices=true
```

Caddy has a separate sandbox with write access to `/var/lib/caddy` and only
`CAP_NET_BIND_SERVICE`. The controller also receives that capability for its
default management port 81; it does not receive network-administration capabilities.

Verify the hardening score:

```bash
systemd-analyze security posternproxy
systemd-analyze security caddy
```

A score below 5.0 is considered well-hardened.

The score is a systemd heuristic, not a release-security acceptance result. The
settings here describe the native installers. Historical installations can have
different units; use the [provisioning guide]({{< relref "/multi-server/provisioning" >}})
when upgrading agents.

### Dedicated system users

| User | Shell | Home | Purpose |
|---|---|---|---|
| `caddy` | `/usr/sbin/nologin` | none | Runs Caddy |
| `posternproxy` | `/usr/sbin/nologin` | none | Runs PosternProxy |

Neither user can log in interactively.

### File permissions

| Path | Owner | Mode | Contents |
|---|---|---|---|
| `/etc/posternproxy/config.env` | `posternproxy` | `0600` | JWT secret, credentials |
| `/var/lib/posternproxy/posternproxy.db` | `posternproxy` | `0600` | SQLite database |
| `/var/lib/posternproxy/certs/` | `posternproxy` | `0700` | Certificate files |

### UFW firewall

The installer leaves UFW unchanged unless called with
`--configure-firewall --ssh-port <actual-port>`. When requested, it adds rules
and enables UFW without resetting rules or changing default policies. It never
changes SSH authentication or root-login settings. Without this option, manage
the host firewall yourself. Cloud firewalls also need the required ports opened.

```bash
ufw status verbose
```

Expected output:
```
To                         Action      From
--                         ------      ----
80/tcp                     ALLOW IN    Anywhere
443/tcp                    ALLOW IN    Anywhere
443/udp                    ALLOW IN    Anywhere (HTTP/3)
81/tcp                     ALLOW IN    Anywhere
<SSH port>/tcp             ALLOW IN    Anywhere
```

Other inbound traffic follows your existing default policy and rules.

### fail2ban

The installer creates a PosternProxy jail that bans IPs after 5 failed login attempts in 60 seconds for 1 hour. Existing jail customizations are preserved.

```bash
# Check jail status
fail2ban-client status posternproxy

# Unban an IP manually (replace the example address)
fail2ban-client set posternproxy unbanip 198.51.100.24
```

### sysctl hardening

```bash
sysctl net.ipv4.conf.all.rp_filter      # should be 1
sysctl net.ipv4.tcp_syncookies           # should be 1
sysctl net.ipv4.conf.all.accept_redirects # should be 0
sysctl net.ipv6.conf.all.accept_redirects # should be 0
```

### Unattended security upgrades

```bash
systemctl status unattended-upgrades
```

Security patches are applied automatically. The installer disables automatic
reboots in `/etc/apt/apt.conf.d/50unattended-upgrades-posternproxy`; existing
customizations to that file are preserved.

## Caddy admin API

The Caddy admin API is bound to `localhost:2019` only and is not accessible from the network. Verify:

```bash
ss -tlnp | grep 2019
# Should show: 127.0.0.1:2019
```

## Recommended additional hardening

- Restrict port 81 (management UI) to your office IP ranges via UFW:
  ```bash
  ufw delete allow 81/tcp
  ufw allow from 203.0.113.0/24 to any port 81
  ```
- Enable two-factor authentication at the reverse proxy level (use another PosternProxy instance in front, or add Caddy `forward_auth` — see the roadmap)
- Keep the JWT/encryption secret stable and backed up. Replacing it also makes
  stored SSH credentials and encrypted history unreadable; rotation requires a
  deliberate re-encryption/recovery procedure, not just an environment edit.
