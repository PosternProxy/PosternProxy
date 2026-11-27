---
title: Beta limitations
weight: 6
---

# Beta limitations

PosternProxy remains beta. A binary build, an online agent and working traffic
are separate outcomes. Use
[release validation]({{< relref "/administration/release-validation" >}}) before
deployment.

Windows amd64 service installation and native Linux amd64/arm64 packaging are now
implemented, with native qualification jobs and Docker/Compose smoke tests in
the release workflows. Controller/agent Windows service lifecycle and custom TLS
passed on an authorized workstation using the development bundle. Ubuntu amd64
native installation/provisioning and source-built Docker/Compose checks pass.
ARM64 installer and container functional checks also pass under QEMU; container
runtime checks used verified cross-built binaries. Candidate runner checks and
native ARM hardware qualification are still required;
local unit tests and cross-compilation are not platform qualification. Tagged
workflows create drafts after qualification; Docker publishing starts only after
the owner publishes the GitHub release. No release was published by this work.

The current development archive has been exercised on an Ubuntu 24.04 amd64 VM:
existing-controller upgrade/rerun, fresh controller data and printed login,
native agent installation, actual SSH provisioning, custom TLS, service-user
certificate permissions and service restarts. Container development qualification
is described above. Exact published downloads remain a release gate. Other
Linux distributions and historical beta database upgrades are outside the
qualified scope; retain an offline installation backup before experimenting.

Certificate validation, encrypted source storage, per-target distribution,
restricted runtime files, manual enrollment, applied provisioning readiness,
admin-only audit access, stream conflict checks and encrypted portable recovery
now have local regression coverage. Real pinned-Caddy acceptance covers custom
TLS, clean recovery with a different controller key, and two outbound agents with
restart reconciliation. These traffic tests also pass natively on that Ubuntu
VM. The certificate group checks exercise real Linux users and systemd Caddy;
firewall mutation and other distributions remain outside that result.

Per-server Caddy proxy trust, visitor ACL/rate identity, canonical upstream client
headers and streaming compatibility have live acceptance coverage. An actual
Cloudflare Tunnel or authenticated-origin deployment has not been qualified;
the beta does not claim a validated Cloudflare-origin setup.

NTLM authentication passthrough is implemented and stays off unless an
administrator enables it. The traffic checks above do not exercise it, and IIS
behavior is not verified.

Docker SSH provisioning requires a published version-matched image and existing
Docker; it remains hidden in the UI. Failed replacements retain the previous
container for operator rollback. Published container/release installation is a
separate release gate.

Audit writes remain best-effort, not a complete compliance trail. Config history
lists 100 entries without automatically pruning older rows. Local dashboard
counters are not per-agent health probes. DNS-01 certificate management,
access-list HTTP Basic Auth and a complete public API schema are not implemented.

Version 1 JSON exports remain partial. Use the new encrypted archive for portable
configuration and custom certificate recovery, and retain offline copies for
whole-installation recovery and unsupported schema rollback.
