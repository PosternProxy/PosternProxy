---
title: Health Checks
weight: 2
---

# Health Checks

PosternProxy exposes Caddy's built-in health-check machinery through the **Health** tab of the proxy host form. Health checks keep your load balancer from routing traffic to backends that are down or degraded.

## Active health checks

Active checks probe each upstream on a schedule by sending an HTTP request and evaluating the response.

| Setting | Default | Description |
|---|---|---|
| **Path** | `/` | URL path Caddy requests on each backend |
| **Interval** | `30s` | How often to run the check |
| **Timeout** | `5s` | Maximum time to wait for a response |
| **Expected status** | `200` | HTTP status code that counts as healthy |

Enable active checks with the **Enable health checks** toggle on the Health tab.

<!-- TODO: Screenshot — Health tab with active check settings -->

## Passive health checks

Passive checks observe real traffic and mark a backend as unhealthy when it produces too many errors or slow responses.

| Setting | Default | Description |
|---|---|---|
| **Fail duration** | `30s` | How long a backend stays ejected after being marked unhealthy |
| **Max fails** | `3` | Number of consecutive failures before ejection |
| **Unhealthy latency** | `0` (disabled) | Mark slow responses (≥ this duration) as failures |

Passive checks require no additional configuration beyond setting the thresholds. They activate automatically once any proxy host has health checks enabled.

## Dashboard widget

The Dashboard shows a live upstream health table for all proxy hosts that have health checks enabled. It refreshes every 30 seconds and includes:

- Current status (healthy / unhealthy)
- Total request count
- Total failure count

See [Dashboard]({{< relref "/dashboard" >}}) for details.

## How Caddy reports health

Caddy tracks health state internally. PosternProxy reads it from the Caddy admin API at `GET /reverse_proxy/upstreams`. The data is available immediately — no polling interval to configure on PosternProxy's side.

## Notes

- Health checks run per upstream, per proxy host. Two proxy hosts sharing the same backend IP will have independent health state.
- Active and passive checks can be enabled simultaneously — passive checks provide faster failure detection while active checks provide recovery detection.
- If all backends fail, Caddy returns a `502 Bad Gateway`. Configure a [custom error page]({{< relref "/proxy-hosts/custom-error-pages" >}}) for 502 to give users a friendlier message.
