---
title: Beta upgrades and recovery
weight: 2
---

# Beta upgrades and recovery

PosternProxy remains in beta. A version label alone does not identify a database
schema: earlier beta development changed the consolidated initial migration.
Do not replace an existing binary and assume its database will be upgraded safely.

## Schema baseline

The development baseline is the schema frozen on October 24, 2026 in
`internal/database/testdata/beta-exit-2026-10-24.sql`. The initial migration is
frozen to that schema. Subsequent schema changes must use new, ordered migrations
and retain the baseline upgrade regression test. Repeated startup must preserve
existing data and applied migrations.

| Source | Qualification status |
| --- | --- |
| Fresh database using the frozen baseline | Covered by development database tests |
| Existing database matching that baseline, with `001_initial.sql` recorded | Covered by the baseline upgrade/repeated-startup regression |
| Published `v0.1.2-beta`, earlier beta databases, or manually modified schemas | Not yet qualified for in-place upgrade |

Baseline tests are a development contract, not evidence that every beta database
matches it. A release candidate must name and test every supported source schema
before claiming a supported upgrade path. Do not remove a migration record or
rerun initial SQL to force an older database to fit.

## Before changing an existing installation

Stop the controller before taking a consistent offline copy of its database and
data directory. Preserve its secret key (or configured JWT secret), uploaded
certificate/key files, environment configuration, service definitions, current
controller/agent/Caddy binaries, and Caddy storage/configuration. Keep these
sensitive copies access restricted. The encryption secret is required to decrypt
stored SSH credentials. Capture the corresponding remote agents' configuration
and Caddy state as well.

Keep the original installation intact and test the proposed upgrade on an
isolated copy. Prevent the copy from connecting to production agents or applying
configuration to production Caddy. Verify accounts, relationships, custom TLS,
configuration application, restart, and recovery before considering deployment.

Version 2 [encrypted recovery archives]({{< relref "/backup-restore" >}})
include custom certificate bytes and explicit remote-server mapping. They exclude
accounts, credentials, runtime settings and Caddy's ACME state. Version 1 JSON
remains a partial configuration export. Keep the offline copy for whole-system
recovery and unsupported-schema rollback.

## Upgrade the controller and agents together

Once you have verified the upgrade on an isolated copy, stop existing agents,
upgrade the controller, and replace and restart each agent with the matching
build. The controller requires agent credentials in the `Authorization` header
and rejects older agents that send their token in the WebSocket query string.
Verify each agent reconnects and reports applied configuration in
[Configuration application status]({{< relref "/multi-server/configuration-status" >}}).

## Rollback

Do not run an older binary against a database changed by newer migrations.
Rollback requires the matching pre-upgrade database/data copy, encryption secret,
binaries, and service configuration. Restore them while services are stopped,
then reconcile and verify local and remote Caddy state. A database rollback by
itself does not undo configuration already applied to an agent.
