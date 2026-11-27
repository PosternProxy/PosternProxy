# 0006. Caddy is built reproducibly from a pinned contract and verified in the artifact

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made during the beta-exit program)
- **Deciders:** Michael (owner)
- **Topics:** build, supply-chain, data-plane

## Context

The data plane is an external Caddy binary (ADR 0001) that must be reproducible, contain the required modules, and be the same across every managed server.

## Decision

1. Caddy is built with **xcaddy** from a single pinned build contract: Caddy, xcaddy, the Go toolchain, `caddy-l4`, `caddy-ratelimit`, and the security replacements, each pinned to an exact version.
2. Artifacts are **verified against the pins** before installation — required module IDs and versions are present and there are no unexpected `replace` directives.

## Consequences

- Easier: every server runs a known, verifiable Caddy build.
- Harder: adding a module (e.g. NTLM, ADR 0013) means extending the pinned contract in every consumer (`build.sh`, `build.ps1`, `verify.go`, fixtures).
- Now a rule, with its check: `internal/caddybuild/verify.go` plus `build.sh` / `scripts/build.ps1` fail the build or install when a pinned module is absent or at the wrong version.

## Evidence

`internal/caddybuild/build.sh`, `internal/caddybuild/verify.go`, `scripts/build.ps1`. See [../evidence/0006-pinned-caddy-build.md](../evidence/0006-pinned-caddy-build.md).
