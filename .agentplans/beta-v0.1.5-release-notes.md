# PosternProxy v0.1.5-beta

This beta adds native Windows services, Linux arm64 bundles and multiarch Docker
images while retaining native Linux amd64 support. Every native bundle includes
the matching controller, remote agent and pinned Caddy build.

## Changes

- Windows controller and agent service installation, separate service identities,
  protected machine-encrypted configuration, reinstall and data-preserving removal.
- Docker process supervision and persistent controller/Caddy data; amd64 and arm64
  images and Compose qualification.
- Stronger management credential revocation and scope enforcement. Agent token
  revocation and owner password, role or enabled-state changes invalidate ongoing
  connections as well as subsequent authentication.
- Durable desired/applied configuration status, visible apply failures and retry,
  offline-agent reconciliation and restart recovery.
- Passphrase-encrypted portable recovery with custom certificates and explicit
  remote-server mapping. Exports use one consistent database snapshot.
- Per-server certificate distribution, protected certificate files and custom TLS
  recovery; HTTP-only parked domains and TLS diagnostics.
- Trusted-proxy/client identity controls, bounded request handling, preserved
  uploads, SSE and WebSockets, and reserved management-port validation.
- Updated installation documentation and expanded browser/platform acceptance.

## Installation

Download the archive for your target: `posternproxy-linux-amd64.tar.gz`,
`posternproxy-linux-arm64.tar.gz` or `posternproxy-windows-amd64.zip`. Verify its
SHA-256 against the release-level `checksums.txt`, extract all files and verify
the enclosed binary checksums. The executable inside Linux archives is
`posternproxy`; on Windows it is `posternproxy.exe`.

Linux: `sudo ./posternproxy install --assets-dir .`

Windows: from elevated PowerShell, `./posternproxy.exe install --assets-dir .`.
Follow the Windows guide for controller/agent service configuration and removal.

Docker controller: `ghcr.io/posternproxy/posternproxy:v0.1.5-beta`.
Docker agent: `ghcr.io/posternproxy/posternproxy-agent:v0.1.5-beta`.
Both images support linux/amd64 and linux/arm64 after the publication workflow
finishes. The `beta` tag follows the beta channel; this release does not move
`latest`. Use the version tag for reproducible deployment.

## Beta scope and recovery

Use matching controller and agent versions. Management ports 80, 443, 2019 and
8443 are reserved. There is no validated historical beta database upgrade
promise. Back up the whole installation, including encryption secrets and Caddy
storage, before an experimental upgrade; version 1 JSON export is not a complete
recovery backup. Use encrypted portable backups for configuration/custom
certificate recovery. Keep the passphrase separately.

This remains a beta. Extended multi-server workload, outage and certificate
renewal soak continues during beta before production qualification. Cloudflare
origin integration, Tailscale, managed Gitea/Hugo deployment, log-based ACL actions,
WAF and SSO/MFA are not new features in this release. See the published beta
limitations and installation guides.
