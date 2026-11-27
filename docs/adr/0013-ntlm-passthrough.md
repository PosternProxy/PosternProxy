# 0013. NTLM passthrough is provided by a project-maintained fork of Caddy's http_ntlm transport

- **Status:** Accepted
- **Date:** 2026-11-27
- **Deciders:** Michael (owner)
- **Topics:** proxy-hosts, caddy-build, authentication

## Context

Legacy IIS applications use Integrated Windows Authentication (NTLM/Negotiate), which is connection-oriented: a reverse proxy that pools or multiplexes upstream connections breaks the security context, and open-source nginx (and NPM) cannot preserve it without the commercial `ntlm` directive. Caddy's `http.reverse_proxy.transport.http_ntlm` module keeps the required connection affinity, but upstream `caddyserver/ntlm-transport` has been dormant since 2023, and its latest commit (`a593c877`) still returns from `RoundTrip` while holding `transportsMu` when `newTransport` fails (a process-wide deadlock) and leaks client-bound transports on failed or canceled requests. Shipping that commit, vendoring a patch, a separate install variant, and a license-gated build were all rejected (the dual-license model is "same software, different terms").

## Decision

1. NTLM passthrough is an opt-in per-host boolean (`ntlm_enabled`, default off) that makes the route builder emit `transport.protocol = "http_ntlm"`. PosternProxy never implements the handshake or becomes a Windows identity provider.
2. The module is a project-maintained fork, `github.com/PosternProxy/ntlm-transport` (Apache-2.0), pinned by pseudo-version into the standard Caddy build — not the upstream module, a floating branch, or a vendored patch. The fork fixes the `RoundTrip` lock leak and the map-entry leak and sets `MaxConnsPerHost = 1` (one downstream connection maps to one upstream socket).
3. A Caddy without the module fails the apply closed: never fall back to `http`, the previously applied config keeps serving, `applied_revision` is unchanged, and one unrenderable NTLM host withholds the whole server's new configuration.
4. Settings that would break the exchange are rejected on create, update, import and rollback: extra upstreams, path locations, TLS passthrough, nonempty advanced route JSON, header rules that alter `Authorization`/`WWW-Authenticate`, and custom 401 error pages. TLS verification is never weakened.
5. The feature ships as **legacy and IIS-unverified**, with a warning on the control. No "verified IIS support" claim is made until AC7–AC11 (see the spec) are recorded.

## Consequences

- Easier: one standard artifact carries NTLM; the controller only selects the transport; the AGPL/commercial model is unchanged.
- Harder: a project-maintained fork must track Caddy releases; every server must run the pinned Caddy build before the flag is enabled.
- Now a rule, with its check: fail-closed apply and no `http` fallback — `BuildFullConfig` returns on the first route error, and `internal/caddybuild/verify.go` + `build.sh`/`build.ps1` require the module at the pinned version; conflict rejections — `models.ValidateNTLMPassthrough`, called from the host service, the route builder, and backup import.

## Evidence

[../evidence/0013-ntlm-passthrough.md](../evidence/0013-ntlm-passthrough.md)
