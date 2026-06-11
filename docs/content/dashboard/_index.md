---
title: Dashboard
weight: 140
---

# Dashboard

The Dashboard is the first page you see after logging in. It gives a real-time overview of your PosternProxy installation.

## Summary cards

At the top of the page, four cards show counts:

| Card | Shows |
|---|---|
| **Proxy Hosts** | Total proxy hosts (enabled + disabled) |
| **Redirection Hosts** | Total redirection hosts |
| **Stream Hosts** | Total stream hosts |
| **Dead Hosts** | Total dead hosts |

Clicking a card navigates to the corresponding resource list.

<!-- TODO: Screenshot — Dashboard summary cards row -->

## Upstream health widget

The health widget shows the current state of all upstreams that have health checks enabled. It auto-refreshes every 30 seconds.

| Column | Description |
|---|---|
| **Address** | Upstream host and port |
| **Status** | Green (healthy) or red pulsing (unhealthy) |
| **Requests** | Total requests forwarded to this upstream |
| **Failures** | Total failed requests |

A summary line at the bottom shows the total healthy/unhealthy count. Click the **↻** button to refresh immediately.

If no proxy hosts have health checks enabled, the widget shows an informational prompt with a link to the Proxy Hosts page.

<!-- TODO: Screenshot — Upstream health widget with three upstreams (two healthy, one unhealthy) -->

## Expiring certificates

Below the health widget, a table lists certificates expiring within the next 30 days:

| Column | Description |
|---|---|
| **Certificate** | Name / primary domain |
| **Domains** | All subject alternative names |
| **Expires** | Expiry date |
| **Days left** | Colour-coded: red (≤ 7 days), amber (≤ 14 days), yellow (≤ 30 days) |

If no certificates are expiring, this section is hidden.

<!-- TODO: Screenshot — Expiring certificates table with one cert at 6 days -->

## Multi-server dashboard

In [multi-server mode]({{< relref "/multi-server" >}}), the dashboard aggregates data across all connected servers. Each server's status (online / offline) is shown in the header. Upstream health is shown per-server.

## Notes

- Dashboard data is fetched once on page load and then refreshed on the intervals described above. There is no persistent WebSocket connection for the dashboard.
- The upstream health data comes from Caddy's `/reverse_proxy/upstreams` admin endpoint. It reflects Caddy's internal state, not PosternProxy's database.
