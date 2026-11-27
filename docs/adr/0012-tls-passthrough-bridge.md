# 0012. TLS passthrough coexists with terminating hosts via a layer-4 bridge on a reserved port

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made during the beta-exit / forwarding work)
- **Deciders:** Michael (owner)
- **Topics:** tls, routing, architecture

## Context

Some hosts (for example IIS) must terminate their own TLS, so the proxy must pass their TLS through by SNI — while other HTTPS hosts on the same `:443` still need normal termination.

## Decision

1. When any host enables TLS passthrough, `:443` moves to a Caddy **layer-4** handler that SNI-routes passthrough hosts straight to their upstreams and **bridges** other HTTPS hosts to the HTTP app over a **reserved internal port** using PROXY protocol.
2. **HTTP/3 is disabled** on that path and the bridge port is reserved.

## Consequences

- Easier: passthrough and terminating hosts coexist on one `:443`.
- Harder: HTTP/3 is unavailable while passthrough is active; an internal port is reserved; NTLM passthrough (ADR 0013) cannot combine with TLS passthrough on the same host.
- Now a rule: the bridge port is reserved and the L4 restructure is driven by whether any host enables passthrough.

## Evidence

`internal/caddy/config.go` (layer-4 app, `internalHTTPSAddr`); git `cb4d231` "Harden TLS-passthrough bridge: disable HTTP/3, reserve bridge port". See [../evidence/0012-tls-passthrough-bridge.md](../evidence/0012-tls-passthrough-bridge.md).
