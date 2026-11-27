# Evidence: ADR 0010 — Portable encrypted backup format

- **ADR:** [../adr/0010-portable-backup-format.md](../adr/0010-portable-backup-format.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master).
- **Implementation:** `internal/services/backup.go` and `internal/services/portable_backup.go` produce a versioned, encrypted archive with server mapping; restore reports apply state via ADR 0008.
- **Tests:** backup and portable-restore tests in `internal/services`, including NTLM backup round-trip and conflict rollback (`internal/services/ntlm_backup_test.go`).
- **Command and result:** `go test ./...` — exit 0.
- **Confirmed:** version semantics are enforced; restore reflects real apply outcome, not database-write success.
- **Contradicted:** none observed.
- **Not tested or built:** no new clean-install restore on fresh hardware for this backfill.
- **Owner follow-up:** none.
