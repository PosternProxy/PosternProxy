# Evidence: ADR 0011 — Caddy-L4-only forwarding

- **ADR:** [../adr/0011-caddy-l4-forwarding.md](../adr/0011-caddy-l4-forwarding.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master).
- **Implementation:** forwarding is implemented as Caddy L4 stream hosts; iptables/socat forwarding is absent from the controller, agent and docs; a forwards health monitor reports unreachable ports.
- **Command and result:** `go test ./...` — exit 0.
- **Confirmed:** a single Caddy-L4 forwarding mechanism; no OS-level forwarding packages or capabilities.
- **Contradicted:** none observed.
- **Learned:** the forwarding plan noted a `/api/health/forwards` discrepancy to reconcile in release validation; track separately.
- **Not tested or built:** no new live blocked-port probe for this backfill.
- **Owner follow-up:** none.
