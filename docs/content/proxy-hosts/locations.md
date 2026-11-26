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

## Adding a location

Click **+ Add Location** on the Locations tab. Each location has:

| Field | Description |
|---|---|
| **Path** | URL path prefix to match (e.g. `/api`, `/static`) |
| **Forward Scheme** | `http` or `https` for this path |
| **Forward Host** | Upstream host for this path |
| **Port** | Upstream port for this path |

<!-- TODO: Screenshot — Locations tab with two locations (/api to port 4000, / to port 3000) -->

## Path matching

Paths are matched as prefixes. A location with path `/api` will match:
- `/api`
- `/api/users`
- `/api/v2/orders`
- `/apix` (matching is a raw prefix, not a path-segment boundary)

Locations retain their saved order; there is no automatic specificity sort.
Place `/api/admin` before `/api` if it must take priority. The path is forwarded
unchanged; the prefix is not stripped.

## Order of evaluation

Locations are evaluated before the default proxy host route. The first matching location wins. The default upstream (from the Details tab) is used for any request that does not match a location.

## Advanced configuration

The location form currently exposes only path, scheme, host, and port. Although
the model has an `advanced_config` field, the location route builder does not
apply it. Use the host-level Advanced tab only when you can supply valid Caddy
route JSON and verify its resulting configuration.

## Notes

- Locations are stored as a JSON array in the database alongside the proxy host record.
- Changing locations saves desired configuration and requests application; no
  service restart is normally required. Verify the configuration notice.
- A proxy host with no locations has a single Caddy subroute covering all paths.
