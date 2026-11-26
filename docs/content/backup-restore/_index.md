---
title: Backup & Restore
weight: 100
---

# Backup & Restore

Administrators can export a passphrase-encrypted recovery archive from **Backup**.
Version 2 archives include proxy, redirection, stream and dead hosts, access
lists, custom certificate/key bytes and credential-free server descriptions.
They can restore onto a controller with a different encryption key and storage
root. Certificate sources are encrypted again using the destination's key.

Users, sessions, API/join/agent tokens, SSH credentials, SSH host keys, runtime
settings and Caddy's automatic certificate storage are not transferred. Keep a
consistent offline recovery copy as described in
[Beta upgrades and recovery]({{< relref "/administration/upgrading" >}}).

## Create an encrypted archive

1. Open **Backup** as an administrator.
2. Enter and confirm a passphrase of at least 12 characters.
3. Click **Export Backup** and save the JSON archive securely.
4. Keep the passphrase separately. There is no passphrase recovery.

The format uses scrypt with a random per-archive salt and authenticated AES-GCM
encryption. Export fails visibly if required custom certificate material cannot
be read. Archives are limited to 50 MiB, with a 32 MiB decrypted payload limit.

API clients use `POST /api/backup/export` with JSON `{"passphrase":"..."}` through
the authenticated admin API. Keep passphrases in protected request files, not
shell arguments or logs; use the normal CSRF header for cookie authentication.

## Restore

Restore replaces managed hosts, certificate metadata and access lists. It
preserves destination users, sessions, server credentials and runtime settings.
Keep an offline copy of the destination first.

1. Select the archive and enter its passphrase.
2. For each remote server, provide a source-to-destination numeric ID mapping,
   such as `{"2":52}`, or explicitly select **Create unmapped remote servers
   without enrollment credentials**.
3. Confirm the replacement.
4. Enroll any new remote records using **Servers -> Rotate enrollment** and the
   [agent installer]({{< relref "/multi-server/provisioning" >}}#manual-enrollment).
5. Check configuration status and actual proxy/TLS traffic on every server.

Local source server 1 always maps to destination server 1. Existing mapped
servers keep their own credentials. New remote records are pending, use manual
deployment, and have no SSH or agent credentials. There is no implicit hostname
mapping, credential reuse or remote installation during restore.

API clients use `POST /api/backup/restore` with `archive` (the archive JSON object),
`passphrase`, `server_mapping` and `create_unenrolled`. A successful data commit
returns `committed: true`, the restored count, `synced`, and per-server
configuration states. Configuration rejection or offline agents do not undo the
committed desired state. `synced: false` requires checking configuration status
and retrying application, rather than blindly repeating the data restore.

Wrong passphrases, unsupported formats, invalid certificate material and invalid
server mappings fail before destination configuration is replaced. All SQL
changes are transactional; staged certificate sources are immutable, so a failed
replacement cannot overwrite the previously active pair.

## Legacy partial JSON compatibility

`GET /api/backup/export` still returns the version 1 unencrypted database export;
`POST /api/backup/import` and the UI accept that format. It includes settings and
certificate file paths, but omits server registrations and certificate bytes.
It needs existing matching server IDs and original certificate storage. It is
not a portable recovery archive. Unknown versions, tables and columns are
rejected. Treat legacy exports as sensitive configuration.

Automatic certificates remain in Caddy storage. Preserve that storage in offline
backups or allow Caddy to obtain them again after recovery. The encrypted archive
does not preserve ACME account state or eliminate certificate-authority limits.
