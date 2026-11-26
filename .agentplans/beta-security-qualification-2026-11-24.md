# Current beta correctness and qualification evidence

Date: 2026-11-24 (America/Indianapolis). No release was tagged or published.
Source: working tree based on 45cd2a4b11816f47ea34ae66a0f195572bbd7a3d,
including previously existing changes. Version: v0.1.3-beta-dirty.

Linux amd64 archive SHA-256:
`1e10ca59d98fb374aa9036a114ce8477fa013db86719e74b0f0c5e87761b5fd9`.
Individual hashes and build pins: [artifact metadata](beta-next-fixes-artifacts.json).

## Correctness changes

- Controller-owned POSTERNPROXY_CADDY_TRUSTED_PROXIES JSON maps explicit server
  IDs to CIDRs. No management-policy inheritance, no default trusted networks,
  no /0, invalid/duplicate IDs rejected. Policies are static; no network range
  fetcher can widen trust on failure. Invalid input rejects startup or generation.
- Caddy strict right-to-left XFF, client_ip ACL matchers and http.vars.client_ip
  rate keys. Both default and location proxy routes send canonical X-Real-IP/XFF.
  Caddy retains socket-peer identity separately. Access-log viewer remains future.
- Header-read deadline 10s, header size 1MiB, keepalive idle 2m; verified Caddy
  default dial timeout 3s; explicit upstream TLS handshake 10s without weakening
  certificate verification. No global body/response deadline for website streams.
  Certificate multipart uploads capped at 5MiB, PEM inputs independently at 2MiB.
- Empty/unreadable persisted secret fails startup rather than regenerating;
  exclusive creation avoids replacing an existing file.
- Actual SSH qualification reproduced architecture probe failure on Linux amd64.
  Machine-readable architecture/root/version commands now capture stdout apart
  from diagnostics. Linux OS/architecture checked together. Full SSH rerun passed.
- Frontend compatible security updates; npm audit now reports zero findings.

## Executed checks

- Full Go tests, build and vet; focused new policy-isolation, secret-preservation,
  multipart-bound and architecture regressions.
- Frontend type-check, build, lint and all six Chromium smoke scenarios pass
  after dependency updates. npm audit: zero vulnerabilities.
- Hugo builds 61 pages; existing shortcode-deprecation warnings remain.
- Application source and all three final Linux binaries: govulncheck v1.9.0,
  no known vulnerabilities found. Pinned Caddy and dependency contract unchanged.
- Installer wrappers and shared Caddy shell contract pass. Archive has exactly
  controller, agent, Caddy and checksums; all hashes match, all binaries are
  static Linux amd64 ELF without an interpreter.
- Live pinned native Caddy on Windows and Ubuntu: HTTP, rate rejection, TCP;
  direct/forged/trusted/multi-hop/malformed chains and IPv6 visitor identity;
  visitor ACLs and rate separation, default/location forwarding; 2MiB upload,
  oversized-header 431, immediate SSE flush and WebSocket echo.
- Live Ubuntu custom TLS, rejected-config TLS preservation, encrypted clean
  recovery with another key, two outbound agents and controller/agent restarts.

## Real VM lifecycle

User-authorized Ubuntu 24.04 amd64, 458MiB RAM, no swap. Package prerequisites
already present. Original controller/data existed and was privately backed up
before modifications. Installer reruns used --assets-dir with the exact bundle.

- Controller upgraded, services active, original config hash unchanged.
- Fresh controller data/config temporarily substituted: printed credentials log
  in; sensitive files 0600; transient password env removed; initial password absent
  from service logs; rerun does not print/reset password; restart preserves login.
- Native agent installed using protected enrollment JSON against an isolated
  loopback controller. Authenticated/applied revision observed, not just online.
- Actual root SSH provisioning through loopback using a short-lived key: SFTP
  bundle staging, installer and controller online/applied readiness all pass.
- Uploaded custom certificate serves verified HTTPS through actual systemd Caddy.
  Runtime files 0640 with certificate GID; Caddy user/group reads them, unrelated
  nobody user cannot. Agent and Caddy restart each recover working HTTPS.
- Original controller data/config restored. Updated controller/Caddy active.
  Caddy admin bound to 127.0.0.1:2019. Temporary agent disabled, enrollment config
  and temporary SSH key removed. Backups/test artifacts remain private on the VM.

Reproduction harnesses: scripts/qualify-native-controller.py and
scripts/qualify-native-agent.py (manual and --provision-via-ssh). Agent harness
temporarily owns Caddy config; caller stops original controller and restarts it
in a shell EXIT trap. These are disposable-VM tools, not production maintenance.

## Still unqualified; no release-ready claim

- Fresh OS package installation, other distributions and arm64; firewall mutation.
- Offered Docker configurations (Docker absent on local host and VM).
- Actual published historical beta database fixtures and published asset download.
- Linux race CI execution, UDP and TLS-passthrough wider matrix, broader browser
  HTTPS/auth/upload/restore matrix and representative multi-server observation.
- Actual Cloudflare Tunnel/customer-specific origin authentication deployment.
  Documentation does not market validated Cloudflare-origin support.

The current correctness changes and tested Ubuntu paths are complete. The broader
beta-exit release qualification plan remains active; this report does not close
untested platforms or authorize publication.
