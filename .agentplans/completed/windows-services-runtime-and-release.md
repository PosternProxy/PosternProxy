# Windows services, runtime guard and release artifacts

Status: implementation complete, 2026-11-25. User explicitly authorized all unwritten code.
Preserve the existing uncommitted browser tests and the outbound-only agent model.

Acceptance and increments:
1. One shared management-port/address rule rejects 80/443/2019/8443 for runtime
   startup, fresh installs and preserved installer configuration. Invalid addresses
   fail before opening the application database. Existing valid defaults remain.
2. Windows amd64 controller and agent install from verified matching PE assets or
   exact release downloads. No Bash dependency. Service lifecycle supports clean
   stop/restart and removal while retaining data. Elevation required for SCM writes.
   Separate virtual service identities for app and Caddy, explicit protected ACLs,
   encrypted service enrollment/config at rest, no secrets in service command lines.
   Reinstall retains configuration, encryption identity, database and credentials.
   Fresh controller prints its login only after readiness and removes bootstrap
   staging. Agent verifies controller enrollment before installation mutations.
3. Windows executable startup handles SCM via shared internal package. Controller
   and agent honor cancellation; Caddy is supervised as a child and stopped through
   its loopback admin endpoint. No duplicated cmd helpers or public admin listener.
4. Tagged draft release produces Windows amd64 zip + native Linux amd64/arm64 archives,
   matching controller/agent/Caddy assets/checksums. Docker controller/agent images
   built/tested before draft publication, version matched; architecture support
   matches published artifacts. No push/tag/publication during this task.
5. Regression/unit checks, platform builds, browser checks, source scans, meaningful
   Windows service qualification in an isolated environment when available. Never
   claim actual SCM or Docker execution based on compilation alone. Docs/changelog
   name verified behavior and any remaining environment qualification.

Windows defaults: Program Files for binaries, ProgramData for protected data;
virtual service accounts (no reusable account passwords). Controller and agent
installations on the same machine are refused to avoid competing Caddy ownership.
Removal unregisters/stops services and retains binaries/data for recovery; no
implicit recursive data deletion or firewall modification.

## Implementation and verification

- Shared runtime/installer management address validation is covered by independent
  rejection tests, including fail-before-database startup and preserved config.
- Shared Windows service host, DPAPI definitions, virtual identities, protected
  storage, cancellable controller/agent lifecycle, Caddy job supervision and native
  installer/removal are implemented. The owner/DACL checks precede retained-config
  consumption. Fresh managed roots use protected ACLs atomically at creation.
- Review corrections include explicit SCM service type, socket/request cancellation,
  trusted retained-file ownership, atomic root creation and Compose environment
  propagation. Independent final review reports no remaining confirmed blockers.
- A released Windows controller can fetch matching verified Linux provisioning
  assets; development/revision versions and unsupported architectures are refused
  before download. Native Linux amd64 publication was initially omitted through
  an incorrect scope interpretation; the user clarified it remains supported.
  See `restore-native-linux-amd64.md` in this directory for the correction.
- Release workflows package Windows amd64 and native Linux amd64/arm64, qualify native
  service lifecycle and Docker/Compose on both architectures, and create a draft
  only after those jobs pass. Docker publication repeats qualification first.
- Docker supervises both processes, exits on either failure and persists Caddy
  storage. Build contexts exclude local secrets/data and generated artifacts.

Passed locally: `go test ./...`, `go build ./...`, `go vet ./...`, govulncheck
(no vulnerabilities), Windows DPAPI/SCM handler/process-job tests, frontend
type-check/lint/build, all 15 Chromium HTTP/HTTPS scenarios, Hugo (62 pages),
native Windows and Linux arm64 controller/agent builds and native version output.
Shell/PowerShell/Python parsing and actionlint pass. The elevated protected-root
creation regression is skipped on the unelevated workstation and runs on the
Windows qualification runner.

Not executed locally: SCM service registration on a disposable Windows host,
native Linux arm64 runtime checks, Docker image builds/Compose runs (no Docker
CLI available). Their harnesses/jobs are written; successful candidate runner
results and exact public release assets remain gates in
`../release-readiness-beta-v1-2026-11-25.md`. No commit, tag, registry push or
release publication was performed. No workstation services were installed.
