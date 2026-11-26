# Proposed next release: optional Tailscale integration

Status: proposed TODO; implementation and architecture are not yet approved.
Requested for consideration: 2026-11-24. Audience: home/self-hosted users.

Concrete follow-up: [private Gitea static publishing and public ingress to tailnet
origins](next-release-static-sites-and-tailnet-ingress.md). This use case requires
Caddy data-plane access to the tailnet, reinforcing the host-service option for
the first complete integration; embedded management connectivity alone is insufficient.

## Intended outcomes

- [ ] Provide a Connect Tailscale setup flow, connection status, tailnet name,
  node identity/address and clear disconnect/reconnect controls.
- [ ] Support private access to the management UI and controller/agent transport
  through the user's tailnet. Preserve outbound-only agent connections.
- [ ] Let Caddy reach private upstreams over Tailscale, including a home server
  behind NAT. Explain that the existing agent WebSocket carries management
  commands, not public proxy traffic.
- [ ] Keep public services and tailnet-only services explicit. Tailnet membership
  alone does not make a service publicly reachable.

## Architecture to decide before implementation

- [ ] Compare optional integration with a host-installed tailscaled service
  against embedded Go tsnet, including existing Tailscale installations.
  Initial recommendation: the host service for a complete first integration,
  because Caddy is a separate process that needs tailnet routing and DNS too.
- [ ] Evaluate tsnet for self-contained private management and agent transport.
  Embedding tsnet in the controller/agent does not automatically give Caddy or
  other OS processes access to its userspace network. Any data-plane adapter
  must be explicit and retain Caddy's proxy/TLS ownership.
- [ ] Measure binary size, idle/active memory and relayed throughput on the
  supported small-memory node before choosing an embedded default.
- [ ] Design private HTTPS, MagicDNS/custom-name handling, certificate renewal
  and Secure-cookie behavior. Avoid confusing private TLS with public HTTP-01
  issuance; retain the existing login/CSRF protections.

## Initial product boundaries

- [ ] Make Tailscale optional; ordinary installs continue to work without an
  account, SDK dependency at runtime, or changes to network routing.
- [ ] Prefer browser-based enrollment. Protect any unattended auth keys and
  node state, review supported encrypted storage, and exclude secrets/auth links
  from logs, audit details and portable configuration exports.
- [ ] Keep PosternProxy authentication and roles. Do not turn a tailnet identity
  or forwarded identity header into automatic management-admin authority.
- [ ] Require admin authorization and attributed audit events for connection,
  disconnection and exposure changes. Use constrained APIs rather than arbitrary
  privileged shell execution.
- [ ] Verify the private management path works before an explicit switch to
  tailnet-only access; retain a documented local recovery path.
- [ ] Preserve existing host Tailscale configuration and policies. Do not enable
  exit-node behavior, subnet advertisements, Tailscale SSH or public Funnel
  exposure as a side effect of connecting PosternProxy.
- [ ] Consider subnet routing and Funnel as separate later scope. Funnel has
  domain/port/bandwidth constraints and must not be presented as unrestricted
  public hosting or enabled for the management UI by default.
- [ ] Review dependency/distribution/service terms and preserve the project's
  AGPL/commercial dual-licensing model and owner-only commercial licensing.

## Acceptance

- [ ] Test enrollment, reboot/state persistence, expired/revoked identity,
  disconnect, offline operation and disabled integration.
- [ ] Verify private management login/refresh/CSRF and agent reconciliation,
  plus actual Caddy traffic to a tailnet upstream across two nodes.
- [ ] Exercise NAT/relay paths, denied tailnet policy, DNS failure, node key
  expiry, existing host installations and recovery from a failed setup.
- [ ] Demonstrate that private-only endpoints remain inaccessible outside the
  tailnet and unauthorized identities cannot acquire PosternProxy privileges.
- [ ] Document supported Linux/container behavior and performance, key/state
  recovery, privacy defaults and the distinction between control and proxy traffic.

## Primary references checked

- [tsnet overview](https://tailscale.com/docs/features/tsnet)
- [tsnet implementation and userspace isolation](https://github.com/tailscale/tailscale/blob/main/tsnet/tsnet.go)
- [Tailscale firewall and NAT behavior](https://tailscale.com/docs/reference/faq/firewall-ports)
- [Tailscale Funnel and its limits](https://tailscale.com/docs/features/tailscale-funnel)
