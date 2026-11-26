# Beta-exit blocker closure ledger (S1)

Updated: 2026-11-25. Source of record for Gate 1 of the
[broader beta release execution plan](broader-beta-release-execution.md).
Blocker definitions: [beta-exit review 2026-09-18](beta-exit-review-2026-09-18.md).
Group ownership: [development index](beta-exit-development-index.md).

Fresh independent source review covered the authority, lifecycle, provisioning,
installer and release changes. Two additional findings were fixed: agent API
revocation did not invalidate the transport credential, and portable export could
mix database snapshots. Both fixes received independent follow-up review with no
actionable findings. Exact-candidate supported Linux race/CI is still pending;
no blocker is marked fully closed on the strength of local checks alone.

## Closure legend

- Fix: implemented in the working tree (uncommitted, part of the larger diff).
- Regression: named test reproduces the original failure and passes locally.
- Review: independent reviewer sign-off — **none recorded for any blocker**.
- State: `fix+regression` = both present, awaiting review/Linux-CI; not closed.

## Ledger

| # | Blocker (P1) | Group | Fix anchor | Regression test(s) | State |
|---|--------------|-------|-----------|--------------------|-------|
| 1 | Read-only API tokens can obtain write access | A | Context-carried claims + `RequireManagementScope`/`RequireScope` (internal/auth/middleware.go:85-107, internal/api/router.go:138); scope validation in internal/services/api_token.go | `TestReadTokenCannotMutateThroughRouter`, `TestAuditLogRejectsNonAdmin` (internal/api/authorization_test.go) | fix+regression |
| 2 | Disabled/demoted users retain token authority | A | Resolver checks current owner status/role; revocation semantics; session rotation/logout (internal/services/api_token.go, internal/auth/jwt.go, middleware.go) | `TestAuthorityChangesRevokeExistingCredentials`, `TestSessionRotationAndLogoutThroughRouter` (authorization_test.go); `TestMiddleware_RefreshTokenRejected` (internal/auth/middleware_test.go) | fix+regression |
| 3 | Client-supplied forwarding headers bypass rate limits | B | Trusted-proxy boundary + canonical client_ip (internal/caddy/proxy_trust.go); controller-owned trusted-proxies map | `TestProxyPolicyValidation`, `TestGeneratedClientIdentityAndLimits` (internal/caddy/proxy_trust_test.go); `TestPinnedCaddyClientIdentityAndStreaming` (proxy_trust_live_test.go) | fix+regression (live) |
| 4 | Agent auth secret reaches controller request logs | B | Token moved to Authorization header; consistent redaction (internal/api/handlers/servers.go, cmd/posternproxy-agent/main.go) | `TestAgentHeaderAuthentication` (internal/api/agent_transport_test.go) | fix+regression |
| 5 | Any access-list row blocks full config generation | C | Outer ACL query released before loading clients; one-connection pool retained; ACL mutations dirty their servers (internal/services/caddy_sync.go) | `TestAccessListsDoNotHoldTheOnlyConnection` (internal/services/config_state_test.go) | fix+regression |
| 6 | SSH provisioning cannot reliably produce a working agent | D | Shared installer, env-var/arch validation, readiness = authenticated + applied revision (internal/provisioner/*, internal/installer/*) | `TestValidateInput*` ×5 (validation_acceptance_test.go), `TestRemoteArchitecture` (architecture_test.go), `TestProvisionReadinessRequiresAuthenticatedAppliedRevision`, `TestManualEnrollmentReturnsCredentialOnlyOnceAndRotationRevokes` (beta_fixes_test.go) | fix+regression; VM-qualified 11-24 |
| 7 | Failed Caddy application reported as success | C | Desired/queued/applied/failed state (internal/database/migrations/003_config_state.sql); `ConfigApplyError`; handlers return 502 `saved:true` | `TestRejectedConfigurationIsSavedButNotApplied`, `TestRollbackReportsSavedButUnapplied`, `TestDirtyRevisionRollsBackWithTheMutation` (config_state_test.go) | fix+regression |
| 8 | Backup export insufficient for disaster recovery | F | Versioned passphrase-encrypted portable format with PEM bytes + explicit server mapping (internal/services/portable_backup.go) | `TestPortableRecoveryAcrossKeysAndExplicitServers`, `TestLegacyBackupRejectsUnknownVersionAndTables`, `TestPortableRecoveryAuthenticatedMalformedPayloadCannotMutateDestination` (portable_recovery_acceptance_test.go); `TestLiveCustomTLSAndPortableRecovery` | fix+regression (live) |
| 9 | Every agent receives all uploaded TLS private keys | E | Per-server certificate selection; no cross-server leakage (internal/certstore/*, internal/services/caddy_sync.go) | `TestCaddyTrustDoesNotLeakAcrossServers` (proxy_policy_test.go), `TestCertificateAcceptanceSelectsOnlyTargetTLSReferences`, `TestCertificateAcceptanceMissingReferenceFails` (certificate_acceptance_test.go) | fix+regression |
| 10 | Uploaded cert permissions conflict with service users | E | Immutable runtime pairs, 0640 + Caddy cert group, symlink/traversal guards (internal/services/certificate.go, certstore) | `TestCertificateSourcesEncryptedAndApplyFailurePreservesRuntime`, `TestRuntimeManifestRejectsUnreferencedAndUnsafeMaterial`, `TestRuntimeRefusesSymlinkAndRepairsTraversalMode`, `TestCertificateRejectsExtraPrivateMaterial` (certificate_runtime_test.go) | fix+regression; VM perms verified 11-24 |
| 11 | Pending remote updates lost across controller restart | C | Durable per-server revisions + reconcile on every connect and startup (003_config_state.sql, CaddySyncService) | `TestDirtyRevisionSurvivesServiceRestartAndMove`, `TestReconnectRebuildsDurableOfflineWorkAfterControllerRestart` (config_state_test.go); `TestTwoAgentsConvergeAfterControllerRestart` (config_acceptance_test.go) | fix+regression |

## Remaining to close each blocker

1. **Independent review** � completed 2026-11-25 by fresh read-only agents
   `beta_authority_final_review`, `beta_lifecycle_final_review` and
   `beta_release_final_review`. Authority and backup findings were fixed and
   independently re-reviewed. These are source reviews, not owner publication
   approval. Final regression evidence and release steps are in
   [today's runbook](beta-release-runbook-2026-11-25.md).
2. **Linux race/CI execution** — C's checkpoint notes GCC was unavailable locally
   and several reconciliation/acceptance tests ran Windows-only. The S3 CI (race +
   govulncheck) must run green on Linux against these regressions before closure.
   Blockers 7 and 11 (concurrency/restart) depend on this most.
3. **Per-blocker residuals**
   - #2: human password/role/disabled credential regressions and review passed.
     Agent owner authority now also revokes its transport credential, with public
     token-DELETE, preflight, reconnect and existing-connection regressions.
   - #6, #10: VM-qualified on Ubuntu 24.04 amd64 (11-24 evidence); other
     platforms/arch are gate-3 scope, not blocker closure.
   - #4: confirm rollout handling of previously-written query-token credentials.

## Closure procedure

A blocker moves to closed only when: its regression passes on Linux CI, an
independent reviewer signs off its fix, and a closure entry is recorded here and
in the development index. When all 11 are closed, Gate 1 of the broader beta plan
is satisfied (audit coverage S2 is tracked separately under the same gate).
