# Next release: access logs, TLS events and denied-request actions

Status: TODO; not started. Target: next release, version to be assigned.
Requested: 2026-11-24.

## Scope

- [ ] Add optional HTTP access logging for proxy, redirect and parked hosts.
  Show timestamp, assigned server/host, method, sanitized path, response status,
  duration, bytes and the configured client-IP representation.
- [ ] Add structured TLS events for certificate issuance/renewal attempts,
  successes, failures and retry information. Keep these distinct from the existing
  serving-certificate probes and management audit log.
- [ ] Provide filtering by server, host, time and status, with errors-only and
  denied-only views, pagination and optional live tail.

## Remote agents

- [ ] Keep high-volume access logs in bounded, rotating files on each node.
  Fetch filtered pages on demand through the existing outbound authenticated
  WebSocket; do not require inbound agent ports or SSH for log viewing.
- [ ] Stream bounded batches while a live-tail panel is open; stop on close,
  cancellation or timeout. Use cursors that handle rotation and truncation.
- [ ] Automatically forward important sanitized TLS events to the controller so
  previously received failures remain visible while a node is offline.
- [ ] Make offline behavior explicit: uncached access history needs a connected
  agent; cached records/events show their timestamp and source node.
- [ ] Apply backpressure and message/queue limits so logging cannot delay
  configuration application or consume unbounded memory on small nodes.

## Highlight denials and update access lists

- [ ] Clearly highlight denied requests with text/icons as well as color. Show
  the client address, host, time, rule/list when known, and denial reason.
- [ ] Distinguish a confirmed PosternProxy/Caddy access-list denial from an
  upstream HTTP 403 and a rate-limit 429. A status code alone is not proof that an
  access-list rule blocked the request; label unknown causes honestly.
- [ ] Add an action on a denied event to add its verified client IP to an
  existing access list. Let the administrator explicitly choose an allow or deny
  rule and the destination list, then review and submit the change.
- [ ] Show existing/conflicting rules, suppress duplicates, and validate IP/CIDR
  input. Explain that adding an allow rule does not override an existing explicit
  deny under the current rule precedence.
- [ ] Show which hosts/servers use the selected list before saving. Apply the
  update through the existing access-list service and configuration-revision
  workflow, with truthful queued/applied/failed status.
- [ ] Require admin authorization and audit the actor, source event/node,
  selected list, rule and safe before/after state. Reuse existing API, CSRF and
  validation protections; log records cannot initiate changes automatically.
- [ ] Use a trusted client-IP resolution policy. Never use an unchecked forwarded
  header or the reverse proxy's address as the visitor address. Enable the action
  only when a complete, authoritative IP is available; never infer an address or
  broad CIDR from a masked value.

## Defaults and safeguards

- [ ] Start with opt-in per-host collection and admin-only viewing/actions.
- [ ] Sanitize before writing or forwarding: omit bodies, headers and query
  strings; redact sensitive path segments and secrets in runtime/TLS messages.
- [ ] Provide client-IP masking controls. Decide explicitly how full client IPs
  are retained for denied-event actions when ordinary access logs are masked;
  keep that choice visible to the operator.
- [ ] Start with configurable retention of seven days or 100 MB per node,
  whichever limit is reached first. Enforce an aggregate node quota, not just a
  separate cap for each host. Preserve essential TLS events within bounded
  controller retention.
- [ ] Use a constrained structured log API with host/server IDs, validated
  filters and bounded responses. Keep filesystem paths and arbitrary journal
  access outside the API contract. Audit exports and privileged deletion.
- [ ] Limit stream/TLS-passthrough diagnostics to available connection metadata;
  do not claim HTTP methods, paths or statuses without HTTP termination.
- [ ] Preserve the outbound-only agent model, Caddy data plane, single controller
  binary, existing security controls and dual-licensing model.

## Acceptance before release

- [ ] Verify local and two-agent log collection, filtering, cursor pagination,
  rotation, reconnect, offline behavior and live-tail cancellation.
- [ ] Test forged client-IP headers, masked addresses, ACL versus upstream denials,
  duplicate/conflicting rules, unauthorized actions and secret redaction.
- [ ] Demonstrate adding a rule from a denied event, its attributed audit record,
  correlated configuration application and the resulting traffic behavior.
- [ ] Exercise quotas/backpressure on a small-memory controller and agent; log
  volume must not interrupt proxy traffic or configuration updates.
- [ ] Add API/UI regression coverage and update user docs and Unreleased when
  implementation ships. Run the required Go/frontend and live Caddy checks.
