# Evidence: ADR 0008 — Desired/applied config reconciliation

- **ADR:** [../adr/0008-config-reconciliation.md](../adr/0008-config-reconciliation.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master).
- **Implementation:** `internal/services/caddy_sync.go` and `config_state.go` carry `desired_revision`/`applied_revision`; `internal/agent/protocol.go` and `hub.go` carry command/revision IDs and durable apply correlation.
- **Tests:** `internal/services/config_state_test.go` and the NTLM apply tests (`internal/services/ntlm_apply_test.go`) exercise the fail-closed apply path.
- **Command and result:** `go test ./...` — exit 0.
- **Confirmed:** `applied_revision` advances only on confirmed success; stale commands are ignored; apply failures are reported truthfully.
- **Contradicted:** none observed.
- **Learned:** this contract is what lets NTLM (ADR 0013) fail closed without an `http` fallback.
- **Not tested or built:** earlier native two-agent acceptance is recorded in the beta-exit evidence; no new multi-agent run for this backfill.
- **Owner follow-up:** none.
