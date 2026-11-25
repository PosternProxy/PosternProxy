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
| **Path** | `/health` | URL path Caddy requests; leave empty to disable active checks |
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
| **Max fails** | `3` | Failure threshold within the passive fail-duration window |
| **Unhealthy latency** | `0` (disabled) | Mark slow responses (≥ this duration) as failures |

Passive checks are emitted for each host with **Enable Health Checks** on and
nonzero passive thresholds. Enabling checks on one host does not enable them for
every other host.

## Dashboard widget

The Dashboard shows Caddy's cached local upstream counters. It refreshes every
30 seconds and includes:

- Derived status (healthy / unhealthy / unknown)
- Caddy's reported request and failure counters

See [Dashboard]({{< relref "/dashboard" >}}) for details.

## How Caddy reports health

Caddy tracks health internally. PosternProxy polls the local
`GET /reverse_proxy/upstreams` endpoint at startup and every 30 seconds. The UI
reads that cache. Zero failures with zero requests is shown as **unknown**, not
as a successful reachability probe.

## Notes

- The dashboard counters are keyed by upstream address, not by host or remote
  server. Do not treat the widget as isolated per-host or remote health evidence.
- Active and passive checks can be enabled simultaneously — passive checks provide faster failure detection while active checks provide recovery detection.
- If all backends fail, Caddy returns a `502 Bad Gateway`. Configure a [custom error page]({{< relref "/proxy-hosts/custom-error-pages" >}}) for 502 to give users a friendlier message.
