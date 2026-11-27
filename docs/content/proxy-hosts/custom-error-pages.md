---
title: Custom Error Pages
weight: 6
---

# Custom Error Pages

PosternProxy can serve custom HTML for errors raised within Caddy's HTTP
handling. The UI exposes 502 and 503 pages. Ordinary error-status
responses returned successfully by an upstream are not intercepted by this
server-level error handler.

## Configuration

Open the **Errors** tab of the Add/Edit Proxy Host modal. It has two panels:

| Code | Meaning | Common cause |
|---|---|---|
| 502 | Bad Gateway | Upstream is down or returned an invalid response |
| 503 | Service Unavailable | Upstream is overloaded or in maintenance mode |

Paste HTML into the relevant textarea. Blank fields use PosternProxy's
bundled error pages. An upstream's ordinary 404 response passes through unchanged.

The bundled pages display the PosternProxy logo and automatically follow the
visitor's system light or dark theme, including preference changes while the page
is open. Styles and both logo variants are embedded in the response; no JavaScript
or connection to the controller is required. Custom HTML controls its own branding
and theme behavior.

<!-- TODO: Screenshot — Errors tab with a 502 page containing HTML -->

## Example 502 page

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Service Unavailable</title>
  <style>
    body { font-family: sans-serif; text-align: center; padding: 4rem; }
    h1 { font-size: 3rem; color: #e53e3e; }
  </style>
</head>
<body>
  <h1>502</h1>
  <p>This service is temporarily unavailable. Please try again in a few moments.</p>
</body>
</html>
```

## How it works

PosternProxy generates a Caddy `handle_errors` block at the HTTP server level. Each error page becomes a route that matches on both:

1. The **host** (domain name of the proxy host)
2. The **error status code**

This ensures error pages are scoped per proxy host — a 502 page on `app.example.com` will not affect `api.example.com`.

## Notes

- [NTLM authentication passthrough]({{< relref "/proxy-hosts" >}}#ntlm-authentication-passthrough) rejects a custom 401 page, including one supplied through the API, a backup, or a history snapshot. This tab edits 502 and 503 pages.
- Error pages are served with the original error status code (Caddy does not change it to 200).
- The `Content-Type` header is set to `text/html; charset=utf-8`.
- Error pages can include inline CSS and JavaScript. External resources (fonts, scripts) will load only if the client has network access to them, which may not be true if the error is caused by a network partition.
- There is no file upload — the HTML is stored directly in the database. For large or complex error pages, consider hosting a static file externally and using a redirect.
