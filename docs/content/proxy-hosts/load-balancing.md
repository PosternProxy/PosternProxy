---
title: Load Balancing
weight: 1
---

# Load Balancing

PosternProxy supports distributing traffic across multiple upstream backends for a single proxy host. Configuration is on the **Upstreams** tab of the Add/Edit Proxy Host modal.

## Adding backends

The first upstream is always the **Primary** backend (set on the **Details** tab). Additional backends are added on the **Upstreams** tab. Each extra backend requires:

- **Host / IP** — the internal address of the backend server
- **Port** — the port the backend listens on

There is no enforced limit on the number of backends.

<!-- TODO: Screenshot — Upstreams tab with three backends and lb-policy radio cards -->

## Load balancing policies

Select a policy using the radio cards on the Upstreams tab. The policy only takes effect when two or more upstreams are configured.

| Policy | Description |
|---|---|
| **Round Robin** | Requests cycle through backends in order. Simple and predictable. |
| **Random** | Each request is sent to a randomly selected backend. Good default. |
| **Least Connections** | Requests go to the backend with the fewest active connections. Best for long-lived connections. |
| **IP Hash** | The client's IP address determines which backend handles the request (sticky sessions). |
| **URI Hash** | The request URI determines which backend handles the request. Useful for cache affinity. |
| **First** | Always sends to the first healthy backend; subsequent backends are used only as fallback. |

## Retries and failover

PosternProxy automatically sets `retries` to `len(upstreams) - 1`, meaning Caddy will try every available backend before returning an error. If a backend fails mid-request, Caddy retries on the next available upstream.

Combine this with [passive health checks]({{< relref "/proxy-hosts/health-checks" >}}) to automatically eject unhealthy backends from the rotation.

## Example: three backends with round robin

| # | Host | Port | Policy |
|---|---|---|---|
| 1 (primary) | `192.168.1.10` | `3000` | — |
| 2 | `192.168.1.11` | `3000` | Round Robin |
| 3 | `192.168.1.12` | `3000` | — |

Caddy will cycle: `10 → 11 → 12 → 10 → …`

## Notes

- Changing the load balancing policy takes effect immediately — Caddy is updated via its admin API without dropping existing connections.
- The primary upstream is always included in the pool. You cannot remove it from the Upstreams tab; change it on the Details tab instead.
- Backends with a blank host are ignored when saving (partial rows are silently dropped).
