---
title: Backup & Restore
weight: 100
---

# Backup & Restore

PosternProxy exports selected configuration tables as JSON. The current beta
format is not a portable disaster-recovery backup: it omits server records and
certificate/key file contents. Hosts referencing remote servers can fail to
restore into a fresh installation. Preserve a consistent offline copy of the
database, data, encryption secret, and Caddy state before changing an installation;
see [Beta upgrades and recovery]({{< relref "/administration/upgrading" >}}).

## What is exported

The backup JSON includes:

- All proxy hosts (including all settings, custom headers, error pages, etc.)
- All redirection hosts
- All stream hosts
- All dead hosts
- All certificates (metadata only — private keys are **not** included; see note below)
- All access lists and IP client rules
- Settings

Users, API/join tokens, server records, SSH credentials, and certificate/key bytes
are not included. Caddy L4 forwarding rules are represented by stream hosts.

## Creating a backup

1. Go to **Backup** in the sidebar
2. Click **Export Backup**
3. Save the `.json` file securely; export uses your authenticated admin session

<!-- TODO: Screenshot — Settings > Backup & Restore section with Download and Restore buttons -->

The download is served over the management API:

```bash
curl -H "Authorization: Bearer <token>" \
     http://your-server:81/api/backup/export \
     -o posternproxy-backup.json
```

## Restoring a backup

{{< hint danger >}}
**Restore replaces the exported configuration tables.** Current hosts,
certificate metadata, access lists, and settings are cleared and replaced. Users
and server records are preserved. Keep an offline recovery copy before restoring.
{{< /hint >}}

1. Go to **Backup** in the sidebar
2. Select the backup `.json` file under **Restore from Backup**
3. Click **Import Backup**
4. Confirm the operation
5. PosternProxy will restore the database and re-sync Caddy

Restore queues affected local and remote configuration for reconciliation. The
response separates restored data from application using `synced` and per-server
configuration states. Offline agents remain queued; apply rejection is reported
without discarding restored desired state. See
[Configuration application status]({{< relref "/multi-server/configuration-status" >}}).
Verify actual proxy traffic and server state after restoring.

## Certificate private keys

Private keys for **custom** certificates are stored on disk at `POSTERNPROXY_CERT_DIR`, not in the database. The backup JSON does not include private key material.

Copying this directory alone does not make a restore portable: stored file paths,
server references, service permissions, and encryption secrets also matter. Use
the offline recovery procedure above rather than assuming JSON plus certificate
files can migrate an installation safely.

Certificates managed automatically by Caddy (Let's Encrypt HTTP-01) are stored in Caddy's own certificate store and must be re-obtained on the new server.

## Automated backups

For regular automated backups, use the API endpoint with a cron job:

```bash
# /etc/cron.daily/posternproxy-backup
#!/bin/bash
curl -s -H "Authorization: Bearer $POSTERNPROXY_TOKEN" \
     http://localhost:81/api/backup/export \
     -o /var/backups/posternproxy/backup-$(date +%F).json
```

Store JSON exports securely outside the server. For disaster recovery, also
preserve the offline copies listed in
[Beta upgrades and recovery]({{< relref "/administration/upgrading" >}}).
