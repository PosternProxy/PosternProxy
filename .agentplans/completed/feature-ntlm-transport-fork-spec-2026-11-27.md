# NTLM transport fork — implementation spec

Date: 2026-11-27. Status: **Complete (fork module), 2026-11-27.** Fork built and
reviewed at commit `66b6501bd51c704ebd714eda0d6888080f0ff7c4` (Caddy `v2.11.9`):
build clean, 7/7 tests pass, module path renamed, no replace directive, Apache-2.0
`NOTICE`. Outstanding: run `go test -race` on a cgo-capable host; optionally
re-split the squashed commit for upstreaming. The PosternProxy-side build-contract
pin remains open as the design doc's PR 3.
Scope: create and maintain a minimal fork of the Caddy `http_ntlm` transport
module so PosternProxy can pin a correct, immutable revision, instead of waiting
on an upstream repository that has been effectively dormant since 2023.

This spec covers **the fork module only** and the build-contract wiring that
consumes it. It is the resolution of Open Question 1 / Key Decision 3 /
Alternative 2 in `.agentplans/design-ntlm-passthrough.md`, which previously left
the pin blocked on upstream and rejected a fork. Those are now superseded: the
decision is a minimal, upstreamable fork. All other contracts in the design doc
and `.agentplans/feature-ntlm-passthrough-spec.md` stand unchanged.

## Task and authority

Fork `github.com/caddyserver/ntlm-transport` (Apache-2.0) into a project-owned
repository, apply the smallest set of correctness fixes that make the transport
safe to compile and run, and pin that revision into the PosternProxy build
contract. Do **not** add features, change the Caddy module ID, reimplement the
NTLM handshake, or adopt client-controlled headers as connection identity.

The fork exists to fix lifecycle defects in the upstream source, nothing more.
If a fix cannot be made without changing observable transport behavior beyond the
three defects below, stop and report the conflict rather than expanding scope.

Do not open any pull request, upstream or within the fork. Push branches and
commits and draft PR descriptions; the owner opens every pull request.

## Repository decision

- **Separate repository**, not a subdirectory of the controller module. Proposed
  path `github.com/PosternProxy/ntlm-transport` (confirm the exact org with the
  owner before creating it).
- **Rename the Go module path** in `go.mod` to that fork path. Pin the fork by
  that path directly in the build contract. Do **not** use a Go `replace`
  directive anywhere — `internal/caddybuild/verify.go` requires `Replace == nil`.
- **Keep the Caddy module ID string `http.reverse_proxy.transport.http_ntlm`**
  (registered via `caddy.RegisterModule`). It is independent of the Go module
  path. Generated Caddy JSON (`"protocol": "http_ntlm"`) must not change.
- **Preserve Apache-2.0.** Keep upstream copyright and license headers. Add a
  `NOTICE` recording that this is a fork of `caddyserver/ntlm-transport` and
  summarizing the changes. Prepare Fix 1 and Fix 2 as clean, cherry-pickable
  commits against the upstream module path and draft a PR description for them, so
  the owner can submit them upstream later; do not open any pull request, and do
  not block the pin on whether upstream accepts them.
- The controller `go.mod` gains nothing. The controller never imports the module.

## Defects to fix (and only these)

Read the forked `ntlm.go` in full before editing. Match the real control flow;
the line numbers below are from the reviewed upstream source and may shift.

### Fix 1 — lock leak in `RoundTrip` on `newTransport` failure

`RoundTrip` acquires `transportsMu` and, on the path where `n.newTransport()`
returns an error, returns the error **without releasing the mutex**. The only
unlocks are in the cache-hit branch and after a successful `newTransport`. Once
`newTransport` fails once, the mutex is held for the life of the process: every
later `RoundTrip` blocks acquiring it, and `Cleanup` (reload/stop) blocks too,
hanging config reloads.

- Restructure the critical section so **every** return path releases
  `transportsMu` exactly once. Prefer a deferred unlock scoped to the critical
  section; if you keep explicit unlocks, ensure no path double-unlocks and no
  path leaks.
- Leave `Cleanup`'s existing ordering intact (it already releases `transportsMu`
  before `CloseIdleConnections`; that is the one correct recent upstream fix).

### Fix 2 — client-bound transport leaks after failed or canceled RoundTrip

A per-client transport is created and stored in the map on first authenticated
request, and removed only when the wrapped upstream connection's `Close` runs.
On a failed dial, a failed authenticated `RoundTrip`, or a canceled request
context, that `Close` may never fire, so the map entry (and its transport)
persists until the next `Cleanup`. Under churn this grows unbounded.

- On a failed or canceled authenticated `RoundTrip` for a transport this call
  created, remove it from the map under the lock and close it, so the map does
  not accumulate dead entries. Do not remove a transport that a concurrent
  in-flight request is still using; key the cleanup to the entry this call owns.

### Fix 3 — enforce one upstream socket per downstream connection

The upstream module never sets `MaxConnsPerHost`, so one downstream address maps
to one `*http.Transport` that may still open multiple upstream sockets under
concurrency. That is the cross-user isolation gap.

- In `newTransport`, set `MaxConnsPerHost = 1` on the created `*http.Transport`
  (with keep-alive left enabled, as upstream does). This binds one downstream
  connection to exactly one upstream socket, which is what NTLM correctness and
  user isolation both require.
- Requests that share a single downstream connection then serialize over that one
  upstream socket. That is intended. Parallelism across distinct downstream
  connections is unaffected (distinct `RemoteAddr` → distinct transport).

### Explicitly out of scope / unchanged

- Keep keying by `req.RemoteAddr` only. **Never** read `X-Forwarded-For`,
  `X-Real-IP`, or any client-supplied header as identity.
- Keep forcing upstream HTTP/1.1 (`Versions = []string{"1.1"}` in `Provision`).
- Keep the module ID, the embedded `*reverseproxy.HTTPTransport`, and the dial
  wrapper. Do not "fix" keep-alive to match the stale README; follow the code.
- Do not weaken TLS verification or add an insecure-skip switch.

## Compatibility

- `go.mod`: require `github.com/caddyserver/caddy/v2 v2.11.9` (upstream pins an
  older Caddy). `go` directive `1.26`.
- Must build under xcaddy `v0.4.6` with `GOTOOLCHAIN=go1.26.11`, alongside the
  existing L4 pin, rate-limit pin, and security replacements from
  `internal/caddybuild/build.sh`. Do not alter those.

## Tests the fork must contain

Table-driven where it fits; run the whole package under `go test -race`.

1. **Lock release on error.** Force `newTransport` to fail, then issue another
   `RoundTrip` from a goroutine guarded by a timeout; assert it returns rather
   than blocking. A hang is a failed test.
2. **No map growth on failure/cancel.** Drive failed and context-canceled
   authenticated requests and assert the transports map returns to its prior size
   (entry removed, transport closed).
3. **Single upstream socket.** Assert the transport `newTransport` builds has
   `MaxConnsPerHost == 1`.
4. **Keying.** Distinct `RemoteAddr` values get distinct transports; the same
   `RemoteAddr` reuses one; non-NTLM/Negotiate requests do not allocate one.
5. **Cleanup.** `Cleanup` swaps the map out and closes idle connections without
   deadlocking when called concurrently with in-flight `RoundTrip`.

Unit tests prove the lifecycle fixes only. Cross-user identity isolation and IIS
behavior are proven later, in the PosternProxy qualification described by the
feature spec — not in this module.

## Build-contract wiring in PosternProxy

This lands in the controller repo as the design doc's PR 3, using the fork pin.

- `internal/caddybuild/build.sh`:
  - `CADDY_NTLM_VERSION=<pseudo-version xcaddy resolved for the fork commit>`
  - `build_caddy` adds `--with github.com/PosternProxy/ntlm-transport@$CADDY_NTLM_VERSION`
    after the existing plugins; keep every security replacement.
  - `verify_caddy` also requires the list-modules line
    `http.reverse_proxy.transport.http_ntlm <that version>`.
- `internal/caddybuild/verify.go`: add `github.com/PosternProxy/ntlm-transport`
  to the `want` map with `Replace == nil`.
- `scripts/build.ps1`: add `CADDY_NTLM_VERSION` to the pin regex, require seven
  pins instead of six, and add the third `--with`.
- Fixtures that stub `list-modules` / buildinfo: `internal/caddybuild/build_test.go`,
  `internal/caddybuild/build_test.sh`, `internal/caddy/acceptance_test.go`,
  `internal/caddy/stream_acceptance_test.go`.
- Record the pin: Apache-2.0 notice for the fork in the README plugin table and
  installation docs, plus `CHANGELOG.md` Unreleased. No new license workflow.

To resolve the pseudo-version: from a clean module cache, build Caddy `v2.11.9`
with xcaddy `v0.4.6` under `GOTOOLCHAIN=go1.26.11` and the fork `--with`; record
the version `go list -m` selects (or the binary's buildinfo), confirm
`caddy list-modules --versions` prints the module at that version, and confirm
`VerifyArtifact` sees it with no replace directive.

## Deliverables and reporting

- The fork repo: renamed module path, the three fixes, the tests, Apache-2.0
  `NOTICE`, `go.mod` on Caddy `v2.11.9`. A ready-to-submit upstream branch and a
  drafted PR description for Fixes 1–2 — the owner opens any PR, not the implementer.
- The resolved pin string and the commit it maps to.
- `go test -race ./...` output for the fork.
- The controller-side diff wiring the pin through `build.sh`, `verify.go`,
  `build.ps1`, and the fixtures, with `go build ./...` and the build-contract
  tests green.
- A short report: exact commands, resolved revision, changed files, and any point
  where the real source forced a deviation from this spec.

Do not mark anything verified beyond what the tests cover. Shipping a release that
emits `http_ntlm` still depends on the feature spec's acceptance work; this spec
delivers a safe, pinnable module and the build wiring, not the end-to-end proof.
