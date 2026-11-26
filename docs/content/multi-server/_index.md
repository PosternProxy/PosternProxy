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
                        Caddy            Caddy
```

The agent makes an **outbound** WebSocket connection to the controller and needs
no inbound agent port. Open the node's actual HTTP/HTTPS, stream, and SSH ports
as required. The bundle includes the agent and Caddy; no controller service is
needed on a remote node.

## Adding a server

1. Go to **Servers** in the sidebar (admin only)
2. Click **+ Add Server**
3. Fill in the connection details:

| Field | Description |
|---|---|
| **Name** | Display name for the server |
| **Hostname / IP** | Address for SSH provisioning |
| **SSH Port** | Default: 22 |
| **SSH User** | Root or passwordless sudo |
| **Authentication** | Password or private key |

4. Start provisioning and inspect its output. Read the
   [provisioning prerequisites]({{< relref "/multi-server/provisioning" >}})
   first, including artifact architecture and advertised controller URL.

<!-- TODO: Screenshot — Add Server wizard Step 1 (connection details) -->

## Provisioning

During provisioning, PosternProxy streams real-time output to the **terminal panel** in the wizard. The provisioner:

1. Verifies the remote architecture and matching local agent/Caddy artifacts
2. Uploads private credential and checksum files
3. Runs the native agent installer with dedicated users and hardened services
4. Preserves existing SSH settings and firewall rules
5. Starts `caddy` and `posternproxy-agent`
6. Waits for authentication and acknowledgement of the desired configuration

Manual enrollment skips SSH and displays a one-time private JSON credential for
`posternproxy-agent install`. See the provisioning guide below.

<!-- TODO: Screenshot — Provisioning terminal panel showing live log output -->

Once correctly configured, the agent authenticates to the controller WebSocket
endpoint. Check connection status and the separate desired/applied configuration
status; script completion alone does not establish readiness.

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

The controller saves desired configuration for the selected server and reconciles
it through that server's agent. Offline changes persist in the database and are
coalesced into current state on reconnect.

## Server selector

A **Server** picker in the sidebar selects the server for resource views. There
is no combined **All Servers** option in the current picker.

## Offline handling

If an agent goes offline:
- Its status changes to **Offline** in the Servers list
- Check the Servers page for its connection status
- Desired configuration and its revision persist in the controller database
- On reconnect, the controller sends a **full sync** to ensure the agent is up to date

## Re-provisioning

Click **Redeploy** on a server to re-run provisioning. This can be used for:
- Updating the agent binary after a PosternProxy upgrade
- Reapplying security hardening after OS changes
- Recovering from a broken agent installation

Redeploy rotates credentials, disconnects the old agent, and preserves firewall rules. Review the provisioning prerequisites before using it on an existing host.

## Removing a server

Click **Delete** on a server to remove it from PosternProxy. This does not uninstall the agent from the remote server; you must do that manually:

```bash
# For a manually installed posternproxy-agent service:
systemctl disable --now posternproxy-agent
```

Historical installations may use `caddyproxy-agent` instead. Remove binaries,
service files, and credentials only after confirming which paths your node uses.
Deleting a server does not reassign its hosts to Local: move or delete assigned
resources first, or foreign-key references can prevent deletion. The Local server
cannot be deleted.
