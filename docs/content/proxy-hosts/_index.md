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
| **Security** | NTLM authentication passthrough, TLS passthrough, rate limiting |
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
| Access List | Restrict access by IP via an access list. Choose it on the basic host form and on the SSL tab. |

## NTLM authentication passthrough

**Legacy.** The Security tab checkbox is for older IIS apps that use Integrated Windows Authentication (NTLM/Negotiate). It is off for existing hosts, old backups, and history snapshots that predate the column. IIS behavior is not verified.

Known limitations and risks:

- One upstream, and a direct browser connection. An intermediary that shares one connection across users can break per-user isolation. Forwarded client headers are not the connection identity.
- Do not add header rules, path locations, or a custom 401 page on this host. They can break authentication. Path locations, a custom 401 page, TLS passthrough, and nonempty advanced route JSON are rejected. A header rule is rejected when it can change `Authorization` or `WWW-Authenticate`; other header rules can still be saved.
- Terminating TLS at PosternProxy can conflict with IIS Extended Protection. Do not disable Extended Protection or TLS verification to force the host to work.
- Not verified against all IIS configurations. There is no upstream failover or NTLM load balancing. If the required IIS settings cannot be met, use TLS passthrough instead of this control.

Turning the checkbox on emits the `http_ntlm` transport for that host only. The upstream connection is HTTP/1.1. The server HTTP/2 setting is unchanged. Turning it off restores the ordinary HTTP transport on the next successful apply. Access lists, rate limits, HSTS, and health checks stay available. A health check that expects HTTP 200 marks the only upstream unhealthy when the probe receives 401. Point the check at an anonymous URL or leave it off.

Every server that will use the flag must run the pinned Caddy build before the flag is enabled. Saving the host does not prove a remote Caddy has loaded the module. Check configuration status for `applied`. One NTLM host that cannot be rendered blocks applying a new configuration for every host on that server. PosternProxy does not fall back to ordinary HTTP. The previous Caddy configuration keeps serving, and the saved setting is left unchanged. Correct or disable the host, then retry sync.

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
