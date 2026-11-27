# 0010. Backups use a portable, versioned, encrypted format that restores to a clean install

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made during the beta-exit program)
- **Deciders:** Michael (owner)
- **Topics:** backup, recovery, security

## Context

Backup and restore must work onto a clean installation, not only the same machine, without mishandling key material or falsely reporting success. Earlier export omitted servers, carried certificate paths without a portable key story, enforced only a nonempty version, and reported `synced: true` after database restore even when the apply failed.

## Decision

1. Backups use a **portable, versioned, encrypted** archive (AES-256-GCM helpers) that includes server enrolment/mapping.
2. **Version semantics** are enforced on restore; unsupported versions are rejected.
3. Restore reports **real apply state** via the reconciliation model (ADR 0008), not database-write success.

## Consequences

- Easier: recovery onto a fresh install; data encrypted at rest.
- Harder: backup/restore must stay consistent with the apply contract and the per-server certificate model (ADR 0009).
- Now a rule: a restore never claims success on database write alone; portability is tested, not assumed.

## Evidence

`internal/services/backup.go`, `internal/services/portable_backup.go`; beta-exit backup-and-recovery (blocker 8). See [../evidence/0010-portable-backup-format.md](../evidence/0010-portable-backup-format.md).
