# 0011. Port forwarding is Caddy-L4 only; OS-level iptables/socat forwarding is dropped

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision requested by the owner)
- **Deciders:** Michael (owner)
- **Topics:** forwarding, architecture, provisioning

## Context

Port forwarding could be done with OS tools (iptables/socat) or with Caddy's layer-4 stream handling. Caddy L4 (stream hosts) covers the real-world forwarding cases for this audience, and keeping OS-level forwarding adds packages, capabilities and provisioning surface.

## Decision

1. Port forwarding is handled **solely by Caddy L4** (stream hosts). iptables/socat forwarding is removed from the controller, the agent, and the docs.
2. A **health monitor** reports a forward as **blocked** when its port is not reachable.
3. Docs and UI state that the forwarded port and any upstream/cloud firewall must be opened (a UFW notice on forward creation).

## Consequences

- Easier: one forwarding mechanism; no OS forwarding packages or elevated agent capabilities.
- Harder: forwarding is bounded by Caddy L4's capabilities; the firewall is explicitly the operator's responsibility.
- Now a rule: do not reintroduce OS-level forwarding; forwards are Caddy stream hosts.

## Evidence

Stream-host forwarding and the forwards health monitor; the Caddy-L4-only forwarding plan; shipped forwarding commits. See [../evidence/0011-caddy-l4-forwarding.md](../evidence/0011-caddy-l4-forwarding.md).
