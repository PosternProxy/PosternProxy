# Evidence: ADR 0001 — Caddy is the data plane

- **ADR:** [../adr/0001-caddy-data-plane.md](../adr/0001-caddy-data-plane.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master); in effect since the first release (v0.1.2-beta).
- **Implementation:** `internal/caddy/` generates Caddy JSON; `internal/caddybuild/` builds Caddy with `caddy-l4` and `ratelimit`. No in-process proxy/TLS code exists.
- **Command and result:** `go test ./...` — exit 0 (full suite green at this revision).
- **Confirmed:** all proxying/TLS/L4/rate-limiting is expressed as Caddy config; the controller holds no data-plane implementation.
- **Contradicted:** none observed.
- **Not tested or built:** documents an already-shipped architectural decision; no new isolated exercise was run for this backfill.
- **Owner follow-up:** none.
