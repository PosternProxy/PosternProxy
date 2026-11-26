# F. Portable backup and clean restore

Status: planned. Lead: Opus 4.8 provisionally; GPT6.1 fallback. Independent review: Grok 4.7 provisionally. Parent: [development index](beta-exit-development-index.md).

Owns blocker **8**. C owns truthful apply state, including the restore false-success behavior from blocker 7. This assignment reflects recovery format, secret handling, and database/filesystem consistency decisions.

## Current evidence and scope

`internal/services/backup.go` exports hosts with server references but omits servers, carries certificate paths without key material, does not enforce supported version semantics beyond a nonempty version, and invokes local-only sync. Its handler returns `synced: true` after database restoration even when application failed. The existing local restore test does not establish portability to a clean installation.

Touch points: backup service/handler/tests, models and validation, encryption helpers, certificate storage, server enrollment/mapping, frontend Backup view/types, recovery docs, migration/version fixtures.

## Development steps

- [ ] Define a versioned portable configuration-recovery format with server descriptors, stable references, required custom certificate bytes, bounded sizes, and explicit exclusions. Exclude users/auth sessions, API/join tokens, SSH secrets, and agent authentication by default. State whether ACME state is recreated or separately recovered; do not promise full machine recovery from configuration alone.
- [ ] Specify explicit old-server to target-server mapping. Preserve the clean controller's local identity; recreated remote records remain unenrolled until the operator re-enrolls agents. Never blindly restore a remote credential or send restored config to the wrong existing server.
- [ ] Protect sensitive archive material using the project's AES-256-GCM helpers with a documented portable key/passphrase and recovery procedure. A fresh installation must not need the lost controller's only encryption key. Do not expose private keys in ordinary JSON, logs, audit records, or unprotected temporary files.
- [ ] Fully validate version, schema, allowlisted fields, relationships, mappings, cert/key pairs, bounds, and archive authentication before destructive work. Unknown formats fail safely. Define legacy v1 support or an explicit safe rejection/migration path; missing PEMs must not be silently fabricated.
- [ ] Use E's safe cert staging and a recoverable database/filesystem commit sequence. Record restore job state and define rollback/cleanup for validation failure, disk error, transaction failure, crash before/after commit, and retry. Preserve existing users and avoid overwriting local security settings with unsafe archived values.
- [ ] After commit, reconcile every mapped target via C. Report database restore success separately from local/remote queued, applied, failed, or waiting-for-enrollment results. Expose partial application and authorized retry; never return unconditional sync success.
- [ ] Audit restore actor, source format, safe counts/mapping summary, and outcomes without secret bytes. Update UI copy, format/recovery docs, upgrade policy, and changelog.

## Regression and acceptance

- [ ] Export a controller with local and multiple remote host types, ACLs, and a custom cert. Restore into a fresh installation with different IDs, storage paths, and encryption secret; explicitly map/re-enroll and prove local/remote traffic and served TLS.
- [ ] Test conflicting IDs, absent mappings, offline agents, invalid/mismatched certs, unknown versions/tables/columns, corrupt ciphertext, wrong passphrase/key, oversized input, and malformed relations. Invalid input leaves DB/files unchanged.
- [ ] Inject file/disk and transaction failures, then restart at each restore phase. Retrying completes or restores the documented previous state without missing active certs or half-restored references.
- [ ] Restore cannot resurrect old user/session/agent credentials or weaken management security settings. An unsupported legacy backup fails with a useful, non-destructive message.
- [ ] Reject local/remote Caddy applies and assert API/UI partial status is truthful. Run shared Go/frontend checks and G's clean recovery acceptance; record actual artifacts and recovery instructions.

Dependencies: G migration/compatibility policy; E storage and secure staging; C status/reconciliation contract. Format validation and fresh-database fixtures can begin before integration.

Completion evidence: pending. A same-database round trip alone cannot close this blocker.


## 2026-11-24 implementation checkpoint

Implementation and local regression/traffic evidence are tracked in
[Next beta fixes](beta-next-fixes.md). Native installers, encrypted certificate
storage/runtime distribution, explicit enrollment, truthful provisioning readiness
and portable encrypted recovery are implemented. Local pinned-Caddy TLS and
cross-key clean recovery, two-agent restart, browser smoke and Go checks pass.
This is not closure of the target-VM/container/published-release acceptance gates
above. Preserve the reported successful controller VM install as separate evidence.
