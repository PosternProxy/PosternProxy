# Evidence: ADR 0004 — Outbound-only agent model

- **ADR:** [../adr/0004-outbound-only-agent.md](../adr/0004-outbound-only-agent.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master).
- **Implementation:** `cmd/posternproxy-agent` dials out to the controller; `internal/agent/hub.go` and `internal/agent/protocol.go` carry config push and apply results over that connection. No inbound/controller-dials-in path exists.
- **Command and result:** `go test ./...` — exit 0 (includes `internal/agent`).
- **Confirmed:** managed servers connect outbound only; the controller never initiates connections to them.
- **Contradicted:** none observed.
- **Not tested or built:** no new live two-agent deployment was run for this backfill (earlier native two-agent acceptance is recorded in the beta-exit evidence).
- **Owner follow-up:** none.
