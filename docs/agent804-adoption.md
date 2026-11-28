# Agent804 adoption

- Destination: Level 4 -  Security module is triggered (SSH credentials, remote-infra control, externally reachable). Not a claim about current status.
- Current assessment: run in-session on 2026-11-27 — Level 0 earned (strong ADRs/tests/CI, but the Level 1 "one check command" and maintainability limits were missing). Not committed as a file.
- Decisions already made: a Go-based check runner (`scripts/check`, run via `go run ./scripts/check`) replaces the Makefile, so one command works on Windows, Linux and CI with only the Go toolchain. Stack: Go controller + Vue/TS frontend. Reviewer/owner: Michael. Agent hosts: Claude Code, Codex.
- Completion evidence: `go run ./scripts/check` passes locally (both halves); CI runs the same command.

## Progress

| Increment | Status | Changes and evidence | Remaining limits |
|---|---|---|---|
| Single check command + scanner baseline | complete | `scripts/check/main.go`; Makefile removed; 16 staticcheck fixes; gosec G104 excluded + ~40 intentional findings annotated `#nosec`; `.gitleaks.toml` allowlists the e2e test cert. Commit `5cba95c`. Full check green in 54s. | gosec findings are accepted per-site, not all re-reviewed; complexity ceiling is today's worst (62), to ratchet down. |
| chi/v5 security bump | complete | `go.mod`/`go.sum` chi v5.3.0 → v5.5.0 (X-Forwarded-For spoofing advisory). Commit `183da9d`. `go mod verify` clean; check green. | — |
| CI runs `go run ./scripts/check` | in progress | `.github/workflows/ci.yml` Go + Frontend jobs call the runner; tool versions pinned in `scripts/check/main.go` (staticcheck v0.8.2, gosec v2.30.0, gocyclo v0.6.0). Specialized jobs (go mod verify, installer checks, caddy-acceptance, Playwright e2e, docs) kept. | Closes CK-1 "CI runs the same command"; CK-8 still needs branch protection + required review enabled on GitHub (owner-only, external). |

- Resume here: confirm the updated CI workflow is green on GitHub after push.
- Next useful increment: enforce review-before-merge — enable branch protection on `master` (required status check = CI, required PR review) to close CK-8/RV-1/GR-3. This is a GitHub setting the owner applies; unauthorized until agreed.
- Optional later possibilities: TS-12 (print test counts / fail on empty discovery); IN-3 (enforce the instruction-file line budget in the check).
- Urgent risks: none new in examined scope. The one real dependency advisory (chi) is fixed.

An implemented increment does not establish a level. This record cannot waive a failed
check, approve an exception or accept a security finding. Keep real approvals in their
existing authoritative location.
