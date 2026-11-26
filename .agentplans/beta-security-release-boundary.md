# Proposed security boundary: current beta versus next release

Status: correctness implemented and Ubuntu qualification recorded; remaining
release gates are explicit below. See [qualification evidence](beta-security-qualification-2026-11-24.md).
Date: 2026-11-24. Complements the existing beta-exit plans; does not close them.

## Current beta: correctness and safe deployment

- [x] Correct verified client identity in generated Caddy configurations before
  recommending operation behind Cloudflare or another proxy. Keep socket-peer
  identity separate from visitor identity; apply the latter consistently to host
  IP rules, rate-limit keys, forwarded client headers and access-log attribution.
  Routes use client_ip and http.vars.client_ip with explicit per-server static
  trust. Management proxy policy remains separate.
- [x] Test direct requests, trusted single/multiple proxy hops, forged forwarding
  headers, malformed chains, IPv4/IPv6 and proxy-range refresh failure. Unknown
  peers must not acquire trusted-header authority. Avoid trusting entire private
  networks merely because a deployment is behind a proxy.
- [x] Verify baseline header/body/resource limits and client/upstream timeouts;
  add missing safeguards conservatively. Exercise uploads, SSE, WebSockets and
  streaming so blanket limits do not silently break supported applications.
  Keep richer policy presets/UI as future scope.
- [x] Validate private/restricted management and deployment guidance, protected
  Caddy admin access, least-privilege service/file permissions, stable encrypted
  secrets, normal application auth/CSRF and audited privileged operations.
  An externally installed VPN/tailnet or SSH access can restrict administration
  now; built-in Tailscale is not required to recommend private management.
  Ubuntu checks prove loopback Caddy admin, real certificate group access and
  unrelated-user denial, private files, stable credentials and service restarts.
- [ ] Provide a tested Cloudflare reference deployment if claiming support:
  correct client identity, verified origin TLS and controlled origin access.
  Prefer a tested Tunnel or customer-specific authenticated-origin design when
  appropriate. Document the limits of network-IP allowlisting alone and prohibit
  direct public-origin bypass. Keep one-click setup outside this beta.
  No validated Cloudflare deployment is claimed; docs explicitly identify this
  unqualified path. Actual account/origin deployment remains future evidence.
- [ ] Close the promised target-environment release gates: current controller
  and agent/systemd installs, certificate group permissions, safe provisioning,
  supported upgrade/recovery paths, published asset installation and offered
  container configurations. Existing successful user controller installation
  remains evidence; it does not qualify every later build or agent deployment.
  Current Ubuntu amd64 archive: controller upgrade and fresh-data initialization,
  native agent, actual SSH provisioning, certificate permissions, TLS, recovery
  and restart checks pass. Other OS/CPU combinations, fresh packages, Docker,
  historical beta databases and published downloads remain open.
- [ ] Retain full Go/frontend/browser/live-traffic checks, pinned component
  verification and source/binary vulnerability scans. Release claims must match
  demonstrated support and identify unqualified deployment paths.

These are requirements/review items, not a statement that all remain unfixed.
Do not add optional security engines simply to increase the current beta's scope.
If proxy trust/origin protection cannot be qualified, do not market the build as
a validated Cloudflare-origin deployment.

## Next release: productized and optional integrations

See [next-release security](next-release-security.md), alongside the already
requested logging, Tailscale and managed static-site deployment proposals.

- Native management MFA/OIDC and optional application forward-auth integration.
- Per-host policy presets and a guided Cloudflare-origin setup/status experience.
- Coraza/OWASP CRS evaluation, detection mode, scoped rule exceptions and tuning.
- CrowdSec evaluation with bounded, attributable remediation and safe recovery.
- Rich security-event visibility, denied-log actions and remote log collection.
- Optional Tailscale and managed Git/static publishing, with isolated builders.
- Release signatures, SBOM/provenance and a maintainable security-rule update path.

## References

- [Caddy proxy trust](https://caddyserver.com/docs/caddyfile/options#trusted_proxies)
- [Cloudflare origin protection](https://developers.cloudflare.com/fundamentals/security/protect-your-origin-server/)
- [Current implementation/local evidence](beta-next-fixes.md)
- [Release qualification gates](beta-exit-release-qualification.md)
