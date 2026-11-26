# Broader beta release execution

Date: 2026-11-24, revised 2026-11-25 (America/Indianapolis). Status: in progress.
Owner's clarified scope: retain native Linux amd64, adding Windows with service
installation, native Linux arm64 and Docker.

Extends, and does not replace, the [release qualification plan](beta-exit-release-qualification.md)
and [next beta fixes](beta-next-fixes.md). Those documents keep their gates and
evidence; this plan only sequences the *remaining* work to reach a tagged broader
beta and assigns acceptance criteria. Preserve all current uncommitted work.

## Scope decision

In scope for this beta, revised by the user November 25:
- UDP forwarding and wider TLS-passthrough matrix (gate 2).
- Native Linux amd64/arm64, Windows including a service installer, and Docker/Compose
  clean install and release-asset smoke checks (gate 3). No additional distribution
  matrix or historical beta database upgrade gate: no existing user deployments.
- Broader browser matrix incl. HTTPS/non-localhost cookie and CSRF (gate 4).
- Targeted before-beta correctness smoke; representative multi-server observation
  with predefined acceptance criteria during beta, before production (gate 6).

Explicitly excluded, documented as a known limitation (not a blocker):
- Validated Cloudflare Tunnel / authenticated-origin support. The beta's docs must
  state Cloudflare is not qualified in this release.

No changes to licensing, the outbound-only agent model, single controller binary,
embedded Vue UI, or the Caddy data plane are authorized by this plan.

## Execution streams

Each stream records acceptance evidence in its owning gate plan; this table is the
cross-cutting sequence. "In-repo" = runnable here; "Env" = needs the user's VM,
Docker host, arm64 target, or published assets.

| # | Stream | Gate | Where | Depends on |
|---|--------|------|-------|-----------|
| S1 | Blocker closure ledger (all 11: fix + regression + reviewer) and dev-index closure update | 1 | In-repo + review | A–F evidence |
| S2 | Privileged-operation audit coverage completion (settings/host snapshot gaps; verify actor, canonical IP, resource, safe before/after, outcome, audit-write-failure handling) | 1 | In-repo | B canonical IP |
| S3 | CI hardening: `go test -race` on Linux, `govulncheck`, frontend behavioral (Playwright) wired as permanent gates | 5 | In-repo | none |
| S4 | Final real-router auth/revocation/log-redaction review | 5 | In-repo + review | S3 |
| S5 | UDP forwarding qualification (harness + live) and TLS-passthrough wider matrix, local and remote agent | 2 | In-repo harness → Env | pinned Caddy |
| S6 | Browser matrix completion: host create/edit/disable, cert upload, provisioning readiness, restore; HTTPS/non-localhost cookie + CSRF | 4 | In-repo | existing Playwright infra |
| S7 | Windows service installer + clean install, Linux amd64/arm64, Docker/Compose and exact published-artifact installation | 3 | In-repo + Env | rebuilt artifacts |
| S8 | Beta candidate artifact/platform matrix, known limitations, smoke, owner decision; multi-server observation runs during beta before production | 6 | In-repo + Env + owner | S1–S7 |

## Order and parallelism

1. Start now, in-repo, independent: **S1, S2, S3, S6**. S4 follows S3.
2. **S5** build the UDP/TLS-passthrough harness in-repo, then run live on the VM.
3. **S7** needs Windows installer implementation and Docker/arm64 qualification
   environments. Historical published database fixtures and extra distributions
   are no longer prerequisites under the user's revised scope.
4. **S8** qualifies the exact beta artifacts after S1–S7; longer multi-server
   observation runs during beta and contributes to the production decision.

## Predefined acceptance criteria (define before running, not after)

- Multi-server observation (S8): 1 local controller + >=2 remote agents. Define
  observation duration, the create/change/restart/recovery cycles exercised, and
  acceptable error-rate and convergence-time limits **before** starting; record
  observed results and defects, never a pass inferred from elapsed time.
- Platform breadth (S7): each target records OS/arch/build identity, the exact
  installed bundle hash, and pass/fail per fresh-install, upgrade, and recovery.
- Data-plane (S5): accepted config plus observed traffic for each of UDP and TLS
  passthrough, local and through a remote agent.

## Environment the user must supply for S7 / S5-live / S8

- A Docker host (host and prior VM had no Docker CLI).
- An arm64 Linux target.
- A clean Windows service-install test environment (the current host is Windows,
  but changing its real services requires a suitably isolated test setup).
- The published download/asset location to test the download-install path.
- A disposable VM able to run >=2 agents for the multi-server observation.

## Execution log

- 2026-11-25 — User narrowed platform scope and moved extended observation into
  beta. Browser gate implementation now has 15 passing scenarios, covering UI
  encrypted recovery, real-agent applying/failure/retry readiness, and a separate
  non-localhost HTTPS secure-cookie/CSRF/cross-site fixture. Type/lint/build pass.
  [Completed browser evidence](completed/browser-release-workflows-2026-11-25.md).
  Windows service installer remains separate required platform implementation.

- 2026-11-24 — **S3 CI hardening**: on review, the working-tree `.github/workflows/ci.yml`
  already wired the three S3 targets as gates: `go test -race ./...`, pinned
  `govulncheck@v1.9.0 ./...`, and the Playwright behavioral run (`npm run test:e2e`,
  @playwright/test 1.64.1), plus `npm audit` and a pinned-Caddy two-agent
  integration acceptance job. Added a `go mod verify` module-checksum step to the
  Go job as the one missing supply-chain gate. These CI changes are uncommitted
  (part of the larger working-tree diff); S3 is not an enforced gate until
  committed and observed green on a PR. S4 (final auth/revocation/redaction review)
  is unaffected and still open.

- 2026-11-24 — **S2 audit coverage**: inventoried every privileged mutation. All
  are already audited with actor + resource + canonical IP: auth login/logout/
  failed-login, api_token create/revoke (incl. revoke-on-authority-change),
  join_token issue/revoke, server create/update/delete/provision/reprovision/
  reset_join/resync/rotate-credentials, proxy/redirect/stream/dead host create/
  update/delete/enable-disable/rollback (safe before/after via `HostChange`),
  access_list, certificate create/delete, user create/update/password/delete,
  backup, settings. Confirmed canonical IP is not a gap: `ClientIdentity`
  rewrites `r.RemoteAddr` to the trusted-proxy-resolved client IP and mirrors it
  in `security.RequestIP(ctx)`, so handler and service call sites record the same
  canonical value. Closed two real gaps:
  (1) audit-write failures were silently dropped at most call sites — added a
  single chokepoint in `AuditService.Log` that logs every failed insert, so a
  dropped privileged-action record is always observable;
  (2) settings edits audited only `{success}` — added `SettingsChange` with a
  redacted before/after snapshot and explicit outcome (`redactSettings` masks any
  secret-bearing key). Handler now reads a single `requireClaims` to stay
  single-response-safe. Tests: `TestAuditLogFailureIsObservable`,
  `TestSettingsChangeRedactsSensitiveValues` (internal/services/audit_test.go).
  go build, go vet, and the full services suite pass. Residual: an explicit
  outcome field exists for settings/provision but most other privileged events are
  emitted only after success (implicit success); standardizing an outcome field
  across all events is deferred as a non-blocking consistency item. Changes are
  uncommitted.

- 2026-11-24 — **S6 browser matrix**: added 6 Chromium scenarios to the Playwright
  suite (now 12 total, all passing in ~16s): CSRF double-submit enforcement
  (missing/mismatched/correct token) and csrf_token cookie attributes
  (Strict/non-HttpOnly/non-Secure); proxy host create through the form; proxy
  host disable + edit through the UI (API-verified persistence, since the dead-
  Caddy harness intentionally fails applies so the UI does not optimistically
  flip); valid custom certificate upload through the form; invalid certificate
  rejection without persistence. Added a disposable self-signed cert as an
  embedded `.ts` fixture (`frontend/e2e/testdata/cert-fixture.ts`, so it survives
  the repo's *.pem/*.key gitignore) and a shared `frontend/e2e/helpers.ts`.
  Harness enabler: the per-IP login throttle (hardcoded 5/min) rate-limited a
  growing per-test-login suite. Made it configurable via
  `POSTERNPROXY_LOGIN_RATE_LIMIT` (config.go/app.go/router.go), **default
  unchanged at 5/min in production**, and set it high only in the disposable e2e
  harness (start-server.mjs). go build, go vet, go test (config/api), frontend
  type-check and lint all pass. Deferred to the VM/HTTPS matrix (gate 2/3): the
  backup restore workflow, HTTPS/non-localhost secure-cookie semantics (this
  harness serves plain HTTP on loopback), and provisioning readiness beyond the
  existing manual-enrollment scenario. Changes are uncommitted.

- 2026-11-24 — **S5 UDP + TLS-passthrough acceptance**: added live pinned-Caddy
  integration tests in `internal/caddy/stream_acceptance_test.go`
  (`TestPinnedCaddyUDPForwarding`, `TestPinnedCaddyTLSPassthrough`) with shared
  harness helpers (`requirePinnedCaddyBinary`, `launchCaddy`, `neutralizeHTTP`).
  Built the pinned Caddy locally (v2.11.9 + L4 v0.1.1 + ratelimit v0.1.0); all 4
  `PinnedCaddy` acceptance tests pass, plus caddy unit tests and vet.
  **Found and fixed a real UDP-forwarding bug**: `BuildStreamServer` dialed the
  upstream over TCP regardless of protocol, so UDP stream hosts silently failed
  to forward. Fixed in `internal/caddy/layer4.go` (udp protocol now dials
  `udp/host:port`), covered by `internal/caddy/layer4_test.go` and the live UDP
  test, with a CHANGELOG entry. TLS passthrough is verified end-to-end: the
  client sends SNI `passthrough.test` for routing and confirms the server
  presents the upstream's exact certificate, proving Caddy did not terminate.
  These run in the existing `caddy-acceptance` CI job. Remaining for S5:
  `tcp+udp` mixed-protocol forwarding (the single upstream dial can't serve both
  transports — needs design, not just a prefix), the passthrough fallthrough/
  non-passthrough-SNI termination path (needs a live HTTPS app — belongs to the
  HTTPS matrix), and the live remote-agent UDP/TLS-passthrough runs on a VM
  (S5-live). Changes are uncommitted.

- 2026-11-24 — **S5 review corrections** (three findings, now fixed):
  - **[P1] TLS passthrough was broken in production and the first test hid it.**
    The earlier passthrough test stripped the generated fallthrough route before
    launching Caddy, masking that the production config used a nonexistent
    `layer4.handlers.http` module — pinned Caddy rejects it, so enabling any
    passthrough host took down the whole data plane. Fixed `config.go` to bridge
    non-passthrough SNI to an internal loopback HTTPS listener over PROXY protocol
    (preserving client IP); added `ListenerWrapper`/`proxy_protocol` types. The
    live test now loads the **real** config (fallthrough present) and would catch
    the regression; a structural unit test
    (`TestPassthroughConfigBridgesWithProxyProtocol`) locks the wiring.
  - **[P2] Combined tcp+udp forwarding.** `BuildStreamServers` now emits one
    server per transport so UDP is not dialed over TCP in combined mode; unit
    test `TestBuildStreamServersCombinedSplitsPerTransport`.
  - **[P2] Settings audit secret leak.** Rejected settings edits masked no
    values and the redactor missed `api_key`; now all attempted values are
    masked on rejection and `key`/`auth` are redacted markers. Reproduced and
    guarded by `TestSettingsChangeRejectionMasksAttemptedValues`.
  All 4 pinned-Caddy acceptance tests, caddy/services unit suites, go vet and
  build pass. CHANGELOG updated. Supersedes the earlier S5 claim that TLS
  passthrough was verified end-to-end.

- 2026-11-24 — **S5 passthrough-bridge follow-ups** (two more review findings on
  the bridge fix, now resolved):
  - **[P2] HTTP/3 advertised an unreachable endpoint.** The bridged HTTPS server
    still offered h3, advertising the private bridge port (`:8443`) as its QUIC
    endpoint. `config.go` now restricts the HTTPS protocols to h1/h2 whenever
    passthrough is active (the TCP bridge can't carry QUIC). Guarded by
    `TestPassthroughConfigBridgesWithProxyProtocol`.
  - **[P2] Bridge port collided with an allowed management port.** The installer
    permitted `--port 8443`, which the bridge also uses, so enabling passthrough
    produced `address already in use`. The installer now reserves 8443 (alongside
    80/443/2019), guarded by a new `OptionsValidate` case. The bridge port is
    documented as reserved next to the `internalHTTPSAddr` constant.
  All caddy/installer unit tests and the pinned-Caddy passthrough/UDP acceptance
  tests pass; go build and vet clean; CHANGELOG updated.

## Release decision

Keep beta if any in-scope gate lacks evidence; the project owner makes the final
call. S8 produces a final dated readiness review with code/artifact identities,
per-gate evidence, residual risks, migration instructions, and rollback/recovery
steps, then the owner decides and the release is tagged. Update the development
index with individual blocker closures and move completed plans into
`.agentplans/completed/` only after their evidence is recorded.
