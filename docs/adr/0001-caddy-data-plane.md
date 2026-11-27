# 0001. Caddy is the data plane; PosternProxy generates config and does not reimplement proxying

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made at project inception)
- **Deciders:** Michael (owner)
- **Topics:** architecture, data-plane

## Context

A reverse-proxy manager can either implement its own proxy/TLS/forwarding stack or delegate to an existing engine. Reimplementing TLS termination, ACME, HTTP/2–3, load balancing, rate limiting and L4 stream forwarding is a large, security-sensitive surface.

## Decision

1. All proxying, TLS, load balancing, rate limiting and L4 stream forwarding are handled by **Caddy**, with the `mholt/caddy-l4` and `caddy-ratelimit` plugins.
2. The controller's job is to generate Caddy JSON and apply it; it never reimplements the data plane.

## Consequences

- Easier: inherits Caddy's TLS/ACME/HTTP stack and plugin ecosystem; features are expressed as Caddy configuration.
- Harder: capabilities are bounded by Caddy's config model; the project is tied to Caddy's release cadence (managed by ADR 0006).
- Now a rule: new data-plane behaviour is added as Caddy config or a pinned Caddy module, not as custom proxy code.

## Evidence

Shipped since the initial release; see `internal/caddy/`. See [../evidence/0001-caddy-data-plane.md](../evidence/0001-caddy-data-plane.md).
