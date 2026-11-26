# E. Certificate isolation and service access

Status: planned. Lead: Opus 4.8 provisionally; GPT6.1 fallback. Independent review: Grok 4.7 provisionally. Parent: [development index](beta-exit-development-index.md).

Owns blockers **9 and 10**. Keep certificate selection, delivery, activation, and retirement together. The assignment reflects security-sensitive key ownership and cross-process lifecycle reasoning, not a verified model benchmark.

## Current evidence and scope

`internal/services/caddy_sync.go` loads all certificates and bundles custom PEM/key bytes for every remote server. `internal/caddy/tls.go` emits all supplied custom certs. Controller upload and agent writes use private directories/files, while installers run Caddy as a different user without a compatible access contract. Agent service sandbox paths also conflict with the certificate destination.

Touch points: sync/build input, TLS builder, certificate service/handlers, agent cert writer, shared internal filesystem helpers, installers/service units and container volumes, certificate and sync tests. C owns apply revisions; D owns final installer integration; F owns archive encryption/restore.

## Development steps

- [ ] Define the necessary certificate set from enabled target-server hosts across all host types that actually use custom TLS. Cover shared cert references, redirects/dead hosts, disabled/moved/deleted hosts, and TLS passthrough. Filter before both config generation and payload creation, including local Caddy.
- [ ] Fail visibly if a required certificate/key cannot be read or validated; do not silently skip it and present successful application. Validate uploaded PEM/key pairing and reject unsafe inputs before activation.
- [ ] Choose and document one narrowly scoped access mechanism for Caddy (dedicated shared group/ACL or equivalent supported loader). Specify parent-directory traversal, key read permissions, ownership, write permissions, systemd sandbox paths, container mounts, and existing-install migration. Other users must remain unable to read keys; Caddy must not gain broad controller-data access.
- [ ] Stage complete validated cert/key pairs atomically in managed directories, with safe paths and symlink protections. Put filesystem logic shared by controller/agent in an `internal/` package. Preserve the project's sensitive-data encryption requirements and document any unavoidable plaintext runtime material for Caddy with restricted access.
- [ ] With C, define a revision-scoped certificate manifest and activation/cleanup ordering: stage, load, acknowledge, then retire unreferenced files only after successful activation. Failed apply must preserve the material required by the last applied configuration.
- [ ] Reconcile old excessive copies safely; removing future distribution does not erase previously delivered keys. Document incident/rotation guidance for keys exposed to unnecessary agents. Redact material from all logs, audit, history, and UI responses.
- [ ] Integrate D's installer changes and F's restore writer through the same access/staging mechanism. Update documentation and changelog.

## Regression and acceptance

- [ ] Two-server fixture: A receives only A's required certs; B receives only B's; a server with no references receives zero private keys. Inspect both generated config and serialized messages for unrelated key sentinels.
- [ ] Test shared certs, every TLS-bearing host type, disabled resources, moves/deletes, unavailable files, invalid PEM/mismatched keys, failed writes, interrupted staging, and malicious filesystem paths/symlinks.
- [ ] On real supported Linux/systemd and Docker deployments, actual Caddy users can traverse/read required material and load uploaded certs; an unrelated user cannot read keys. Agent/controller restart and cert replacement retain correct permissions.
- [ ] Failed apply retains previous usable keys; successful removal eventually deletes stale managed copies. No cleanup touches paths outside the managed certificate root. Confirm the served TLS certificate over traffic, not just file existence.
- [ ] Run shared checks and record permissions, build identities, redacted payload assertions, traffic tests, and review outcome.

Dependencies: selection can start immediately. Freeze access contract before D finalizes units and before F restore writes. C's correlated acknowledgment is required for safe retirement.

Completion evidence: pending. Both distribution isolation and actual service-user access must pass before closure.


## 2026-11-24 implementation checkpoint

Implementation and local regression/traffic evidence are tracked in
[Next beta fixes](beta-next-fixes.md). Native installers, encrypted certificate
storage/runtime distribution, explicit enrollment, truthful provisioning readiness
and portable encrypted recovery are implemented. Local pinned-Caddy TLS and
cross-key clean recovery, two-agent restart, browser smoke and Go checks pass.
This is not closure of the target-VM/container/published-release acceptance gates
above. Preserve the reported successful controller VM install as separate evidence.
