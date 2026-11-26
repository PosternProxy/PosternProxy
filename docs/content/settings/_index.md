---
title: Settings
weight: 160
---

# Settings

Administrators can view the active **Caddy Admin API URL** and **Certificate
Storage Path** in Settings. These values are read-only and reflect the actual
service environment. Attempts to change them through the API return an explicit
error rather than saving values that do not affect the running services.

Change `POSTERNPROXY_CADDY_API` or `POSTERNPROXY_CERT_DIR` in
`/etc/posternproxy/config.env`, then restart the controller. Moving certificate
storage requires migrating the source files and database path references first;
changing the environment alone does not move existing material.

Set `POSTERNPROXY_CONTROLLER_URL` in the environment for the base URL agents use.
See [Installation]({{< relref "/getting-started/installation" >}}#environment-variables)
for the environment reference.

Use [Users]({{< relref "/users" >}}) for accounts,
[Backup]({{< relref "/backup-restore" >}}) for recovery, and **Re-sync** on Servers
for configuration reconciliation. See
[Configuration application status]({{< relref "/multi-server/configuration-status" >}}).
