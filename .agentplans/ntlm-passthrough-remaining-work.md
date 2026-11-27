# NTLM passthrough — remaining work

Date: 2026-11-27. Status: **Sections 1, 2, 4, and 5 are done. IIS qualification (section 3) is still open and blocks any verified-IIS claim.**

## Context

The feature from `.agentplans/feature-ntlm-passthrough-spec.md` is implemented and
sits uncommitted in the working tree. `go build ./...` and `go test ./...` pass;
the controller code was reviewed on 2026-11-27 and found faithful to the spec —
fail-closed route building, correct validate-before-snapshot ordering, the
decoded-error classifier, audit/rollback/backup wiring, and the build-contract
pin of the fork (`github.com/PosternProxy/ntlm-transport` `v0.0.0-20261127162849-66b6501bd51c`,
see `.agentplans/completed/feature-ntlm-transport-fork-spec-2026-11-27.md`).

This document covers only what is **not** done. Do not re-open the implemented
work or strip the existing validation; it is built and passing. Address the
implementer. Do not open a pull request — prepare the work and let the owner
commit.

## 1. Legacy / known-risks warning (UI + docs)

The Security-tab control currently shows a mild help line. Replace it with a
warning that names the legacy status and the real risks. No behavior change —
this is copy only; the checkbox stays a labeled, keyboard-accessible control.

- File: `frontend/src/views/proxy-hosts/form/tabs/SecurityTab.vue`. Keep the
  label "NTLM authentication passthrough". Replace the help paragraph with the
  text below, styled as a warning (e.g. an amber/warning note, not muted grey).

  > **Legacy.** For older IIS apps using Integrated Windows Authentication
  > (NTLM/Negotiate). Known limitations and risks:
  > single upstream and direct browser connections only — an intermediary that
  > shares one connection across users can break per-user isolation; do **not**
  > add header rules, path locations, or custom 401 pages on this host (they
  > break authentication and are rejected); terminating TLS here can conflict
  > with IIS Extended Protection. Not verified against all IIS configurations.

- Docs: mirror the same limitations in the proxy-host page that documents the
  control and in `docs/content/administration/upgrading.md`. State plainly that
  the feature is legacy and IIS behavior is not yet verified.

Acceptance: the warning renders on the Security tab; `frontend/e2e/hosts.spec.ts`
NTLM cases still pass; docs state the legacy/limitations text.

## 2. Concurrency verification — `go test -race`

The fork's correctness is its concurrency behavior (the lock-leak fix, the
map-entry cleanup, `MaxConnsPerHost = 1`). The race detector has **not** run:
the review host has no cgo.

- Run `go test -race ./...` for the fork repo and for this controller on a
  cgo-capable host (Linux, or Windows with a C toolchain).
- Record the command and result. If the detector reports a race in the fork,
  fix it there, re-resolve the pseudo-version, update `CADDY_NTLM_VERSION` in
  `internal/caddybuild/build.sh` (and the `verify.go` want-map / `build.ps1`
  consume the same value), and re-run.

Acceptance: `-race` green for both. This gates release of anything that emits
`http_ntlm`.

## 3. IIS qualification (AC7–AC11)

Real end-to-end verification against a live IIS application using Integrated
Windows Authentication. Use the acceptance outcomes in
`.agentplans/feature-ntlm-passthrough-spec.md` (AC7–AC11): real users
authenticate as themselves, concurrent users from one source address never see
each other's identity, failure/restart/idle/reload recover safely, and a
sustained workload shows no identity leak or unbounded retained connections.
Record OS/IIS version, module/Caddy revision, browser, negotiated downstream
protocol, upstream scheme, certificate arrangement, and Extended Protection
policy; cover a local Caddy target and an outbound agent target.

This does **not** block committing the code. It blocks any claim of verified
IIS support. Until it is recorded, docs and README say the feature is
implemented but IIS behavior is not verified.

## 4. Docs accuracy pass

The change touched nine docs files. Review them against shipped behavior,
in particular: the Security-tab access-list correction, the coordinated
Caddy-upgrade note and the poisoned-host rule (fix or disable the host, then
retry sync; no `http` fallback) in `upgrading.md`, the build pin in
`building-from-source.md`, and `installation.md` / `docker.md`. Correct any
line that overstates support or describes unbuilt behavior.

Acceptance: docs match what the code does; no "verified IIS" claim.

## 5. Commit

Commit the whole change as a **single commit** (not a series). Include the
migration, the new test files, and the docs. Update `CHANGELOG.md` Unreleased
if not already covered. Do not mark `feature-ntlm-passthrough-spec.md` Built and
do not move it to `completed/` until IIS qualification (section 3) is recorded.
Do not open a pull request; the owner commits and opens any PR.

## Recorded results (2026-11-27)

- Security tab help is an amber legacy warning. The checkbox label is unchanged. `npx playwright test e2e/hosts.spec.ts -g "NTLM"` passed after the copy change.
- Docs for the proxy host, upgrade, install, Docker, source build, README, and changelog call the control legacy and say IIS behavior is not verified. Access-list steps still point at the basic host form and the SSL tab. The poisoned-host rule is unchanged: correct or disable the host, then retry sync, with no HTTP fallback.
- Race detector, WSL Ubuntu, gcc 11.4.0, `go1.26.11`, `CGO_ENABLED=1`, `GOTOOLCHAIN=local`:
  - Fork `/mnt/c/Users/Michael/Repos/PosternProxy-ntlm-transport`: `go test -race -count=1 -timeout 15m ./...` exited 0 (`ok github.com/PosternProxy/ntlm-transport`).
  - Controller: `go test -race -count=1 -timeout 25m ./...` exited 0.
  - No race was reported. The pin stays `v0.0.0-20261127162849-66b6501bd51c`.
- IIS qualification was not run. No OS, IIS, browser, Extended Protection, or workload result is recorded.

## Done when

- Warning renders in the UI and appears in the docs (section 1).
- `go test -race` is green for the fork and the controller (section 2).
- Docs reviewed and accurate (section 4).
- Committed as one change (section 5).
- IIS qualification (section 3) is tracked as the one remaining gate before the
  feature is advertised as verified; it may trail the commit.

## Out of scope

- Any further validation or new rejection rules — the implemented set stays.
- Reworking or stripping the feature.
- Automating an IIS test lab.
