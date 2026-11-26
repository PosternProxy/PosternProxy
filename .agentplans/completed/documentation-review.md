# Documentation review

Status: complete, November 23, 2026. Documentation corrected; product fixes and
release qualification remain with the existing beta-exit plans.

User request: review the rest of the documentation for completeness and confirm
whether the build and archive include the agent. Scope: README, documentation
site pages, docs developer guidance, and security/deployment references. Correct
verified drift without changing application behavior or licensing terms.

## Acceptance

- Build/packaging instructions distinguish the controller, bundled agent and
  Caddy, manual agent deployment, and the current local archive versus published
  release assets. No separate agent build is required for the provided bundle.
- Configuration, routes, UI actions, security defaults, deployment/recovery
  instructions and advertised capabilities agree with inspected code. Pending
  release qualification and known implementation limitations remain explicit.
- Read all current docs-site pages, verify internal references and relevant
  commands safely, and build the Hugo site. Record coverage and remaining gaps;
  source inspection is not live deployment or feature acceptance evidence.

## Coverage and changes

Reviewed all 37 current docs-site Markdown pages (including the new beta-limits
page), root README and SECURITY, docs README, and frontend README. Licensing terms
were preserved. Compared claims with the Vue router/sidebar/views/forms, Go API
router/middleware/models/services, Caddy translation, native installer, legacy
systemd/Docker provisioning, Docker entrypoint/Compose, build scripts and CI.

- Verified the local archive contains controller, agent, Caddy and checksums.
  Documented loose build outputs, manual archive creation, individual release
  downloads, architecture selection, agent installation path and remote deployment.
- Corrected frontend ports, embedded-UI copy requirements, development cookies,
  Compose HTTP override, canonical configuration names/defaults and log locations.
- Corrected missing-CSRF API examples and nonexistent Settings sync/reset/profile
  controls. Stored settings do not reconfigure the runtime services.
- Reconciled legacy provisioning's real service names, missing bootstrap fields,
  root requirement, firewall reset, incomplete manual enrollment and readiness.
- Corrected import preview/confirmation semantics and enabled-by-default import.
- Corrected history listing versus pruning, prefix/order behavior for locations,
  header presets/grouping, error-page scope, dashboard/local counters, stream
  collisions, direct-peer ACL/rate-limit behavior and dead-host ACL non-enforcement.
- Corrected audit coverage/access/filter/export claims, certificate storage versus
  validation/expiry/access/distribution claims, and unsafe secret-rotation advice.
- Added `docs/content/administration/beta-limitations.md` and navigation to keep
  verified product gaps visible instead of describing planned work as delivered.
- Updated CHANGELOG. No application code, policy/licensing terms, generated UI or
  pre-existing build artifacts were changed by this documentation pass.

## Verification

- `hugo --minify`: passed; existing deprecated hint/columns shortcode warnings remain.
- Checked local Markdown file references and 2,246 rendered local links/anchors
  across generated HTML: passed.
- Parsed 56 shell examples with `bash -n`: passed after correcting a bare `<ip>`
  placeholder. Syntax checks do not execute provisioning or prove live API/OS behavior.
- `git diff --check`: passed.
- `tar -tzf bin/posternproxy-linux-amd64.tar.gz`: confirmed all four expected files.

No live production access, remote provisioning, new release publication, external
website link validation, advisory scanning or fresh VM/container acceptance was
performed. Go/frontend tests were not rerun for docs-only changes; the preceding
implementation/build checks remain recorded in the install-subcommand notes.

## Remaining product work

The existing provisioning, certificate, recovery and release-qualification plans
remain active. Settings runtime wiring, audit gaps, incomplete enrollment,
certificate isolation/access/validation, stream conflict validation and broader
acceptance need implementation work. This documentation review does not authorize
or claim those fixes. A public API schema and screenshots remain additional
documentation opportunities; current coverage is a route/feature guide, not a
complete generated API specification.
