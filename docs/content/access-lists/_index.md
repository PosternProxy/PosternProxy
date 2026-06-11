---
title: Access Lists
weight: 70
---

# Access Lists

Access lists control who can reach a proxy host. They support IP-based allowlisting/blocklisting and HTTP Basic Authentication, either independently or in combination.

## Creating an access list

1. Click **Access Lists** in the sidebar
2. Click **+ Add Access List**
3. Give the list a **Name**
4. Choose **Satisfy** behaviour:
   - **Any** — client must pass *at least one* of the configured checks (IP rule *or* auth)
   - **All** — client must pass *all* checks (IP rule *and* auth)
5. Add **Client Rules** and/or **Authentication** entries
6. Click **Create**

<!-- TODO: Screenshot — Add Access List modal with client IP rules and authentication entries -->

## Client rules (IP rules)

Each rule has three fields:

| Field | Description |
|---|---|
| **Address** | IPv4/IPv6 address or CIDR range (e.g. `10.0.0.0/8`, `192.168.1.50`) |
| **Directive** | `allow` or `deny` |
| **Order** | Rules are evaluated top-to-bottom; first match wins |

### Example: allow internal network, deny everything else

| Order | Address | Directive |
|---|---|---|
| 1 | `10.0.0.0/8` | allow |
| 2 | `172.16.0.0/12` | allow |
| 3 | `192.168.0.0/16` | allow |
| 4 | `0.0.0.0/0` | deny |

## Authentication (HTTP Basic Auth)

Add username/password pairs. PosternProxy hashes passwords with bcrypt before storing them.

Clients that match an `allow` IP rule but have no credentials will still need to authenticate if authentication entries are present and the **Satisfy** mode is **All**.

<!-- TODO: Screenshot — Access List form showing auth section with two username rows -->

## Assigning an access list to a proxy host

1. Open the Add/Edit Proxy Host modal
2. Go to the **Security** tab
3. Select the access list from the **Access List** dropdown

A proxy host can only be assigned one access list at a time. Access lists can be reused across multiple proxy hosts.

## Pass Auth to Upstream

When **Pass Auth to Upstream** is enabled, PosternProxy forwards the authenticated username in the `X-Authenticated-User` request header to your upstream service.

## How it works in Caddy

Access list IP rules translate to Caddy `remote_ip` matchers. Denied requests receive a `403 Forbidden` response before reaching the upstream. Basic Auth translates to Caddy's `authentication` handler using the `http_basic` provider.

## Notes

- IP matching uses the client's direct connection IP. If PosternProxy is behind another proxy, configure that proxy to set `X-Forwarded-For` and ensure Caddy is configured to trust it.
- Bcrypt passwords are hashed at cost factor 12. Password verification adds ~300ms per request when auth is required.
- Deleting an access list that is assigned to one or more proxy hosts will succeed — the proxy hosts retain the association but Caddy will no longer enforce the list until a new one is assigned.
