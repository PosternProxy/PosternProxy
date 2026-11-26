# D. Installation and SSH provisioning

Status: planned. Lead: Sonnet 5.5 provisionally; GPT6.1 fallback/reviewer for readiness integration. Parent: [development index](beta-exit-development-index.md).

Owns blocker **6** and Docker login/cookie defaults. The assignment is for concrete installer, environment, and service-unit fixes with VM acceptance tests; it does not assume verified capabilities of this exact model version.

## Current evidence and scope

Controller install writes `POSTERNPROXY_AGENT_BIN_PATH`; config reads `POSTERNPROXY_AGENT_BIN`. Systemd provisioning uploads an empty placeholder if no binary exists, omits required server identity, references an uncreated Caddy JSON file, builds only the L4 plugin, suppresses startup failures, and resets UFW while allowing the default SSH service. Script completion is currently sufficient for success. Current scripts retain OS-forwarding packages and agent capabilities despite the Caddy-only direction.

Touch points: `internal/provisioner/`, `internal/services/provision_run.go`, config, app/handlers and Servers UI readiness, `scripts/install.sh`, `scripts/install-agent.sh`, build scripts, Dockerfiles/Compose/entrypoints, deployment docs.

## Development steps

- [ ] Standardize agent binary configuration; support the old env alias only with an explicit documented transition. Fail before modifying a remote host when the binary is missing, empty, incompatible with target architecture, or inaccessible. Remove the placeholder path.
- [ ] Pass and validate server ID, controller URL, and all required settings for systemd and Docker. Use safe shell/environment encoding: Go `%q` alone must not be assumed to prevent shell interpolation. Never echo credentials into provisioning logs; use protected temporary files and cleanup.
- [ ] Bootstrap valid minimal Caddy config, ensure its admin endpoint is local/protected, and fail visibly on service errors. Use the same pinned Caddy, L4, and rate-limit plugin contract as G; verify existing Caddy installations have the required modules too.
- [ ] Preserve existing firewall rules; allow the configured SSH port before any firewall enablement and verify continued access. Remove destructive UFW reset. Test default and nonstandard SSH ports. Keep agent access outbound-only.
- [ ] Apply E's certificate directories, narrowly scoped access, service-user/group, and sandbox writable-path contract. Remove obsolete OS-forwarder packages/capabilities only after checking actual remaining uses; keep only privileges required by Caddy and the agent.
- [ ] Distinguish installing, waiting for authenticated agent, waiting for first successful applied revision, ready, failed, and timed out. Readiness must be based on B-authenticated connection and C's apply acknowledgment, with deadlines and actionable redacted errors.
- [ ] Resolve documented Docker HTTP login behavior explicitly: prefer documented TLS deployment, and if HTTP mode is supported, configure that mode deliberately with clear cookie semantics rather than disabling secure cookies globally. Verify CSRF/login/refresh/mutation in a real browser.
- [ ] Make re-provisioning safe and repeatable; preserve host-key verification and encrypted stored SSH credentials. Audit privileged provisioning and update docs/changelog.

## Regression and acceptance

- [ ] Unit/script tests cover env compatibility, missing binary, server identity, hostile shell metacharacters, module validation, startup failure, and redacted progress/errors. Run shell syntax/lint checks where supported.
- [ ] Fresh supported Linux VMs: controller installation, standalone agent installation, and UI-triggered systemd provisioning all reach authenticated-and-applied readiness. Repeat with SSH on a nonstandard port and existing firewall rules; both access and rules survive.
- [ ] Test Docker controller/agent deployment, published image login, refresh, CSRF mutation, agent reconnect, custom cert apply, and service restart. Match real release artifacts, not only local developer binaries.
- [ ] Missing plugin, bad controller URL, invalid token, unavailable Caddy, rejected first apply, timeout, and controller restart never produce false ready. Repeat provisioning on the same host without destructive side effects.
- [ ] Record VM OS/architecture, artifact versions, command results, traffic verification, and redacted logs. Run shared Go/frontend checks when their code changes.

Dependencies: corrective script work can start immediately; final acceptance requires B transport, C acknowledgment/status, E permissions, and G's supported-platform/build contract.

Completion evidence: pending. Source inspection and shell syntax checks alone cannot close this blocker.

G handoff (2026-10-24): pinned Caddy builds/module checks are now shared with
provisioning. govulncheck reports seven called-symbol SSH advisories in x/crypto
v0.50.0; v0.56.0 covers the newest reported fixes. Include compatible dependency
updates and provisioning regression checks; see [G checkpoint](beta-exit-g-foundation-checkpoint.md).


## 2026-11-24 implementation checkpoint

Implementation and local regression/traffic evidence are tracked in
[Next beta fixes](beta-next-fixes.md). Native installers, encrypted certificate
storage/runtime distribution, explicit enrollment, truthful provisioning readiness
and portable encrypted recovery are implemented. Local pinned-Caddy TLS and
cross-key clean recovery, two-agent restart, browser smoke and Go checks pass.
This is not closure of the target-VM/container/published-release acceptance gates
above. Preserve the reported successful controller VM install as separate evidence.
