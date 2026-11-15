---
title: Traffic Forwarding
weight: 80
---

# Traffic Forwarding

Traffic Forwarding routes raw TCP/UDP traffic from an incoming port on the server
to an internal host and port — for anything that isn't HTTP: databases, SSH, game
servers, MQTT brokers, and so on. It is powered entirely by **Caddy's Layer 4
(L4) proxy**; there is no longer any iptables or socat path.

> Traffic Forwarding and [Stream Hosts]({{< relref "/stream-hosts" >}}) are the
> same underlying capability (Caddy L4). Traffic Forwarding is the simpler,
> port-oriented view of it.

## Creating a forward

1. Click **Traffic Forwarding** in the sidebar
2. Click **+ Add Rule**
3. Fill in:

| Field | Description |
|---|---|
| **Incoming Port** | Port on this server to listen on (1–65535) |
| **Protocol** | `tcp`, `udp`, or `tcp+udp` |
| **Forward Host** | Destination host or IP |
| **Forward Port** | Destination port |
| **SNI Match** | *(optional, TLS only)* only route TLS connections for this hostname |

4. Click **Create**

## Open the firewall — required

A forward will silently receive **no traffic** until the incoming port is allowed
through the firewall. PosternProxy does **not** manage the firewall for you.

On the server (host firewall, e.g. ufw):

```bash
sudo ufw allow 25565/tcp
# tcp+udp: add the UDP rule too
sudo ufw allow 25565/udp
```

{{< hint warning >}}
**Cloud / upstream firewalls**
If your server sits behind a provider firewall — DigitalOcean Cloud Firewall, AWS
security groups, GCP firewall rules, etc. — you must open the port **there as
well**. PosternProxy cannot see or manage cloud firewalls. The same applies to any
firewall in front of the destination host.
{{< /hint >}}

## Notes

- Forwarding survives Caddy config reloads (existing connections are preserved on
  graceful reload). A full Caddy restart drops in-flight connections, as with any
  proxy.
- SNI matching only applies to TLS traffic. Leave it blank to forward everything
  arriving on the port.
- Forwarding privileged ports (below 1024) works without extra configuration —
  Caddy has `CAP_NET_BIND_SERVICE`.
