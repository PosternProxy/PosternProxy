---
title: Certificates
weight: 60
---

# Certificates

PosternProxy manages TLS certificates through Caddy. The Certificates page lets you add custom or DNS-challenge certificates that proxy hosts can then reference.

## Certificate types

### Automatic (Let's Encrypt HTTP-01)

For most proxy hosts, you do not need to create a certificate here. When you leave the SSL Certificate field blank on a proxy host, Caddy automatically obtains and renews a Let's Encrypt certificate using the HTTP-01 challenge.

**Requirements:**
- Domain must have a public DNS A/AAAA record pointing to this server
- Port 80 must be accessible from the internet

### DNS Challenge (wildcard and private domains)

For wildcard certificates (`*.example.com`) or domains that are not publicly accessible, use the DNS-01 challenge. This requires API credentials for your DNS provider.

Supported DNS providers include Cloudflare, Route 53, DigitalOcean, Namecheap, GoDaddy, and many others via the [lego](https://go-acme.github.io/lego/dns/) library that Caddy uses.

### Custom (upload your own)

Upload your own certificate and private key files. Use this for:
- Certificates from a private/internal CA
- Wildcard certificates obtained outside of PosternProxy
- Short-lived certificates managed by your own tooling

## Adding a DNS challenge certificate

1. Go to **Certificates → Add Certificate**
2. Select **Let's Encrypt (DNS Challenge)**
3. Fill in:

| Field | Description |
|---|---|
| **Domain Names** | Domains to include on the certificate (supports `*.example.com`) |
| **DNS Provider** | Your DNS registrar/host |
| **Credentials** | API key / secret for the DNS provider |
| **Email** | Contact email for Let's Encrypt notifications |

4. Click **Create** — PosternProxy will request the certificate and show its status

<!-- TODO: Screenshot — Add Certificate modal with DNS Challenge selected and Cloudflare credentials -->

## Adding a custom certificate

1. Go to **Certificates → Add Certificate**
2. Select **Custom**
3. Upload your `.crt` (or `.pem`) file and `.key` file
4. Click **Create**

<!-- TODO: Screenshot — Add Certificate modal with Custom selected and files uploaded -->

## Certificate list

The Certificates page shows all managed certificates with:

| Column | Description |
|---|---|
| Name | Display name (from the first domain) |
| Domains | Subject alternative names |
| Expires | Expiry date (highlighted when ≤ 30 days away) |
| Status | `active`, `pending`, or `error` |
| Provider | `auto`, `dns`, or `custom` |

## Renewing a certificate

Click the **↻ Renew** button on any certificate to trigger an immediate renewal attempt. Caddy normally handles renewals automatically 30 days before expiry.

## Downloading a certificate

Click **Download** to get a ZIP containing the certificate and private key. This is useful for deploying the same certificate to other services.

## Expiry alerts

The Dashboard shows certificates expiring within the next 30 days. See [Dashboard]({{< relref "/dashboard" >}}).

## Notes

- Certificate private keys are stored on disk at `POSTERNPROXY_CERT_DIR` (default `/var/lib/posternproxy/certs`), owned by the `posternproxy` user with mode `0600`.
- DNS provider credentials are encrypted at rest in the SQLite database using AES-256-GCM.
- Auto (HTTP-01) certificates are managed entirely by Caddy and do not appear on this page.
