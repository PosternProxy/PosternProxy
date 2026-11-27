---
title: Nginx Config Import
weight: 110
---

# Nginx Config Import

Administrators can upload an Nginx configuration file and preview the proxy and
redirection hosts detected by the parser. Open **Import Nginx** in the sidebar.

## Importing

1. Click **Choose File**, or drag a configuration file onto the upload area.
2. Review the detected proxy hosts, redirections, and parser warnings.
3. To change the import, edit the source file and upload it again. The preview
   does not provide inline editing or per-host selection.
4. Confirm the import. The server reparses the original file and imports the
   detected hosts.

The preview does not write to the database or Caddy. Confirmation creates hosts
using their normal enabled defaults and can immediately change routing. Import
only reviewed configurations; use an isolated instance to test a migration.

## Scope

The parser handles common `server_name`, `proxy_pass`, SSL-listener, and
`return` redirect patterns. It is a best-effort converter, not an Nginx runtime
or a complete configuration translator. Custom certificate and key files must be
uploaded separately; Nginx filesystem paths are not transferred.

Locations, upstream groups, variables, cache directives, scripting, and complex
nesting require manual review. Parsed location information is not copied into
created proxy hosts by the current import service. Imported proxy hosts leave
[NTLM authentication passthrough]({{< relref "/proxy-hosts" >}}#ntlm-authentication-passthrough)
off. Parser warnings do not prove DNS resolution, upstream reachability, or
successful Caddy application.

## After import

Review the created hosts, SSL choices, server assignments, and any errors in the
result. Import counts hosts saved to the database even if configuration
application fails. Check
[Configuration application status]({{< relref "/multi-server/configuration-status" >}})
and actual traffic before switching production DNS.
