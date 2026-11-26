# Current beta client identity and qualification

Status: implementation/local/Ubuntu qualification complete, 2026-11-24;
broader target gates remain in the parent release qualification plan.

Scope: close Caddy visitor identity correctness, conservative HTTP safeguards,
and reproducible qualification coverage. Preserve the current working tree.
Next-release integrations remain out of scope.

Acceptance:
- Explicit controller-owned JSON environment policies keyed by managed server ID;
  management trust stays separate; no implicit private-network or global trust.
- Caddy strict right-to-left XFF parsing, client_ip ACLs and rate keys, canonical
  upstream X-Real-IP/XFF for default and location routes. Socket peer stays available
  to Caddy logs. No remote policy inheritance.
- Invalid policies fail startup/config generation. Static ranges only: failed
  external refresh cannot expand trust. Operators validate/redeploy range updates.
- Pinned live Caddy tests cover direct/spoofed/trusted/multi-hop/malformed/IPv6,
  ACL and rate isolation, upload, SSE, WebSocket and header rejection.
- Header deadline 10s, idle keepalive 2m, headers 1MiB; no blanket body-size or
  whole-response deadline that breaks uploads/streams. Verify upstream defaults.
- Reconcile deployment/security docs and run Go/frontend/browser/live checks,
  vulnerability scans and rebuilt archive verification.
- Record Linux systemd/container/published-release/Cloudflare qualification only
  when actually exercised. Windows tests do not close those external gates.

Evidence and outstanding target checks:
[dated qualification report](../beta-security-qualification-2026-11-24.md).
