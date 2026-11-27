# Evidence: ADR 0002 — Single static Go binary with an embedded Vue frontend

- **ADR:** [../adr/0002-single-binary-embedded-frontend.md](../adr/0002-single-binary-embedded-frontend.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master); in effect since the first release.
- **Implementation:** `cmd/posternproxy` embeds the built `frontend/` assets; there is no separately deployed web tier.
- **Command and result:** `go build ./...` — exit 0; `go test ./...` — exit 0.
- **Confirmed:** the controller ships as one binary serving both API and embedded UI.
- **Contradicted:** none observed.
- **Not tested or built:** documents an already-shipped decision; no new isolated exercise was run for this backfill.
- **Owner follow-up:** none.
