# AGENTS.md

**Project:** PosternProxy — self-hosted reverse proxy manager on Caddy with native multi-server support via outbound agents.

## Checks

Run `go run ./scripts/check` before claiming work is done. It is the single command
that runs every check — gofmt, `go vet`, staticcheck, complexity and file-size limits,
tests, security scanners (gosec, govulncheck, gitleaks), and the frontend type-check,
lint, build and dependency audit. It needs only the Go toolchain, so the same command
runs on Windows, Linux, and CI, and a green run locally means what CI enforces. Flags:
`-go` or `-frontend` for one half, `-fast` to skip network-fetched tools, `-race` to
add the race detector to the Go tests.

## Core Architecture
- Single static Go binary (controller) with embedded Vue 3 frontend.
- Remote servers run lightweight `posternproxy-agent` that maintains outbound WebSocket connection only.
- Controller applies config to local Caddy or pushes updates to agents.
- All proxying, TLS, load balancing, rate limiting, and L4 streams handled by Caddy (l4 + ratelimit plugins).

The project emphasizes operational features such as SSH-based server provisioning, detailed audit logging with before/after snapshots, configuration history with rollback, and UI-managed port forwarding. These are listed in the main README Features section.

## Licensing & Commercial Model (Strict)
- Open edition: AGPLv3.
- Any commercial use (hosted services, proprietary derivatives, MSP offerings, closed products) requires a separate non-AGPL license purchased directly from the project owner.
- The owner is the sole legal source of non-AGPL rights.
- Never introduce changes that would let third parties offer competing commercial services without a commercial license from the owner.
- Preserve dual-licensing model in all licensing-related work.

## Development Workflow
- Before starting significant work, review documents in the `.agentplans/` directory for relevant context on current initiatives and priorities.
- Update `CHANGELOG.md` (Unreleased section) for user-facing or architectural changes.
- Run `go run ./scripts/check` (add `-go` or `-frontend` to scope it) before committing non-trivial changes; see the Checks section above.
- Prefer small, focused changes. Update docs (README, LICENSING.md, etc.) in the same change when behavior shifts.
- AI agents: keep changes minimal, follow existing patterns, and consider commercial implications of every modification.

## Working with Plans
- When agents create, update, or work on plans, task breakdowns, or execution notes, always save the resulting documents to the `.agentplans/` directory.
- Use clear, descriptive filenames (e.g. `feature-xyz.md` or `task-breakdown-*.md`).
- Place plans directly in `.agentplans/`; use subdirectories only if they match the existing structure.
- At the end of any planning activity, ensure updated plans are saved and referenced in the change (e.g. commit message or PR description).
- `.agentplans/` is the single source of truth for all active and working plans.
- when plans are fully completed, move the completed plans to `.agentplans/completed`

## Security Standards
- Validate and sanitize all external input at the API boundary.
- Use only parameterized queries and prepared statements — no string concatenation for SQL.
- Encrypt all sensitive data at rest (SSH credentials, tokens) using the project's AES-256-GCM helpers.
- Use constant-time comparisons for all token/secret validation.
- Log no secrets, tokens, or credentials.
- Protect mutating and privileged operations with audit logging (actor, IP, before/after where applicable).
- Apply least privilege: admin-only routes must use `RequireAdmin`.
- Treat agent authentication and provisioning paths as high-security — any change here requires extra scrutiny.
- Add HSTS, secure headers, CSRF, and rate limiting on management paths; do not remove or weaken them.

## DRY Principles
- Extract repeated logic (validation, error mapping, response formatting, host route mounting) into shared helpers, middleware, or services.
- Do not duplicate handler or service patterns across resource types — extend existing abstractions (e.g., mountHostRoutes, resource clients).
- Centralize configuration, crypto, and auth helpers.
- Reuse composables and utilities on the frontend; avoid copy-paste between views or forms.
- When similar code appears in three or more places, extract it.

## Code Style Standards
**Go:**
- Follow gofmt and Effective Go.
- Use `zerolog` for all logging (structured, no fmt.Print* in production paths).
- Wrap errors with `%w` and add context.
- Keep functions small and single-purpose.
- Explicit error handling; never ignore errors silently.
- Use `context.Context` for cancellation, deadlines, and request values.
- Prefer table-driven tests.
- Shared logic used by both `cmd/posternproxy` and `cmd/posternproxy-agent` must live in an `internal/` package — never duplicate functions between binaries.

**Frontend (Vue 3 + TypeScript + Tailwind):**
- Use `<script setup>` and Composition API.
- Strong typing — avoid `any`.
- Extract shared logic into composables under `src/composables`.
- Use Tailwind utilities; keep component styles minimal.
- Centralize API error handling via existing utils.

**General:**
- Write clear "why" comments for non-obvious decisions.
- Update tests when adding or changing logic.
- Small functions and files preferred.

## AI Code Generation Hygiene
Code in this project must read as if a human wrote it. Follow these rules strictly:
- **No audit-trail citations.** Never reference ticket IDs, checklist items, or audit codes in comments (e.g. "audit item H-3", "fixes #123"). If the reasoning matters, state it plainly.
- **No pedagogical comments.** Do not explain what standard library functions do, why constant-time comparison is used, or how HTTP status codes work. Comment only what would surprise a Go developer reading the code for the first time.
- **No identical code across packages.** If two `cmd/` binaries need the same function, extract it to an `internal/` package immediately — not on the third use.
- **No uniform templates.** Handlers, services, and tests should reflect the actual complexity of their logic. Do not force every handler into an identical structure when the logic doesn't warrant it.
- **No justification comments on status codes, middleware placement, or standard patterns.** `http.StatusCreated` does not need a comment explaining what 201 means. Middleware ordering does not need a paragraph if it follows the standard stack.

## Key Files & Directories
- `.agentplans/` — the directory for all active task plans, execution directives, and working plans. Agents must save any plans they create or update here. Consult relevant files before starting complex work.
- `LICENSING.md` + `docs/commercial-licensing.md` — licensing model and commercial options.
- `CLA.md` — contributor agreement (required for contributions).
- `SECURITY.md` — security policy.
- `.claude/` — Claude-specific plans and todos (use when working with Claude).

## When Making Changes
- Small fixes: implement + test + changelog.
- Anything affecting agents, provisioning, auth, licensing, or multi-server behavior: review `.agentplans/` first and consider dual-licensing impact.
- Always preserve the outbound-only agent model and single-binary controller constraint.

Stay concise. Prioritize security, the dual-licensing model, and consistency with existing patterns.