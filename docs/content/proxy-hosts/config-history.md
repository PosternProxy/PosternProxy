---
title: Config History
weight: 9
---

# Config History

PosternProxy records a snapshot of every proxy host's configuration before each change. You can browse the history and roll back to any previous state.

## Viewing history

Open the **⋮** menu on any proxy host row in the table and click **History**. A drawer slides open showing the last 100 snapshots in reverse chronological order.

Each entry shows:
- Timestamp of the change
- The user who made it
- A brief diff of what changed

<!-- TODO: Screenshot — Config History drawer with several entries -->

## Rolling back

Click **Restore** on any snapshot to revert the proxy host to that state. PosternProxy will:

1. Save the current state as a new snapshot (so you can undo the rollback)
2. Write the restored configuration to the database
3. Push the updated Caddy config immediately

The rollback is atomic — if the Caddy push fails, the database is not changed.

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

Audit log entries and enabled/disabled state are **not** included in snapshots (enable/disable has its own event log).

## Retention

PosternProxy keeps the 100 most recent snapshots per proxy host. Older entries are pruned automatically when new snapshots are added.

## Notes

- Config history is stored in the `config_history` table in SQLite.
- History is per-resource: proxy hosts have their own history, independent of redirection hosts, stream hosts, etc.
- History entries are not deleted when a proxy host is deleted. If you recreate a proxy host with the same ID, old history will not be associated with it.
