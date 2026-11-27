# Evidence: ADR 0012 — TLS passthrough via a layer-4 bridge

- **ADR:** [../adr/0012-tls-passthrough-bridge.md](../adr/0012-tls-passthrough-bridge.md)

## 2026-11-27: backfill of a shipped decision

- **Revision:** `0ea37ed` (master); hardened in `cb4d231`.
- **Implementation:** `internal/caddy/config.go` moves `:443` to a layer-4 app when any host enables passthrough, SNI-routing passthrough hosts and bridging other HTTPS hosts over a reserved internal port with PROXY protocol; HTTP/3 is disabled on that path.
- **Tests:** `internal/caddy` config and stream-acceptance tests.
- **Command and result:** `go test ./...` — exit 0.
- **Confirmed:** passthrough and terminating hosts coexist on `:443`; the bridge port is reserved and HTTP/3 disabled; NTLM (ADR 0013) is rejected together with passthrough on one host.
- **Contradicted:** none observed.
- **Not tested or built:** no new live passthrough traffic test for this backfill.
- **Owner follow-up:** none.
