---
title: Rate Limiting
weight: 4
---

# Rate Limiting

PosternProxy can cap the number of requests a single IP address can make to a proxy host within a sliding time window. Rate limiting is configured on the **Security** tab.

## How it works

Rate limiting is implemented via the [caddy-ratelimit](https://github.com/mholt/caddy-ratelimit) Caddy plugin, which is included in the PosternProxy Caddy build.

When a client exceeds the limit, Caddy returns a `429 Too Many Requests` response. The limit resets on a sliding window basis — it is not a hard per-minute bucket.

## Configuration

Enable the **Rate Limit** toggle on the Security tab, then set:

| Field | Description |
|---|---|
| **Requests** | Maximum number of requests allowed per window |
| **Window** | Duration of the sliding window (e.g. `60s`, `1m`, `5m`) |

The rate limit is keyed by the verified visitor IP. With no proxy trust policy,
this is the immediate connection peer. For proxy/CDN traffic, configure the
explicit per-server `POSTERNPROXY_CADDY_TRUSTED_PROXIES` policy in
[Security Hardening]({{< relref "/administration/security-hardening" >}}).
Shared NAT addresses still share a bucket. Management
`POSTERNPROXY_TRUSTED_PROXIES` remains separate.

<!-- TODO: Screenshot — Security tab with rate limit enabled (100 req / 1m) -->

## Example

To allow a maximum of 100 requests per minute per IP:

- Requests: `100`
- Window: `60s`

## Notes

- Rate limiting applies to all requests to the proxy host, including static assets. Consider setting a high limit for asset-heavy applications.
- The management UI (`/api` endpoints) has its own separate built-in rate limits (120 req/min per IP on authenticated routes, 5 login attempts/min per IP) that are not configurable here.
- To completely block certain IP ranges rather than rate-limit them, use an [access list]({{< relref "/access-lists" >}}) instead.
- Rate limiting state is in-memory and does not persist across Caddy restarts.
