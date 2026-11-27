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

The separate **HTTP Only** option forces the host onto the plain-HTTP route and
excludes it from the normal HTTPS grouping. Check configuration application
status after changing any of these options.

## Automatic Let's Encrypt

Enable **Force HTTPS** without assigning a custom certificate to use automatic
issuance. When a custom certificate is assigned, Caddy loads that certificate
instead. For automatic certificates, Caddy will:

1. Manage certificate issuance for the configured public domains
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
Let's Encrypt certificate for this normal proxy-host path when Force HTTPS is on
and no custom certificate is supplied. HTTP-only hosts do not request one.
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

## TLS termination and NTLM

[NTLM authentication passthrough]({{< relref "/proxy-hosts" >}}#ntlm-authentication-passthrough)
terminates TLS when the host is served over HTTPS. That can conflict with IIS
Extended Protection. Do not turn off Extended Protection or certificate
verification to make the upstream connect. When the upstream must see the client
handshake, use [TLS passthrough]({{< relref "/proxy-hosts/tls-passthrough" >}}) and
leave the NTLM control off. IIS behavior of the NTLM control is not verified.

## HTTP/2

HTTP/2 is enabled by default. It is a **server-wide** setting (the HTTPS listener
serves all hosts), not a per-host option — toggle it per server on the **Servers**
page. Turning it off restricts that server's HTTPS listener to HTTP/1.1. The
NTLM transport still uses HTTP/1.1 on its upstream connection.

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
