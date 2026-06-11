---
title: PosternProxy Documentation
type: docs
---

# PosternProxy

PosternProxy is an enterprise-grade reverse proxy manager built on [Caddy](https://caddyserver.com/). It provides a web UI for managing proxy hosts, SSL certificates, access control, port forwarding, and multi-server deployments — all backed by a single Go binary with an embedded Vue 3 frontend.

## Why PosternProxy?

| Feature | Nginx Proxy Manager | PosternProxy |
|---|---|---|
| Proxy engine | Nginx | Caddy (automatic HTTPS by default) |
| Deployment | Docker required | Single binary, bare-metal Debian/Ubuntu |
| Multi-server | ✗ | ✓ WebSocket agent architecture |
| Load balancing | ✗ | ✓ 6 policies, per-host |
| Health checks | ✗ | ✓ Active + passive, live dashboard |
| Rate limiting | ✗ | ✓ Per-host, per-IP sliding window |
| Header injection | ✗ | ✓ Request + response, with presets |
| Custom error pages | ✗ | ✓ Per-host 404/502/503 HTML |
| TLS passthrough | ✗ | ✓ SNI-matched L4 tunnelling |
| Config history | ✗ | ✓ 100-entry rollback per resource |
| API tokens | ✗ | ✓ Bearer token automation |
| Nginx import | ✗ | ✓ Parse + preview existing configs |
| Backup / restore | ✗ | ✓ JSON export with authenticated download |
| Audit log | Partial | ✓ Full mutation log with IP + user |

## Quick navigation

{{< columns >}}
**Getting started**
- [Installation]({{< relref "/getting-started/installation" >}})
- [Quick start]({{< relref "/getting-started/quick-start" >}})

<--->

**Core features**
- [Proxy hosts]({{< relref "/proxy-hosts" >}})
- [Certificates]({{< relref "/certificates" >}})
- [Access lists]({{< relref "/access-lists" >}})

<--->

**Advanced**
- [Load balancing]({{< relref "/proxy-hosts/load-balancing" >}})
- [Multi-server]({{< relref "/multi-server" >}})
- [API tokens]({{< relref "/access-tokens" >}})
{{< /columns >}}
