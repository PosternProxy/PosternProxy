# Install subcommand

Status: implementation complete; local checks passed November 23, 2026. Fresh VM
and published-artifact qualification remain in the existing beta-exit plans.

User request: build `posternproxy install` and update the docs, following the
agreed single-download installation direction. Existing provisioning and release
qualification plans remain active; this change does not qualify older upgrades.

## Acceptance criteria

- A supported Linux root user can install from one controller binary. Matching
  release assets are fetched over HTTPS and checked against SHA-256 checksums;
  Caddy version/plugins and binary architecture are checked before system changes.
- Local assets support source builds and offline artifact delivery. Development
  builds require local assets or an explicit release. Missing, corrupt, or wrong
  architecture assets fail before package/service/config changes.
- Installation creates dedicated users, protected configuration, valid initial
  Caddy configuration, and hardened systemd units. Existing config/secrets/data
  survive reruns. Firewall rules and SSH configuration survive; firewall setup is
  opt-in and requires an explicit SSH port.
- No Go compiler runs on the installation target. Service/startup errors fail
  visibly; success requires running services and reachable local APIs.
- Release packaging and source builds produce the asset names/checksums consumed
  by the installer. README, installation, source-build, and security docs describe
  the actual behavior. Existing shell controller entry point delegates to Go.

## Scope and decisions

Implement native Go orchestration in `internal/installer`, dispatched before app
startup. OS operations use argument-based subprocesses; no user input enters a
shell command. Preserve the outbound-only agent model and licensing. Downloaded
agent remains a separate executable for existing SSH provisioning. Downloads use
the controller's exact tag, never an implicit latest release. Offline assets are
trusted input and must include their own checksums. Checksums protect integrity;
they are not independent release signatures.

This adds a new installer path without changing remote SSH provisioning. Keep
the existing standalone agent installer until that separate path is migrated.

## Verification

- Independent artifact/CLI regression authoring initially caught dirty development
  versions incorrectly accepted as release tags. Fixed; focused tests pass.
- Independent read-only implementation review caught the macOS checksum command
  assumption and incompatible sandbox permissions on custom storage reruns.
  Added checksum fallback and rejection before mutations, with regression coverage.
- `go test ./...`, `go build ./...`, and `go vet ./...` passed locally. Tests use
  fake host commands and temporary files; HTTP fixtures use isolated loopback.
- Linux amd64 and arm64 `go build ./...` cross-builds passed.
- Shell syntax, controller wrapper argument/status propagation, standalone-agent
  missing-helper handling, and shared Caddy contract checks passed. PowerShell
  build-script parsing passed.
- `go run ./cmd/posternproxy install --help` and `hugo --minify` passed. Hugo
  reports existing deprecated-shortcode warnings.
- Added a regression connecting the new fail2ban pattern to the real JSON request
  logger, including IPv4/IPv6 failures and successful-login exclusion. Installed
  services use JSON; existing operator filter customizations remain preserved.

No release was published, no production host was modified, and no fresh systemd
VM install or release workflow execution was performed. Actual supported-host
installation, restart/recovery, and published artifact downloads remain release
qualification work. Existing custom-certificate filesystem access remains owned
by the certificate plan; this installer does not close that gate. Remote SSH
provisioning is unchanged. Pre-existing UI build output and other plans were
left intact. Public docs and changelog describe the implemented installer.
