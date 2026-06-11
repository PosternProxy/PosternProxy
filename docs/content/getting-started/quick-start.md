---
title: Quick Start
weight: 2
---

# Quick Start

This guide walks you through the most common first-day tasks after installation.

## 1. Log in and change the default password

Open `http://<server-ip>:81` in your browser.

<!-- TODO: Screenshot — login page -->

On first boot, PosternProxy generates a random admin password and prints it to the service logs. Retrieve it before logging in:

```bash
journalctl -u posternproxy --no-pager | grep -A5 "INITIAL SETUP"
```

Log in with:
- **Email:** `admin@posternproxy.local` (or the value of `POSTERNPROXY_ADMIN_EMAIL`)
- **Password:** the generated password from the startup banner above

Immediately navigate to **Users → (your account) → Change Password** and set a strong password.

{{< hint info >}}
**Can't find the password?** Run `posternproxy reset-password` on the server to generate and print a new one without restarting the service.
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
4. Click **SSL** tab and select a certificate (or leave blank for Caddy automatic Let's Encrypt)
5. Click **Create**

<!-- TODO: Screenshot — Add Proxy Host modal, Details tab filled in -->

Caddy will immediately start routing traffic for your domain.

## 3. Verify traffic is flowing

```bash
curl -I https://app.example.com
```

You should see a `200 OK` (or whatever your upstream returns) along with Caddy-managed TLS.

## 4. Set up a certificate

For domains Caddy cannot automatically obtain a Let's Encrypt certificate (e.g. wildcard domains), go to **Certificates → Add Certificate** and either:

- Choose **Let's Encrypt DNS challenge** and enter your DNS provider credentials
- Choose **Custom** and upload your own certificate and key files

See [Certificates]({{< relref "/certificates" >}}) for full details.

## 5. Restrict access with an access list

To protect a proxy host with IP allowlisting or HTTP Basic Auth:

1. Go to **Access Lists → Add Access List**
2. Add client IP rules (CIDR ranges) or authentication credentials
3. Edit your proxy host and select the access list on the **SSL** tab

<!-- TODO: Screenshot — Access List form with client IP rules -->

## 6. Enable the API for automation

To use the REST API from scripts or CI/CD:

1. Go to **Access Tokens → Create Token**
2. Copy the token — it is only shown once
3. Use it as a Bearer token:

```bash
curl -H "Authorization: Bearer <token>" http://your-server:81/api/proxy-hosts
```

See [Access Tokens]({{< relref "/access-tokens" >}}) for the full API reference approach.
