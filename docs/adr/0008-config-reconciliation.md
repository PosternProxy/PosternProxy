# 0008. Configuration uses a desired/applied model with per-server revisions and acknowledged apply

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made during the beta-exit program)
- **Deciders:** Michael (owner)
- **Topics:** architecture, configuration, agent-protocol

## Context

Configuration must be applied to local Caddy and to remote agents (ADR 0004) reliably, and the system must distinguish what was intended from what actually applied. Earlier code logged apply errors after reporting success and kept agent command state only in memory with no durable correlation.

## Decision

1. Configuration uses a **desired/applied** model with **per-server revisions**: `desired_revision` advances on a change; `applied_revision` advances only after a confirmed successful load.
2. The agent protocol carries **command and revision identifiers**; agents acknowledge apply results with **durable correlation**, and stale commands/revisions are ignored.
3. Apply outcomes are reported **truthfully** (no false "synced"); history/rollback and backup restore operate on this model.

## Consequences

- Easier: reliable multi-server reconciliation with an auditable status (`queued`/`applying`/`applied`/`failed`).
- Harder: the protocol, database state and UI all carry the revision/ack contract.
- Now a rule: a save or restore reports real apply state; never mark desired state applied without a confirmed load.

## Evidence

`internal/services/caddy_sync.go`, `internal/services/config_state.go`, `internal/agent/protocol.go`, `internal/agent/hub.go`; beta-exit config-lifecycle (blockers 5/7/11). See [../evidence/0008-config-reconciliation.md](../evidence/0008-config-reconciliation.md).
