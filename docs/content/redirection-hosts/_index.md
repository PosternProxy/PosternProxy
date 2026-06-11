---
title: Redirection Hosts
weight: 30
---

# Redirection Hosts

Redirection hosts issue HTTP redirects from one domain (or path) to another. They are useful for:

- Redirecting `www.example.com` → `example.com` (or vice versa)
- Redirecting an old domain to a new one after a migration
- Sending a vanity URL to a longer canonical URL

## Creating a redirection host

1. Click **Redirection Hosts** in the sidebar
2. Click **+ Add Redirection Host**
3. Fill in the form:

| Field | Description |
|---|---|
| **Domain Names** | The incoming domain(s) to redirect from (press Enter after each) |
| **Forward Scheme** | `http` or `https` — the scheme of the destination URL |
| **Forward Domain Name** | The destination domain |
| **HTTP Code** | The redirect status code (301, 302, 307, or 308) |
| **Preserve Path** | If enabled, appends the request path to the destination |
| **SSL Certificate** | Optional — select a certificate if you want to redirect HTTPS → HTTPS |

4. Click **Create**

<!-- TODO: Screenshot — Add Redirection Host modal -->

## Status codes

| Code | Type | Description |
|---|---|---|
| **301** | Moved Permanently | Browser caches the redirect. Use for permanent domain changes. |
| **302** | Found (Temporary) | Browser does not cache. Use for temporary redirects. |
| **307** | Temporary Redirect | Like 302, but preserves the HTTP method (POST stays POST). |
| **308** | Permanent Redirect | Like 301, but preserves the HTTP method. |

For permanent migrations, use **301**. For A/B testing or temporary maintenance pages, use **302**.

## Preserve Path

When **Preserve Path** is enabled, the original request path is appended to the destination:

```
https://old.example.com/blog/post-1
          ↓  (301, Forward Domain: new.example.com, Preserve Path: on)
https://new.example.com/blog/post-1
```

When disabled, all requests to the source domain redirect to the bare destination domain.

## SSL on redirecting domains

If you want to redirect `https://old.example.com` (rather than just `http://`), you need a certificate for `old.example.com`. Either:

- Let Caddy obtain one automatically (the domain must be publicly resolvable to this server)
- Upload a custom certificate on the [Certificates]({{< relref "/certificates" >}}) page and select it

## Enable / disable

Redirection hosts can be toggled without deleting them. Disabled hosts are removed from Caddy's routing table immediately.
