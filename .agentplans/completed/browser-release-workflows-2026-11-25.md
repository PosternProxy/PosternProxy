# Browser release workflows

Status: complete, 2026-11-25. User authorized execution of browser gate 4.

Acceptance:
- Export a real encrypted archive through the UI; reject incorrect passphrases
  without mutations; restore a deleted host and certificate metadata through UI;
  distinguish committed restore from unapplied Caddy configuration.
- Run a real outbound agent against a controlled Caddy admin fixture. Browser
  shows online separately from applying/failed/applied configuration, and retry
  reaches an acknowledged applied revision. This is not an SSH installer test.
- Exercise a non-localhost HTTPS origin with real secure-cookie configuration:
  login/refresh/reload/logout, Secure/HttpOnly/Strict attributes, browser CSRF
  rejection/acceptance and cross-site cookie exclusion.
- Keep credentials, archive bodies and traces out of logs/artifacts. Isolate data,
  ports and processes; clean up fixtures. Run browser, type, lint and build checks.

User clarified the added targets supplement native Linux amd64: Windows with
service installation, Linux arm64 and Docker. No external existing user
configurations are assumed; historical database migration is not a beta gate.
Targeted correctness before beta; extended multi-server observation during beta
is production-readiness evidence. Windows service installation is separate work
from this browser implementation; this plan does not claim it implemented.

## Execution evidence

- All 15 Chromium scenarios pass together (25.4s, retries disabled), including
  the three new backup, agent-readiness and HTTPS workflows.
- The encrypted archive is downloaded through UI; wrong passphrase leaves the
  deleted host absent; correct restore recovers host/certificate records and
  explicitly shows committed-but-unapplied status against the dead local Caddy.
- A compiled real agent makes an outbound connection. A loopback admin fixture
  holds and rejects its first load; browser distinguishes Online from Applying
  and Failed. Retry succeeds only after the real agent acknowledges the revision.
- Separate secure-cookie controller and TLS gateway use non-localhost Chromium
  host mapping, leaving system DNS/hosts/trust untouched. Secure/Strict/HttpOnly,
  refresh/reload/logout, CSRF refusal/acceptance and cross-site exclusion pass.
- Type-check includes e2e files now; type-check/lint/build pass. Docs and changelog
  describe the harness and its boundaries. Native services/real SSH and real
  Caddy traffic remain separate integration/platform qualification, not hidden
  claims made by these browser tests.
- Recovery/readiness use distinct fixture client IPs with explicit loopback
  trust, preserving the production management API rate limit during the suite.
