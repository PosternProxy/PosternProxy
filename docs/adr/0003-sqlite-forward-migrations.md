# 0003. State is stored in SQLite with forward-only, frozen migrations

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made at project inception)
- **Deciders:** Michael (owner)
- **Topics:** architecture, persistence

## Context

The controller needs a local store for hosts, servers, certificates, users, audit and configuration state. A single-binary product favours an embedded store over an external database.

## Decision

1. State is stored in **SQLite**.
2. Schema changes are **forward-only, numbered migrations**; historical migrations are frozen and never edited.

## Consequences

- Easier: a single-file store with no external dependency; migrations are append-only and auditable.
- Harder: no destructive in-place schema edits; new columns/tables come as new migration files.
- Now a rule, with its check: the initial migration is frozen against a committed baseline (`TestInitialMigrationIsFrozen` / `internal/database/migrate_test.go`).

## Evidence

`internal/database/migrations/`, `internal/database/migrate_test.go`. See [../evidence/0003-sqlite-forward-migrations.md](../evidence/0003-sqlite-forward-migrations.md).
