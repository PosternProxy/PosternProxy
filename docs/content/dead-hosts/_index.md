---
title: Dead Hosts (404 Hosts)
weight: 50
---

# Dead Hosts

Dead hosts (also called 404 hosts) tell Caddy to return a `404 Not Found` for all requests to specified domain names. They are useful for:

- Parking domains you own but are not yet using
- Returning a clean 404 instead of Caddy's default "no route" response
- Blocking access to domains that should not be publicly accessible
- Preventing certificate issuance errors for domains pointed at your server

## Creating a dead host

1. Click **Dead Hosts** in the sidebar
2. Click **+ Add Dead Host**
3. Fill in the form:

| Field | Description |
|---|---|
| **Domain Names** | One or more domains to serve 404 for (press Enter after each) |
| **SSL Certificate** | Optional — select a cert if you want HTTPS on the dead host |
| **Access List** | Optional — restrict which clients can even reach the 404 page |

4. Click **Create**

<!-- TODO: Screenshot — Add Dead Host modal -->

## SSL on dead hosts

Selecting a certificate allows Caddy to serve TLS for the domain. Without a certificate, Caddy will still attempt to obtain one automatically if the domain resolves to the server.

To suppress automatic certificate issuance (e.g. for internal domains that should not have public certs), leave SSL blank and ensure the domain does not resolve to the public internet.

## Access lists on dead hosts

Combining a dead host with an [access list]({{< relref "/access-lists" >}}) lets you create a "blackhole" that only your internal network can reach. This is useful for domains reserved for internal use where you want to prevent external certificate issuance probes from succeeding.

## Enable / disable

Dead hosts can be toggled. When disabled, the domain no longer has any route in Caddy, and Caddy's default "no such site" response applies.
