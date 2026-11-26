> Current status (2026-11-25): implementation and platform development
> qualification are complete; fresh review follow-ups are fixed. Exact candidate
> CI, draft artifact checks and owner publication remain. See
> [release runbook](beta-release-runbook-2026-11-25.md) and
> [closure ledger](beta-exit-blocker-closure-ledger.md). Older checkpoints below
> are historical and do not replace this status.

# Beta-exit development program

Date: 2026-10-24, updated 2026-10-25. Status: in progress. G foundations and A-C implementation have started; D-F remain planned. C's core implementation and local acceptance are recorded below.

Source: [beta-exit review](beta-exit-review-2026-09-18.md). The original review remains the historical evidence record. This index and its linked plans define the follow-up work.

## Review basis

Read the review and inspected the current working-tree implementations of authentication, routing, rate limiting, agent transport/protocol, synchronization, access lists, provisioning, certificates, backup, migrations, and CI. The reported failure patterns remain visible in source. This is a planning/source review; the six original reproductions, builds, live deployments, and browser tests were not rerun. Do not interpret this document as a fresh exploit reproduction or a completed fix.

The working tree already contains substantial forwarding changes. Preserve them and establish a reproducible baseline before implementation. The forwarding plan says complete, but the current router still has no `/api/health/forwards` route; resolve that discrepancy in the release-validation plan.

## Groups and model assignments

| Group | Original blockers | Lead model | Development plan |
| --- | --- | --- | --- |
| A. Identity and credential authority | 1, 2 | GPT6.1 | [Identity](beta-exit-identity-and-credentials.md) |
| B. Request trust and secret-safe transport | 3, 4 | GPT5.6 | [Request trust](beta-exit-request-trust-and-agent-auth.md) |
| C. Desired/applied configuration and reconciliation | 5, 7, 11 | GPT6.1 | [Configuration lifecycle](beta-exit-config-lifecycle.md) |
| D. Installation and SSH provisioning | 6 | Sonnet 5.5 | [Provisioning](beta-exit-provisioning.md) |
| E. Certificate isolation and file access | 9, 10 | Opus 4.8 | [Certificate lifecycle](beta-exit-certificates.md) |
| F. Portable backup and clean restore | 8 | Opus 4.8 | [Recovery](beta-exit-backup-and-recovery.md) |
| G. Release qualification and remaining readiness gaps | All six exit gates; other readiness gaps | GPT6.1 | [Release qualification](beta-exit-release-qualification.md) |

Assignments are engineering judgments based on task ambiguity, coupling, and consequence of mistakes, not measured comparisons of these exact versions. Use the user's labels as written. GPT6.1 is interpreted as the GPT-6.1 Sol option; GPT5.6 requires selecting the intended 5.6 variant at execution. [Official OpenAI guidance](https://developers.openai.com/api/docs/guides/model-selection) supports GPT-6.1 Sol for complex technical work. Exact availability and comparative capabilities of Grok 4.7, Opus 4.8, Sonnet 5.5, and Haiku 5.5 were not verified; their assignments are provisional, subject to access and a representative task check. No pricing or benchmark ranking is assumed.

Use Grok 4.7 provisionally for independent adversarial review of A/B/C/E/F after implementation, with repository access and runnable tests. Use Haiku 5.5 provisionally for bounded documentation, checklist reconciliation, and evidence indexing after technical outcomes are established. Neither owns security design or release sign-off. If a named model is unavailable or fails the task check, use GPT6.1 for that work. There is no requirement to use every model for implementation, and no agents or chats have been launched by these assignments.

## Order and dependencies

1. G first establishes the supported upgrade baseline, pinned Caddy/plugin build, test fixtures, and evidence format. These foundation tasks do not wait for fixes.
2. A and B are independent initial security work. C first fixes the access-list deadlock, then establishes the revision/acknowledgment/status contract.
3. E can develop certificate selection independently; coordinate its activation/retirement contract with C. D can correct installer defects immediately, but its final readiness checks consume B's transport, C's apply acknowledgments, and E's permission contract.
4. F can design and test format validation immediately. Final restore uses C's per-server reconciliation and E's secure certificate staging. New schema work uses G's migration policy.
5. G runs the full acceptance matrix after A-F, resolves documentation/audit gaps, and evaluates the release candidate.

These are logical work streams, not authorization to run concurrent edits in a shared checkout. C/E/F overlap in `caddy_sync.go`; B/C/D/E overlap in the agent; A/B share router middleware. Agree contracts first and integrate focused changes in order.

## Shared implementation rules and completion evidence

- Preserve the outbound-only agent connection, embedded Vue UI, single controller binary, Caddy data plane, and existing dual-license model. No licensing changes are planned.
- Validate API input; retain CSRF, HSTS, secure headers, rate limiting, admin checks, secret encryption helpers, and safe token verification. Use parameterized SQL values and strictly allowlisted identifiers.
- Reuse shared services/helpers under `internal/`, structured zerolog, contexts and deadlines, typed frontend APIs, and existing audit patterns. Redact secrets from logs, audit snapshots, history, and test artifacts.
- Each implementation change updates Unreleased in `CHANGELOG.md` and relevant docs. Planning-only changes do not claim a product change in the changelog.
- For Go changes: `go test ./...`, `go build ./...`, and `go vet ./...`. For frontend changes, run `npm run type-check`, `npm run build`, and `npm run lint:check` in `frontend/`. Run relevant regression/integration tests in addition. Linux race and dependency checks belong in G.
- Each plan records checkboxes, implementation revision, tests actually run, artifacts, residual risks, and reviewer outcome. A failure log becoming quiet is not acceptance evidence.
- Move a completed plan into `.agentplans/completed/` only after its criteria pass and evidence is recorded; update this index's links when moving it. Keep the source review as a historical record and maintain blocker closure here.

## Current blocker closure

Every blocker has exactly one implementation owner in the table above. C's
original failure patterns (5/7/11) now have passing local regressions and native
two-agent acceptance; release qualification remains open. A/B changes are in
progress and have not received final sign-off. Do not infer closure of the
remaining beta-exit gates from local implementation results.

Per-blocker fix + regression status for all 11 is tracked in the
[blocker closure ledger](beta-exit-blocker-closure-ledger.md) (S1). As of
2026-11-24 every blocker has an implemented fix and a passing local regression,
but none is closed: closure is gated on independent reviewer sign-off and Linux
race/CI execution of the regressions.

Plan G execution evidence: [foundation checkpoint](beta-exit-g-foundation-checkpoint.md).

Plan C execution evidence: [2026-10-25 checkpoint](beta-exit-c-checkpoint-2026-10-25.md).


## Next-release feature backlog

Current-beta correctness and Ubuntu qualification completed November 24:
[evidence and remaining release gates](beta-security-qualification-2026-11-24.md).
The original broader release gates remain open; this is not a release approval.

Owner selected a **broader beta** scope on November 24, 2026. Remaining work to a
tagged release is sequenced in the
[broader beta release execution plan](broader-beta-release-execution.md), which
extends gate G without replacing it. Cloudflare is the one documented exclusion.

- [Access logs, TLS events and denied-request actions](next-release-logging.md)
  — requested November 24, 2026; TODO. Includes outbound agent log browsing,
  highlighted denials and an audited action to add a denied client IP to an access
  list. This is future feature scope, separate from the current release gates.

- [Optional Tailscale integration](next-release-tailscale.md) — proposed November
  24, 2026. Explore private management, outbound agent connectivity and Caddy
  tailnet upstreams; decide host-service versus embedded tsnet before implementation.

- [Static sites and public tailnet ingress](next-release-static-sites-and-tailnet-ingress.md)
  — proposed November 24, 2026. Connect private Gitea for Postern-managed builds
  and publication on a public droplet, with external CI uploads optional; Caddy
  also exposes selected tailnet origins on custom domains.

- [Security release boundary](beta-security-release-boundary.md) and
  [next-release security integrations](next-release-security.md) — proposed
  November 24, 2026. Current beta prioritizes correct Caddy client identity,
  safe deployment and release qualification; optional engines and productized
  integrations follow in the next release.
