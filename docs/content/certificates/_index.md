---
title: Certificates
weight: 60
---

# Certificates

PosternProxy handles TLS in two ways: **automatic** certificates managed by Caddy,
and **custom** certificates you upload. The Certificates page manages the custom
certificates that proxy and redirection hosts can then reference.

## Automatic (Let's Encrypt HTTP-01)

For most public hosts you do not create a certificate here at all. Turn on
**Force HTTPS** for a proxy host (or assign a custom certificate) and Caddy
obtains and renews a Let's Encrypt certificate automatically using the HTTP-01
challenge.

**Requirements:**
- The domain must have a public DNS A/AAAA record pointing to this server
- Port 80 must be reachable from the internet for the ACME challenge

Automatic certificates are managed entirely by Caddy. They do **not** appear on
the Certificates page. If provisioning can't complete yet (DNS not pointed, port
80 blocked), Caddy keeps retrying on its own; the Proxy Hosts list shows the live
TLS status (OK / pending / failed) for each HTTPS host, and flips to OK on its own
once a certificate is obtained.

If a host does not have Force HTTPS on and no certificate assigned, it is served
over plain HTTP and no certificate is requested.

## Custom (upload your own)

Upload your own certificate and private key when you have:
- A certificate from a private/internal CA
- A wildcard certificate obtained outside of PosternProxy
- Short-lived certificates managed by your own tooling

### Adding a custom certificate

1. Go to **Certificates → Add Certificate**
2. Upload your `.crt` (or `.pem`) file and `.key` file
3. Click **Create**

Then select the certificate on a proxy or redirection host's **SSL** tab.

## Certificate list

The Certificates page shows uploaded custom certificates with:

| Column | Description |
|---|---|
| Name | Display name (from the first domain) |
| Domains | Subject alternative names |
| Expires | Expiry date (highlighted when ≤ 30 days away) |
| Status | `pending` until the files are processed, then `active` |

## Expiry alerts

The Dashboard shows certificates expiring within the next 30 days. See
[Dashboard]({{< relref "/dashboard" >}}). Custom certificates do not auto-renew —
upload a replacement before expiry.

## Not yet available

The following are planned but not implemented:

- **Let's Encrypt DNS-01 challenge** (wildcard / private domains via a DNS
  provider API). Today, wildcard or non-public domains require a custom upload.
- **Per-certificate renew / download** buttons.

## Notes

- Certificate private keys are stored on disk at `POSTERNPROXY_CERT_DIR` (default
  `/var/lib/posternproxy/certs`), owned by the `posternproxy` user with mode `0600`.
- Automatic (HTTP-01) certificates are stored in Caddy's own certificate store and
  do not appear on this page.
