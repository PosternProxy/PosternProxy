---
title: Settings
weight: 160
---

# Settings

The Settings page (admin only) provides application-wide configuration options and maintenance operations.

## Accessing settings

Click **Settings** in the sidebar. The page is divided into sections:

## General settings

| Setting | Default | Description |
|---|---|---|
| **Caddy API URL** | `http://localhost:2019` | URL for the local Caddy admin API |
| **Certificate storage path** | `/var/lib/posternproxy/certs` | Where custom certificate files are stored |
| **Controller URL** | `http://<host>:81` | URL agents use to connect (must be reachable from remote servers) |

Changes to these settings take effect immediately without a restart.

<!-- TODO: Screenshot — Settings page General section -->

## Backup & Restore

See [Backup & Restore]({{< relref "/backup-restore" >}}) for full documentation.

## Import from Nginx

See [Nginx Config Import]({{< relref "/nginx-import" >}}) for full documentation.

## Changing the admin email

1. Go to **Settings → General**
2. Update the **Admin Email** field
3. Click **Save**

Or update the user account directly on the [Users]({{< relref "/users" >}}) page.

## Environment variables

Settings can also be set via environment variables in `/etc/posternproxy/config.env`. Environment variables take precedence over database-stored settings. See [Installation → Environment variables]({{< relref "/getting-started/installation" >}}#environment-variables) for the full list.

## Caddy sync

PosternProxy maintains Caddy's config in sync with the database automatically. If you suspect the configs have diverged:

1. Go to **Settings**
2. Click **Force Caddy Sync**
3. PosternProxy rebuilds the entire Caddy config from the database and pushes it via `POST /load` to the Caddy admin API

This is a safe operation — `POST /load` is atomic in Caddy.

<!-- TODO: Screenshot — Settings page showing Force Caddy Sync button -->

## Danger zone

The **Danger Zone** section at the bottom of the Settings page contains destructive operations:

| Action | Effect |
|---|---|
| **Restore Backup** | Replace the entire database with a backup file |
| **Reset to defaults** | Delete all resources and reset to a clean install state |

Both operations require password confirmation before executing.
