# Evidence: ADR 0006 — Reproducible pinned Caddy build, verified in the artifact

- **ADR:** [../adr/0006-pinned-caddy-build.md](../adr/0006-pinned-caddy-build.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master).
- **Implementation:** `internal/caddybuild/build.sh` pins Caddy, xcaddy, the Go toolchain, `caddy-l4`, `ratelimit`, the security replacements, and the NTLM fork; `internal/caddybuild/verify.go` checks module IDs/versions and rejects unexpected `replace` directives; `scripts/build.ps1` mirrors the pins.
- **Tests:** `internal/caddybuild/build_test.go`, `build_test.sh`, and the list-modules / buildinfo fixtures.
- **Command and result:** `go test ./...` — exit 0 (includes `internal/caddybuild`).
- **Confirmed:** the build contract is enforced; an absent or wrong-versioned module fails verification.
- **Contradicted:** none observed.
- **Not tested or built:** no fresh native artifact was built for this backfill.
- **Owner follow-up:** none.
