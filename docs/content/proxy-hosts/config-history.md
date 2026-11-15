---
title: Config History
weight: 9
---

# Config History

PosternProxy records proxy host configuration snapshots on creation, before
edits, and before deletion. You can use the history to roll back an existing
proxy host.

## Viewing history

Open the **⋮** menu on a proxy host row and click **History**. The dialog
shows the last 100 snapshots in reverse chronological order.

Each entry shows:
- Timestamp of the change
- The user who made it
- The type of change

<!-- TODO: Screenshot — Config History drawer with several entries -->

## Rolling back

Click **Rollback** on a snapshot, then **Confirm Rollback**. PosternProxy will:

1. Save the current state as a new snapshot (so you can undo the rollback)
2. Write the restored configuration to the database
3. Push the updated Caddy config immediately

Rollback saves the restored configuration as desired state. If Caddy rejects
the apply, the saved state remains available for an administrator to retry. An
offline agent applies the desired state after reconnecting. See
[Configuration application status]({{< relref "/multi-server/configuration-status" >}}).

## What is captured

Each snapshot captures the full proxy host record, including:

- Domain names
- Primary upstream (scheme, host, port)
- All SSL / HSTS settings
- Access list assignment
- Load balancing policy and extra upstreams
- Rate limit settings
- Health check settings
- TLS passthrough flag
- Custom error pages
- Custom headers
- Custom locations
- Advanced config

Rollback leaves the host's current enabled or disabled state in place.

## Retention

PosternProxy keeps the 100 most recent snapshots per proxy host. Older entries are pruned automatically when new snapshots are added.

## Notes

- Snapshot contents are encrypted at rest. The history list shows metadata,
  and the controller decrypts snapshots internally during rollback.
- History is per-resource: proxy hosts have their own history, independent of redirection hosts, stream hosts, etc.
- History entries are not deleted when a proxy host is deleted. If you recreate a proxy host with the same ID, old history will not be associated with it.
