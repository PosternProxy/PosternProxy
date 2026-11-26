# Traffic acceptance reload failure - 2026-11-26

User-supplied candidate log: the services package, including
TestTwoAgentsConvergeAfterControllerRestart, passed (7.34 seconds for that test).
The single failing subtest was TestPinnedCaddyClientIdentityAndStreaming /
upload_and_header_bounds, with broken pipe during the upload POST immediately
after a policy reload. SSE, WebSocket, other traffic and recovery checks passed.

The fixture shared the default HTTP transport across policy reloads. Caddy retires
idle connections asynchronously; a non-idempotent upload can race connection
closure, and the test may use a socket from the preceding rate-limited server.
Give the test its own transport and close its idle connections after each reload.
Keep connection reuse within each policy case and all upload/header assertions.
This changes the fixture only; no production behavior, retries or timeouts change.

Baseline repeated checks: 15 Windows and 30 native Linux runs passed; the CI race
was not reproduced locally. Do not describe the cause as experimentally proven.
Validation with the fix: affected Windows integration suites (Caddy and services,
including live TLS/recovery and two-agent restart), full Go regression suite and
Go build pass. The full native Linux Caddy integration suite passes, and 30
repeated fixed traffic runs pass. No retries or assertions were weakened.
The two-agent controller restart scenario also passed a fresh native Linux rerun.
The CI race was not reproduced before the change, so the exact updated candidate
still requires a green Actions run before release.

Windows SCM readiness is an independent open CI blocker tracked in
windows-ci-readiness-timeout-2026-11-26.md. This change does not resolve that gate.
