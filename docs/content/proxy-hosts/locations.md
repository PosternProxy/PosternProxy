---
title: Custom Locations
weight: 8
---

# Custom Locations

Custom locations let you override proxy behaviour for specific URL paths within a proxy host. They are configured on the **Locations** tab.

## What locations do

By default, a proxy host forwards all paths to the same upstream. Locations allow you to:

- Send `/api/*` to a different backend than `/`
- Apply different `forward_scheme` per path
- Set path-specific Caddy directives using **Advanced Config**

## Adding a location

Click **+ Add Location** on the Locations tab. Each location has:

| Field | Description |
|---|---|
| **Path** | URL path prefix to match (e.g. `/api`, `/static`) |
| **Forward Scheme** | `http` or `https` for this path |
| **Forward Host** | Upstream host for this path |
| **Port** | Upstream port for this path |
| **Advanced Config** | Raw Caddy JSON directives merged into this location's subroute |

<!-- TODO: Screenshot — Locations tab with two locations (/api to port 4000, / to port 3000) -->

## Path matching

Paths are matched as prefixes. A location with path `/api` will match:
- `/api`
- `/api/users`
- `/api/v2/orders`

More specific paths take priority. If you have both `/api` and `/api/admin`, requests to `/api/admin/users` will use the `/api/admin` location.

## Order of evaluation

Locations are evaluated before the default proxy host route. The first matching location wins. The default upstream (from the Details tab) is used for any request that does not match a location.

## Advanced Config per location

The **Advanced Config** field on each location accepts raw Caddy JSON that is merged into the subroute for that path. This is the same mechanism as the [Advanced Config]({{< relref "/proxy-hosts" >}}) on the main proxy host, but scoped to a single path.

Use this for path-specific directives such as:

```json
{
  "rewrite": {
    "strip_path_prefix": "/api"
  }
}
```

## Notes

- Locations are stored as a JSON array in the database alongside the proxy host record.
- Changing locations triggers an immediate Caddy config update — no service restart required.
- A proxy host with no locations has a single Caddy subroute covering all paths.
