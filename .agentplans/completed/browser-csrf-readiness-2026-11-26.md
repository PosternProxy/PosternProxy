# Browser CSRF initialization race - 2026-11-26

## Confirmed failure
The test helper POSTs /api/auth/refresh then immediately expects csrf_token.
The server seeds that cookie only on safe requests. Dashboard navigation can
finish before its first GET response, so this helper relied on background UI
requests having already initialized the cookie.

A new API-only login regression deterministically reproduced the exact failure
at helpers.ts's cookie assertion before the change. It has no dashboard requests
that could accidentally satisfy the helper's precondition.

## Fix
The shared helper explicitly GETs authenticated /api/auth/me and checks success
before reading the cookie. Duplicated header-building sequences in auth.spec.ts
now reuse that helper. Production CSRF/auth logic, timeouts and Playwright retries
are unchanged. Keep v0.1.5-beta; failed unpublished candidates do not require a
new version. Branch qualification precedes any final unpublished-tag replacement;
published release tags remain immutable. The owner runbook now says this.

## Completed verification
- New regression failed deterministically with the original helper: csrf cookie
  undefined at helpers.ts, matching the reported CI failure.
- After the fix, all 20 isolated-context regression runs pass.
- Full 16-test HTTP/HTTPS suite passes in three independent fresh-backend runs:
  31.1, 26.2 and 26.0 seconds. Retries remain zero.
- Frontend type-check, lint, build and git diff --check pass.
- No version bump or tag/reference mutation was performed. The release runbook
  documents branch qualification and lease-guarded replacement only for a
  verified unpublished/undistributed candidate. Published releases are immutable.

Implementation and local qualification are complete. GitHub's corrected-commit
candidate run remains the final release gate; these local results do not assert
that an unrun Actions job is green.

## Release flow correction
The owner explicitly retains the existing tag-push release flow. A proposed
manual-package/version-input workflow change was reverted completely before
commit. Neither release workflow has a diff. The actual CSRF helper fix and
regressions remain. Normal branch CI can verify them; pushing the corrected
unpublished candidate tag then runs the existing checks/package/draft pipeline.
