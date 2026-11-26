# Beta release preparation -- 2026-11-25

Target: v0.1.4-beta (v0.1.3-beta already exists locally). Preserve native Linux amd64; add Linux arm64, Windows amd64 service installation and Docker amd64/arm64.

## Acceptance
- Fix fresh review findings: agent API-token/owner revocation must invalidate ongoing transport; backup configuration and server descriptors must share one snapshot.
- Add regressions and obtain fresh independent review of fixes.
- Require complete CI in release draft creation, including browser/docs checks.
- Run local checks and record their actual results.
- Prepare exact owner release steps and candidate notes. No tag, push, publication or claim of remote CI success during preparation.

## Evidence
Prior platform qualification is recorded in linux-qualification-165-227-3-116.md and windows-local-installer-qualification.md. ARM emulation is supplemental; native ARM release CI remains mandatory. Prior platform evidence predates the final revocation and backup changes.

## Completed preparation
- Migration005 makes API/owner revocation invalidate the transport credential;
  existing connections check their credential lease before processing messages
  and on an idle timer. Public DELETE, owner changes and rotation regressions pass.
- Legacy and portable exports share a read transaction including server identity;
  a real WAL concurrent-write regression produces a restorable consistent archive.
- Both findings received fresh read-only follow-up review, no actionable findings.
- Complete local Go tests/build/vet/module verification, frontend type/lint/build,
  15 browser tests, actionlint, Hugo and govulncheck pass. The first sandboxed
  networking tests failed due to denied loopback; authorized reruns passed.
- Affected API/services/database full test executables pass natively on Ubuntu.
  Initial database executable lacked its relative baseline fixture; supplying
  the checked-in testdata made the complete database suite pass.
- Candidate release jobs require full reusable CI before draft creation.
- Changelog v0.1.4-beta, release notes and owner runbook are prepared.

## Owner release gates remaining
Exact committed/tagged CI including native ARM/race/source images, draft archive
smoke, owner publication, public downloads/GHCR visibility and real-domain ACME.
These are in ../beta-release-runbook-2026-11-25.md. Nothing has been committed,
pushed, tagged or published. All preparation implementation is complete.

