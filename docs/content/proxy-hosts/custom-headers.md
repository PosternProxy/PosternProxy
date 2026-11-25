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

Use valid HTTP header names and values. Invalid operations can be rejected when
Caddy applies the generated configuration.

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
| **Hide Server header** | Delete `Server` from response |
| **Hide X-Powered-By** | Delete `X-Powered-By` from response |
| **CORS Allow-Origin \*** | Set `Access-Control-Allow-Origin: *` on response |
| **X-Frame-Options DENY** | Set `X-Frame-Options: DENY` on response |
| **Cache-Control no-store** | Set `Cache-Control: no-store` on response |
| **X-Request-ID passthrough** | Set `X-Request-ID: {http.request.uuid}` on request |

## How it works

Custom headers are implemented as a standalone Caddy `headers` handler placed before the `reverse_proxy` handler in the route chain:

- **Request headers** are modified before the request is forwarded upstream.
- **Response headers** are modified after the upstream response is received, using Caddy's `Deferred` mode so that the response body is not buffered.

## Dynamic values

Caddy supports [placeholder variables](https://caddyserver.com/docs/conventions#placeholders) in header values. You can use these in the **Value** field:

| Placeholder | Resolves to |
|---|---|
| `{http.request.remote.host}` | Immediate client connection IP address |
| `{http.request.scheme}` | `http` or `https` |
| `{http.request.host}` | Request host |
| `{http.request.method}` | HTTP method |
| `{http.request.uri}` | Full request URI |

## Notes

- Rules are grouped into Caddy set/add/delete operations. Avoid contradictory
  operations on the same name; row order is not a general ordering guarantee.
- Response header rules are applied after the upstream responds. They cannot prevent upstream from sending a header; they can only modify the copy that reaches the client.
- Header names are case-insensitive per HTTP/1.1 spec; Caddy normalises them to canonical form.
