# C. Configuration application and reconciliation

Status: core implementation and local regression acceptance complete; cross-plan release qualification remains open (2026-10-25). Lead: GPT6.1. Review: Grok 4.7 provisionally, GPT6.1 fallback. Parent: [development index](beta-exit-development-index.md).

Owns blockers **5, 7, and 11**. Treat them as one desired/applied-state lifecycle. The model assignment reflects database, protocol, concurrency, API, and UI coupling.

## Current evidence and scope

`loadAccessLists` in `internal/services/caddy_sync.go` issues child queries while outer rows remain open; `internal/database/database.go` allows one connection. Access-list service mutations have no sync dependency. Host services log apply errors after saving. `internal/agent/hub.go` keeps pending state only in memory and logs command results without durable correlation. `internal/agent/protocol.go` has no command/revision identifiers. Startup sync targets only local Caddy. Backup reports sync success independently of the actual apply outcome.

Touch points: sync, all host/access-list/certificate-dependent mutation paths, app wiring, hub/protocol, agent command handling, migrations/models, history/rollback, server and resource status APIs, shared frontend types/composables and affected views. F owns backup format and restore orchestration; C owns its shared apply-result contract.

## Development steps

- [x] Fix the access-list single-connection deadlock; parent rows are closed before client queries, with row/cancellation errors propagated.
- [x] Persist desired/applied revisions and sanitized state via forward migration 003 and transaction-coupled SQLite triggers. Rollback of a mutation also rolls back its revision.
- [x] Separate saved/queued/applying/applied/failed states through `/api/configuration`, typed saved-but-failed HTTP 502 results, correlated resync results, and truthful restore `synced` plus state data. Server state survives resource deletion.
- [x] Correlate command identity and revision; reject mismatched, stale, duplicate, and replaced-connection results. Agents acknowledge after file writes and Caddy application. Document coordinated controller/agent upgrades.
- [x] Reconcile at startup and every authenticated connection; retry every five seconds with a 30-second failure cooldown/ack timeout and serialize per server. Periodically resync previously applied state after one minute to recover Caddy restarts. Saturated/failed connections retain database work.
- [x] Track host create/edit/move/enable/disable/delete, referenced access-list mutations, and certificate mutations with shared synchronization. ACLs dirty referencing servers; certificates currently dirty all servers pending E's distribution isolation.
- [x] Add shared UI status/retry, audit retry requests and safe before/after host fields, encrypt existing/new rollback snapshots, omit snapshot contents from listings, and retain internal decryption for rollback. Restore and imports distinguish saved data from apply errors.
- [ ] Coordinate certificate staging/activation with E; never acknowledge a revision before required cert files and config are usable. Update docs and changelog.

## Regression and acceptance

- [x] Bounded ACL regression passes under the production one-connection pool; real two-agent traffic changes between allow/deny without dirtying an unrelated server.
- [x] HTTP 400 rejection, unavailable Caddy, and negative agent results retain desired data and unchanged applied revisions. Browser/API tests show saved-but-failed state and retry.
- [x] Offline edit, controller/database reopen, reconnection, Caddy process restart, mismatched/duplicate results, lost-ack timeout/retry, queue saturation, writer shutdown, and both agent restarts have local regression coverage. Crash-before-dispatch is represented by persisted dirty state with no in-memory queue.
- [x] Existing CRUD tests plus lifecycle regressions cover create/edit, all four host types' disable/enable/delete failures, moves, encrypted rollback with failed apply, ACL updates, resync, and restore. Correlation tests prevent an old acknowledgment marking newer desired state applied. Broader stress/crash-cut combinations remain release qualification.
- [ ] With pinned Caddy and two agents, verify actual local/remote traffic changes and bounded eventual convergence after restart. Run Go/frontend checks and Linux race tests through G.

Dependencies: G's migration policy before schema commits. Initial deadlock fix is independent. Freeze revision/result contract before D/F integrate; coordinate shared sync/agent edits with E.

Completion evidence: [2026-10-25 checkpoint](beta-exit-c-checkpoint-2026-10-25.md). The original failure patterns in blockers 5/7/11 have passing local regressions and native two-agent HTTP/ACL acceptance. Keep this plan active until E's certificate integration, Linux race/CI and broader stress/crash qualification through G, and independent review are recorded. Do not archive it or declare beta exit from these local results.
