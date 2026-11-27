# Evidence: ADR 0013 — NTLM passthrough via a maintained fork of http_ntlm

- **ADR:** [../adr/0013-ntlm-passthrough.md](../adr/0013-ntlm-passthrough.md)

## 2026-11-27: implementation, review, and race verification

- **Spec and criteria:** [../specs/ntlm-passthrough.md](../specs/ntlm-passthrough.md); AC1–AC6 (AC7–AC11 pending).
- **Revision:** controller `0ea37ed`; fork `github.com/PosternProxy/ntlm-transport` `66b6501` (pseudo-version `v0.0.0-20261127162849-66b6501bd51c`). Fork repo at `C:\Users\Michael\Repos\PosternProxy-ntlm-transport`.
- **Tests:** `internal/models/ntlm_test.go`, `internal/services/ntlm_{proxy_host,apply,backup}_test.go`, `internal/caddy/ntlm_route_test.go`, `internal/app/ntlm_rollback_test.go`, `internal/api/ntlm_passthrough_test.go`; fork `ntlm_test.go`.
- **Command and result:**
  - `go test ./...` (controller) — exit 0.
  - `go build ./...` — exit 0.
  - Fork `go test -race -count=1 ./...` — exit 0 (`ok github.com/PosternProxy/ntlm-transport`); WSL Ubuntu, gcc 11.4.0, `go1.26.11`, `CGO_ENABLED=1`. Independently reproduced.
  - Controller `go test -race -count=1 ./...` — exit 0 (recorded, same WSL toolchain; not independently re-run).
- **Confirmed:** fail-closed apply with no `http` fallback (`BuildFullConfig` returns on the first route error); validate-before-snapshot ordering (rejected update writes no history row); conflict rejections incl. exact/wildcard header deletes and `401`/`0401`/`401abc`/`+401` error-page keys; decoded missing-module classifier logs only the stable sentence, never the raw Caddy body; fork concurrency race-clean (lock-leak and map-leak fixes, `MaxConnsPerHost = 1`).
- **Contradicted:** none observed.
- **Learned:** upstream `caddyserver/ntlm-transport` is dormant (last real work 2023; one external 2026 commit still carrying the lock leak); terminating TLS at the proxy can conflict with IIS Extended Protection; locations/header/401 misconfigurations fail closed (broken auth, not an identity leak), which is why a warning plus rejection — rather than exhaustive runtime guards — is sufficient.
- **Not tested or built:** AC7–AC11 against live IIS (real-user identity, cross-user isolation under concurrency, failure/idle/reload recovery, sustained workload). No IIS environment available.
- **Owner follow-up:** accept ADR-0001 to freeze it; record IIS qualification before advertising verified support.
