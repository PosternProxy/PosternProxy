---
title: Audit Log
weight: 130
---

# Audit Log

The Audit Log records every configuration change, login attempt, and administrative action. It is available to administrators at **Audit Log** in the sidebar.

## What is logged

| Event type | Examples |
|---|---|
| **Auth events** | Login success, login failure, logout, token creation/deletion |
| **Proxy host mutations** | Create, update, delete, enable, disable |
| **Redirection host mutations** | Create, update, delete, enable, disable |
| **Stream host mutations** | Create, update, delete, enable, disable |
| **Dead host mutations** | Create, update, delete, enable, disable |
| **Certificate events** | Create, upload, renew, delete |
| **Access list mutations** | Create, update, delete |
| **Port forward mutations** | Create, update, delete, enable, disable |
| **User mutations** | Create, update, delete, password change |
| **Settings changes** | Any settings key updated |
| **Backup/restore** | Backup downloaded, restore completed |

## Log entry fields

Each audit log entry contains:

| Field | Description |
|---|---|
| **Timestamp** | When the action occurred (UTC) |
| **User** | Name and email of the user who performed the action |
| **Action** | The operation (`create`, `update`, `delete`, `login`, etc.) |
| **Resource Type** | What kind of resource was affected (`proxy_host`, `user`, etc.) |
| **Resource ID** | ID of the specific resource |
| **Details** | JSON payload with the relevant change data |
| **IP Address** | Client IP of the request |

## Filtering

Use the filters at the top of the Audit Log page to narrow results:

- **Resource type** — show only events for a specific resource type
- **Action** — filter by operation type
- **User** — filter by user
- **Date range** — start and end date
- **Server** (in multi-server mode) — filter by server

<!-- TODO: Screenshot — Audit Log page with filters applied -->

## Pagination

The Audit Log is paginated at 50 entries per page. Use the page controls to navigate. Results are always shown newest-first.

## API access

```bash
# Get the last 50 audit log entries
curl -H "Authorization: Bearer <token>" \
     "http://your-server:81/api/audit-log?page=1&limit=50"

# Filter by resource type
curl -H "Authorization: Bearer <token>" \
     "http://your-server:81/api/audit-log?resource_type=proxy_host"
```

## Retention

Audit log entries are retained indefinitely. There is no automatic purge. If the log grows very large, you can export and truncate it manually:

```bash
# Export (via API)
curl -H "Authorization: Bearer <token>" \
     "http://your-server:81/api/audit-log?limit=0" > audit-export.json

# Truncate (via SQLite)
sqlite3 /var/lib/posternproxy/posternproxy.db \
    "DELETE FROM audit_log WHERE created_at < datetime('now', '-1 year');"
```

## fail2ban integration

The install script configures fail2ban to watch the audit log for failed login attempts. After 5 failures from the same IP within 10 minutes, that IP is banned for 1 hour. The ban is applied via iptables.

To view current bans:

```bash
fail2ban-client status posternproxy
```
