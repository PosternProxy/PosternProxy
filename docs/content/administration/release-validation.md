---
title: Release validation
weight: 7
---

# Release validation

The beta-exit work is tracked in `.agentplans/beta-exit-development-index.md`.
Passing the development checks below establishes a foundation; it does not remove
beta status or close the authentication, provisioning, synchronization,
certificate, and recovery blockers.

## Caddy build contract

`internal/caddybuild/build.sh` is the source of the Caddy, L4, rate-limit, and
xcaddy versions. Bash and PowerShell builds, Dockerfiles, standalone installers,
and native SSH provisioning consume it. Installers check the core version and
both plugin versions. Native installers also verify the pinned Go toolchain and
security dependency replacements from the contract. The controller installer stages and verifies its release's
Caddy, then replaces the installed binary. The native agent installer stages and verifies Caddy before replacing it.

`posternproxy install` and `posternproxy-agent install` embed the contract; neither
needs a helper file. Both shell entry points delegate to these Go subcommands.
Docker builds use the repository root as their build context.

The current contract uses Caddy 2.11.9 and Go 1.26.11 with explicit official
security fixes for cryptography, networking, compression and OpenTelemetry. Both
application binaries expose `version` without starting services. Release stamping
is checked against native executable output, not just build metadata.

## Controller installation

The tagged-release workflow builds Windows amd64 and native Linux amd64/arm64
controller, agent and Caddy bundles, loose installer assets and a combined SHA-256
manifest. Windows uses a ZIP archive; Linux uses a tar archive. Draft creation
depends on the complete quality CI suite (including browser, dependency and docs
checks), Windows service qualification, native Linux amd64/arm64 installation and
traffic checks, and Docker/Compose checks on amd64 and arm64. Local build scripts
also emit a manifest. A draft must be published before the installer can fetch
its assets without authentication. Each Linux architecture has loose controller,
agent and Caddy assets plus a matching archive and checksum entries.

The Windows runner executes `scripts/windows-service-smoke.ps1 -DisposableHost`
after building. It checks service installation, retained credentials on reinstall,
controller login, outbound agent enrollment and applied readiness, custom TLS,
restart and removal with retained data. It requires an elevated disposable host
and refuses existing installations. The Linux runner executes the native
controller and agent qualification harnesses after installing the matching bundle.
For an explicitly authorized workstation test, `-LocalMachine` is available
instead of `-DisposableHost`; it still refuses existing installations and binds
the test management listeners to loopback. Uninstall retains protected test data
and binaries, and the harness does not change firewall rules.

`docker-qualification.yml` builds and exercises both image roles through the real
Compose templates on each native architecture. The smoke checks include login,
outbound agent reconciliation, HTTP/custom TLS, agent restart, Caddy failure and
controller replacement with retained volumes. `docker-release.yml` repeats these
checks on the release tag before pushing versioned GHCR images; multiarchitecture
version and channel tags follow only after both architectures pass. Publishing
a prerelease selects the `beta` channel, while a stable release selects `latest`.

`go test ./internal/installer ./cmd/posternproxy` covers artifact validation,
option parsing, isolated host-file writes, preserved config, additive firewall
commands, startup error propagation, and HTTP readiness probes. Native-agent tests cover identity, protected configuration
and redirect rejection. These are not
fresh VM installation tests. Before publishing a candidate, exercise installation
and reruns on supported systemd hosts, verify custom SSH ports and existing
firewall rules survive, and verify actual proxy traffic and certificate access.

On a native Linux build host with Go installed:

```bash
. internal/caddybuild/build.sh
build_caddy /tmp/posternproxy-test-caddy
verify_caddy /tmp/posternproxy-test-caddy
POSTERNPROXY_TEST_CADDY=/tmp/posternproxy-test-caddy \
  go test -tags=integration -count=1 -v ./internal/caddy
```

The acceptance fixtures build configuration using the production
translator, launches an isolated native Caddy on temporary ports, and checks HTTP
proxy traffic, a rate-limit rejection, and TCP forwarding. It checks the binary's
embedded dependency versions. Missing or incompatible binaries fail the test.
The additional service acceptance fixture runs two real outbound agents against
two native Caddy processes, proves host and ACL traffic changes, saves an offline
edit, reopens the controller database on restart, and verifies convergence and
deletion. It remaps listeners to isolated loopback ports and disables TLS for
the HTTP fixture. HTTPS/custom certs, UDP, TLS passthrough, Linux/container
artifacts, and the wider failure matrix remain release acceptance scenarios.

The client-identity fixture also exercises untrusted spoofed headers, trusted
single/multiple hops, malformed chains, IPv6 visitor addresses, location-route
forwarding, ACLs and independent visitor rate buckets. It verifies a 2 MiB upload,
oversized-header rejection, incremental SSE delivery and a WebSocket echo.
Static per-server trust policy validation rejects invalid or all-address ranges;
there is no remote range updater whose failure can widen trust.

```bash
go build -o /tmp/posternproxy-test-agent ./cmd/posternproxy-agent
POSTERNPROXY_TEST_CADDY=/tmp/posternproxy-test-caddy \
POSTERNPROXY_TEST_AGENT=/tmp/posternproxy-test-agent \
  go test -tags=integration -count=1 -v ./internal/services
```

Pinned component versions do not prove byte-for-byte reproducibility of the
entire dependency graph or establish vulnerability clearance. Record the actual
Go toolchain, module build information, and artifact hashes for release candidates.

## Browser smoke tests

From `frontend/`:

```bash
npm ci
npx playwright install --with-deps chromium
npm run test:e2e
```

The harness compiles the controller and agent and creates disposable data. The
HTTP controller uses `127.0.0.1:18181` and Vite uses `127.0.0.1:18180`. A separate
secure-cookie controller uses `127.0.0.1:18185`, behind a test TLS proxy on
`127.0.0.1:18184`. All four ports must be unused. Chromium maps
`cert-upload.e2e.test` and the cross-site `attacker.test` to loopback without
editing the hosts file. The HTTPS project accepts the disposable self-signed
fixture certificate; it checks cookie/browser behavior, not public CA trust.
Controller output, browser traces, videos and screenshots are disabled. Test
credentials and generated test results are confined to the disposable harness.

The scenarios cover login/refresh/reload/logout, host create/edit/disable,
certificate upload/rejection, manual enrollment, saved-but-failed configuration,
parked HTTP/TLS diagnostics and branding. Encrypted export/restore is exercised
through the UI, including wrong-passphrase refusal and a committed restore
waiting for application. A real outbound agent with a controlled Caddy admin
fixture proves online is distinct from applying/failed/applied, and retry reaches
the acknowledged revision. It does not perform an SSH installation.

The non-localhost HTTPS scenario checks Secure/HttpOnly/Strict cookie attributes,
refresh and logout, missing/mismatched/correct CSRF tokens, and browser exclusion
of cookies on a cross-site request. Recovery/readiness scenarios use distinct
test client identities behind an explicitly trusted loopback proxy; the normal
management API rate limit remains unchanged. OS/service installation and actual
Caddy traffic are qualified separately by native/integration checks.

## Required checks

Run Go tests/build/vet, frontend type-check/build/lint, Hugo, and the smoke and
integration checks. CI additionally runs Linux race detection, Go vulnerability
scanning, and npm audit. Dependency findings must be investigated; adding a scan
does not resolve its findings. Fresh supported Linux VM installs, container
artifacts, upgrade sources, clean recovery, and representative multi-server
observation still require evidence before a release decision.

## Current local evidence

The November 25 Windows/runtime/release changes pass Windows Go tests, build and
vet, Go vulnerability scanning, frontend type-check/build/lint, all 15 browser
scenarios and the Hugo documentation build. Windows tests exercise encrypted
configuration, retained-file trust, service cancellation and child-process
cleanup. Workflow and qualification-script syntax checks pass.

An additional authorized Windows workstation run used the version-stamped
`v0.0.0-windows-local-test` controller/agent and pinned Caddy. It passed actual SCM
controller installation/login/version, separate virtual service identities,
bootstrap cleanup, stable encryption key and credentials on reinstall, restart,
and uninstall retaining data. Native agent installation passed encrypted enrollment,
authenticated outbound connectivity, acknowledged applied revision, verified custom
TLS through Caddy, restart and uninstall. Final independent checks found no managed
services/processes or listeners on the test ports. Binaries and protected data were
retained. The run exposed and fixed SDDL read/execute encoding and a Caddy admin
listener mismatch that prevented Windows reloads; elevated filesystem regressions
also passed. Candidate CI, native arm64 and Docker execution still need their own
successful runs; this does not qualify published release assets.

A separate Ubuntu 24.04.5 amd64 VM passed current development-bundle installation,
fresh login, private files, bootstrap cleanup, preserved configuration on rerun,
native agent enrollment and actual SSH provisioning. Real custom TLS, certificate
group permissions and unrelated-user denial, agent/Caddy restarts, stream and
proxy-trust fixtures, portable TLS recovery and two-agent convergence passed.
All compiled command/internal package test executables also passed natively.
Original SSH authorized keys and firewall rules were unchanged by the native
qualification, before Docker was subsequently installed for container testing.

On that x86-64 VM, QEMU ARM64 controller/agent installation, login, reinstall,
TLS, certificate permissions and restarts passed. ARM64 Caddy passed the pinned
traffic, rate-limit, trust, streaming, UDP and passthrough fixtures. A native test
driver exercising ARM64 Caddy/agents passed recovery and two-agent convergence.
The emulator required a temporary controller service JIT exception; it was removed
after testing and production unit hardening remains intact. These are functional
emulation results, not native ARM hardware, performance or hardening qualification.

ARM64 runtime images also passed the actual Compose smoke suite under QEMU:
enrollment/applied readiness, HTTP/custom TLS, agent restart, Caddy-failure
supervision, controller replacement and retained volumes. These images used the
production Dockerfiles with explicit build contexts supplying the verified
cross-built ARM64 binaries. They establish runtime/package compatibility; native
ARM source-image builds remain a candidate CI check.

The amd64 production Dockerfiles subsequently built both images from source and
passed the same complete Compose suite. Live qualification fixed missing shared
branding paths in the frontend stage and invalid quoting in the Compose secret
validation expression. The smoke harness refreshes ephemeral published ports
after restart, matching observed Docker behavior. The small build VM needed an
explicit frontend build heap limit and temporary swap for compilation. All test
containers and the source builder were stopped afterwards; native services were
disabled, temporary swap removed and the emulator service exception absent.
Docker/QEMU, installed test data and the native test images remain on the VM.

The November 2026 fixes add live custom-certificate HTTPS traffic and encrypted
recovery onto a fresh database with a different controller key. Tests also verify
that rejected configuration retains working TLS. The two-agent restart fixture
continues to check outbound-only reconciliation and traffic.

The user's earlier successful controller installation remains separate evidence.
Container and published-asset installation still need target checks before a
release claim. Local native Windows tests alone do not establish Linux service
permissions; the Ubuntu results below provide that narrower Linux evidence.

Additional November 24 qualification used the current Linux amd64 development
archive on Ubuntu 24.04. It passed controller upgrade/rerun with preserved config,
fresh controller data/login and transient-password cleanup, native agent install,
actual SSH provisioning, applied readiness, verified custom TLS through systemd
Caddy, group-read/unrelated-user-denial checks, and agent/Caddy restarts. The
original controller data was restored afterwards. Pinned traffic, recovery and
two-agent restart fixtures also passed on Linux. Package prerequisites already
existed on this VM; this does not qualify a fresh OS or every supported platform.

The reproducible VM harnesses are `scripts/qualify-native-controller.py` and
`scripts/qualify-native-agent.py`. They require a disposable backed-up systemd
VM and the matching extracted bundle. Read their preconditions before running:
they temporarily replace Caddy configuration, and the agent harness requires
the controller stopped and no existing native agent configuration. The SSH mode
uses a temporary loopback SSH key and removes it afterwards. Never use these
harnesses against production.
