---
title: Port Forwarding
weight: 80
---

# Port Forwarding

Port Forwarding routes raw network traffic from an incoming port on the server to an internal host and port, bypassing Caddy entirely. This is implemented via Linux `iptables` DNAT rules or `socat` processes.

## When to use port forwarding vs stream hosts

| Feature | Port Forwarding | Stream Hosts |
|---|---|---|
| Implementation | iptables / socat | Caddy L4 plugin |
| Protocol | TCP, UDP | TCP, UDP |
| HTTP awareness | None | None |
| SNI matching | No | Yes |
| Survives Caddy restart | iptables: yes; socat: no | No |
| IPv6 support | iptables: limited | Yes |
| Use case | Non-HTTP ports, e.g. game servers, SMTP | TLS-aware stream routing |

For most use cases, **stream hosts** are preferred because they are managed by Caddy and survive as part of the Caddy config. Use **port forwarding** when you need kernel-level forwarding or when the traffic type is incompatible with Caddy.

## Creating a port forward

1. Click **Port Forwarding** in the sidebar
2. Click **+ Add Port Forward**
3. Fill in:

| Field | Description |
|---|---|
| **Protocol** | `tcp`, `udp`, or `tcp+udp` |
| **Incoming Port** | Port on this server to listen on |
| **Forward Host** | Destination host or IP |
| **Forward Port** | Destination port |
| **Interface** | Network interface to bind (default: all) |
| **Method** | `iptables` or `socat` |

4. Click **Create**

<!-- TODO: Screenshot — Add Port Forward modal -->

## Methods

### iptables (recommended for TCP)

PosternProxy adds a `PREROUTING` DNAT rule and a `POSTROUTING` MASQUERADE rule:

```bash
iptables -t nat -A PREROUTING -p tcp --dport 2222 -j DNAT --to-destination 192.168.1.10:22
iptables -t nat -A POSTROUTING -d 192.168.1.10 -p tcp --dport 22 -j MASQUERADE
```

iptables rules are applied at the kernel level and do not require a running process. They persist across Caddy restarts. PosternProxy restores all enabled iptables rules on startup.

**Requirements:**
- `CAP_NET_ADMIN` capability (granted by the systemd service unit)
- IP forwarding enabled (`net.ipv4.ip_forward=1`, set by the install script)

### socat

For UDP forwarding or when iptables is unavailable, PosternProxy launches a `socat` process:

```bash
socat UDP-LISTEN:53,reuseaddr,fork UDP:192.168.1.53:53
```

socat processes are managed by PosternProxy. They terminate when PosternProxy restarts, and are re-started on PosternProxy startup for all enabled UDP forwards.

**Requirements:**
- `socat` installed (`apt install socat`)

## Startup restoration

On startup, PosternProxy re-applies all enabled port forwards:
- **iptables** rules are re-added (existing rules from the previous run are flushed first to avoid duplicates)
- **socat** processes are restarted

## Enable / disable

Port forwards can be toggled. Disabling removes the iptables rule or terminates the socat process immediately.

## Notes

- iptables rules do not survive a server reboot unless iptables-persistent is installed. The install script does not configure iptables-persistent by default — PosternProxy's own startup restoration handles this.
- UDP forwarding with socat is stateless. Each "connection" is a new UDP datagram.
- Forwarding ports below 1024 requires no special configuration — PosternProxy has `CAP_NET_BIND_SERVICE` in addition to `CAP_NET_ADMIN`.
