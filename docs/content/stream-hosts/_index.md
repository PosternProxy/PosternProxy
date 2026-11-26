---
title: Stream Hosts
weight: 40
---

# Stream Hosts

Stream hosts provide Layer 4 (TCP/UDP) port forwarding via Caddy's `caddy-l4` plugin. Unlike proxy hosts, stream hosts do not understand HTTP — they forward raw byte streams.

## When to use stream hosts

- Forwarding a database port (PostgreSQL, MySQL, Redis) through the server
- Forwarding non-HTTP connections to a single upstream
- Transparent forwarding where the application manages its own TLS

For HTTP/HTTPS traffic, use [Proxy Hosts]({{< relref "/proxy-hosts" >}}) instead. For encrypted traffic that must not be decrypted, use [TLS Passthrough]({{< relref "/proxy-hosts/tls-passthrough" >}}) on a proxy host.

## Creating a stream host

1. Click **Traffic Forwarding** in the sidebar (the former Stream Hosts URL redirects here)
2. Click **+ Add Rule**
3. Fill in the form:

| Field | Description |
|---|---|
| **Incoming Port** | The port on this server to listen on |
| **Forward Host** | Destination host or IP |
| **Forward Port** | Destination port |
| **Protocol** | `tcp`, `udp`, or `tcp+udp` |
| **SNI Match** | (Optional) Only match streams with this SNI hostname |

4. Click **Create**

<!-- TODO: Screenshot — Add Stream Host modal -->

## Protocol selection

| Protocol | Use case |
|---|---|
| `tcp` | Most services: databases, SSH, HTTP (without parse), SMTP |
| `udp` | DNS, game servers, VoIP, WireGuard |
| `tcp+udp` | When a service uses both protocols on the same port |

## SNI matching

If **SNI Match** is set, the TCP listener only matches TLS connections with that
hostname in the ClientHello. This does not merge independent stream listeners
with Caddy's HTTPS listener. Use proxy-host
[TLS Passthrough]({{< relref "/proxy-hosts/tls-passthrough" >}}) for managed SNI
routing alongside normal HTTPS on port 443.

Leave blank to accept all connections on the incoming port regardless of SNI.

## Port conflicts

Use an unused incoming port on the selected server. The current API validates
port ranges but does not reliably reject all listener collisions before saving.
Caddy can reject application after a conflicting rule has been saved. Correct or
disable the rule and check configuration application status.

Avoid ports already used by the installation, including:
- 80 and 443 are Caddy's default HTTP/HTTPS listeners
- 81 (or your configured management port) is the PosternProxy management UI
- 2019 is Caddy's local admin API

Open each incoming protocol/port in the host and cloud firewalls. See
[Traffic Forwarding]({{< relref "/port-forwarding" >}}) for commands.

## Caddy L4 plugin

Stream hosts require the `github.com/mholt/caddy-l4` plugin, which is included in the Caddy binary installed by `posternproxy install`.

## Enable / disable

Stream hosts can be enabled or disabled independently. Disabling removes the
managed listener after successful configuration application. Check the
configuration notice for rejection or offline agents.
