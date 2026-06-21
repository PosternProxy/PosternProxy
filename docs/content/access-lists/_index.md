---
title: Access Lists
weight: 70
---

# Access Lists

Access lists control who can reach a proxy host using IP-based
allowlisting / blocklisting.

## Creating an access list

1. Click **Access Lists** in the sidebar
2. Click **+ Add Access List**
3. Give the list a **Name**
4. Add one or more **Client Rules**
5. Click **Create**

## Client rules (IP rules)

Each rule has three fields:

| Field | Description |
|---|---|
| **Address** | IPv4/IPv6 address or CIDR range (e.g. `10.0.0.0/8`, `192.168.1.50`) |
| **Directive** | `allow` or `deny` |
| **Order** | Sort order for the rule |

Explicit `deny` rules are enforced first (those IPs are always blocked). If any
`allow` rules are present, they form an allowlist: any client not in the allowlist
receives `403 Forbidden`.

### Example: allow internal network only

| Address | Directive |
|---|---|
| `10.0.0.0/8` | allow |
| `172.16.0.0/12` | allow |
| `192.168.0.0/16` | allow |

With these `allow` rules, any address outside the listed ranges is denied.

## Assigning an access list to a proxy host

1. Open the Add/Edit Proxy Host modal
2. Go to the **Security** tab
3. Select the access list from the **Access List** dropdown

A proxy host can only be assigned one access list at a time. Access lists can be
reused across multiple proxy hosts.

## How it works in Caddy

Access list IP rules translate to Caddy `remote_ip` matchers. Denied requests
receive a `403 Forbidden` response before reaching the upstream.

## Notes

- IP matching uses the client's direct connection IP. If PosternProxy is behind
  another proxy, configure that proxy to set `X-Forwarded-For` and ensure Caddy is
  configured to trust it.
- Deleting an access list assigned to one or more proxy hosts will succeed — the
  proxy hosts retain the association but Caddy no longer enforces the list until a
  new one is assigned.

## Not yet available

HTTP Basic Authentication (username/password protection) on access lists is
planned but not yet implemented; access lists currently enforce IP rules only.
