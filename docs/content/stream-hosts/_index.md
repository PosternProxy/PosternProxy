---
title: Stream Hosts
weight: 40
---

# Stream Hosts

Stream hosts provide Layer 4 (TCP/UDP) port forwarding via Caddy's `caddy-l4` plugin. Unlike proxy hosts, stream hosts do not understand HTTP — they forward raw byte streams.

## When to use stream hosts

- Forwarding a database port (PostgreSQL, MySQL, Redis) through the server
- TCP load balancing for non-HTTP services
- Transparent forwarding where the application manages its own TLS

For HTTP/HTTPS traffic, use [Proxy Hosts]({{< relref "/proxy-hosts" >}}) instead. For encrypted traffic that must not be decrypted, use [TLS Passthrough]({{< relref "/proxy-hosts/tls-passthrough" >}}) on a proxy host.

## Creating a stream host

1. Click **Stream Hosts** in the sidebar
2. Click **+ Add Stream Host**
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

If **SNI Match** is set, the stream host only accepts TLS connections where the client sends a matching SNI hostname in the ClientHello. This allows multiple stream hosts to share port 443 with regular proxy hosts by distinguishing TLS connections at the L4 level.

Leave blank to accept all connections on the incoming port regardless of SNI.

## Port conflicts

Each incoming port can only be assigned to one stream host. If you attempt to create a stream host on a port that is already in use (by another stream host or by Caddy's built-in listeners on 80/443), you will receive a validation error.

Ports 80, 443, and 81 are reserved:
- 80 and 443 are Caddy's default HTTP/HTTPS listeners
- 81 is the PosternProxy management UI

## Caddy L4 plugin

Stream hosts require the `github.com/mholt/caddy-l4` plugin, which is included in the Caddy binary built by `scripts/install.sh`.

## Enable / disable

Stream hosts can be enabled or disabled independently. Disabling a stream host removes the Caddy L4 listener immediately, freeing the port.
