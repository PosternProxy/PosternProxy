# PosternProxy beta-exit review — 2026-09-18

**Recommendation: remain in beta.** The architecture is a reasonable foundation, but there are confirmed authorization, configuration application, and recovery failures. Stabilize the existing feature set before adding more features or declaring 1.0.

Follow-up planning (2026-10-24): [development groups, model assignments, dependencies, and closure tracking](beta-exit-development-index.md). The findings and verification below remain the historical September review; the follow-up plans do not mark any blocker resolved.

Reviewed HEAD `380b6d0` plus the pre-existing working-tree changes, including removal of OS-level port forwarding. This was a source review with local builds, the existing test suite, and temporary targeted reproductions. No production systems were contacted. Application source was left unchanged; temporary reproduction tests were removed after execution.

## Verification

| Check | Result |
| --- | --- |
| `go test -count=1 ./...` | Passed |
| `go build ./...` | Passed |
| `go vet ./...` | Passed |
| `npm run type-check` | Passed |
| `npm run build` | Passed |
| `npm run lint:check` | Passed |
| Targeted bug reproductions | Six expected-behavior checks failed, confirming the problems below |

Go and Vite initially encountered sandbox cache/subprocess restrictions; normal verification succeeded with approved execution. A coverage attempt could not finish globally because this Go installation lacks `covdata`. It did report handlers at 0.9%, services at 25.3%, agent hub/protocol at 32.6%, Caddy translation/client at 53.3%, and auth at 93.3%. These are package measurements, not an overall coverage figure. Several critical packages have no tests. Frontend package scripts contain no behavioral test runner.

Not validated: live Caddy/plugin acceptance and proxy traffic, Linux/systemd provisioning, Docker deployment, browser end-to-end behavior, race detection, dependency vulnerability scanning, or release artifact installation. Passing builds should not be interpreted as validation of these flows.

## Release blockers

### 1. P1 — Read-only API tokens can obtain write access

`internal/services/api_token.go:203-233`, `internal/auth/middleware.go:73-90`, `internal/api/router.go`.

The resolver reads scopes only to reject agent tokens. Returned claims discard read/write scopes, and the router does not mount `RequireScope`. That helper also trusts an optional client-supplied `X-Token-Scope` header, so simply wiring it in would not be sufficient.

**Reproduced through the real router:** create an admin-owned token with `scopes: ["read"]`, then use it for `POST /api/tokens` requesting read/write scopes. The API returns **201** and mints the stronger token. The existing CSRF check was satisfied with a matching cookie/header; it is not a token authorization boundary.

**Required:** carry trusted token identity and scopes in request context, enforce scopes on operations, and prevent a token from minting a more privileged credential. Cover the real router with authorization tests.

### 2. P1 — Disabled or demoted users retain API-token authority

`internal/services/api_token.go:164-233`, `internal/services/user.go`, `internal/database/migrations/001_initial.sql:228`.

Resolution uses the role copied into the token at creation and never checks the current user or active status. User deletion sets `created_by` to NULL rather than revoking the credential. API tokens can be non-expiring.

**Reproduced:** after changing the token owner's role from admin to viewer and setting `is_active=0`, the same token still minted another credential with HTTP **201** and the stored admin role.

**Required:** define and enforce account revocation semantics, check current ownership/status/role, and revoke affected credentials. Separately review JWT/session revocation: password changes do not invalidate existing refresh tokens, and logout only clears the browser cookie.

### 3. P1 — Client-supplied forwarding headers bypass rate limits

`internal/api/router.go:55`, `internal/security/ratelimit.go:98-101`.

The router unconditionally uses chi `RealIP`; rate limiting and audit attribution then trust the rewritten `RemoteAddr`. There is no trusted-proxy boundary. This is reachable in the documented direct-to-port-81 deployment.

**Reproduced with the same middleware chain:** two requests from the same network peer, changing only `X-Forwarded-For`, both passed a one-request limiter. The second returned **204**, rather than 429.

**Required:** trust forwarding headers only from explicitly configured proxy addresses and normalize direct peer IPs independently of their source ports.

### 4. P1 — Agent authentication secrets reach controller request logs

`internal/api/router.go:56`, `internal/api/handlers/servers.go:343-355`, `cmd/posternproxy-agent/main.go:105-107`.

The agent sends its token in the WebSocket URL query. The global chi logger formats `r.RequestURI`, including the query, on request completion. This was confirmed in the locally installed chi v5.2.5 implementation (`middleware/logger.go:118`). The agent redacts its own log URL, but controller logging does not.

**Required:** move the credential into an authorization header and redact authentication material consistently. Account for previously written credentials when rolling out the fix.

### 5. P1 — Any access-list row can block full config generation

`internal/services/caddy_sync.go:305-338`, `internal/database/database.go:30`.

The database allows one open connection. `loadAccessLists` holds an outer query open and requests another connection for each list's clients before consuming or closing the outer result set. This blocks even if the list has no clients or is not assigned to a host.

**Reproduced:** insert one access list into a fresh production-configured database and call `buildInput` with a 250 ms context. It fails with **`load access lists: context deadline exceeded`**. Startup uses a long-lived context, so this can also monopolize the only database connection until cancellation.

**Required:** finish reading/closing the outer rows before loading clients, or use a joined/batched query. Also wire access-list mutations into synchronization: `AccessListService.Update` currently changes only the database and audit log, leaving live Caddy rules unchanged.

### 6. P1 — SSH provisioning cannot reliably produce a working agent

`scripts/install.sh:188-194`, `internal/config/config.go:38-44`, `internal/provisioner/provisioner.go:122-138`, `internal/provisioner/scripts.go:64-80`, `cmd/posternproxy-agent/main.go:37`.

Several directly traceable mismatches exist in the supported systemd path:

- The controller installer writes `POSTERNPROXY_AGENT_BIN_PATH`, while Go reads `POSTERNPROXY_AGENT_BIN`. An unset path causes provisioning to upload an empty executable placeholder.
- Systemd provisioning never writes `POSTERNPROXY_SERVER_ID`; agent startup requires it and exits without it.
- Its Caddy unit names `/etc/caddy/config.json`, but the script never creates that file on a fresh machine and suppresses the Caddy startup error.
- This provisioning path builds Caddy with L4 only, omitting the rate-limit plugin used by generated configs.
- It resets UFW and allows the default SSH service rather than the configured SSH port, risking loss of access on nonstandard SSH installations.

These are source-confirmed inconsistencies; no remote server was provisioned during review.

**Required:** consolidate installer/provisioner assumptions and exercise the actual UI provisioning path on fresh supported Linux VMs, including nonstandard SSH ports. Declare success only after agent authentication and successful config application.

### 7. P1 — Failed Caddy application is reported as success

`internal/services/proxy_host.go:159-171,240-246,298-319`, `internal/agent/hub.go:228-236`, `internal/api/handlers/backup.go:83-86`.

Host changes commit to SQLite, log Caddy errors, and return success. Agent command failures are likewise logged without durable applied/error state. Backup import reports `synced: true` even when its sync failed.

**Reproduced:** a local HTTP test server rejected every Caddy load with HTTP 400. `ProxyHostService.Create` still returned a host and a nil error. This also affects disabling/deleting routes: saved state can claim a route is gone while the active proxy still serves it.

**Required:** distinguish saved, queued, applied, and failed states; report application failures to users and persist enough state for retry/reconciliation. Test rejected configs, unreachable Caddy, and agent command rejection.

### 8. P1 — Backup export is insufficient for disaster recovery

`internal/services/backup.go:26-35,153-176`.

Backups include hosts with `server_id` references but omit the servers themselves. Certificate backups contain file paths rather than PEM/key contents. Restore only resynchronizes the local server.

**Reproduced:** export a database containing one remote server and its proxy host, then import into a fresh installation. Restore fails with **`FOREIGN KEY constraint failed (787)`** when inserting the host. The existing backup test restores a local-only backup into its original database, so it misses this case.

**Required:** define a portable restore format and server mapping/enrollment policy, include necessary certificate material securely, and verify restoration into a fresh installation with remote hosts. If intentionally offering configuration-only export, explicitly limit its claims and reject unsupported restores safely.

### 9. P1 — Every agent receives all uploaded TLS private keys

`internal/services/caddy_sync.go:76-103,157,277-283`, `internal/caddy/tls.go:18-42`.

Certificate loading is global rather than limited to the target server's host references. Remote synchronization reads and transmits every custom certificate and private key, even to servers that use none of them. This materially expands the impact of a compromised remote server.

**Required:** compute the certificate set needed by each server and distribute only that set. Test that an unrelated server receives no key material and decide how to retire no-longer-needed remote files.

### 10. P1 — Uploaded certificate permissions conflict with Linux service users

`scripts/install.sh:145-147,225-226,254-255`, `internal/services/certificate.go:90,144`, `cmd/posternproxy-agent/main.go:313-320`, `scripts/install-agent.sh:163-164,190-191`.

The installers run Caddy and the controller/agent as different users. Uploaded certificate directories are 0700 and files are 0600, owned by the controller or agent. Caddy receives file paths but cannot read them under these default Unix permissions. The controller's parent certificate directory is also explicitly 0700. Remote service sandbox writable paths need attention as well.

**Required:** establish narrowly scoped file access for Caddy using an appropriate shared group/ACL or certificate-loading mechanism. Verify the actual systemd users can load an uploaded cert. Do not solve this by exposing keys to all users.

### 11. P1 — Pending remote updates are lost across controller restart

`internal/agent/hub.go:40-42,92-100,111-128`, `internal/app/app.go:140-143`, `internal/services/caddy_sync.go:32-35`.

Offline updates are represented only by an in-memory `pendingSync` flag. Reconnection sync runs only if that flag exists, and controller startup syncs only the local server. Save a change while an agent is offline, restart the controller, then reconnect the agent: no full sync is requested by this path, so stale remote config can persist until another change or manual resync.

**Required:** reconcile desired and applied state on every connection or persist configuration revisions and pending work. Cover restart, queued-send failure, rejected apply, and reconnect scenarios.

## Other readiness gaps

- **Docker HTTP defaults:** `internal/config/config.go:46` defaults secure cookies on, while the supplied Compose/entrypoint exposes plain HTTP port 81 without overriding that flag. For the documented non-localhost `http://server-ip:81` access, this conflicts with browser cookie requirements. Exercise login and a mutation through the actual published image.
- **Upgrade policy:** there is only a consolidated initial migration, and the changelog explicitly records schema edits without an upgrade path. Before declaring stable, establish a baseline and test upgrade from every beta version you promise to support, or clearly require a documented migration/export process.
- **Audit completeness:** token create/revoke, backup restore, and settings edits lack corresponding audit calls in their current service/handler paths. Proxy-host update audit entries contain domains rather than the advertised before/after snapshots; config history is separate. The README's audit promise is broader than implementation.
- **Claims and implementation drift:** README still advertises access-list Basic Auth while the changelog says it is unimplemented. The existing forwarding plan calls its reachability monitor complete, but this tree has no `/api/health/forwards` route or monitor implementation. Security documentation also says general API rate limiting is unimplemented although the router now has it. Documentation and completion notes are not reliable substitutes for tests.
- **Integration coverage:** service tests explicitly use a failing `nullCaddy`, and config tests inspect generated structures rather than validate them with the shipped Caddy build. Neither detects successful DB writes paired with a broken running proxy.

## What is working well

The single Go controller with an embedded Vue UI keeps deployment understandable. Delegating proxy/TLS behavior to Caddy avoids reimplementing a sensitive data plane. The outbound agent model is a useful boundary. Services, models, handlers, and the Caddy translation layer are separated well enough to repair these issues without a wholesale rewrite. Parameterized value queries, bcrypt, AES-GCM helpers, HttpOnly refresh cookies, in-memory browser access tokens, and explicit admin route middleware provide useful foundations.

The central weakness is integration and lifecycle behavior: what happens after a save, permission change, restart, provisioning attempt, or restore. The current happy-path tests do not establish those guarantees.

## Gates for leaving beta

1. Resolve the authorization, secret-handling, synchronization, provisioning, certificate, and restore blockers above; retain reproductions as regression tests.
2. Add CI acceptance tests using the exact pinned Caddy/plugin build: local and remote HTTP/HTTPS, custom certificates, access lists, rate limiting, TCP/UDP forwarding, and TLS passthrough.
3. Prove fresh systemd and Docker installs, a supported beta upgrade, and recovery onto a clean controller. Include agent disconnect/reconnect and controller/Caddy restarts.
4. Add browser smoke tests for login/refresh, proxy create/edit/disable, certificate upload, provisioning status, and backup restore. Verify that rejected application is visibly distinguishable from success.
5. Test permissions, secret redaction, and account/token revocation through the real router; add race and dependency checks in a supported CI environment.
6. Ship a release candidate with a defined support/migration policy and observe it under representative multi-server use before removing beta.

No new features are needed to justify a stable release. Reliable operation of the advertised features is the remaining work.
