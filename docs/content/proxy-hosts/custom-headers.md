---
title: Custom Headers
weight: 7
---

# Custom Headers

PosternProxy can inject, modify, or remove HTTP headers on requests sent to your upstream and on responses returned to clients. Header rules are configured on the **Headers** tab.

## Adding a rule

Each rule has four fields:

| Field | Options | Description |
|---|---|---|
| **Direction** | Request / Response | Whether the rule applies to the upstream request or the downstream response |
| **Operation** | Set / Add / Delete | What to do with the header |
| **Name** | Any string | The header name (case-insensitive) |
| **Value** | Any string | The header value (not used for Delete) |

Click **+ Add Rule** to insert a new row. Rows with an empty Name are ignored on save.

<!-- TODO: Screenshot — Headers tab with three rules (Set X-Real-IP on request, Set X-Frame-Options on response, Delete Server on response) -->

## Operations

| Operation | Behaviour |
|---|---|
| **Set** | Creates the header if absent; replaces it if already present |
| **Add** | Appends the value (creates a new header instance — does not replace existing values) |
| **Delete** | Removes all instances of the header |

## Presets

The Headers tab includes a **Presets** panel with six common rules you can add with a single click:

| Preset | Rule |
|---|---|
| **X-Real-IP** | Set `X-Real-IP: {remote_host}` on request |
| **X-Forwarded-Proto** | Set `X-Forwarded-Proto: {scheme}` on request |
| **Remove Server header** | Delete `Server` from response |
| **X-Frame-Options** | Set `X-Frame-Options: SAMEORIGIN` on response |
| **X-Content-Type-Options** | Set `X-Content-Type-Options: nosniff` on response |
| **Strict-Transport-Security** | Set `Strict-Transport-Security: max-age=31536000` on response |

## How it works

Custom headers are implemented as a standalone Caddy `headers` handler placed before the `reverse_proxy` handler in the route chain:

- **Request headers** are modified before the request is forwarded upstream.
- **Response headers** are modified after the upstream response is received, using Caddy's `Deferred` mode so that the response body is not buffered.

## Dynamic values

Caddy supports [placeholder variables](https://caddyserver.com/docs/conventions#placeholders) in header values. You can use these in the **Value** field:

| Placeholder | Resolves to |
|---|---|
| `{remote_host}` | Client IP address |
| `{scheme}` | `http` or `https` |
| `{host}` | Request host header |
| `{method}` | HTTP method |
| `{uri}` | Full request URI |

## Notes

- Rules are applied in the order they appear. A `Delete` followed by a `Set` will result in the header being present with the new value.
- Response header rules are applied after the upstream responds. They cannot prevent upstream from sending a header; they can only modify the copy that reaches the client.
- Header names are case-insensitive per HTTP/1.1 spec; Caddy normalises them to canonical form.
