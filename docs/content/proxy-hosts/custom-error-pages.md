---
title: Custom Error Pages
weight: 6
---

# Custom Error Pages

PosternProxy can serve custom HTML pages when a proxy host returns a 404, 502, or 503 status code. This replaces Caddy's default plain-text error responses with your own branded or user-friendly HTML.

## Configuration

Open the **Errors** tab of the Add/Edit Proxy Host modal. You will see three panels, one for each supported status code:

| Code | Meaning | Common cause |
|---|---|---|
| 404 | Not Found | Route exists but upstream returned 404; or path not matched |
| 502 | Bad Gateway | Upstream is down or returned an invalid response |
| 503 | Service Unavailable | Upstream is overloaded or in maintenance mode |

Paste your HTML into the relevant textarea. Leave a field blank to use Caddy's default behaviour for that code.

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

- Error pages are served with the original error status code (Caddy does not change it to 200).
- The `Content-Type` header is set to `text/html; charset=utf-8`.
- Error pages can include inline CSS and JavaScript. External resources (fonts, scripts) will load only if the client has network access to them, which may not be true if the error is caused by a network partition.
- There is no file upload — the HTML is stored directly in the database. For large or complex error pages, consider hosting a static file externally and using a redirect.
