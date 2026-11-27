# 0004. Remote servers are managed by an outbound-only agent

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made at project inception)
- **Deciders:** Michael (owner)
- **Topics:** architecture, multi-server, agent

## Context

Multi-server management must work across NAT and firewalls without requiring inbound ports on every managed host. The controller could poll/dial each server, or servers could connect out to the controller.

## Decision

1. Each remote server runs a lightweight `posternproxy-agent` that maintains an **outbound WebSocket** connection to the controller. The controller never dials into managed servers.
2. Configuration is pushed over that connection and apply results/acknowledgements return over it.

## Consequences

- Easier: works through NAT/firewalls with no inbound exposure on managed hosts.
- Harder: liveness and identity centre on the controller's hub; the agent's authentication and transport are high-security (ADR 0007), and config delivery relies on the reconciliation contract (ADR 0008).
- Now a rule: never add an inbound/controller-dials-in path; preserve the outbound-only model.

## Evidence

`cmd/posternproxy-agent`, `internal/agent/hub.go`, `internal/agent/protocol.go`; `AGENTS.md`. See [../evidence/0004-outbound-only-agent.md](../evidence/0004-outbound-only-agent.md).
