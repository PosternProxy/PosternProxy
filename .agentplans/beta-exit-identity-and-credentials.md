# A. Identity and credential authority

Status: implementation in progress from the earlier A/B/C work; final qualification/sign-off pending. Lead: GPT6.1. Review: Grok 4.7 provisionally, with GPT6.1 fallback. Parent: [development index](beta-exit-development-index.md).

Owns blockers **1 and 2** and the related JWT/session-revocation readiness question. High scrutiny: changes affect every privileged route. The assignment reflects coupled authorization rules and lifecycle design.

## Current evidence and scope

`internal/services/api_token.go` resolves the stored creator role without checking the current owner and discards management scopes. `internal/auth/middleware.go` reads `X-Token-Scope`; the router does not mount trusted scope enforcement. `internal/auth/jwt.go` carries no revocation version. Refresh checks current account activity, but password changes and logout do not revoke a copied refresh token server-side.

Touch points: those files, `internal/services/user.go`, API-token/auth handlers, `internal/api/router.go` and route helpers, database migrations, token UI/types, and relevant tests. B owns canonical client IP; use its helper for audit attribution when integrated.

## Development steps

- [ ] Write a role/scope/credential-type route matrix, including host routes, imports, rollback, provisioning, settings, backup, token administration, and join tokens. Distinguish read operations from mutating POST operations that appear read-like.
- [ ] Add trusted authentication metadata in request context: credential kind, API-token identity, exact parsed scopes, owner, effective current role, and applicable resource restriction. Reject malformed/unknown scopes and orphaned or inactive owners. Keep agent credentials barred from management routes.
- [ ] Replace header-derived scope checks with exact scope membership from context and enforce the matrix centrally using existing mounting helpers. Preserve role and ownership checks independently of scope. Empty scopes must fail closed.
- [ ] Make credential creation non-escalating. Default to requiring a user session to mint management API tokens; if token-based minting is retained, require a proven subset of caller authority, expiry, and resource restriction. Enforce in the service as well as route policy.
- [ ] Resolve current account authority on use and revoke owner credentials on deletion/disable. Demotion takes effect on the next authenticated operation; no stale stored admin role may authorize a request.
- [ ] Define and implement server-side session revocation with migration-backed state: password reset/change and disable/delete revoke sessions, role changes constrain existing access, logout revokes its refresh session, and refresh rotation rejects replay. Document access-token invalidation semantics and ensure revoked authority cannot remain privileged for its normal lifetime.
- [ ] Audit token issue/revoke and account authority changes with actor and safe metadata; never include raw credentials. Update UI errors, API docs, and changelog.

## Regression and acceptance

- [ ] Through the real router, a read token reads permitted resources but cannot create/edit/delete, rollback, import, provision, or mint a stronger credential. Omitted/spoofed `X-Token-Scope` cannot change the result.
- [ ] Test every role/credential-type combination in the matrix, token ownership boundaries, malformed/empty scopes, expiry, revocation, missing owner, and agent-token rejection. Satisfy CSRF in authorization tests so it cannot mask a scope defect.
- [ ] Mint a token, then demote, disable, or delete its owner; assert the next request has the documented reduced authority or rejection. Test concurrent revocation/use and database errors fail closed.
- [ ] Replay old access/refresh credentials after password change, logout, role change, account disable, and refresh rotation. Assert the specified revocation policy after restart too.
- [ ] Run shared checks from the index and record revision/results. All matrix rows have explicit tests; no privilege can be supplied through a client header.

Dependencies: G's upgrade policy before new migrations; B for final audit IP attribution. Can implement trusted scope enforcement without waiting for B/C/D/E/F.

Completion evidence: pending. Move to `completed/` only after both blockers and the chosen session policy have regression coverage and review.
