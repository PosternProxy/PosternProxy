# Architecture Decision Records

Hard-to-reverse decisions only (see Agent804 `framework/rules.md` DC-1). Detailed designs are specs (`../specs/`); dated results are evidence records (`../evidence/`). These records sit outside `docs/content/`, so the Hugo docs site does not publish them.

Status lifecycle: Proposed → Validated / Accepted / Rejected / Superseded by NNNN. An Accepted record is frozen except its Status, Topics and append-only evidence.

| ADR | Decision | Status |
|---|---|---|
| [0001](0001-caddy-data-plane.md) | Caddy is the data plane; PosternProxy generates config, not a proxy | Accepted |
| [0002](0002-single-binary-embedded-frontend.md) | Single static Go binary with an embedded Vue frontend | Accepted |
| [0003](0003-sqlite-forward-migrations.md) | SQLite with forward-only, frozen migrations | Accepted |
| [0004](0004-outbound-only-agent.md) | Remote servers managed by an outbound-only agent | Accepted |
| [0005](0005-dual-license.md) | AGPLv3 open edition plus a separate commercial license | Accepted |
| [0006](0006-pinned-caddy-build.md) | Reproducible pinned Caddy build, verified in the artifact | Accepted |
| [0007](0007-client-identity-model.md) | Visitor identity as a trusted client IP, separate from the socket peer | Accepted |
| [0008](0008-config-reconciliation.md) | Desired/applied config with per-server revisions and acknowledged apply | Accepted |
| [0009](0009-certificate-isolation.md) | Per-server certificate isolation with a key-safe access contract | Accepted |
| [0010](0010-portable-backup-format.md) | Portable, versioned, encrypted backup that restores to a clean install | Accepted |
| [0011](0011-caddy-l4-forwarding.md) | Port forwarding is Caddy-L4 only; OS iptables/socat dropped | Accepted |
| [0012](0012-tls-passthrough-bridge.md) | TLS passthrough via a layer-4 bridge on a reserved port | Accepted |
| [0013](0013-ntlm-passthrough.md) | NTLM passthrough via a project-maintained fork of `http_ntlm` | Accepted |

All records were accepted by the owner on 2026-11-27 (GR-7: only a person accepts an ADR). Each links an append-only evidence record under `../evidence/`; the shipped code and tagged releases are the initial evidence.
