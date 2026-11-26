# B. Request trust and secret-safe agent transport

Status: implementation in progress from the earlier A/B/C work; final qualification/sign-off pending. Lead: GPT5.6. Review: Grok 4.7 provisionally, GPT6.1 fallback. Parent: [development index](beta-exit-development-index.md).

Owns blockers **3 and 4**. This is bounded middleware/transport work, suited to GPT5.6 with explicit adversarial cases; escalate ambiguous proxy-chain policy to GPT6.1.

## Current evidence and scope

`internal/api/router.go` uses unconditional chi RealIP and Logger. `internal/security/ratelimit.go` uses `RemoteAddr` as-is. `cmd/posternproxy-agent/main.go` sends `token` in the URL; `internal/api/handlers/servers.go` reads it there. This leaves a query credential on the request-logging path.

Touch points: router, security helpers, config/env parsing, agent dial and connection handler, API/provisioning docs, logger tests. Coordinate agent rollout with D and new protocol capabilities with C.

## Development steps

- [ ] Add an explicit trusted-proxy CIDR configuration, empty by default. Validate it at startup. Parse the actual socket peer separately from forwarding headers and normalize IPv4/IPv6 without source ports.
- [ ] Define one supported forwarding-header policy: accept only behind a trusted immediate peer, walk trusted hops from the peer backwards, reject malformed chains safely, and ignore competing headers. Share canonical client identity between rate limits and audit logs while retaining the actual peer for diagnostics.
- [ ] Replace chi's request logger with structured zerolog request logging that omits sensitive query values and authorization/cookie headers. Avoid raw URI/error serialization that can reintroduce credentials. Protect logging before any compatibility handling is deployed.
- [ ] Put the WebSocket bearer token in `Authorization` and keep only non-secret server identity in the URL. Validate identity/token before upgrade, using existing secure validation. Reject conflicting credentials and query-only authentication in the final configuration.
- [ ] Specify controller/agent upgrade order. Prefer updating controller handling and logger first, then agents. If a temporary query compatibility window is necessary, bound and document its removal; it must remain redacted and must not silently become permanent.
- [ ] Document rotation of agent credentials potentially exposed in existing controller, reverse-proxy, or access logs; record only identifiers and rotation outcome. Update deployment examples, env docs, and changelog.

## Regression and acceptance

- [ ] Same peer with different ephemeral ports shares one rate-limit bucket. Changing forged X-Forwarded-For, X-Real-IP, or Forwarded cannot evade limits or forge audit attribution in direct deployment.
- [ ] Test trusted single/multiple hops, untrusted intermediary, malformed input, missing headers, IPv4/IPv6, and overlapping CIDRs. Show valid trusted-proxy clients remain distinguishable.
- [ ] Real WebSocket tests cover header authentication, invalid/expired/revoked credentials, wrong server, missing credentials, conflicts, and the final rejection of URL tokens.
- [ ] Capture logs on successful handshake, rejected auth, failed upgrade, reconnect, and request errors. Assert known test token/header/cookie sentinels never occur; also exercise legacy query redaction even after query authentication is removed.
- [ ] Run shared checks and record results. Login, management API, and agent limiter behavior all use the same canonical identity policy. HSTS, CSRF, secure headers, and admin checks remain enforced.

Dependencies: none for initial work; coordinate auth route changes with A. D consumes the finished deployment/upgrade contract.

Completion evidence: pending. Both blocker tests and final authentication transport policy must pass before closure.

G handoff (2026-10-24): govulncheck reports three called-symbol chi RealIP
advisories in v5.2.5, fixed upstream in v5.3.0. Include dependency remediation
with the trusted-peer implementation; see [G checkpoint](beta-exit-g-foundation-checkpoint.md).
