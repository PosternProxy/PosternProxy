# Windows candidate readiness timeout - 2026-11-26

## Evidence
The tagged candidate passed controller install/login/version/identities, restart,
reinstall/secret persistence, uninstall and isolated enrollment-controller startup.
It timed out waiting for agent authentication and applied revision. The polling
helper swallowed every exception, and cleanup removed the fixture, so the pasted
log cannot identify authentication, configuration apply, listener conflict or API
poll failure as the cause. Do not claim the installer failure is resolved.

A real local HTTP fixture confirmed PowerShell can select the applied remote
configuration from the returned two-element array. An array-selection bug was not
reproduced. No timeout increase or readiness bypass was introduced.

## Changes and verification
- Polling retains the final exception and supports timeout diagnostics.
- Readiness failure reports online/configuration/revisions, service states, port
  owners and selected structured events from agent/Caddy/fixture logs.
- Known passwords, session/CSRF/enrollment credentials, bearer tokens, API tokens
  and PEM material are redacted. Raw configurations/enrollment/log artifacts are
  not uploaded. Fixture stderr is now JSON for structured selection.
- Readiness explicitly requires exactly one matching configuration state.
- Added PowerShell regressions for retry, timeout diagnostics, retained failure
  and credential redaction. Regressions and PowerShell parsing pass locally.
- CI invokes those regressions before the existing Go checks.

## Remaining release blocker
Run Release candidates manually on the branch containing these changes (workflow
is already available on default master), inspect diagnostics if it fails, fix the
proven underlying cause, and repeat all gates. Rerunning the existing tagged job
uses the old script. Do not publish the failed v0.1.4-beta candidate or move its
tag; qualify/tag a new candidate once the changes are accepted. The existing
owner-runbook clarification remains an independent uncommitted change.
