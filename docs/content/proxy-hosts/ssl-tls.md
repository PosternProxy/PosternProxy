---
title: SSL / TLS
weight: 3
---

# SSL / TLS

PosternProxy leverages Caddy's automatic HTTPS to handle TLS with minimal configuration. The **SSL** tab controls certificate selection and security options.

## Automatic Let's Encrypt (default)

Leave the **SSL Certificate** field set to **None / Caddy automatic**. Caddy will:

1. Obtain a certificate from Let's Encrypt the first time a request arrives for the domain
2. Store the certificate in its managed certificate store
3. Renew automatically before expiry

**Requirements for automatic certificates:**
- The domain must resolve publicly to the server's IP address
- Port 80 must be reachable for the ACME HTTP-01 challenge

## Custom certificates

If you have your own certificate (e.g. a wildcard or internal CA certificate), upload it first on the [Certificates]({{< relref "/certificates" >}}) page, then select it from the **SSL Certificate** dropdown on the SSL tab.

<!-- TODO: Screenshot — SSL tab with a custom certificate selected -->

## Force HTTPS

When **Force HTTPS** is enabled, PosternProxy adds a Caddy route that issues a 301 redirect from `http://` to `https://` for all requests to this domain.

{{< hint info >}}
**Caddy default behavior**  
Even without Force HTTPS, Caddy will serve HTTPS if it can obtain a certificate. Force HTTPS ensures the HTTP→HTTPS redirect is active even for clients that explicitly use `http://`.
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

HTTP/2 is enabled by default in Caddy. The **HTTP/2 Support** toggle in PosternProxy corresponds to Caddy's protocol negotiation. Leave it enabled unless your upstream specifically cannot handle HTTP/2 upstream connections.

## TLS Passthrough

For services that must handle TLS themselves (e.g. databases with mutual TLS, or services that require client certificates), use [TLS Passthrough]({{< relref "/proxy-hosts/tls-passthrough" >}}) mode instead of a regular proxy host.

## SSL tab reference

| Field | Description |
|---|---|
| SSL Certificate | Select a managed or custom certificate, or leave blank for auto |
| Force HTTPS | Issue 301 redirect for HTTP requests |
| HSTS Enabled | Add `Strict-Transport-Security` header |
| Include Subdomains | Add `includeSubDomains` to HSTS header |
| HTTP/2 Support | Enable HTTP/2 protocol negotiation |
