---
title: Nginx Config Import
weight: 110
---

# Nginx Config Import

PosternProxy can parse existing Nginx configuration files and convert them into proxy hosts. This makes migrating from Nginx Proxy Manager or a custom Nginx setup significantly faster.

## How to import

1. Go to **Settings → Import from Nginx**
2. Paste or upload your Nginx configuration (one or more `server {}` blocks)
3. Click **Parse** — PosternProxy shows a preview of the hosts it detected
4. Review the preview. Edit or deselect entries you do not want to import.
5. Click **Import Selected**

<!-- TODO: Screenshot — Import page with parsed Nginx config preview showing three server blocks -->

## What is parsed

| Nginx directive | PosternProxy field |
|---|---|
| `server_name` | Domain names |
| `proxy_pass` | Forward scheme, host, and port |
| `listen 443 ssl` | SSL forced = true |
| `ssl_certificate` / `ssl_certificate_key` | Custom certificate (uploaded separately) |
| `proxy_set_header Upgrade` | WebSocket support = true |

## What is not imported

The following Nginx directives have no equivalent in PosternProxy and are silently ignored:

- `location` blocks (basic ones are previewed but multi-level `location` nesting is not supported)
- `proxy_cache` directives
- `lua_*` directives
- `geo {}` and `map {}` blocks
- Custom `upstream {}` groups (the primary upstream address is extracted, but load balancing config is not)

## Preview and dry-run

The import is always shown as a preview before any data is written. You can:

- Deselect individual server blocks
- Edit the detected upstream address before importing
- See validation warnings (e.g. a `proxy_pass` with a hostname that cannot be resolved)

No changes are made to Caddy or the database until you click **Import Selected**.

## After import

Imported hosts are created as **disabled** by default. Review each one in the Proxy Hosts list and enable when ready. This prevents accidentally routing traffic before you have verified the configuration.

## Notes

- The parser handles Nginx configuration syntax but does not execute Nginx's variable expansion (`$host`, `$scheme`, etc.). Literal values are imported; variable references are left as-is and flagged as warnings.
- Comments are preserved in the preview for context but are not stored in PosternProxy.
- If you have a large number of server blocks, consider splitting the config into smaller files and importing in batches.
