# Evidence: ADR 0003 — SQLite with forward-only, frozen migrations

- **ADR:** [../adr/0003-sqlite-forward-migrations.md](../adr/0003-sqlite-forward-migrations.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master).
- **Implementation:** `internal/database/migrations/` holds numbered forward migrations (`001`…`006`); the store is SQLite.
- **Tests:** `internal/database/migrate_test.go` (`TestInitialMigrationIsFrozen` compares `001_initial.sql` to a committed baseline).
- **Command and result:** `go test ./...` — exit 0, including the migration package.
- **Confirmed:** migrations are forward-only and the baseline is frozen; new schema arrives as new files (e.g. `006_proxy_host_ntlm.sql`).
- **Contradicted:** none observed.
- **Not tested or built:** no new isolated exercise beyond the existing migration tests.
- **Owner follow-up:** none.
