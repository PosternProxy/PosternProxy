---
title: Multi-Server
weight: 150
bookCollapseSection: true
---

# Multi-Server

PosternProxy supports managing multiple remote proxy servers from a single central UI. Each remote server runs a lightweight `posternproxy-agent` binary that connects back to the controller.

## Architecture

```
Browser → PosternProxy Controller (:81) → REST API → SQLite
                                               ↓
                          ┌────────────────────────────────────┐
                          │         WebSocket Hub              │
                          ├──────────────┬─────────────────────┤
                          ↓              ↓
                    Agent (Server A)   Agent (Server B)
                          ↓              ↓
                     Caddy + iptables  Caddy + iptables
```

The agent makes an **outbound** WebSocket connection to the controller. No inbound ports are needed on the remote server beyond 80 and 443 (and SSH for provisioning).

## Adding a server

1. Go to **Servers** in the sidebar (admin only)
2. Click **+ Add Server**
3. Fill in the connection details:

| Field | Description |
|---|---|
| **Name** | Display name for the server |
| **Hostname / IP** | Address for SSH provisioning |
| **SSH Port** | Default: 22 |
| **SSH User** | Username with root or sudo access |
| **Authentication** | Password or private key |

4. Click **Provision** — PosternProxy SSH's into the server and installs everything automatically

<!-- TODO: Screenshot — Add Server wizard Step 1 (connection details) -->

## Provisioning

During provisioning, PosternProxy streams real-time output to the **terminal panel** in the wizard. The provisioner:

1. Installs Go and builds Caddy with the L4 and rate-limit plugins
2. Copies the `posternproxy-agent` binary to the server
3. Creates dedicated system users (`caddy`, `posternproxy-agent`) with no login shell
4. Writes the agent config (`/etc/posternproxy-agent/config.env`) with the controller URL and a unique pre-shared token
5. Installs systemd services with full hardening
6. Configures UFW, fail2ban, unattended-upgrades
7. Applies sysctl hardening
8. Starts the agent service

<!-- TODO: Screenshot — Provisioning terminal panel showing live log output -->

Once the agent starts, it connects to the controller WebSocket endpoint. The server status changes from **Provisioning** to **Online**.

See [Provisioning]({{< relref "/multi-server/provisioning" >}}) for full details.

## Server list

The Servers page shows all registered servers:

| Column | Description |
|---|---|
| **Name** | Display name |
| **Hostname** | IP or FQDN |
| **Status** | Online / Offline / Provisioning / Error |
| **Caddy version** | Reported by the agent |
| **Last seen** | Time of last heartbeat |
| **Active routes** | Number of proxy hosts, stream hosts, etc. |

<!-- TODO: Screenshot — Server list with two servers (one online, one offline) -->

## Assigning resources to servers

When creating or editing a proxy host, redirection host, stream host, or dead host, a **Server** dropdown lets you assign the resource to a specific server (or leave it on the local server).

The controller pushes the Caddy config update to the correct agent automatically. If the agent is offline, the command is queued and sent when the agent reconnects.

## Server selector

A **Server** dropdown in the top header lets you filter all resource views to show only the resources on a specific server, or "All Servers" for a combined view.

## Offline handling

If an agent goes offline:
- Its status changes to **Offline** in the Servers list
- The Dashboard shows the server as offline
- Pending config changes are queued in memory
- On reconnect, the controller sends a **full sync** to ensure the agent is up to date

## Re-provisioning

Click **Re-Provision** on a server to re-run the provisioning script. This is useful for:
- Updating the agent binary after a PosternProxy upgrade
- Reapplying security hardening after OS changes
- Recovering from a broken agent installation

## Removing a server

Click **Delete** on a server to remove it from PosternProxy. This does not uninstall the agent from the remote server; you must do that manually:

```bash
systemctl stop posternproxy-agent
systemctl disable posternproxy-agent
rm /usr/local/bin/posternproxy-agent
```

All proxy hosts assigned to the deleted server remain in the database but will be reassigned to the local server.
