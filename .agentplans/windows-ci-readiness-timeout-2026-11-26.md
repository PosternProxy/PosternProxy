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

## Follow-up evidence and fix
The next candidate diagnostics identify the actual failure: online=true,
configuration revision desired=1/applied=0/status=failed. Caddy repeatedly rejects
its configuration because :80 cannot bind; port 80 is owned by Windows System,
PID 4 (HTTP.sys). Agent authentication and both SCM services work. The log proves
the port conflict, not which HTTP.sys client owns the registration.

CI preparation now stops the IIS W3SVC/WAS services on the disposable GitHub-hosted
runner, guarded by RUNNER_ENVIRONMENT=github-hosted. No application installer or
local-machine qualification stops unrelated services. The smoke harness probes
its required TCP ports and UDP 443 before installation, with bounded polling for
port release; another owner/reservation fails with an explicit port error.

Regression checks for occupied/released TCP and UDP ports pass locally, alongside
redaction/retry/timeout helper tests. PowerShell script/inline-step parsing,
actionlint and git diff --check pass. This workstation's services were not altered.
The hosted-runner IIS preparation and full SCM scenario need the updated CI run.

## Remaining release blocker
Run Release candidates manually on the branch containing these changes (workflow
is already available on default master), confirm IIS preparation releases port 80 and the full
SCM/agent/TLS scenario passes, and repeat all gates. Rerunning the existing tagged job
uses the old script. Do not publish the failed v0.1.4-beta candidate or move its
tag; qualify/tag a new candidate once the changes are accepted. The existing
owner-runbook clarification remains an independent uncommitted change.
