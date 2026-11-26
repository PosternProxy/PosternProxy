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
| **Errors** | Custom HTML for Caddy-generated 502/503 errors |
| **Headers** | Request and response header injection |
| **Advanced** | Raw Caddy JSON merged into the route |

## Quick reference

| Setting | Description |
|---|---|
| Domain Names | One or more hostnames (press Enter after each) |
| Forward Scheme | `http` or `https` — protocol used to reach your upstream |
| Forward Hostname / IP | Internal address of the service to proxy |
| Port | Upstream port number |
| Force HTTPS | Redirect HTTP to HTTPS (otherwise the host is served over HTTP) |
| Access List | Restrict access by IP via an access list |

## Enable / disable

Each proxy host can be toggled without deleting it. After successful application,
disabling removes its managed route. The response then depends on other matching
routes and listeners; it is not guaranteed to be 404. Check
[Configuration application status]({{< relref "/multi-server/configuration-status" >}})
for rejected changes or offline agents.

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
