# NTLM passthrough for IIS — implementation spec

Date: 2026-11-26. Status: **Draft. The controller change is in the working tree and is not a release. IIS is not verified.**
Size: large because persistence, API, UI, builds, and local/remote configuration application intersect an authentication boundary.

## Task and authority

Add opt-in NTLM authentication passthrough to HTTP Proxy Hosts using the
project-maintained fork `github.com/PosternProxy/ntlm-transport` of the Caddy
`http_ntlm` module (see `.agentplans/completed/feature-ntlm-transport-fork-spec-2026-11-27.md`). IIS authenticates the user;
PosternProxy forwards the exchange without collecting credentials or becoming a
Windows identity provider.

The owner requested this spec after discussing legacy IIS compatibility. The
requirements below are proposed defaults, not a record of owner acceptance.
Obtain agreement on the acceptance criteria before implementation. This document
authorizes no deployment, publication, or changes to a live IIS environment.

For the implementer: follow the contracts and scope below literally. Inspect the referenced
code before editing. Do not replace the module with a homemade NTLM handshake,
broaden the feature into SSO, or report simulated handshakes as IIS qualification.
If evidence contradicts a requirement, report the specific conflict and proposed
resolution; do not silently relax authentication isolation or support claims.

## Outcome and scope

An administrator can enable **NTLM authentication passthrough** on a proxy host
pointing to one IIS application. A supported browser can authenticate to IIS
through that host, and concurrent users remain isolated.

Proposed first-version boundaries:

- Ship the pinned module in standard managed Caddy artifacts. Enable it per host,
  default off. No separate installation variant or subscription gate.
- One upstream per host, with HTTP or verified HTTPS to IIS.
- Direct browser-to-PosternProxy connections are the initial supported topology.
  Additional HTTP proxies/CDNs and multiplexing intermediaries require separate
  qualification; forwarded IP headers are not connection identities.
- No additional upstreams, path-specific locations, or TLS passthrough on an
  NTLM-enabled host. Reject conflicting settings instead of discarding them.
- Preserve the embedded Vue frontend, existing controller binary architecture,
  outbound-only agents, management authentication, and AGPL/commercial model.
- Do not add Kerberos delegation, AD enrollment, Windows credential storage,
  automatic IIS changes, or support for management login through NTLM.
- No claim of seamless upstream failover or NTLM load balancing.

## Verified repository baseline

These are code observations, not evidence that NTLM works in a deployment:

| Area | Existing home and implication |
|---|---|
| Caddy build contract | `internal/caddybuild/build.sh` pins core, plugins, toolchain and security replacements; builds and verifies required modules. Extend this contract. |
| Artifact verification | `internal/caddybuild/verify.go` checks embedded Go build metadata against pins. Extend dependency verification too. |
| Windows builds | `scripts/build.ps1` reads selected pin names and explicitly adds plugins. Update both parsing and build arguments. |
| Config generation | `internal/caddy/routes.go` creates the primary and location transports with protocol `http`; `internal/caddy/types.go` defines transport JSON. |
| Host persistence | `internal/models/proxy_host.go`, `internal/services/proxy_host.go`, and `internal/database/migrations/`. The service has explicit column lists, scans and writes. |
| API | `internal/api/handlers/proxy_hosts.go`; reuse existing permissions, validation mapping and resource routes. |
| Form and client types | `frontend/src/types/index.ts`, `frontend/src/api/proxyHosts.ts`, `frontend/src/views/proxy-hosts/form/useProxyHostForm.ts`, and `form/tabs/SecurityTab.vue` under the same views directory. |
| Backup | `internal/services/backup.go` contains explicit allowed columns; a database field alone does not ensure round trips. |
| Apply lifecycle | `internal/services/caddy_sync.go` and existing agent revision/result handling distinguish desired data from applied configuration. Preserve this. |

Relevant initiative context: [next-release security](next-release-security.md),
[configuration lifecycle](beta-exit-config-lifecycle.md), and
[stable release criteria](stable-release-exit-criteria-2026-11-26.md).
This is a separate next-release feature; do not add it as a prerequisite to the
existing stable-release gates.

## Product and API contract

1. Add boolean `ntlm_enabled` to host read/write representations, with database
   default false. Missing fields in existing records, old backups and historical
   snapshots mean false. Follow existing full-update semantics; do not introduce
   a separate partial-update contract.
2. The Security tab contains a labelled, keyboard-accessible checkbox and help:
   "Pass Windows authentication through to IIS. Requires a single upstream and
   compatible IIS Extended Protection settings. Uses more upstream connections."
   No credentials or domain fields are requested.
3. Create/edit/reopen preserves the setting. Conflicts show actionable inline
   errors. Keep entered values after errors; never silently clear other settings.
4. API validation rejects NTLM combined with nonempty `extra_upstreams`, nonempty
   `locations`, or `tls_passthrough=true`. Reject malformed types/JSON using the
   existing validation status and error format. A stored default `lb_policy`
   alone is not a conflict when there are no additional upstreams.
5. Authentication headers must reach their destinations unchanged. Reject custom
   header rules that can alter request `Authorization` or response
   `WWW-Authenticate`, including case variations and matching wildcard rules.
   Reject custom error pages for 401 on these hosts. Other policies remain active.
6. Reject nonempty advanced route JSON while NTLM is enabled, except the existing
   semantic empty form `{}`. Arbitrary merged route handlers must not replace or
   bypass the selected transport. Explain the conflict in the form/API error.
7. Apply the same validation to create, update, import and history restoration.
   Invalid input changes neither host state nor desired revision. Do not silently
   skip an invalid imported host or apply a partially altered configuration.
8. Ordinary HTTP hosts retain existing behavior, including settings that are
   disallowed only when NTLM is enabled. TLS trust verification is never weakened.

## Caddy and deployment contract

- Pin the project-maintained fork `github.com/PosternProxy/ntlm-transport` at a
  reviewed immutable commit compatible with the pinned Caddy/toolchain — current
  reviewed commit `66b6501bd51c704ebd714eda0d6888080f0ff7c4` on Caddy `v2.11.9`,
  resolved to a pseudo-version at build time per the fork spec. No `latest` or
  floating branch. Record the pinned revision, dependency changes, license/notices,
  and review evidence.
- Extend the shared build and artifact contracts to require
  `http.reverse_proxy.transport.http_ntlm` and the pinned dependency. Trace all
  consumers, including installer/provisioning, container, CI and acceptance test
  fixtures. Preserve existing security dependency replacements and plugin pins.
- Enabled hosts emit `transport.protocol = "http_ntlm"`; disabled hosts emit
  `"http"`. Preserve upstream TLS options and unrelated route protections.
- Use the existing Caddy apply/reconciliation mechanism locally and remotely.
  Do not add agent listeners, protocol fields, or capability negotiation merely
  to carry a flag already represented in generated Caddy JSON.
- An old/custom Caddy without the module must fail application visibly, retain
  the desired setting, and leave the applied revision unchanged. Never fall back
  to `http`, mark failed desired state applied, or remove working configuration
  as a recovery shortcut. Surface existing retry/recovery controls.
- Document coordinated Caddy upgrades on every target using the feature. A UI
  save alone does not prove a remote Caddy supports or applied the module.
- Disabling NTLM restores ordinary transport through the existing revision
  lifecycle. Existing authenticated sessions need not survive config changes.

## Security and compatibility review

The module maps a downstream connection address to a dedicated transport and
forces upstream HTTP/1.1 with keep-alive. That does not by itself prove a strict
one-to-one socket relationship under concurrency or safe behavior through an
HTTP/2/HTTP/3 intermediary. Inspect the pinned implementation and test actual
connections. Never key identity from `X-Forwarded-For` or a client-controlled header.

Review concurrent requests, failed/canceled dials and requests, idle expiry,
upstream disconnect, downstream disconnect, reload, and cleanup. Failure must not
expose another user's authenticated response. Investigate retained transports,
deadlocks and sustained resource growth; unresolved isolation or lifecycle defects
block shipping rather than becoming documented exceptions.

IIS Extended Protection and channel/service binding depend on TLS topology.
Document the tested HTTPS termination/re-encryption arrangement and exact IIS
settings. Do not recommend disabling Extended Protection or TLS verification as
the general solution. If secure required settings cannot work, narrow support
claims or recommend existing TLS passthrough instead.

Prove browser-side protocol behavior separately from upstream HTTP/1.1. If a
downstream protocol restriction is necessary, report its scope and obtain a
decision before changing shared listeners; do not downgrade every host globally.

Keep existing mutation authorization, CSRF, rate limits, ACLs, audit actor/IP and
before/after records. Include the boolean in safe history/audit representations.
Do not log Authorization values, challenges, cookies, credentials or raw
authentication captures. The toggle grants no additional management privileges.

## Acceptance criteria and verification

Each numbered criterion needs an actual test reference and result before being
marked complete. Author acceptance tests in a separate context before feature
implementation, using these outcomes rather than copying proposed implementation.
Keep intentional red results distinct from broken fixtures or missing tools.

| ID | Observable acceptance outcome | Required verification |
|---|---|---|
| AC1 | Upgrade and old-data restore leave existing hosts off and preserve their behavior. | Migration, old backup and historical snapshot fixtures; ordinary route regression. |
| AC2 | Authorized create/edit/read persists the setting; unauthorized writes are refused with no state change. | Service/API persistence, permissions and audit tests. |
| AC3 | Every conflicting configuration above is refused before mutation, including imports/rollback and header case/pattern variants. | Table-driven boundary tests and unchanged revision assertions. |
| AC4 | Enabling selects NTLM transport; disabling restores HTTP; verified HTTPS and unrelated ACL/rate-limit policy remain effective. | Generated JSON tests plus real pinned-Caddy validation and traffic. |
| AC5 | Every managed artifact includes the reviewed module; absent/wrong versions fail verification. | Build contract fixtures and native artifact/module checks on supported targets. |
| AC6 | Missing-module local/remote targets report failed application, preserve last working traffic and applied revision, and recover after upgrade/retry. | Existing lifecycle harness with real incompatible/compatible Caddy where practical. |
| AC7 | Valid IIS users authenticate as themselves; invalid users remain denied; multiple challenge headers and 401 exchanges survive forwarding. | Protocol fixture tests AND real IIS integration; assert identity, not only HTTP 200. |
| AC8 | Two different users, concurrent requests and reconnects never receive each other's authenticated identity/content. | Separate client connections, same source IP, connection reuse/cancellation, IIS identity endpoint and pinned Caddy. |
| AC9 | IIS restart, timeout, cancellation, idle expiry and Caddy reload fail safely and allow fresh authentication after recovery. | Real traffic with bounded failures; resource/cleanup inspection. |
| AC10 | The form preserves values through errors, shows conflicts, and reports saved-versus-applied state truthfully. | Frontend/browser coverage of create, edit, failed apply, retry and disabling. |
| AC11 | A sustained workload completes without identity leakage, hangs or unexplained growing retained connections. | Proposed baseline: 20 simultaneous clients using at least two identities for 10 minutes; record hardware, request mix, latency/errors and resources before/during/after idle cleanup. |

For AC7–AC11, record OS/IIS version, module/Caddy revision, client/browser version,
actual negotiated downstream protocol, upstream scheme, certificate arrangement
and Extended Protection policy. Cover local Caddy and an outbound agent target.
Exercise the production browser path, not just a handcrafted HTTP/1.1 client.
Never commit credentials, private keys or token-bearing captures.

If IIS is unavailable, code/fixture work may be reported as partial. Real IIS
criteria stay pending and the feature must not be advertised as verified support.
Resource thresholds beyond the proposed workload require a recorded test budget;
do not invent numerical results or call ten minutes of testing capacity proof.

## Delivery requirements

- Keep changes scoped to this feature. Reuse services, validation and frontend
  composables. Use a forward migration without rewriting historical migrations.
- Trace the field through canonical scans/writes, sync loading, audit/history,
  export/import, rollback and shared frontend types. Do not create parallel host
  implementations or duplicate controller/agent utilities.
- Update README support claims, relevant proxy-host/upgrading docs, and
  `CHANGELOG.md` Unreleased when implementing. Preserve all licensing policy;
  record dependency notices through the project's existing practice.
- Run `go test ./...` and `go build ./...`; from `frontend/`, run
  `npm run type-check` and `npm run build`. Run affected build-contract,
  browser, real-Caddy and IIS acceptance checks; use Linux race testing for the
  exercised concurrent paths where supported.
- Report exact commands, actual outcomes, changed files, pinned revision,
  criterion-to-test mapping, limitations and outstanding release gates.
  Passing unit tests are not deployment evidence. Do not mark this spec Built
  until required criteria have evidence. Move it to `.agentplans/completed/`
  only after the work and required verification are complete.

## Open decisions and source references

No unanswered question prevents reviewing this draft. The proposed first-version
boundaries and workload above still need owner acceptance. Exact dependency pin,
secure IIS compatibility matrix, and downstream protocol behavior are technical
qualification gates; do not guess them. If qualification requires a persistent
architecture change, record a Proposed decision in `.agentplans/` before the
dependent change and obtain resolution.

- [Fork module source](https://github.com/PosternProxy/ntlm-transport/blob/66b6501bd51c704ebd714eda0d6888080f0ff7c4/ntlm.go)
  — the pinned revision. Forked from [caddyserver/ntlm-transport](https://github.com/caddyserver/ntlm-transport);
  the fork fixes the RoundTrip lock leak and the map-entry leak and sets
  `MaxConnsPerHost = 1`. Keep-alive stays enabled (the upstream README's "disabled"
  claim is wrong and is corrected in the fork).
- [Caddy module documentation](https://caddyserver.com/docs/json/apps/http/servers/srv0/routes/handle/reverse_proxy/transport/http_ntlm)
  — transport purpose, connection mapping and nonstandard-module status.
- [Microsoft IIS Extended Protection](https://learn.microsoft.com/iis/configuration/system.webserver/security/authentication/windowsauthentication/extendedprotection/)
  — channel/service binding and proxy/TLS topology requirements.

Implementation revision: working tree only, not released. IIS evidence: not collected.
Owner acceptance of the boundaries and AC11 workload: pending. Status remains Draft.

## Implementation record (2026-11-27)

The owner ordered execution of this draft. It is not marked Built, and this file
stays in `.agentplans/` until IIS qualification is recorded.

Pinned module: `github.com/PosternProxy/ntlm-transport` `v0.0.0-20261127162849-66b6501bd51c`
(`http.reverse_proxy.transport.http_ntlm`). The controller `go.mod` does not import it.

Author checks, from the repository root with `GOTOOLCHAIN=go1.26.11`:

- `go test ./...` exited 0.
- `go build ./...` exited 0.
- From `frontend/`: `npm run type-check` exited 0 and `npm run build` exited 0.
- From `frontend/`: `npx playwright test e2e/hosts.spec.ts -g "NTLM"` passed
  (create, saved-but-not-applied, retry uses update, edit shows the checkbox,
  disabling it persists, a location conflict keeps the entered path and writes
  no host). The e2e Caddy endpoint is dead, so that 502 text is the generic
  saved-but-not-applied sentence.
- `go test -race -count=1 ./...` passed for the fork and for this controller
  on WSL (Ubuntu, gcc 11.4.0, `go1.26.11`, `CGO_ENABLED=1`). No race was reported,
  so the pin was not changed.

Criterion mapping, not a completion mark:

- AC1: `TestBaselineUpgradePreservesData`, `TestOldBackupOmitsNTLMAndStillRestoresLocations`, `TestNTLMRollbackRestoresFlagAndRefusesConflicts` (snapshot without the field), `TestDisabledNTLMRouteStaysHTTP`.
- AC2: `TestNTLMCreatePersistsFlagAndEmptyContainers`, `TestNTLMCreateReportsSavedWhenModuleIsMissing`, existing `TestReadTokenCannotMutateThroughRouter`. Audit allowlist includes only `ntlm_enabled`.
- AC3: `TestValidateNTLMPassthrough`, `TestNTLMRejectedUpdateDoesNotMutateOrRecordHistory`, `TestNTLMRejectedCreateWritesNothing`, `TestBackupRejectsIllegalNTLMFlagsWithoutReplacingHosts`, `TestPortableNTLMRoundTripAndConflictLeavesDestination`, rollback conflict in the app test.
- AC4: generated JSON in `internal/caddy/ntlm_route_test.go`. No live pinned-Caddy traffic in this pass.
- AC5: build-contract fixtures under `internal/caddybuild`. No new artifact was built in this pass.
- AC6: `TestApplyFailureSentenceClassifiesOnlyTheNTLMModule`, `TestSyncServerStoresNTLMModuleSentenceWithoutApplying`, `TestAgentNTLMRejectionStoresSentenceOnly`, `TestPoisonedNTLMRowBlocksApplyAndKeepsGenericSentence`. No real incompatible or compatible Caddy process was used.
- AC7, AC8, AC9, AC11: pending. No IIS host, browser, Extended Protection policy, or workload result is recorded.
- AC10: `frontend/e2e/hosts.spec.ts` plus the API 502 body that includes `id`.

Passing these tests is not deployment evidence and is not verified IIS support.
