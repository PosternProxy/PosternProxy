---
title: PosternProxy Documentation
type: docs
---

# PosternProxy

PosternProxy is a beta reverse proxy manager built on [Caddy](https://caddyserver.com/).
The controller is a Go binary with an embedded Vue frontend. Caddy handles
traffic, and remote nodes run a separate outbound agent. Full build bundles
include the controller, agent, custom Caddy and checksums.

Start with [Installation]({{< relref "/getting-started/installation" >}}) and review
[Beta limitations]({{< relref "/administration/beta-limitations" >}}) before deployment.

## Why PosternProxy?

| Capability | Current scope |
|---|---|
| HTTP proxying | Domains, upstreams, load-balancing policies, active/passive checks |
| HTTPS | Caddy-managed certificates and custom uploads; certificate access/validation limits remain |
| Traffic Forwarding | Caddy L4 TCP/UDP forwarding and SNI passthrough |
| Access control | Proxy-host IP rules and per-host rate limits |
| Response customization | Headers and Caddy-generated 502/503 error pages |
| Multi-server | Outbound agents and persisted desired/applied configuration state |
| Config history | Proxy-host rollback; latest 100 entries listed |
| Automation | Scoped API tokens with role and CSRF enforcement |
| Nginx import | File preview and confirmed import; manual review required |
| Configuration export | Partial JSON export; not portable disaster recovery |
| Audit | Recorded management events; coverage is incomplete |

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
