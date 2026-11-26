# Plan C implementation checkpoint — 2026-10-25

Status: core implementation and local acceptance complete; release qualification remains open. Parent: [Plan C](beta-exit-config-lifecycle.md). No subagents, commits, deployments, or external model reviews were performed.

## Scope and contracts

The checkout contains pre-existing forwarding work, Plan G foundations, and A/B work from the previous instruction. These were retained. This checkpoint focuses on C and does not mark A/B complete. It does not change licensing, introduce an inbound agent listener, or add a controller runtime dependency.

- Forward migration `003_config_state.sql` persists per-server desired/applied revisions and queued/applying/applied/failed status. Mutation triggers dirty state within the same transaction, including old/new servers on moves and resource deletion. Referenced ACL changes dirty their servers; certificates currently dirty all servers. The frozen initial SQL was not changed further.
- ACL loading releases the outer query before querying clients, retaining the production one-connection pool. Scan/iteration/close/cancellation failures propagate.
- `CaddySyncService` owns per-server serialization, durable-state reconciliation, retries, timeout tracking, and correlated result handling. Offline servers are queued before reading/building payloads. Reconnect always reconciles; startup does too. The worker polls every five seconds, gives failures a 30-second cooldown, times out unacknowledged commands after roughly 30 seconds, and periodically resynchronizes applied state after a minute. This also recovers a Caddy process restart without a user edit.
- Protocol envelopes carry opaque command identity and desired revision. Agents echo them only after certificate writes and Caddy application return. Failed, uncorrelated, stale/duplicate, and replaced-connection results cannot advance the latest applied revision. Queue saturation/write failure does not erase desired work.
- Unified full-server synchronization replaces the old per-host incremental fallbacks. Host services preserve saved data and return a sanitized `ConfigApplyError` on synchronous failure. HTTP handlers return 502 with `saved: true` and server identity. Async remote application is read through `/api/configuration`. Manual resync returns current state with 200/applied or 202/pending and is audited.
- The shared UI notice shows queued/applying/failed changes and an admin retry action. Server resync wording acknowledges the request without falsely claiming applied success. Restore reports `synced` separately from record restoration; its state list includes remote work. Nginx import counts saved records even if application subsequently fails. Rollback uses the same desired/apply error contract.
- Safe host audit snapshots omit route bodies, custom headers, and other credential-bearing configuration. Rollback snapshots are encrypted with existing AES-256-GCM helpers. Startup protects old plaintext rows in resumable batches; history listings omit snapshot contents, while rollback decrypts internally. Recovery must preserve the existing controller encryption secret.
- Frontend refresh requests are deduplicated so the polling component and concurrent requests share one rotated refresh. Retries are bounded. This integrates with the earlier A session work rather than bypassing it.

## Verification evidence

| Check | Result and practical limits |
| --- | --- |
| Required Go tests/build/vet | Passed after implementation; final supplement below |
| Frontend type-check/build/lint | Passed |
| Chromium smoke | Both scenarios passed: auth refresh/logout and saved host with visible apply failure/retry |
| Bounded ACL regression | Passed with unused/empty/multiple lists, clients, and canceled context under one DB connection |
| HTTP rejection and agent result tests | Passed: desired data retained, applied revision unchanged, error sentinels omitted |
| Revision/queue/connection tests | Passed: wrong IDs/revisions/kinds, duplicate results, newer desired state, timeout/cooldown retry, saturation, replaced connection, writer/reader shutdown |
| Mutation/recovery tests | Passed: transaction rollback preserves revision, host move reconciles old/new servers, all four host types propagate disable/delete failures and retain desired data, enabling recovers, encrypted rollback and restore return saved-but-failed |
| Audit/history tests | Passed: safe before/after fields, encrypted new/legacy history, resumable protection, metadata-only list, decrypted rollback payload |
| Native two-agent acceptance | Passed on Windows with real agent binaries and pinned Caddy v2.11.4/L4 v0.1.1/rate-limit v0.1.0 |
| Documentation build | Hugo passed; existing shortcode deprecation warnings remain |

The native acceptance fixture uses isolated loopback listener ports. It preserves generated routes/handlers but remaps listeners and disables automatic TLS for HTTP. The local controller Caddy endpoint is a fixture; two remote Caddy processes are real. The test proves HTTP routing on both agents, live ACL allow/deny with unrelated-server revision isolation, a real Caddy process restart, an offline edit, controller shutdown/database reopen at the same address, both agent restarts, latest-state convergence, and route deletion. The periodic Caddy-restart test advances the stored last-check time rather than waiting a full minute. These are focused lifecycle tests, not Linux/systemd/container/TLS qualification.

## Upgrade and integration limits

Controller and agent upgrades must be coordinated with the earlier B header-auth change and C's correlated acknowledgments. Older query-token agents cannot authenticate, and missing command identity/revision cannot establish success. New schema changes are forward migrations; old beta schema qualification remains G work. Returning `applied` confirms Caddy accepted configuration, not upstream availability or outside firewall reachability.

E still owns certificate selection/isolation, actual service-user permissions, atomic staging/retirement, and their acceptance tests. C already fails required file reads/writes and never acknowledges before load succeeds; this does not close E's blockers. F still owns portable recovery; truthful apply reporting does not make the current backup format portable.

## Remaining qualification

- Run the added native acceptance and Linux race checks in CI. No GCC was available locally; Windows acceptance does not substitute for Linux race/systemd/container evidence.
- Expand slow-agent/concurrent-write stress and explicit database/filesystem crash cuts. Existing CRUD tests, all-host disable/delete failure cases, and focused lifecycle tests cover the principal transitions but are not exhaustive combinations.
- Integrate and validate E's certificate lifecycle contract, including permission failures and safe retirement after a successful revision.
- Retain the dependency/security findings and release gates in G; this checkpoint is not a vulnerability waiver or beta-exit decision.
- Obtain independent review before final closure. All remaining plans stay active; C stays in `.agentplans/` until its required integration evidence is recorded.

Final verification supplement: Go tests/build/vet passed after the final
implementation; frontend type-check/build/lint passed; both Chromium scenarios
passed with the final startup/history path; the complete pinned-Caddy and
two-agent acceptance suite passed. Hugo passed. Plan links and scoped whitespace
checks passed. The exact pre-existing `health.go` trailing blank line was restored
and is the sole global whitespace finding. No implementation work was committed.

[Artifact metadata](beta-exit-c-artifacts-2026-10-25.json) records native binary
hashes and selected source hashes at this checkpoint. It identifies the tested
working tree but is not a standalone source snapshot or release attestation.
