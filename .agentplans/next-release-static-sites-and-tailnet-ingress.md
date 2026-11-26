# Proposed next release: static sites and public tailnet ingress

Status: proposed TODO; not implemented. Requested use case: 2026-11-24.
Depends on [optional Tailscale integration](next-release-tailscale.md).

## User scenario

The source for a Hugo/static site lives in an internal Gitea repository reachable
only over the user's tailnet. Publish the built site on a small DigitalOcean
droplet running PosternProxy/Caddy, using the user's own public domain. The same
droplet can reverse proxy explicitly selected private services over the tailnet.
Visitors use normal public HTTPS and do not need Tailscale.

This is a self-hosted publishing/ingress gateway. A single droplet does not
provide a global CDN, distributed failover or network-scale DDoS absorption.
WAF, general-purpose hosted CI and unrestricted public forward-proxy service are
outside the proposed initial scope. Managed, isolated static-site builds are part
of the proposed publishing experience. Preserve the existing commercial license model.

## Managed Git deployment: primary proposed workflow

Clarification: the user wants the connect-repository/build/deploy experience of
managed static hosting. Gitea CI setup must not be a prerequisite. The earlier
CI-upload flow remains an optional alternative.

1. Connect a private Gitea repository in PosternProxy using a read-only repository
   credential; reach the repository over the tailnet.
2. Select the production branch, project root, Hugo/build preset and version,
   build command when supported, output directory and public site domains.
3. A verified push webhook triggers a build. Provide manual deploy and optional
   polling when webhook setup/connectivity is unavailable.
4. A Postern-managed worker fetches the configured repository at a verified
   commit and runs the build in an isolated, unprivileged workspace.
5. PosternProxy validates the generated output, stages an immutable release and
   activates it safely. Caddy serves the site on the public domain with HTTPS.

- [ ] Add a Static Sites resource: site name, repository/branch/build settings,
  domain names, hosting node, active release and bounded build/deployment history.
- [ ] Start with a managed Hugo preset and controller/droplet-local publication;
  keep the builder separate from the privileged controller and serving process.
  The operator configures the project through PosternProxy, without creating a
  Gitea Actions workflow or operating a separate CI product.
- [ ] Design the worker/runtime: local sandbox/container or a Postern-managed
  remote builder. Measure CPU, memory and disk on the small droplet, bound job
  concurrency and enforce timeouts. Heavy Node/theme builds must not exhaust the
  controller or interrupt serving; support a separate worker if necessary.
- [ ] Protect repository credentials at rest and in transit. Fetch source with
  least privilege, verify Git host identity, and keep deploy keys/tokens outside
  the untrusted build environment and published output. Do not run repository
  scripts as root or in the controller's filesystem/security context.
- [ ] Configure/validate signed Gitea push webhooks, or clearly guide manual hook
  setup. Request webhook-management rights separately from read-only clone
  credentials. Verify repository/branch/commit against project configuration,
  deduplicate deliveries and prevent an older build from replacing a newer one.
- [ ] Show queued, fetching, building, validating, activating, serving and failed
  states, with redacted build logs, cancellation and manual retry.
- [ ] Keep unsupported build commands/frameworks explicit; the initial preset is
  not a promise to run arbitrary server applications or a general-purpose CI.

## Optional CI/artifact upload workflow

An existing Gitea Actions runner or any other CI can still build the site and
upload its output through a site-scoped deploy API/CLI over the tailnet. Both
workflows reuse the same artifact validation, release activation and rollback.

- [ ] Provide a documented optional Gitea Actions workflow and provider-independent
  deploy API/CLI. GitHub is not required in either workflow.
- [ ] Use a deployment-only token scoped to one site, with expiry/revocation,
  rotation and audit attribution. It must not authorize user, proxy, server,
  Tailscale or access-list administration. Keep secrets out of workflow logs and
  process arguments.
- [ ] Validate compressed/unpacked sizes, file count, digest, paths and content
  layout before activation. Reject traversal, absolute paths, symlinks/hardlinks
  and special files. Avoid publishing Git metadata, credentials or source trees.
- [ ] Keep separate site-owned release directories; Caddy gets read-only access
  to published content and no access to source repositories or deploy secrets.
  Do not rely on a file-server root alone as a filesystem sandbox.
- [ ] Ensure rejected, interrupted or concurrent deployments preserve the last
  working release. Record staging/activation failure separately from successful
  serving, integrate configuration revisions, and support rollback.
- [ ] Show commit/revision, CI run/reference, artifact digest, timestamps,
  active deployment and actionable redacted failure information.
- [ ] Support normal Hugo index/404 behavior, compression and cache headers.
  Make SPA fallback an explicit choice rather than silently applying it to Hugo.
- [ ] Enforce aggregate disk quotas and bounded release retention. Define static
  content backup/recovery separately from the existing limited-size configuration
  archive; do not imply that current config export includes site artifacts.

## Public reverse proxy to private tailnet services

Traffic path: public visitor -> public domain/droplet Caddy -> Tailscale ->
explicitly configured private origin. The existing agent WebSocket remains a
management channel; Tailscale carries the origin connection.

- [ ] Support tailnet IP/DNS upstreams through the host-network integration,
  with explicit per-host public/private exposure and origin configuration.
- [ ] Use custom domains the operator controls, with DNS pointing to the droplet
  and a working certificate-validation path. No Funnel or .ts.net hostname is
  required for these public virtual hosts. Do not promise wildcard issuance with
  the currently unsupported DNS-01 workflow.
- [ ] Keep the management UI and deploy endpoint tailnet-only when configured;
  public publication of a site must not publish management, Gitea or other tailnet
  resources as a side effect.
- [ ] Recommend narrowly scoped tailnet grants/tags for the gateway's required
  origin ports. Avoid exit-node/subnet-router changes unless explicitly chosen.
- [ ] Preserve existing host access lists, rate limiting, client-IP trust rules,
  audit history and TLS verification. Surface unavailable origins and blocked
  tailnet access distinctly from failed public certificate issuance.
- [ ] Integrate the planned per-host access logs/TLS events and denied-request
  actions without exposing private credentials or full source-repository URLs.
- [ ] Measure direct versus relayed throughput and resource use; document that
  availability, bandwidth and capacity depend on the droplet, tailnet path and
  origin, rather than promising CDN-scale behavior.

## Later scope / decisions

- [ ] Decide whether preview environments and branch deployments are public or
  private by default; avoid publishing draft/private repository content implicitly.
- [ ] Design remote-node artifact deployment separately: bounded transfers,
  authenticated downloads and atomic activation without blocking configuration
  or log messages. Do not send unbounded archives on the control WebSocket.

## Acceptance before release

- [ ] Demonstrate connect-private-Gitea -> push -> managed build -> public custom
  domain, without Gitea CI setup, exposing Gitea or requiring GitHub. Also verify
  the optional external-CI artifact upload flow.
- [ ] Confirm a deployed static site remains available when Gitea/the builder/the home
  tailnet connection goes offline after deployment.
- [ ] Exercise invalid archives, quotas, token scope isolation, failed builds,
  interrupted uploads, failed activation, rollback and reboot recovery.
- [ ] Verify public HTTPS to a private tailnet origin, including denied tailnet
  policy, unavailable origin and restart/reconnect behavior.
- [ ] Test boundaries between public sites, private management/deployment,
  different sites' files, and source secrets. Record small-droplet resource use.
- [ ] Update docs/changelog when implemented and run Go/frontend, browser and
  actual Caddy traffic checks.

## Primary references

- [Hugo build output](https://gohugo.io/getting-started/usage/)
- [Caddy static file serving](https://caddyserver.com/docs/caddyfile/directives/file_server)
- [Caddy reverse proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)
- [Gitea Actions](https://docs.gitea.com/usage/actions/)
- [Gitea push webhooks](https://docs.gitea.com/usage/repository/webhooks/)
- [Cloudflare managed build settings](https://developers.cloudflare.com/pages/configuration/build-configuration/)
- [Cloudflare CI Direct Upload alternative](https://developers.cloudflare.com/pages/how-to/use-direct-upload-with-continuous-integration/)

Cloudflare Pages also supports CI upload of prebuilt assets; manual upload is not
inherent to a Gitea workflow. This proposal is for a self-hosted deployment and
ingress option rather than a claim that Cloudflare only accepts GitHub uploads.
