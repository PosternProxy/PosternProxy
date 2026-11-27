# 0002. The controller is a single static Go binary with an embedded Vue frontend

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made at project inception)
- **Deciders:** Michael (owner)
- **Topics:** architecture, packaging

## Context

The product has a Go backend and a Vue 3 single-page UI. These could ship as separate services (API + a web server for assets) or as one artifact.

## Decision

1. The controller is a **single static Go binary** with the built Vue 3 frontend **embedded** at build time.
2. There is no separate web server or asset deployment step.

## Consequences

- Easier: one artifact to distribute and run; no asset/version skew between UI and API.
- Harder: the frontend must be built and embedded during the Go build; UI and API are versioned and released together.
- Now a rule: the frontend ships embedded; do not introduce a separately deployed web tier.

## Evidence

`cmd/posternproxy` with the embedded frontend; `AGENTS.md`. See [../evidence/0002-single-binary-embedded-frontend.md](../evidence/0002-single-binary-embedded-frontend.md).
