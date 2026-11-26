# Next beta and initial production release readiness

Date: 2026-11-25. Assessed revision: cb4d231. User revised scope below after the
initial assessment. This is not release approval.

## Next beta: outstanding agreed gates

1. No existing external deployments/configurations need migration. New native
   installers reserve bridge port 8443. Keep the reserved-port requirement clear;
   direct environment startup and preserved installer configuration now share
   the same guard, covered by regression tests. This correctness item is complete.
2. Obtain green supported Linux CI including race, Go/frontend dependencies,
   browser tests and pinned Caddy traffic. Fresh independent source review and
   follow-up fixes are complete; exact-candidate Linux CI is still required.
   See [today's owner runbook](beta-release-runbook-2026-11-25.md).
3. Before beta, targeted traffic checks must prove advertised proxy/stream paths
   work, including combined TCP/UDP and passthrough alongside normal HTTPS.
   Extended real-network/multi-server workload and outage testing belongs during
   beta and gates production, rather than blocking beta feedback indefinitely.
4. Browser coverage completed: 15 passing scenarios now include UI encrypted
   backup/restore, real outbound-agent applied readiness, and non-localhost HTTPS
   secure cookies/CSRF. See [completed browser work](completed/browser-release-workflows-2026-11-25.md).
5. Qualify only the selected release targets: Windows including service
   installation, native Linux amd64/arm64, and Docker/Compose. Native amd64 remains
   supported; the other targets are additions. The Windows service
   installer and platform qualification jobs are now implemented. Actual Windows
   controller/agent SCM lifecycle, custom TLS and removal passed locally; see
   [Windows evidence](completed/windows-local-installer-qualification.md).
   [Linux/container qualification](completed/linux-qualification-165-227-3-116.md)
   also passed: Ubuntu amd64 native install/provisioning, source-built amd64
   Docker/Compose, and ARM64 installer/runtime images under QEMU. Exact candidate
   CI and native ARM hardware/source-image qualification remain pending. Do not add
   a distribution expansion or historical published-beta DB gate. Test clean
   installation on each supported target and actual release-asset installation.
6. Choose the release commit/version and build matching controller, agent and
   Caddy bundles. Record artifact hashes and run install/start/login/traffic smoke
   tests against those exact bundles. Publish their support matrix, limitations
   and rollback/recovery instructions. Owner chooses publication.
   Exercise version-stamped draft artifacts before publication and verify public
   download/install once published; distinguish draft from public-path evidence.

The user's latest scope decision supersedes the earlier expanded distribution
and historical-upgrade matrix. Multi-server soak runs during beta before v1.

## Initial production / v1 release

- All advertised supported configurations pass the above gates, with no open
  critical/high correctness or security blockers. Excluded capabilities must be
  explicit in the support policy and product claims.
- Observe the beta under representative real use for a predefined period and
  workload, including reloads, offline agents, certificate renewal, restart and
  resource pressure; establish measured acceptance limits before the run.
- Prove supported upgrades and disaster recovery using real deployments: preserve
  secrets/certificates, restore onto a clean controller, reenroll/map agents,
  verify traffic, and exercise rollback or a documented forward recovery path.
- Finish independent security review of management auth/authorization, token
  lifecycle/redaction, agent trust, provisioning and certificate distribution.
  Publish supported versions, compatibility/migration guarantees, known limitations
  and a vulnerability-response/update process.
- Produce and test the exact versioned production artifacts and approve a dated
  readiness report. A production label does not itself add support coverage.

Native Tailscale, managed Gitea/Hugo, access-log browsing/actions, WAF/CrowdSec and
native SSO/MFA remain separate roadmap work. They are not automatically v1 blockers.
Validated Cloudflare-origin support remains explicitly excluded unless separately
qualified and advertised.

References: [broader beta](broader-beta-release-execution.md),
[completed platform implementation](completed/windows-services-runtime-and-release.md),
[release gates](beta-exit-release-qualification.md),
[blocker ledger](beta-exit-blocker-closure-ledger.md),
[prior executed evidence](beta-security-qualification-2026-11-24.md).
