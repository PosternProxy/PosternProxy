# Evidence: ADR 0007 — Trusted client-identity model

- **ADR:** [../adr/0007-client-identity-model.md](../adr/0007-client-identity-model.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master); shipped in v0.1.5-beta.
- **Implementation:** `internal/caddy` route generation derives `{http.vars.client_ip}` and applies it to host IP rules, rate-limit keys, forwarded headers and audit; trusted proxies are configured per server.
- **Command and result:** `go test ./...` — exit 0 (includes `internal/caddy`).
- **Confirmed:** visitor identity is separate from the socket peer and applied consistently; upstream trust is explicit per server.
- **Contradicted:** none observed.
- **Not tested or built:** no new live test behind a real upstream proxy for this backfill.
- **Owner follow-up:** none.
