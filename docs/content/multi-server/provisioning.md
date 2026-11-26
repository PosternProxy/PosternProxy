---
title: Provisioning
weight: 1
---

# Server Provisioning

Remote nodes run Caddy and `posternproxy-agent`. The agent connects outbound to
the controller; it requires no inbound management listener. Configure the
controller's advertised base URL using `POSTERNPROXY_CONTROLLER_URL` and restart
it before enrolling nodes. Use HTTPS for deployments outside a trusted network.

## Packaging

Extract `posternproxy-linux-amd64.tar.gz` (or the arm64 archive) on each matching
node. It contains `posternproxy`, `posternproxy-agent`, `caddy` and `checksums.txt`.
The agent is a separate executable included in the same archive, not embedded in
the controller. Remote nodes do not need to run the controller executable.

## Automatic SSH installation

Choose **Servers -> Add Server -> Automatic SSH install**. Requirements:

- Debian 11/12 or Ubuntu 22.04/24.04 with systemd, on Linux amd64 or arm64.
- Root SSH access or an account with passwordless sudo, using the actual SSH port.
- Outbound APT/HTTPS access and a reachable controller URL.
- Matching prebuilt agent and pinned Caddy artifacts available on the controller.

The controller installer supplies `/usr/local/bin/posternproxy-agent` and
`/usr/bin/caddy`. Override these using `POSTERNPROXY_AGENT_BIN` and
`POSTERNPROXY_CADDY_BIN`. `POSTERNPROXY_AGENT_BIN_PATH` remains a fallback alias.
For another architecture, place same-version files named
`posternproxy-agent-linux-arm64` and `caddy-linux-arm64` beside the configured
artifacts. Provisioning verifies the target architecture and Caddy module pins
before installing anything on the node. It does not install Go or compile Caddy
on the remote host.

Credentials and checksums travel in private staging files. The native agent
installer verifies controller identity, installs `caddy.service` and
`posternproxy-agent.service`, creates bootstrap Caddy configuration, and uses the
shared hardened installer. Existing firewall rules and SSH settings are preserved;
SSH provisioning does not opt into UFW changes.

The wizard succeeds only after the agent authenticates and acknowledges the
current desired configuration as applied. If that does not happen within two
minutes after service installation, the run reports an error. Inspect both logs
and configuration status before retrying. **Redeploy** rotates credentials,
disconnects the old agent, and runs installation again.

## Manual enrollment

1. Choose **Servers -> Add Server -> Manual enrollment**, enter a name and
   hostname, and create the record. No SSH installation starts.
2. Save the displayed one-time JSON credential as `agent.json` on the remote node.
3. From that node's extracted matching Linux archive, run:

```bash
chmod 600 agent.json
chmod +x posternproxy-agent
sudo ./posternproxy-agent install --config-file agent.json --assets-dir .
rm agent.json
```

The JSON fields are `controller_url`, `agent_token`, and numeric `server_id`.
Use the values returned by the controller; a management API token cannot replace
an agent token. The file must be private and contain only one JSON object.
Installation checks credentials against `/agent/verify` before host changes, so
upgrade the controller before installing new agents.

For a replacement or restored node, **Rotate enrollment** returns a new one-time
credential and disconnects the previous identity. The admin API equivalent is
`POST /api/servers/{id}/agent-credentials`. Do not run two agents with the same
identity. The older join-token endpoints do not provide an installation workflow;
they no longer return a nonfunctional bootstrap command.

## Logs and readiness

```bash
sudo systemctl status caddy posternproxy-agent
sudo journalctl -u posternproxy-agent -u caddy -n 100 --no-pager
```

Provisioning output is streamed in the wizard and retained on the server record.
Administrators can also read `/api/servers/{id}/provision-log`. Check
[Configuration application status]({{< relref "/multi-server/configuration-status" >}})
and actual traffic; a later online heartbeat alone does not prove configuration
was accepted. Custom certificates use private encrypted controller storage and
narrow group-readable runtime files created by the installer.

## Docker provisioning API

The SSH Docker deployment mode requires an already running Docker installation
and a published agent image tagged with the controller's release version. Source
or dirty development versions are refused. It passes credentials through a
private environment file, preserves the firewall, and requires the same
controller-side authenticated/applied readiness check. A previous container is
retained as `posternproxy-agent-previous` for operator rollback; remove or rename
that retained container before another replacement. Existing data is preserved.
This path remains hidden in the UI pending fresh container qualification.
