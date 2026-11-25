---
title: Quick Start
weight: 2
---

# Quick Start

This guide walks you through the most common first-day tasks after installation.

## 1. Log in and change the default password

Open `http://<server-ip>:81` in your browser.

<!-- TODO: Screenshot — login page -->

For a native installation, use the initial administrator login printed by
`posternproxy install` after startup succeeds. The installer does not put that
password in the service log. Reinstalling preserves existing credentials.

For Docker, the first-boot banner is available through `docker logs posternproxy`.

Log in with:
- **Email:** `admin@posternproxy.local` (or the value of `POSTERNPROXY_ADMIN_EMAIL`)
- **Password:** the initial password displayed by the installer (or container startup banner)

Immediately navigate to **Users → (your account) → Change Password** and set a strong password.

{{< hint info >}}
**Can't find the password?** Run `reset-password` on the server (as the `posternproxy` user, with the config loaded) to generate and print a new one without restarting the service:
`sudo -u posternproxy bash -c 'set -a; . /etc/posternproxy/config.env; set +a; /usr/local/bin/posternproxy reset-password'`
{{< /hint >}}

## 2. Add your first proxy host

Proxy hosts route incoming domain traffic to upstream services.

1. Click **Proxy Hosts** in the sidebar
2. Click **+ Add Proxy Host**
3. Fill in the **Details** tab:
   - **Domain Names** — type `app.example.com` and press Enter
   - **Forward Scheme** — `http` or `https` depending on your upstream
   - **Forward Hostname / IP** — the internal address of your service (e.g. `192.168.1.10`)
   - **Port** — the upstream port (e.g. `3000`)
4. (Optional) Click the **SSL** tab:
   - Leave **Force HTTPS** off to serve the host over plain HTTP immediately — useful for testing or internal domains with no public certificate.
   - Turn **Force HTTPS** on to have Caddy obtain a Let's Encrypt certificate automatically (needs public DNS + port 80), or assign an uploaded certificate.
5. Click **Create**

<!-- TODO: Screenshot — Add Proxy Host modal, Details tab filled in -->

After saving, check the configuration notice for **Applied** status, then verify
traffic. A saved host can remain queued for an offline agent or fail to apply.

## 3. Verify traffic is flowing

```bash
# Works right away when Force HTTPS is off:
curl -I http://app.example.com
# Once a certificate is provisioned (Force HTTPS on, DNS pointed):
curl -I https://app.example.com
```

You should see a `200 OK` (or whatever your upstream returns).

## 4. Set up a certificate

Public domains with Force HTTPS get a Let's Encrypt certificate automatically — no
action needed here. For a wildcard or a private/internal domain, go to
**Custom Certs → Upload Certificate** and upload your own certificate and key, then
select it on the host's SSL tab.

See [Certificates]({{< relref "/certificates" >}}) for full details.

## 5. Restrict access with an access list

To protect a proxy host with IP allowlisting:

1. Go to **Access Lists → Add Access List**
2. Add client IP rules (CIDR ranges) with `allow` / `deny` directives
3. Edit your proxy host and select the access list on the **Security** tab

<!-- TODO: Screenshot — Access List form with client IP rules -->

## 6. Enable the API for automation

To use the REST API from scripts or CI/CD:

1. Go to **Access Tokens → Create Token**
2. Copy the token — it is only shown once
3. Use it as a Bearer token:

```bash
curl -H "Authorization: Bearer <token>" http://your-server:81/api/proxy-hosts
```

See [Access Tokens]({{< relref "/access-tokens" >}}) for token permissions, routes,
and the CSRF cookie/header required for write requests.
