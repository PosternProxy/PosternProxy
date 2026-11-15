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
and embedded SSH provisioning consume it. Installers check the core version and
both plugin versions, including when Caddy is already installed. An incompatible
existing Caddy is rejected; plan replacement explicitly rather than silently
reusing it.

When distributing an installer separately from the repository, include this file
beside it as `caddy-build.sh`. Repository-layout installations use the internal
path automatically. Docker builds use the repository root as their build context.

On a native Linux build host with Go installed:

```bash
. internal/caddybuild/build.sh
build_caddy /tmp/posternproxy-test-caddy
verify_caddy /tmp/posternproxy-test-caddy
POSTERNPROXY_TEST_CADDY=/tmp/posternproxy-test-caddy \
  go test -tags=integration -count=1 -v ./internal/caddy
```

The initial acceptance fixture builds configuration using the production
translator, launches an isolated native Caddy on temporary ports, and checks HTTP
proxy traffic, a rate-limit rejection, and TCP forwarding. It checks the binary's
embedded dependency versions. Missing or incompatible binaries fail the test.
The additional service acceptance fixture runs two real outbound agents against
two native Caddy processes, proves host and ACL traffic changes, saves an offline
edit, reopens the controller database on restart, and verifies convergence and
deletion. It remaps listeners to isolated loopback ports and disables TLS for
the HTTP fixture. HTTPS/custom certs, UDP, TLS passthrough, Linux/container
artifacts, and the wider failure matrix remain release acceptance scenarios.

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
