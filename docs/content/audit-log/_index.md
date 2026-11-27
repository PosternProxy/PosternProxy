---
title: Audit Log
weight: 130
---

# Audit Log

The Audit Log displays recorded management actions. The UI and API are restricted to administrators.

## Recorded events

Implemented audit calls cover host creation, edits, deletion and enable/disable;
access-list and custom-certificate mutations; user changes and password resets;
server changes and sync requests; successful/failed login; and join-token issue
and revocation. Host changes include a safe subset of before/after fields.
That subset includes `ntlm_enabled`.

Settings update outcomes, backup export/restore, API-token creation/revocation,
logout, provisioning outcomes and enrollment rotation also create attributed,
redacted audit entries. Credential values and backup passphrases are excluded.
Most audit writes remain best-effort; failed privileged requests are not recorded
uniformly, and this is not a complete compliance trail.

## Fields and filters

Records contain a timestamp, actor user ID when known, action, resource type/ID,
JSON details, and client IP. The UI shows time, action, resource, details and IP;
it does not resolve every actor ID into a name/email column.

The current UI and API support **resource type** filtering and pagination.
There are no action, user, date-range, or server filters exposed by this handler.
The UI uses 50 entries per page, newest first.

```bash
curl -H "Authorization: Bearer <read-token>" \
  'http://your-server:81/api/audit-log?page=1&limit=50'

curl -H "Authorization: Bearer <read-token>" \
  'http://your-server:81/api/audit-log?resource_type=proxy_host&page=1&limit=50'
```

## Retention and export

No automatic purge is implemented. To export through the API, iterate through
pages using the returned `total`, `page`, and `limit` fields. `limit=0` selects
the default page size; it does not export the entire log. Preserve a consistent,
access-restricted database backup before any manual retention work.

## fail2ban integration

New controller installations configure fail2ban to read structured failed-login
**request logs** in `/var/log/posternproxy/posternproxy.log`, not SQLite audit
records. The default jail bans an IP on the management port after 5 matching
failures in 60 seconds for 1 hour. Existing filter/jail customizations are kept.

```bash
sudo fail2ban-client status posternproxy
```

The installed service enables JSON logging for this filter. Changing log format
requires updating the filter too. See
[Security Hardening]({{< relref "/administration/security-hardening" >}}).
