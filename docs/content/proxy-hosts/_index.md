---
title: Proxy Hosts
weight: 20
bookCollapseSection: true
---

# Proxy Hosts

Proxy hosts are the core resource in PosternProxy. Each one tells Caddy to accept traffic for one or more domain names and forward it to an upstream service.

## What a proxy host does

When you create a proxy host, PosternProxy:

1. Adds a Caddy route that matches the specified domain names
2. Optionally enforces HTTPS and HSTS
3. Forwards matching requests to the configured upstream (or load-balances across multiple upstreams)
4. Optionally applies an access list, rate limit, custom headers, or custom error pages
5. Manages TLS automatically via Let's Encrypt (unless you supply a custom certificate)

## The form tabs

The **Add / Edit Proxy Host** modal has the following tabs:

| Tab | Purpose |
|---|---|
| **Details** | Domain names, primary upstream, basic options |
| **Upstreams** | Extra backends for load balancing |
| **SSL** | Certificate selection, HTTPS enforcement, HSTS |
| **Locations** | Sub-path routing overrides |
| **Health** | Active and passive health check settings |
| **Security** | Access list, rate limiting |
| **Errors** | Custom HTML for 404/502/503 responses |
| **Headers** | Request and response header injection |
| **Advanced** | Raw Caddy JSON merged into the route |

## Quick reference

| Setting | Description |
|---|---|
| Domain Names | One or more hostnames (press Enter after each) |
| Forward Scheme | `http` or `https` — protocol used to reach your upstream |
| Forward Hostname / IP | Internal address of the service to proxy |
| Port | Upstream port number |
| Websocket Support | Enables `Upgrade` header pass-through |
| Block Exploits | Adds Caddy request-blocking rules for common exploit patterns |
| Cache Assets | *(UI label)* — sets `Cache-Control` on static asset responses |

## Enable / disable

Each proxy host can be toggled on or off without deleting it. When disabled, Caddy removes the route immediately; traffic returns a 404 until re-enabled.

## Sub-pages

- [Load Balancing]({{< relref "/proxy-hosts/load-balancing" >}})
- [Health Checks]({{< relref "/proxy-hosts/health-checks" >}})
- [SSL / TLS]({{< relref "/proxy-hosts/ssl-tls" >}})
- [Rate Limiting]({{< relref "/proxy-hosts/rate-limiting" >}})
- [TLS Passthrough]({{< relref "/proxy-hosts/tls-passthrough" >}})
- [Custom Error Pages]({{< relref "/proxy-hosts/custom-error-pages" >}})
- [Custom Headers]({{< relref "/proxy-hosts/custom-headers" >}})
- [Custom Locations]({{< relref "/proxy-hosts/locations" >}})
- [Config History]({{< relref "/proxy-hosts/config-history" >}})
