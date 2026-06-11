# Changelog

All notable changes to PosternProxy will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project intends to use [Semantic Versioning](https://semver.org/).

## [0.1.1-beta] 2026-06-11

### Added

- A reproducible Hugo documentation site with the pinned Hugo Book theme and CI build validation.
- A dark default theme for the documentation site.
- PosternProxy branding in the documentation sidebar.
- Dark syntax highlighting and higher-contrast code blocks in the documentation site.

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

[Unreleased]: https://github.com/posternproxy/posternproxy/commits/master
[0.1.0-beta.1]: https://github.com/posternproxy/posternproxy/releases/tag/v0.1.0-beta.1
