---
title: Troubleshooting
weight: 3
---

# Troubleshooting

## Service not starting

### PosternProxy fails to start

```bash
journalctl -u posternproxy -n 50 --no-pager
```

Common causes:

| Symptom | Cause | Fix |
|---|---|---|
| `database is locked` | Another PosternProxy instance is running | `systemctl stop posternproxy; systemctl start posternproxy` |
| `migration failed` | Bad database state | Restore from backup |
| `address already in use :81` | Another process owns port 81 | `ss -tlnp \| grep 81` to find it |
| `config.env not found` | Missing config file | Re-run `scripts/install.sh` or create manually |

### Caddy fails to start

```bash
journalctl -u caddy -n 50 --no-pager
caddy validate --config /etc/caddy/Caddyfile  # if using Caddyfile fallback
```

Common causes:

| Symptom | Cause | Fix |
|---|---|---|
| `bind: permission denied` on port 80/443 | Missing `CAP_NET_BIND_SERVICE` | Check systemd unit `AmbientCapabilities` |
| `plugin not found: layer4` | Caddy built without L4 plugin | Replace Caddy using the [pinned build]({{< relref "/administration/release-validation" >}}) |
| `failed to obtain certificate` | DNS not propagated or port 80 blocked | Check DNS and UFW |

## UI and API errors

### "missing CSRF token" (403) when saving

You can log in and load pages, but any create/update/delete returns
`403 {"error":"missing CSRF token"}`.

**Cause:** cookies are flagged `Secure`, but you are accessing the UI over plain
HTTP. Browsers refuse to store or send `Secure` cookies over HTTP, so the
`csrf_token` cookie never reaches the server. Login still works because it uses
a bearer token, not a cookie.

**Fix:** either access the UI over HTTPS, or tell PosternProxy it is not behind
TLS so cookies are not marked `Secure`:

```bash
echo 'POSTERNPROXY_BEHIND_TLS_PROXY=false' >> /etc/posternproxy/config.env
systemctl restart posternproxy
```

Then clear the site's cookies (or hard-refresh) so a fresh, non-`Secure`
`csrf_token` is issued. Set the variable back to `true` once you put TLS in
front of the UI.

## Traffic not routing

### Proxy host created but getting connection refused

1. Check the proxy host is **enabled** in the UI
2. Verify Caddy is running: `systemctl status caddy`
3. Check Caddy's config includes the route:

   ```bash
   curl -s http://localhost:2019/config/apps/http/servers/https/routes | jq .
   ```

4. Verify the upstream is actually listening:

   ```bash
   curl -I http://<upstream-host>:<port>/
   ```

### Getting 502 Bad Gateway

The upstream is not reachable. Check:

1. The upstream service is running on the configured host and port
2. There is no firewall between PosternProxy and the upstream
3. If using a hostname, it resolves correctly: `dig <hostname>`

If health checks are configured, check the upstream health widget on the Dashboard.

### Getting wrong certificate / TLS error

1. Check which certificate is assigned to the proxy host (SSL tab), or whether Force HTTPS is on
2. If using automatic Let's Encrypt, verify the domain resolves to this server: `dig +short <domain>` and that port 80 is reachable from the internet
3. Check the live TLS status on the Proxy Hosts list (OK / pending / failed). Caddy retries provisioning automatically — once DNS/port 80 are correct it flips to OK on its own
4. Inspect Caddy's certificate activity: `journalctl -u caddy | grep -i tls`

## Management UI issues

### Can't log in

- Verify credentials: default email is `admin@posternproxy.local`; the password is randomly generated on first boot and printed to the service logs (`journalctl -u posternproxy | grep password`)
- Check fail2ban has not banned your IP: `fail2ban-client status posternproxy`
- Unban: `fail2ban-client set posternproxy unbanip <your-ip>`
- Check the service is running: `systemctl status posternproxy`

### UI shows stale data

Click the **↻** refresh button on the relevant page, or force a full page reload (Ctrl+Shift+R). PosternProxy does not push real-time updates to the frontend (except the provisioning terminal and dashboard health widget).

### Force Caddy sync

If you suspect Caddy's config has drifted from the database (e.g. after a manual `caddy reload`):

1. Go to **Settings → Force Caddy Sync**
2. Click the button — PosternProxy rebuilds and pushes the full config

Or via API:

```bash
curl -X POST -H "Authorization: Bearer <token>" \
     http://localhost:81/api/settings/caddy-sync
```

## Multi-server issues

### Agent shows as Offline

1. SSH into the remote server and check the agent:

   ```bash
   systemctl status posternproxy-agent
   journalctl -u posternproxy-agent -n 30
   ```

2. Verify the agent can reach the controller:

   ```bash
   curl http://<controller-ip>:81/api/auth/me
   ```

3. Check the agent token in `/etc/posternproxy-agent/config.env` matches what is registered on the controller (re-provision to regenerate)

### Config changes not reaching remote server

- Check agent is online on the Servers page
- If recently reconnected, a full sync is sent automatically — wait a few seconds
- Check the agent journal for errors: `journalctl -u posternproxy-agent -n 50`

## Log locations

| Component | Log command |
|---|---|
| PosternProxy | `journalctl -u posternproxy -f` |
| Caddy | `journalctl -u caddy -f` |
| Agent | `journalctl -u posternproxy-agent -f` |
| fail2ban | `journalctl -u fail2ban -f` |
| UFW | `/var/log/ufw.log` |

## Getting help

If the above steps do not resolve your issue, collect the following for a bug report:

```bash
# System info
uname -a
cat /etc/os-release

# Service status
systemctl status posternproxy caddy
journalctl -u posternproxy -n 100 --no-pager
journalctl -u caddy -n 100 --no-pager

# Caddy config (sanitise any secrets)
curl -s http://localhost:2019/config/ | jq .
```
