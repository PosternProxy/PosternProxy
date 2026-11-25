# Changelog

All notable changes to PosternProxy will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to use [Semantic Versioning](https://semver.org/).

## [Unreleased]

- Apply explicit per-server Caddy trusted-proxy policies to visitor ACLs, rate
  limits and canonical upstream client headers, including location routes.
- Bound proxy request headers, header-read time, idle keepalive and upstream TLS
  handshakes while preserving uploads, SSE and WebSockets.
- Refuse to replace an empty or unreadable persisted controller encryption secret.
- Bound certificate multipart uploads and update frontend dependencies to clear
  the advisories identified during beta qualification.
- Keep SSH diagnostics separate from architecture, privilege and version probe
  output so noninteractive shell warnings do not break native agent provisioning.

### Fixed

- Enabling a TLS-passthrough host no longer rejects the whole Caddy
  configuration. The generated fallthrough used a nonexistent `layer4.handlers.http`
  module; non-passthrough HTTPS is now bridged to an internal loopback HTTPS
  listener over PROXY protocol, preserving the real client IP. Qualified with a
  live pinned-Caddy test that loads the full production config.
- UDP stream hosts now forward to their upstream over UDP. Previously the
  generated Layer4 proxy dialed the upstream over TCP regardless of protocol,
  silently breaking UDP forwarding; qualified with a live pinned-Caddy UDP test.
- Combined `tcp+udp` stream hosts now forward each transport to its own network.
  Previously both listeners shared a single TCP upstream dial, so UDP traffic in
  combined mode was sent over TCP.
- Rejected settings edits no longer record attempted values in the audit log.
  Unknown or secret-bearing keys (e.g. `api_key`) now have their values masked,
  so a rejected bulk update cannot leak a secret into the audit record.
- First-time controller installation prints the initial admin login after readiness,
  using temporary private runtime configuration instead of service password logs.
  Reinstalling preserves existing passwords.

- Native installers normalize the embedded Caddy verification script to Unix
  line endings, fixing Bash failures in Linux binaries built on Windows.
- Custom certificate uploads validate PEM, key pairing, server usage, domains
  and validity before storage, and save expiry metadata. Private source material
  is encrypted; per-server synchronization excludes unrelated certificates.
- Certificate activation stages immutable runtime pairs for Caddy's dedicated
  read-only group and retains the last applied material if replacement fails.
- Reviewed the documentation against the current UI, configuration, routes and
  services. Corrected agent packaging/provisioning guidance, log and password
  recovery commands, API CSRF examples, settings behavior, history retention,
  forwarding and certificate/audit claims; documented remaining beta limitations.
- Installation docs now show bundle extraction, the extracted `posternproxy`
  filename, and `--assets-dir .`, with a separate path for individual release binaries.
- Remote SSH installation uses verified prebuilt Linux artifacts and the native
  agent installer, passes credentials in private files, preserves firewall rules,
  and waits for authenticated acknowledgement of the current configuration.
- Settings display actual runtime values and reject ineffective changes. Audit
  access is admin-only, with additional redacted management events.
- Stream listener conflicts are rejected before saving. Dead hosts persist
  certificate and access-list selections; redirect/dead routes enforce IP lists.
- Release version stamping now survives process initialization; both binaries expose a version command.
- Caddy is updated to the patched 2.11.9 build and uses the pinned Go 1.26.11 toolchain across build paths.
- Controller containers now include the agent executable for SSH provisioning.
- New controller installations use a fail2ban filter matching the actual failed
  login request logs, with structured JSON logging enabled in the service.

### Added

- Admin sign-in and sidebar use the PosternProxy logo. Default proxy error and
  parked-domain pages include matching light/dark logos and automatically follow
  the visitor's system theme without external image requests or JavaScript.

- Parked domains support an explicit HTTP-only mode without HTTPS routes or
  certificate distribution. Existing parked domains keep their HTTPS setting.
- Proxy, redirect and parked host lists share expandable TLS checks, alias results,
  expiry/check times and guidance to Caddy's actual ACME issuance logs. Resource
  IDs cannot collide across host types, and remote checks are labeled unavailable.

- `posternproxy-agent install --config-file` shares native service setup with the
  controller installer. Manual server enrollment and credential rotation display
  credentials once and disconnect replaced identities.
- Passphrase-encrypted version 2 recovery archives include custom certificate
  bytes and credential-free server descriptions, with explicit remote mapping or
  unenrolled creation. Restore separates database commit from configuration apply.
- Live custom TLS and cross-key clean recovery acceptance tests.
- `posternproxy install` installs the controller, release-matched Caddy and agent,
  hardened systemd services, fail2ban, and security updates. Assets are verified
  before system changes; local assets support source builds and offline delivery.
- Tagged-release packaging creates draft Linux amd64/arm64 releases with SHA-256
  manifests. Source build scripts also write manifests for local installation.

### Changed

- Go builds require Go 1.26+, with a patched toolchain preference; SSH cryptography dependencies and the WebSocket masking implementation are updated to fix reachable vulnerability findings.
- Controller installation no longer requires transferring shell scripts or a
  Caddy build helper. The shell entry point delegates to the Go subcommand.
  Existing configuration is preserved; UFW setup is opt-in, requires the actual
  SSH port, and preserves existing rules. SSH configuration is left intact.

## [0.1.3-beta] 2026-11-15

This release remains in beta. Upgrade the controller and agents together.
In-place upgrades from published earlier beta databases are still unqualified.
See the [beta upgrade and recovery guide](https://docs.posternproxy.com/administration/upgrading/).

### Added

- Release qualification foundations: frozen schema baseline tests, a disposable
  browser smoke harness, and live Caddy acceptance/CI checks.
- Persistent desired/applied configuration revisions, correlated agent results,
  automatic reconciliation on reconnect/restart, and a configuration status
  notice with admin retry. Restore reports application state separately from
  saved data. Live acceptance covers two outbound agents and controller restart.

### Fixed

- Access lists no longer block configuration generation on SQLite's single
  connection, and list changes trigger configuration synchronization.
- Host create/edit/disable/delete now report rejected configuration application
  while retaining saved desired state for retry; host moves synchronize both
  the old and new server. Host audit entries include safe before/after fields.
- Configuration history snapshots are encrypted at rest, including legacy
  snapshots protected during startup, and history listings omit their contents.
- Management token scopes come from validated credentials and cannot be elevated
  through headers or token minting. Account/password changes revoke existing
  credentials; sessions rotate refresh tokens and revoke on logout.
- Forwarding headers are accepted only from configured trusted proxy CIDRs.
  Agent authentication uses Authorization, and structured request logs omit
  credential-bearing URLs and headers. Controller and agents must be upgraded
  together; older query-token agents are rejected.

### Changed

- Traffic Forwarding now uses Caddy L4 stream hosts. The legacy iptables,
  socat, and `/api/port-forwards` paths were removed.
- Build scripts, Docker images, installers, and SSH provisioning share pinned
  Caddy, L4, rate-limit, and xcaddy versions. Installers reject incompatible
  existing Caddy builds, including builds missing required plugins.
- Documented the beta schema baseline and recovery limits; corrected unsupported
  Basic Auth, backup portability, and security rate-limit claims.
- Updated the docs site for scoped API tokens, session revocation, trusted
  proxies, coordinated agent upgrades, pinned Caddy provisioning, and rollback.

## [0.1.2-beta] 2026-06-20

### Added

- A firewall reminder in the Traffic Forwarding UI and docs: each forwarded port
  must be opened in the host firewall (ufw) and any upstream/cloud firewall, which
  PosternProxy does not manage.
- HTTP/2 is now a per-server setting (the Local controller and each remote managed
  server can differ), toggled on the Servers page. It replaces the old per-host
  HTTP/2 toggle, which never did anything (HTTP/2 is a property of the shared HTTPS
  listener, not a route). Turning it off restricts that server's HTTPS listener to
  HTTP/1.1.
- Live TLS certificate status for HTTPS-serving proxy hosts. A background probe
  completes a TLS handshake against the local Caddy listener (SNI per host) every
  60 seconds and reports whether Caddy is actually serving a valid certificate —
  shown on the Proxy Hosts list as TLS OK / pending / failed. This surfaces ACME
  provisioning that hasn't completed (e.g. DNS not yet pointed) instead of leaving
  HTTPS silently broken. Exposed at `GET /api/health/tls`.

### Fixed

- The audit log failed to load (HTTP 500 → "Failed to load audit log" in the UI):
  the `details` column was scanned directly into a `json.RawMessage`, which the
  SQLite driver returns as a string and cannot scan into that type. It now scans
  into a string first (matching every other JSON column), with a regression test.
- Critical: the branded host-down error pages used an `error` matcher
  (`http.matchers.error`) that does not exist in Caddy, so Caddy rejected the
  entire `/load` with HTTP 400. Because default 502/503 pages are generated for
  every host, this made **all** config syncs fail — Caddy stayed frozen on a stale,
  host-less config and every request fell through to the HTTP→HTTPS catch-all (so
  even HTTP-only hosts appeared to "always redirect to HTTPS"). Error routes now
  match the status code with Caddy's standard `expression` matcher
  (`{http.error.status_code} == NNN`).
- Proxy hosts now get polished, branded default host-down pages for upstream
  502/503 failures, including helpful operator checks and the running
  PosternProxy version. Custom 502/503 pages apply on plain HTTP routes as well
  as HTTPS routes.
- Dead (parked) hosts now serve a branded PosternProxy page when no custom body
  is supplied, with copy tailored to the status code (200/404/410/503, plus a
  generic fallback). Previously they returned a bare unstyled page, and a 200
  parked domain returned an empty body. A custom body still overrides the default.
- The Proxy Hosts status toggle now says Enabled/Disabled instead of
  Online/Offline, since it reflects whether the route is active, not whether the
  upstream is reachable.
- Backup **export and restore** were completely broken: both referenced an
  `access_list_auth` table that no longer exists in the schema, so every export
  failed with "no such table" and restore failed on its clear step. Removed the
  dead table from the backup table list, column allowlist, and restore clear
  order, and added a round-trip regression test.
- Redirection hosts now honor **Force HTTPS** and serve over plain HTTP. Like
  proxy hosts, they were force-redirected to HTTPS and mounted only on the :443
  server, so a redirect on an internal/un-pointed domain was dead. They now serve
  the redirect directly on :80 when Force HTTPS is off, and on :443 when forced or
  a certificate is assigned.
- Dead (404) hosts are now served over plain HTTP as well as HTTPS, so a parked
  domain responds without requiring a certificate.
- The proxy host **Advanced Configuration** field is now actually applied. The
  JSON object is shallow-merged into the host's generated Caddy route (overriding
  generated fields); previously it was stored and silently ignored. It is
  validated as a JSON object on save, and invalid JSON can never break the config
  push.
- Proxy hosts are now reachable out of the box. Previously every host was
  force-redirected from HTTP to HTTPS regardless of its **Force HTTPS** setting,
  and HTTPS only worked with a publicly-provisioned Let's Encrypt certificate — so
  hosts on internal domains, IPs, or domains without DNS yet were completely dead.
  The per-host `ssl_forced` setting was silently ignored when generating Caddy
  config; it is now honored:
  - **Force HTTPS off (default):** the host is served over plain HTTP on port 80
    immediately, with no certificate required and no ACME attempt.
  - **Force HTTPS on, or a certificate assigned:** the host is served over HTTPS
    (Let's Encrypt when forced without a cert), with HTTP redirected to HTTPS.
  - A host with a certificate but Force HTTPS off is served over both HTTP and HTTPS.
- Caddy's automatic HTTP→HTTPS redirects are now disabled so they cannot shadow
  the direct HTTP routes for hosts that opted out of Force HTTPS.
- Dashboard upstream health no longer reports idle or never-contacted upstreams as
  "Healthy". Caddy's `fails == 0` only means "no failures observed", which is also
  true of a dead backend that has had no traffic; upstreams are now shown as
  Healthy, Unhealthy, or No data (unknown) accordingly.
- The local server's status is now derived from a live Caddy admin-API probe
  instead of a seeded `online` value that was never reconciled, so the UI no
  longer shows green when Caddy is down or unconfigured.
- Adding or enabling a host when Caddy was started without a base config (e.g. the
  boot-time sync lost the race with Caddy) now self-heals: a failed incremental
  apply falls back to a full config resync instead of being silently dropped.
- The initial Caddy sync retries with backoff until Caddy's admin API is ready,
  rather than running once and leaving Caddy unconfigured if it wasn't up yet.

### Changed

- The **Docker** deployment option is hidden in the Add Remote Server wizard;
  remote servers are provisioned via systemd only. Docker agent provisioning
  isn't ready yet and would fail. The `deployment_type` field still defaults to
  `systemd`, and the backend keeps the `docker` code path for when it's ready.
- The management UI favicon now uses the PosternProxy mark instead of the
  default Vue icon.
- Caddy sync failures in the host services are now logged as structured warnings
  instead of `fmt.Printf` to stdout.

### Removed

- The **iptables** and **socat** port-forwarding methods, end-to-end (controller,
  agent, `internal/portforward`, the `port_forwards` table, API routes, and docs).
  Traffic Forwarding is now Caddy Layer 4 only — it covers every real-world
  TCP/UDP forwarding case and removes the OS-level privilege/firewall complexity.
- The per-proxy-host **WebSocket Support** and **HTTP/2** toggles, which were
  no-ops. Removed end-to-end: UI controls, model fields, and the
  `proxy_hosts.http2_support` / `proxy_hosts.websocket_support` columns (folded
  into the initial schema migration — pre-beta, so no upgrade path is provided).
- Phantom **Block Exploits** and **Cache Assets** fields from the proxy-host docs;
  they were never implemented (no code, schema, or UI).
- Documentation for unbuilt features that the app does not actually provide:
  access-list HTTP Basic Auth (IP rules only), certificate Let's Encrypt DNS-01
  challenge, and per-certificate renew/download. These are noted as potential
  future work rather than current capabilities.

## [0.1.1-beta] 2026-06-11

### Added

- A reproducible Hugo documentation site with the pinned Hugo Book theme and CI build validation.
- A dark default theme for the documentation site.
- PosternProxy branding in the documentation sidebar.
- Dark syntax highlighting and higher-contrast code blocks in the documentation site.

### Fixed

- Updated the frontend lockfile to use `shell-quote` 1.8.4, resolving a critical development dependency advisory.
- PowerShell cross-builds for Windows now produce `.exe` controller and agent binaries.
- Empty proxy-host collections now render correctly instead of crashing the embedded frontend, and the dashboard Add Proxy Host button opens the create form directly.
- Failed session refreshes no longer trigger a recursive request loop that can leave protected views blank.

### Changed
- Rebased Git Repository squashing commit history and started from base point 0.1.1-beta

## [0.1.0-beta.1] — 2026-06-09

### Added

- AGPLv3 licensing and documentation for separately available commercial terms.
- A project security policy and private vulnerability reporting process.
- A foundational GitHub Actions workflow for Go and frontend validation.
- This changelog and the staged [Beta release readiness plan](plans/beta-release-readiness.md).
- Root Makefile with `build`, `test`, `lint`, `clean`, `version`, and dev targets.
- Version embedding via `-ldflags` in both controller and agent binaries (all build paths: scripts, Makefile, Docker).
- `runtime/debug` build info fallback when ldflags are not used (e.g. `go run`).
- Unauthenticated `GET /api/version` endpoint for operational visibility.
- Version logged at controller and agent startup.

### Changed

- README overhauled: accurate Go 1.25+ prerequisite, Makefile usage, versioning and CI sections, Beta status note.
- Build scripts now inject version into the agent binary (previously only the controller received it).
- Build script comments updated to reflect Go 1.25+ requirement.
- Dockerfiles accept a `VERSION` build arg for version injection.
- `Strict-Transport-Security: max-age=31536000; includeSubDomains` header added to all responses via `SecureHeaders` middleware.
- Authenticated management API routes now rate-limited at 120 requests/min per IP (previously unprotected).
- All documentation aligned with actual implementation: rate limit values, HSTS header policy, Go version, Docker capabilities.
- Go module path changed from `github.com/reamagnewire/posternproxy` to `github.com/posternproxy/posternproxy`.
- Copyright holder updated to "Cyclone Software Solutions, LLC" across LICENSE, LICENSING.md.
- Docker agent compose file documents capability requirements for port forwarding.

### Added (Tests)

- JWT token generation, validation, expiry, and signing-method tests (`internal/auth/jwt_test.go`).
- Auth middleware tests: valid JWT, missing token, invalid token, fallback resolver, error paths (`internal/auth/middleware_test.go`).
- Password hashing and verification tests (`internal/auth/password_test.go`).
- RequireAdmin and RequireScope middleware tests covering admin, non-admin, no-claims, scope presence/absence.
- Agent protocol message serialization roundtrip tests (`internal/agent/protocol_test.go`).
- Agent hub tests: online/offline state, Send behavior, pending sync, channel-full handling (`internal/agent/hub_test.go`).
- Caddy client tests using httptest: Load, GetConfig, SetPath, PatchPath, DeletePath, AppendToArray, GetUpstreams, Ping (`internal/caddy/client_test.go`).

### Fixed

- Default admin password is now randomly generated on first boot (previously `changeme123` was hardcoded in Docker compose and install script).
- CLA copyright holder corrected to Cyclone Software Solutions, LLC.
- Commercial licensing docs updated with real contact email.
- Install script post-install message now correctly directs users to service logs for the generated password.

[Unreleased]: https://github.com/PosternProxy/PosternProxy/compare/v0.1.3-beta...master
[0.1.3-beta]: https://github.com/PosternProxy/PosternProxy/releases/tag/v0.1.3-beta
[0.1.0-beta.1]: https://github.com/posternproxy/posternproxy/releases/tag/v0.1.0-beta.1
