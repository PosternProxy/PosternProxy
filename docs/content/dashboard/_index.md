---
title: Dashboard
weight: 140
---

# Dashboard

The Dashboard summarizes controller database records and cached local Caddy
upstream counters. It is the first page after login.

## Summary cards

Six cards link to proxy hosts, redirections, stream hosts (Traffic Forwarding),
404 hosts, certificates, and access lists. Counts include records saved in the
database; a count is not proof that their routes are currently applied or reachable.
The page also lists uploaded certificates expiring within 30 days when expiry
metadata is available.

## Upstream health

The widget shows the address, derived status, and Caddy-reported request/failure
counters. It reads `GET /api/health/upstreams` on load and every 30 seconds; the
controller polls its local Caddy at startup and every 30 seconds.

| Status | Meaning |
|---|---|
| Healthy | Caddy reports requests with no recorded failures |
| Unhealthy | Caddy reports failures |
| Unknown | No positive traffic evidence is available |

This is a counter-derived view, not a direct reachability probe or a cumulative
traffic accounting system. It is not restricted to hosts with health checks
turned on. Multiple hosts using the same address can share a reported upstream.

## Multi-server scope

Database totals can include remote resources, but the upstream widget reads the
controller's local Caddy only. It does not aggregate health probes from every
agent. Check **Servers** for agent connection status and
[Configuration application status]({{< relref "/multi-server/configuration-status" >}})
for desired/applied revisions.

Summary statistics are loaded when the page opens. Upstream counters refresh on
a timer; there is no persistent dashboard WebSocket connection.
