# Evidence: ADR 0009 — Per-server certificate isolation

- **ADR:** [../adr/0009-certificate-isolation.md](../adr/0009-certificate-isolation.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master).
- **Implementation:** `internal/caddy/tls.go`, the certificate service/handlers, and the agent cert writer deliver certificates per server with restricted permissions; activation/retirement flow through the apply model (ADR 0008).
- **Command and result:** `go test ./...` — exit 0.
- **Confirmed:** certificates are not bundled to every server; key material is handled under a defined access contract.
- **Contradicted:** none observed.
- **Not tested or built:** no new cross-user service-sandbox deployment test for this backfill.
- **Owner follow-up:** none.
