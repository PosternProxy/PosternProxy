# NTLM authentication passthrough for HTTP proxy hosts

- **Size:** large
- **Zone:** yellow (authentication boundary; opt-in, single-upstream, legacy). No `docs/risk-zones.md` in this repo yet.
- **ADRs:** 0013
- **Security requirements:** no new management-plane exposure; the toggle grants no privilege; `Authorization`/`WWW-Authenticate` pass unmodified; TLS verification unchanged (see ADR-0001 §4–5). No `docs/security.md` IDs exist in this repo yet.
- **Status:** Built
- **Agreed by:** Michael (owner), who ordered execution; delivered in commit `0ea37ed`

<!-- Built record of the delivered revision. Current behaviour is documented at docs/content/proxy-hosts/_index.md. -->

## Why

Administrators with legacy IIS apps using Integrated Windows Authentication need them behind the proxy instead of NAT'd to the internet. A standard reverse proxy breaks NTLM's connection affinity, so today those hosts stay on direct NAT with manually managed certificates.

## Out of scope

SSO, Kerberos delegation, AD enrollment, Windows credential storage, NTLM on the management API, load balancing or failover, multiple upstreams, TLS passthrough on the same host, automatic IIS changes, and global HTTP listener downgrades.

## Change from current behavior

Adds `ntlm_enabled` (migration `006`, default 0) through the model, service scans/writes, API read/write types, frontend form, backup, audit allowlist, and history rollback. Enabled hosts emit `transport.protocol = "http_ntlm"`; disabled hosts emit `"http"`. The shared Caddy build contract pins the fork module and verifies it in the artifact. Access lists, rate limits, HSTS, health checks, and unrelated hosts are preserved. Current behaviour and limitations are documented at `docs/content/proxy-hosts/_index.md` (marked Legacy, IIS not verified).

## Acceptance criteria

1. **Given** an upgrade or old backup/snapshot **when** loaded **then** existing hosts stay off and keep their behaviour. (AC1)
2. **Given** an authorized admin **when** they create/edit/read a host **then** `ntlm_enabled` persists; an unauthorized write is refused with no state change. (AC2)
3. **Given** NTLM enabled with a conflicting setting (extra upstreams, locations, TLS passthrough, advanced JSON, an `Authorization`/`WWW-Authenticate` header rule incl. case/wildcard variants, or a custom 401 page) **when** saved via API, import or rollback **then** it is rejected and neither the row nor `desired_revision` changes. (AC3)
4. **Given** the flag **when** config is generated **then** enabled selects `http_ntlm` and disabled restores `http`, with verified HTTPS and unrelated policy intact. (AC4)
5. **Given** a managed artifact **when** verified **then** it contains the pinned module; an absent/wrong version fails verification. (AC5)
6. **Given** a Caddy without the module **when** applied **then** application fails visibly, the last working config and `applied_revision` are retained, and there is no `http` fallback. (AC6)
7. **Given** real IIS **when** users authenticate **then** they are themselves, concurrent users from one source address never see each other's identity, failures recover safely, and a sustained workload shows no leak or unbounded retained connections. (AC7–AC11 — **pending**, no IIS environment)

## Verification boundaries

Unit/integration cover AC1–AC6 and generated JSON; Playwright e2e covers the form; the fork's concurrency is covered by `-race`. AC7–AC11 require a live IIS lab and are not verified. Expected results come from the criteria, not copied production logic.

## Requirement trace

| Requirement | Criterion | Tests | Implementation revision | Verification evidence |
|---|---|---|---|---|
| Persistence / upgrade | AC1 | `internal/database/migrate_test.go`, `internal/services/ntlm_backup_test.go`, `internal/app/ntlm_rollback_test.go` | `0ea37ed` | pass |
| Persistence / permissions | AC2 | `internal/api/ntlm_passthrough_test.go`, audit allowlist in services tests | `0ea37ed` | pass |
| Conflict rejection | AC3 | `internal/models/ntlm_test.go`, `internal/services/ntlm_proxy_host_test.go`, `ntlm_backup_test.go` | `0ea37ed` | pass |
| Transport selection | AC4 | `internal/caddy/ntlm_route_test.go` | `0ea37ed` | pass (generated JSON) |
| Build contract | AC5 | `internal/caddybuild/*` fixtures | `0ea37ed` | pass |
| Fail-closed apply | AC6 | `internal/services/ntlm_apply_test.go` | `0ea37ed` | pass |
| Fork concurrency | — | fork `ntlm_test.go` | `66b6501` | `-race` clean (reproduced) |
| IIS end-to-end | AC7–AC11 | not written | `0ea37ed` | pending (no IIS) |

See [../evidence/0013-ntlm-passthrough.md](../evidence/0013-ntlm-passthrough.md) for commands and results.

## Open questions

- IIS Extended Protection matrix (which browser-TLS / upstream-scheme / EPA combinations authenticate through a terminating proxy) and downstream protocol behaviour — resolved only by AC7–AC11.

## Decisions this touches

0013
