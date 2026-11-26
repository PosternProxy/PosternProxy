> Updated 2026-11-25: this document retains historical checkpoints. Current
> platform development qualification, browser checks and fresh independent
> source review are complete. The remaining gates are exact-candidate Linux race/
> native ARM/platform CI, draft bundle checks and public release/install checks.
> Use the [owner runbook](beta-release-runbook-2026-11-25.md) and
> [current readiness scope](release-readiness-beta-v1-2026-11-25.md).

# G. Release qualification and remaining readiness gaps

Status: in progress — foundation checkpoint implemented, broader qualification open. Lead: GPT6.1. Supporting assignments: GPT5.6 for scoped CI/test fixtures, Sonnet 5.5 provisionally for browser scenarios, Grok 4.7 provisionally for independent failure analysis, Haiku 5.5 provisionally for evidence/docs reconciliation. GPT6.1 is the fallback. Parent: [development index](beta-exit-development-index.md).

Latest evidence (2026-11-24): [current correctness and Ubuntu qualification](beta-security-qualification-2026-11-24.md)
supersedes stale descriptions of missing browser/integration/scanning tooling and
dependency versions in the initial checkpoint below. Current source/binary/npm
scans have no findings. Ubuntu controller/agent/actual SSH/TLS permissions pass;
Docker, other platforms, published historical upgrades/downloads and remaining
matrix/observation checks are still open. Do not read historical checkpoint
language as a statement that these new checks were never run.

Owns the original review's **six beta-exit gates** and remaining readiness gaps. Model roles follow complexity and scope; external exact-version capability/access is unverified. This plan supplies release evidence, not a second owner for blockers 1-11.

## Current evidence and scope

CI has Go vet/test/build and frontend type/lint/build/dependency checks plus Hugo. It lacks the pinned live Caddy traffic matrix, fresh Linux VM lifecycle tests, browser behavioral runner, race checks, and Go vulnerability checks required by the review. The frontend has no behavioral test script. There is a migration runner, but only a consolidated initial migration. Audit coverage and published claims still need reconciliation.

Touch points: `.github/workflows/ci.yml`, scripts/build and Docker builds, migration fixtures, integration/browser tests, service/handler audit paths, README/security/feature/upgrade docs, changelog, and plans/evidence under `.agentplans/`.

## Phase 1: foundations before schema-changing fixes

- [ ] Establish a reproducible baseline incorporating the existing working-tree changes without discarding them; record revision and intentional local differences. Rerun the six original targeted reproductions as permanent tests in their owning groups.
  - [x] Record starting HEAD and hashes/deletions of all pre-existing tracked changes in [baseline metadata](beta-exit-g-start-baseline.json); leave changes intact. This identifies the starting tree but is not a standalone source snapshot or commit.
  - [ ] A/B/C/F retain the original reproductions when implementing their fixes. No blocker reproduction was marked fixed by this checkpoint.
- [ ] Define supported beta upgrade sources and schema baseline. Add forward migrations for promised upgrades rather than editing already-applied initial SQL. Test each supported source; explicitly document unsupported sources and their recovery path before users run upgrades.
  - [x] Freeze current initial SQL with an independent fixture, reject future initial-SQL edits in tests, and exercise an already-applied baseline through repeated startup while preserving host data and foreign keys.
  - [x] Document the qualified development baseline, unqualified older beta sources, offline recovery prerequisites, and rollback limits.
  - [ ] Qualify actual published beta databases and future forward migrations before promising release upgrades.
- [ ] Pin a single Caddy and L4/rate-limit plugin version contract used by all build/install/container paths. Validate required modules even when Caddy is preinstalled. Produce reusable fixtures for generated configs and actual traffic.
  - [x] Shared embedded/sourced contract: Caddy v2.11.4, L4 v0.1.1, rate-limit v0.1.0, xcaddy v0.4.6. Add module/version rejection tests and wire repository builds, Dockerfiles, installers, and SSH provisioning.
  - [x] Native Windows acceptance proves generated HTTP proxy, rate-limit rejection, and TCP stream traffic; binary dependency versions are checked before execution.
  - [ ] Linux CI execution, container/VM artifacts, HTTPS/custom certs, ACLs, UDP, TLS passthrough, remote agents, and full dependency/artifact provenance remain open.
- [ ] Choose the minimal browser test runner, add reproducible setup/scripts, and use real HTTP/TLS cookie behavior with non-localhost deployment where relevant. Keep integration artifacts sanitized and versioned by code/build identity.
  - [x] Pin Playwright 1.64.1 and add isolated controller/Vite lifecycle, disposable data, no service reuse, and no credential-bearing traces. Chromium login/reload/refresh/logout smoke passes locally.
  - [ ] TLS and non-localhost cookie semantics, CSRF mutations, provisioning, cert upload, restore, and rejected-apply behavior await the broader matrix.

## Phase 2: remaining cross-cutting work

- [ ] Audit completeness: inventory privileged mutations, including token issue/revoke (A), backup restore (F), settings edits, host changes, rollback, and provisioning. Owning groups implement their audit calls; G fills remaining settings/host snapshot gaps using shared helpers. Verify actor, canonical IP from B, resource, safe before/after, outcome, and visible handling of audit-write failures.
- [x] Reconcile README Basic Auth claims against implemented ACL functionality. Removed the unsupported README claim; existing ACL docs already describe the limitation.
- [x] Reconcile the forwarding plan's claimed monitor with missing implementation. Reopened the original plan's monitor task without adding feature implementation to beta-exit scope. Initial live TCP traffic now passes; UDP/full forwarding qualification remains in gate 2.
- [ ] Correct stale security-rate-limit docs, backup/recovery scope, desired/applied semantics, certificate exposure/rotation guidance, and install/upgrade assumptions using verified implementation outcomes. Haiku may prepare edits; the relevant implementation owner validates technical accuracy. Preserve existing licensing documents and dual-license terms.

## Phase 3: gate matrix

| Original gate | Required acceptance evidence | Main dependencies |
| --- | --- | --- |
| 1. Blockers and retained regressions | Every blocker 1-11 has a passing regression and a reviewed closure entry | A-F |
| 2. Exact Caddy/plugin acceptance | Local and remote HTTP/HTTPS, custom certs, ACLs, rate limits, TCP/UDP streams, TLS passthrough; accepted config and observed traffic | C/E; pinned build |
| 3. Install, upgrade, recovery | Fresh supported systemd and Docker installations, promised beta upgrades, clean-controller restore, disconnect/reconnect and controller/Caddy restarts | C/D/E/F |
| 4. Browser smoke tests | Login/refresh, host create/edit/disable, cert upload, provisioning readiness, backup restore, and visibly failed/rejected application | A/C/D/E/F |
| 5. Security/race/dependencies | Real-router auth/revocation, trusted-proxy and log-redaction tests, Linux race tests, Go/frontend dependency review | A/B/C; CI |
| 6. Release candidate and support policy | Defined support/migration policy, tested release artifacts, representative multi-server observation, owner release decision | All previous gates |

## Verification and release decision

- [ ] Run `go test ./...`, `go build ./...`, `go vet ./...`, and `go test -race ./...` on supported Linux. Run Go vulnerability scanning (e.g. `govulncheck`) and the existing npm audit; investigate actionable results and record any justified exceptions with expiry/owner.
- [ ] Run frontend type-check/build/lint, behavioral tests, live pinned Caddy integration, Linux VM install/upgrade tests, Docker release-artifact acceptance, and Hugo build. Use isolated fixtures rather than production credentials or hosts.
- [ ] Exercise failure scenarios and inspect actual traffic: saved-but-rejected configuration, stale revisions, agent rejection/disconnect, controller restart, unavailable Caddy, and restore partial application. A green unit build alone is insufficient.
- [ ] Record a supported release-candidate matrix covering a local controller and at least two remote agents; observe representative create/change/restart/recovery cycles. Define the observation duration and acceptable error/convergence limits before starting; record observed results and defects rather than inventing a pass from elapsed time.
- [ ] Produce a final dated readiness review with code/artifact identities, per-gate evidence, residual risks, migration instructions, and rollback/recovery steps. Keep beta if a blocker or required gate lacks evidence. The project owner makes the release decision.
- [ ] Update the development index with individual blocker closures. Archive each completed implementation plan only after its tests/review are recorded. Archive this plan when all gates pass and the readiness decision is documented; update all moved links.

Dependencies: foundation tasks start first. Final qualification waits for A-F. Audit/documentation work must follow actual implementation contracts and cannot substitute for acceptance tests.

Completion evidence: [foundation checkpoint](beta-exit-g-foundation-checkpoint.md). Plan G remains active; no original blocker is closed. New security scans report outstanding dependency findings, and Linux/VM/container/full multi-server acceptance has not been performed.
