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

The tagged-release workflow builds Linux amd64/arm64 controller, agent, and Caddy
assets, platform tar archives and a combined SHA-256 manifest, then creates a draft release for owner
review. Local build scripts also emit a manifest. A draft must be published before
the command can fetch its assets without authentication.

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

The harness compiles the controller, creates a disposable database/data directory,
and starts it on `127.0.0.1:18181` with a Vite frontend on `127.0.0.1:18180`.
Both ports must be unused. It does not reuse an existing controller. Fixed test
credentials and test-only insecure-cookie mode apply only to this isolated HTTP
fixture. Controller output, browser traces, videos, and screenshots are disabled
to avoid persisting credentials. Generated test results are ignored by Git.

The smoke scenarios check login, HttpOnly/Strict refresh-cookie properties,
refresh after reload, logout, reload after logout, and visible configuration
failure after a saved host cannot reach Caddy. They do not qualify secure
cookies over HTTPS, non-localhost deployment, provisioning,
custom cert upload, or backup restore. These remain release gates.

## Required checks

Run Go tests/build/vet, frontend type-check/build/lint, Hugo, and the smoke and
integration checks. CI additionally runs Linux race detection, Go vulnerability
scanning, and npm audit. Dependency findings must be investigated; adding a scan
does not resolve its findings. Fresh supported Linux VM installs, container
artifacts, upgrade sources, clean recovery, and representative multi-server
observation still require evidence before a release decision.

## Current local evidence

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
