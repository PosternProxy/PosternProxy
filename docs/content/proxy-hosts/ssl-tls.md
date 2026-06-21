---
title: SSL / TLS
weight: 3
---

# SSL / TLS

PosternProxy leverages Caddy's automatic HTTPS to handle TLS with minimal configuration. The **SSL** tab controls certificate selection and security options.

## How a host is served

Whether a proxy host is reachable over HTTP, HTTPS, or both depends on the
**Force HTTPS** toggle and the selected certificate:

| Force HTTPS | Certificate | Result |
|---|---|---|
| Off (default) | None | Served over **plain HTTP** on port 80 immediately. No Let's Encrypt certificate is requested, so the host works even for internal domains or before DNS is pointed. |
| Off | Custom/managed cert | Served over **both** HTTP (port 80) and HTTPS (port 443). |
| On | None | Caddy obtains a **Let's Encrypt** certificate and redirects HTTP → HTTPS. |
| On | Custom/managed cert | Uses the supplied certificate and redirects HTTP → HTTPS. |

This means a newly added host works right away over HTTP; turn on Force HTTPS (or
assign a certificate) when you are ready to serve it over TLS.

## Automatic Let's Encrypt

Enable **Force HTTPS** (or assign a certificate) to have Caddy obtain and renew a
Let's Encrypt certificate automatically. Caddy will:

1. Obtain a certificate from Let's Encrypt the first time a request arrives for the domain
2. Store the certificate in its managed certificate store
3. Renew automatically before expiry

**Requirements for automatic certificates:**
- The domain must resolve publicly to the server's IP address
- Port 80 must be reachable for the ACME HTTP-01 challenge

If these requirements are not met, leave Force HTTPS off (the host stays on HTTP)
or upload a custom certificate instead.

## Custom certificates

If you have your own certificate (e.g. a wildcard or internal CA certificate), upload it first on the [Certificates]({{< relref "/certificates" >}}) page, then select it from the **SSL Certificate** dropdown on the SSL tab.

<!-- TODO: Screenshot — SSL tab with a custom certificate selected -->

## Force HTTPS

When **Force HTTPS** is enabled, the host is served on HTTPS (port 443) and all
plain-HTTP requests receive a 301 redirect to `https://`. Caddy will obtain a
Let's Encrypt certificate automatically if none is assigned.

{{< hint info >}}
**Without Force HTTPS**  
The host is served over plain HTTP and is *not* redirected. Caddy only requests a
Let's Encrypt certificate when Force HTTPS is on or a certificate is assigned, so
a host with Force HTTPS off will never fail on ACME for a non-public domain.
{{< /hint >}}

## HSTS

HTTP Strict Transport Security instructs browsers to only connect via HTTPS for a period of time.

| Setting | Header effect |
|---|---|
| **HSTS Enabled** | Adds `Strict-Transport-Security: max-age=31536000` |
| **Include Subdomains** | Appends `; includeSubDomains` |

{{< hint warning >}}
**Enable HSTS carefully**  
Once a browser receives an HSTS header, it will refuse HTTP connections for the `max-age` period (1 year by default). Do not enable HSTS unless you are certain HTTPS will remain available for the domain.
{{< /hint >}}

## HTTP/2

HTTP/2 is enabled by default. It is a **server-wide** setting (the HTTPS listener
serves all hosts), not a per-host option — toggle it per server on the **Servers**
page. Turning it off restricts that server's HTTPS listener to HTTP/1.1.

## TLS Passthrough

For services that must handle TLS themselves (e.g. databases with mutual TLS, or services that require client certificates), use [TLS Passthrough]({{< relref "/proxy-hosts/tls-passthrough" >}}) mode instead of a regular proxy host.

## SSL tab reference

| Field | Description |
|---|---|
| SSL Certificate | Select an uploaded custom certificate, or leave blank (Let's Encrypt is used automatically when Force HTTPS is on; otherwise the host stays on HTTP) |
| Force HTTPS | Issue 301 redirect for HTTP requests |
| HSTS Enabled | Add `Strict-Transport-Security` header |
| Include Subdomains | Add `includeSubDomains` to HSTS header |

HTTP/2 is configured per server on the **Servers** page, not here.
