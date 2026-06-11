---
title: Backup & Restore
weight: 100
---

# Backup & Restore

PosternProxy can export the entire configuration as a single JSON file and restore it on any PosternProxy instance. This is useful for:

- Migrating to a new server
- Creating a point-in-time backup before a major change
- Seeding a new environment from a known-good configuration

## What is exported

The backup JSON includes:

- All proxy hosts (including all settings, custom headers, error pages, etc.)
- All redirection hosts
- All stream hosts
- All dead hosts
- All certificates (metadata only — private keys are **not** included; see note below)
- All access lists (including client rules and hashed passwords)
- All port forwards
- All users (hashed passwords)
- Settings

## Creating a backup

1. Go to **Settings → Backup & Restore**
2. Click **Download Backup**
3. You will be prompted to authenticate with your password before the download starts
4. Save the `.json` file securely

<!-- TODO: Screenshot — Settings > Backup & Restore section with Download and Restore buttons -->

The download is served over the management API:

```bash
curl -H "Authorization: Bearer <token>" \
     http://your-server:81/api/settings/backup \
     -o posternproxy-backup.json
```

## Restoring a backup

{{< hint danger >}}
**Restore overwrites all existing data.** The restore operation replaces the entire database. All current proxy hosts, certificates, and users will be replaced by the backup's contents. There is no partial restore.
{{< /hint >}}

1. Go to **Settings → Backup & Restore**
2. Click **Restore Backup**
3. Select the backup `.json` file
4. Confirm the operation
5. PosternProxy will restore the database and re-sync Caddy

After restore, Caddy is immediately updated to reflect the restored configuration.

## Certificate private keys

Private keys for **custom** certificates are stored on disk at `POSTERNPROXY_CERT_DIR`, not in the database. The backup JSON does not include private key material.

When migrating to a new server:
1. Export the backup
2. Copy the certificate directory (`/var/lib/posternproxy/certs/`) to the new server
3. Restore the backup on the new server

Certificates managed automatically by Caddy (Let's Encrypt HTTP-01) are stored in Caddy's own certificate store and must be re-obtained on the new server.

## Automated backups

For regular automated backups, use the API endpoint with a cron job:

```bash
# /etc/cron.daily/posternproxy-backup
#!/bin/bash
curl -s -H "Authorization: Bearer $POSTERNPROXY_TOKEN" \
     http://localhost:81/api/settings/backup \
     -o /var/backups/posternproxy/backup-$(date +%F).json
```

Store the backup outside the server (e.g. object storage, NFS, or a secondary server) for disaster recovery.
