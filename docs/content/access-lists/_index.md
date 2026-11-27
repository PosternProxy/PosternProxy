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
2. Select the access list from the **Access List** dropdown on the basic host form, or on the **SSL** tab

A proxy host can only be assigned one access list at a time. Access lists can be
reused across multiple proxy hosts.

## How it works in Caddy

Access list IP rules translate to Caddy `client_ip` matchers. Denied requests
receive a `403 Forbidden` response before reaching the upstream.

## Notes

- IP matching uses the verified visitor address. With no proxy trust policy,
  this is the immediate connection peer. Behind a proxy, configure the explicit
  per-server `POSTERNPROXY_CADDY_TRUSTED_PROXIES` policy described in
  [Security Hardening]({{< relref "/administration/security-hardening" >}}).
  Management `POSTERNPROXY_TRUSTED_PROXIES` remains separate.
- Deleting an access list assigned to one or more proxy hosts will succeed — the
  database clears the host association (`ON DELETE SET NULL`) and reconciliation
  removes the restriction. Review affected hosts before deleting a list.

## Not yet available

HTTP Basic Authentication (username/password protection) on access lists is
planned but not yet implemented; access lists currently enforce IP rules only.
