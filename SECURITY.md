# Security Policy

## Reporting a Vulnerability

Do not report suspected vulnerabilities in a public issue or discussion.

Use the repository's **Security** tab to submit a private vulnerability report
through GitHub Security Advisories. Include the affected version or commit,
reproduction steps, impact, and any suggested mitigation. If private reporting
is unavailable, contact the repository owner through their GitHub profile and
request a private reporting channel without disclosing vulnerability details.

The maintainers will acknowledge a report as soon as practical, investigate it,
and coordinate remediation and disclosure with the reporter. Please allow time
for a fix to be developed and distributed before publishing details.

## Supported Versions

PosternProxy is currently preparing its first Beta release. Security fixes are
provided for the latest Beta release and the current `master` branch only.

| Version | Supported |
|---|---|
| Latest Beta | Yes |
| `master` | Yes |
| Older builds | No |

## Security Architecture

- Management access uses short-lived JWT access tokens, refresh cookies, CSRF
  protection, bcrypt password hashes, and tighter rate limiting on login and
  agent authentication endpoints.
- General authenticated API rate limiting is not yet implemented. Deployments
  should restrict management access at the network or reverse-proxy layer.
- Stored SSH credentials are encrypted with AES-256-GCM using a key derived
  from the configured JWT secret. Keep that secret stable, private, and backed
  up; changing it prevents existing encrypted credentials from being read.
- PosternProxy delegates proxying and TLS termination to Caddy. Protect the
  Caddy admin API from network access and bind it to a trusted local interface.
- PosternProxy does not include a WAF. Internet-facing deployments should use a
  suitable upstream WAF or filtering layer when their threat model requires it.
- The install scripts configure systemd sandboxing, firewall rules, fail2ban,
  and unattended security updates. Review those defaults for your environment.

See [Security Hardening](docs/content/administration/security-hardening.md) for
deployment guidance and verification commands. Some planned hardening work is
tracked in [the Beta release readiness plan](plans/beta-release-readiness.md).
