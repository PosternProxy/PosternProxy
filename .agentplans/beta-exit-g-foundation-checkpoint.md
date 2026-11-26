# Plan G foundation checkpoint — 2026-10-24

Status: implemented locally; release qualification remains open. Executor: current chat, no delegated agents launched. Parent: [Plan G](beta-exit-release-qualification.md).

## Baseline and scope

Starting HEAD: `380b6d0218970360443aa1bea45849347a22470a`. The checkout already included the forwarding removal and associated documentation/UI/schema changes. [Starting metadata](beta-exit-g-start-baseline.json) records every pre-existing tracked modification/deletion and its byte hash. No pre-existing changes were reverted or committed. These hashes identify the original tree but do not reconstruct its contents without the checkout; a standalone reproducible source snapshot/revision is still required before release qualification.

This checkpoint implements G's initial infrastructure and directly verifiable claim corrections. It does not execute the fixes owned by A-F or claim the September reproductions now pass.

## Changes

- Frozen initial-schema fixture and regression tests: initial migration must remain unchanged; future changes use new migration files. Existing baseline data survives repeated migration startup with no FK violations. Older beta schemas are explicitly unqualified in upgrade docs.
- Shared Caddy contract under `internal/caddybuild/`, embedded into controller SSH provisioning and sourced/read by Bash/PowerShell, both Dockerfiles, and installers. Installer verification rejects wrong core/plugin versions. Separate installer distributions must include `caddy-build.sh`.
- Pins come from the existing `bin/caddy` build metadata: Caddy v2.11.4, L4 v0.1.1, rate-limit v0.1.0. xcaddy v0.4.6 matches the installed build tool. A new native Windows Caddy was built from these pins for acceptance. This is a component contract, not a full transitive lockfile or vulnerability clearance.
- Tagged live Caddy fixture checks embedded component versions, generated HTTP config/traffic, rate-limit rejection, and TCP forwarding. Fixture listeners bind loopback and disable automatic TLS for temporary HTTP ports. TLS/UDP/ACL/remote acceptance remains open.
- Playwright 1.64.1 Chromium smoke uses an isolated controller/database and Vite, checks login and refresh after reload, cookie flags, logout, and subsequent reload. Production data/settings are not used. Test HTTP cookie mode does not qualify HTTPS or non-localhost deployment.
- CI now includes Linux race checks, pinned govulncheck v1.9.0, browser smoke, and pinned Caddy traffic. Existing npm audit remains enforced. The new workflow has not been executed on GitHub.
- Corrected README Basic Auth, security rate-limit, and backup/upgrade claims. Reopened the missing reachability monitor in the original forwarding plan. No licensing changes.

## Verification

| Check | Local result / limits |
| --- | --- |
| `go test ./...` | Passed using workspace-local Go cache, including new database tests |
| `go build ./...` | Passed |
| `go vet ./...` | Passed |
| Frontend type-check/build/lint | Passed |
| Chromium auth smoke | Passed, one scenario, isolated HTTP |
| Native pinned Caddy HTTP/rate-limit/TCP acceptance | Passed on Windows |
| Shared Caddy contract negative cases | Passed: wrong core, missing/wrong rate-limit, wrong L4 rejected |
| Bash/PowerShell syntax | Passed |
| Hugo docs | Passed after final documentation changes; existing shortcode deprecation warnings remain |
| Linux race/VM/systemd/Docker/multi-server acceptance | Not run here; no Docker executable found; Linux CI is pending |
| npm audit | Fails release threshold: 15 dependency-package findings (1 low, 2 moderate, 11 high, 1 critical) |
| govulncheck v1.9.0 | Fails: 10 called-symbol findings in 2 modules; additional package/module-only findings need triage |

The initial sandboxed Go test command completed package tests but exited on denied cache trim; rerunning with workspace-local `GOCACHE` passed. Browser/Vite and Git Bash subprocess restrictions were resolved with approved execution. govulncheck v1.1.4 could not analyze this Go 1.25 project; v1.9.0 was verified and selected instead (it builds with an automatically downloaded Go 1.26 toolchain). These failures were not suppressed in CI.

## Dependency triage — open release gates

| Area | Observed findings | Owner / required next step |
| --- | --- | --- |
| Request trust | chi/v5 v5.2.5: GO-2026-5774, 5775, 5777; scanner reports v5.3.0 as fixed | B: review dependency update alongside replacement of unconditional RealIP; retain trusted-peer tests |
| SSH provisioning | x/crypto v0.50.0: GO-2026-5013, 5017, 5018, 5019, 5020, 6354, 6355; scanner reports v0.56.0 covers the newest findings | D: update compatible dependency/toolchain set, rerun SSH/security and provisioning checks |
| Frontend/build tooling | npm identifies axios, vue/server-renderer, vite, shell-quote, and other transitive packages; shell-quote includes a critical advisory | G: scoped dependency remediation followed by clean npm install, audit, type/build/lint/browser regression. No blanket audit fix or audit suppression applied |
| Caddy dependencies | Exact component pins established, full binary dependency security analysis pending | G: scan the release Caddy artifact and record full toolchain/dependency provenance before accepting the pin set for release |

Severity counts are dependency-package counts, not counts of confirmed exploitable application paths. Go called-symbol results need scenario review too; they do not prove every reported path is exploitable. No vulnerability exception or release waiver was granted.

## Remaining work and handoff

A/B/C/F must retain the six original reproductions with their fixes. G still needs a reproducible source snapshot, actual historical-schema fixtures, complete live/remote/TLS/UDP/browser/failure matrices, audit coverage, clean install/upgrade/recovery evidence, dependency remediation, Linux race results, and a representative release-candidate observation period with predeclared criteria. A-F remain planned; all blockers 1-11 remain open.

No independent model review, release sign-off, commit, deployment, or publication was performed. Keep Plan G in `.agentplans/`.

Final verification supplement: Go test/build/vet passed again after the browser
finished; a concurrent earlier attempt hit Playwright replacing its results
directory during Go package discovery. Keep these checks sequential in shared
checkouts. Chromium smoke passed again after cleanup handling changed. Final
Hugo, plan links, scoped diff whitespace, and pre-existing-file hash/deletion
checks passed. The only global diff whitespace finding is the pre-existing final
blank line in `internal/api/handlers/health.go`, left untouched. README/changelog
are intentional overlaps with original work and were reviewed separately.

[Artifact metadata](beta-exit-g-foundation-artifacts.json) records the native
Caddy hash/full module build information, frozen schema LF hash, lockfile hash,
and tools. This metadata is not a standalone source snapshot or a release
artifact attestation. Dockerfile entrypoint COPY paths now match the documented
repository-root build context, but container builds themselves remain unverified.
