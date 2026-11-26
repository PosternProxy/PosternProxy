# Plan: Caddy-L4-only traffic forwarding + firewall awareness

## Decisions (from user)
- Drop **iptables** and **socat** forwarding entirely (controller + agent + docs).
  Caddy L4 (stream hosts) covers every real-world forwarding case for this audience.
- Add a **health monitor** that reports a forward as **blocked** when its port
  isn't reachable.
- Make docs clear that the **forwarded port and any upstream/cloud firewall** must
  be opened.
- Add a **UI notice** for configuring ufw when a forward is created.

## Feasibility note (firewall detection)
A reliable same-host firewall probe is impossible: dialing the box's own public IP
routes via loopback and bypasses the external firewall; `ufw status` needs root
(controller runs non-root); cloud security groups are invisible to the host.
Therefore the monitor uses honest, per-server-type semantics:
- **Remote server:** controller dials `hostname:incoming_port` over the network →
  real reachability → `open` / `blocked`.
- **Local server:** controller checks whether Caddy is actually listening on
  `127.0.0.1:incoming_port` → `open` (listening) / `down` (not served). The
  external firewall is covered by the UI notice + docs, not auto-detected.

## Work
1. **Remove iptables/socat (controller).**
   - Delete `internal/portforward/` (iptables.go, socat.go, manager.go).
   - Delete `internal/services/port_forward.go`, `internal/api/handlers/port_forwards.go`,
     `internal/models/port_forward.go`.
   - Remove `PortForwardService` wiring + `RestoreAll` from `app.go`, the
     `/port-forwards` routes and RouterConfig field, and any crud_helpers usage.
   - Drop the `port_forwards` table from `001_initial.sql` and remove it from
     backup.go (tableOrder, columns, clearOrder).
2. **Remove iptables/socat (agent).**
   - Remove `KindPortForwardApply` / `KindPortForwardRemove` handling and
     `PortForwardPayload` from `cmd/posternproxy-agent/main.go` and
     `internal/agent/protocol.go`, plus the `portforward` import.
3. **Frontend: Traffic Forwarding = Caddy L4 only.**
   - Remove the method selector (caddy/iptables/socat) and all port-forward API
     calls/types; the page manages stream hosts only.
   - Add a firewall notice in the create flow (open the port in ufw + cloud FW).
4. **Health monitor (stream-host reachability).**
   - New service modeled on TLSProbeService: poll each enabled stream host, set
     `open` / `blocked` / `down` per the semantics above; expose `GET /api/health/forwards`.
   - Surface the status on the Traffic Forwarding list.
5. **Docs.** Rewrite port-forwarding docs: Caddy L4 only, firewall requirement
   (host ufw + upstream/cloud firewall), remove iptables/socat.
6. **Verify + CHANGELOG.**

## Status: incomplete — reachability monitor remains open
The original completion note overstated coverage. The beta-exit review and
2026-10-24 source check found no reachability monitor or `/api/health/forwards`
route. Tasks 1-3 and the documentation cleanup are present in the working tree;
task 4 is still open. The UI firewall reminder does not detect reachability.
Prior build/test results do not establish task 4 completion.

The monitor remains a follow-up here; do not add it to the beta-exit scope merely
to satisfy the old note. Track release claim accuracy in
[Plan G](beta-exit-release-qualification.md).

## Notes
- Pre-beta, zero deployments: the `port_forwards` table is removed from the initial
  migration (dev DBs must be recreated), consistent with prior cleanup.
- Stream hosts already are the Caddy-L4 backend; this removes the *duplicate* OS
  forwarders, not the L4 capability.
