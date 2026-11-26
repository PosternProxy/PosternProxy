# Next release: proposed security integrations

Status: proposed TODO; not started. Date: 2026-11-24.
Scope boundary: [current beta versus next release](beta-security-release-boundary.md).

- [ ] Add management MFA/OIDC with explicit role mapping and recovery. Preserve
  application authorization; external/tailnet identity is not automatic admin.
- [ ] Evaluate optional Caddy forward-auth with an established identity provider
  such as Authelia for published applications. Define header trust, failure
  behavior and exclusions without bypassing application/API authentication.
- [ ] Add per-host request-limit/timeouts policy presets for static sites, APIs,
  large uploads and streaming, with tests for protocol-specific exceptions.
- [ ] Productize a Cloudflare-origin profile: trusted visitor identity, origin
  exposure restrictions, verified origin TLS, validation/status and recovery.
- [ ] Evaluate Coraza with OWASP CRS as an optional pinned Caddy integration.
  Start in detection mode, expose rule IDs, support host/path/parameter-scoped
  exceptions, and define safe activation, updates, rollback and failure behavior.
- [ ] Evaluate CrowdSec and its Caddy remediation component after structured
  logging is available. Define temporary decisions, expiry, allowlists, auditing,
  outage behavior and recovery. Do not turn one upstream 403 into an automatic
  permanent ban. Document optional threat-intelligence/data-sharing behavior.
- [ ] Integrate [logs and denial actions](next-release-logging.md); distinguish
  ACL, rate-limit, upstream, authentication and WAF denials. An IP allow rule must
  not silently bypass WAF or authentication policy.
- [ ] Measure each optional engine's memory, CPU, latency and concurrency on
  small nodes before defining defaults. Preserve a lightweight core install.
- [ ] Add release signatures and verification, SBOM/provenance, dependency and
  ruleset update policies, and reproducible artifact evidence.
- [ ] Apply the isolated-build/credential boundaries in the
  [static deployment proposal](next-release-static-sites-and-tailnet-ingress.md).
  A managed builder must not run repository code with controller privileges.
- [ ] Review dependency compatibility, maintenance, distribution terms and the
  project's AGPL/commercial licensing model before adopting components.

## Primary references

- [Caddy forward authentication](https://caddyserver.com/docs/caddyfile/directives/forward_auth)
- [Coraza connectors](https://www.coraza.io/connectors/)
- [CrowdSec remediation components](https://app.crowdsec.net/hub/remediation-components)
