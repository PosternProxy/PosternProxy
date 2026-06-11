---
title: Security Hardening
weight: 1
---

# Security Hardening

The PosternProxy install script applies a comprehensive set of security hardening measures. This page documents what is applied and how to verify it.

## Application-level hardening

### Authentication

- JWT access tokens with 15-minute TTL
- Refresh tokens with 7-day TTL stored in HttpOnly / Secure / SameSite=Strict cookies
- bcrypt password hashing at cost factor 12
- Login rate limit: 5 attempts/min per IP (enforced by the API and by fail2ban)
- Account lockout is not time-based — use fail2ban to block IPs

### API security

- **Rate limiting:** 120 requests/min per IP on authenticated management API routes; 5 login attempts/min per IP; 30 agent WebSocket connection attempts/min per IP
- **CSRF protection:** Double-submit cookie pattern on all state-changing requests
- **Secure headers on all responses:**
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains`
  - `Content-Security-Policy: default-src 'self'`
- **Input validation:** All user input validated at the handler layer; domain names validated per RFC 1123; ports validated 1–65535; CIDRs parsed and validated; all database queries use parameterized statements (no SQL interpolation)

### Encrypted storage

DNS provider credentials and SSH credentials are encrypted at rest using AES-256-GCM with a key derived from the JWT secret.

## OS-level hardening

### Systemd sandboxing

Both `caddy` and `posternproxy` services run with:

```ini
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=/var/lib/posternproxy /etc/posternproxy/certs
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
SystemCallFilter=@system-service
MemoryDenyWriteExecute=true
RestrictRealtime=true
RestrictSUIDSGID=true
PrivateDevices=true
```

Verify the hardening score:

```bash
systemd-analyze security posternproxy
systemd-analyze security caddy
```

A score below 5.0 is considered well-hardened.

### Dedicated system users

| User | Shell | Home | Purpose |
|---|---|---|---|
| `caddy` | `/usr/sbin/nologin` | none | Runs Caddy |
| `posternproxy` | `/usr/sbin/nologin` | none | Runs PosternProxy |

Neither user can log in interactively.

### File permissions

| Path | Owner | Mode | Contents |
|---|---|---|---|
| `/etc/posternproxy/config.env` | `root:posternproxy` | `0640` | JWT secret, credentials |
| `/var/lib/posternproxy/posternproxy.db` | `posternproxy` | `0600` | SQLite database |
| `/var/lib/posternproxy/certs/` | `posternproxy` | `0700` | Certificate files |

### UFW firewall

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

All other inbound ports are blocked by the default deny policy.

### fail2ban

The install script creates a PosternProxy jail that bans IPs after 5 failed login attempts in 10 minutes for 1 hour.

```bash
# Check jail status
fail2ban-client status posternproxy

# Unban an IP manually
fail2ban-client set posternproxy unbanip <ip>
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

Security patches are applied automatically. The system reboots automatically if a kernel update requires it (configurable in `/etc/apt/apt.conf.d/50unattended-upgrades`).

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
- Rotate the JWT secret annually: update `POSTERNPROXY_JWT_SECRET` in `/etc/posternproxy/config.env` and restart. This invalidates all active sessions.
