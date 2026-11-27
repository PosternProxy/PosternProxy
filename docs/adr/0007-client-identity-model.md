# 0007. Visitor identity is derived as a trusted client IP, separate from the socket peer

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made during the beta-exit program)
- **Deciders:** Michael (owner)
- **Topics:** security, identity, routing

## Context

Behind Cloudflare or another proxy, the TCP socket peer is the upstream proxy, not the visitor. Access-list rules, rate-limit keys, forwarded headers and audit attribution must use the real visitor, but trusting arbitrary forwarded headers lets clients spoof identity. This was the one genuine design decision inside the beta-exit request-trust work (the rest — moving the agent token out of the URL, fixing `RealIP`/`RemoteAddr` usage — was remediation, recorded in the changelog, not here).

## Decision

1. Visitor identity is derived as a distinct **client IP** (`{http.vars.client_ip}`), kept separate from the socket peer.
2. It is applied **consistently** to host IP rules, rate-limit keys, forwarded client headers, and access-log / audit attribution.
3. Upstream proxies are trusted only via **explicit per-server static trust**, never implicitly from forwarded headers.

## Consequences

- Easier: correct identity behind declared trusted proxies.
- Harder: operators must declare trusted proxies per server; management-proxy policy is handled separately.
- Now a rule: never key identity off an untrusted client-supplied header; trust is explicit and per-server.

## Evidence

`internal/caddy` route generation (`client_ip`, trusted-proxy config); beta-exit request-trust work; shipped in v0.1.5-beta. See [../evidence/0007-client-identity-model.md](../evidence/0007-client-identity-model.md).
