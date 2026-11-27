---
title: Configuration application status
weight: 5
---

# Configuration application status

Saving a host updates desired configuration in the controller database. Applying
that configuration to Caddy is a separate operation. A successful save does not
by itself prove that traffic changed.

| Status | Meaning |
| --- | --- |
| Queued | Desired changes are saved and waiting for dispatch or an agent connection |
| Applying | The controller is applying locally or waiting for a correlated agent result |
| Applied | Caddy accepted the latest desired revision |
| Failed | Configuration application failed or the agent acknowledgment timed out |

The shared configuration notice displays outstanding changes on management
pages. Admins can use **Retry sync** there or request synchronization from the
Servers page. A sync request being accepted is distinct from its completion.
Applied status confirms Caddy accepted configuration; it does not prove upstream
health, certificate validity, or external firewall reachability.

## Recovery and retry

The controller persists a desired revision with host changes, including deletion
and moves between servers. Moves reconcile both the previous and new server.
Access-list changes queue servers referencing the list. Certificate changes
currently queue all servers; limiting certificate distribution is separate work.

Local rejection leaves the desired change saved and reports an application error.
Remote work stays queued when the agent is offline or its send queue is full.
An agent must report success with the matching command identity and revision
after writing required certificate files and successfully applying Caddy config.
Stale, duplicate, uncorrelated, or replaced-connection results do not complete
current work. A missing acknowledgment fails after roughly 30 seconds, checked
by the worker. Failed attempts have a 30-second retry delay; the worker polls
every five seconds. Explicit admin retries can run sooner.

Every controller startup and authenticated agent connection reconciles from
database state. Previously applied configuration is also rechecked by periodic
full synchronization after one minute, so a Caddy restart does not require a
new user edit. Transient failures can extend convergence time. Configuration
application is serialized per server; the latest desired state is coalesced
while an earlier revision is in flight.

One NTLM host that the route builder rejects blocks the new configuration for
every host on that server. Status then keeps the generic application failure,
the desired row stays saved, and the previous Caddy configuration keeps serving.
PosternProxy does not fall back to ordinary HTTP. Correct or disable the host,
then retry sync. See
[NTLM passthrough and Caddy]({{< relref "/administration/upgrading" >}}#ntlm-passthrough-and-caddy).

When Caddy reports `unknown module: http.reverse_proxy.transport.http_ntlm`,
status stores `Caddy rejected the configuration because the NTLM transport module is missing. Upgrade Caddy on this server and retry sync.`
Other Caddy rejections keep the generic sentence. The create or update HTTP 502
body does not include that module sentence.

## API behavior

- `GET /api/configuration` returns per-server desired/applied revisions, status,
  a sanitized error message, and the last status update time. Authentication
  and read scope are required.
- Host create/update success means the resource was saved. Read configuration
  status separately to distinguish queued remote work from an applied revision.
- A rejected synchronous apply returns HTTP 502 with `saved: true` and the
  affected `server_id`. The saved resource or deletion remains desired state;
  retry synchronization instead of recreating the resource.
- `POST /api/servers/{id}/resync` requires admin authority and write scope. It
  returns the configuration state: HTTP 200 when applied, HTTP 202 while queued
  or applying. Rejected local application returns HTTP 502.
- Restore reports `synced` only when all reported configuration states are
  applied at their desired revisions. The response also includes those states.
  The current export format still has the portability limits described in
  [Backup & Restore]({{< relref "/backup-restore" >}}).
- Nginx import counts saved hosts even when their apply fails and includes
  application failures in its error list. A rollback restores desired state and
  uses the same application/error contract.

Host audit entries contain a safe before/after field set. Rollback history is
encrypted using the controller's existing AES-256-GCM helpers and stable
encryption key. Startup protects existing plaintext snapshots in resumable
batches. History list responses expose metadata without snapshot contents;
rollback decrypts snapshots internally. Preserve the controller encryption
secret with offline recovery copies.

## Controller and agent upgrade

This change requires coordinated controller and agent upgrades. Stop old agents,
upgrade the controller, then replace/restart every agent with the matching build.
The controller requires agent credentials in the Authorization header; old
query-token agents cannot authenticate. Agents must echo command identities and
revisions to establish applied status. Uncorrelated results are not accepted as
success. Rotate credentials that may have been written to older access logs.

New schema changes use forward migrations after the frozen development baseline.
Historical beta-schema upgrade qualification is still tracked separately; see
[Beta upgrades and recovery]({{< relref "/administration/upgrading" >}}).
