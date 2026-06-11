---
title: Access Tokens
weight: 90
---

# Access Tokens

Access tokens let you authenticate to the PosternProxy REST API without using your username and password. They are designed for scripts, CI/CD pipelines, and automation tools.

## Creating a token

1. Click **Access Tokens** in the sidebar (or go to your profile → **API Tokens**)
2. Click **+ Create Token**
3. Enter a **Name** to identify the token (e.g. `ci-deploy`, `monitoring`)
4. Click **Create**
5. **Copy the token immediately** — it is only displayed once and cannot be retrieved afterwards

<!-- TODO: Screenshot — Create Token dialog with the token value shown -->

## Using a token

Include the token as a Bearer token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer <your-token>" \
     http://your-server:81/api/proxy-hosts
```

All API endpoints that require authentication accept Bearer tokens.

## Token permissions

Tokens inherit the permissions of the user who created them. An admin user's token has admin-level API access; a viewer's token has read-only access.

## Managing tokens

The Access Tokens page lists all tokens belonging to the current user:

| Column | Description |
|---|---|
| Name | The label you gave the token |
| Created | When the token was created |
| Last used | When the token was last used to make an API request |

Click the **Delete** icon to revoke a token immediately. Revoked tokens are rejected on next use.

## API reference

The full REST API is available at `http://your-server:81/api`. All resources follow the same pattern:

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/proxy-hosts` | List all proxy hosts |
| `POST` | `/api/proxy-hosts` | Create a proxy host |
| `GET` | `/api/proxy-hosts/{id}` | Get a specific proxy host |
| `PUT` | `/api/proxy-hosts/{id}` | Update a proxy host |
| `DELETE` | `/api/proxy-hosts/{id}` | Delete a proxy host |
| `POST` | `/api/proxy-hosts/{id}/enable` | Enable a proxy host |
| `POST` | `/api/proxy-hosts/{id}/disable` | Disable a proxy host |

The same pattern applies to `redirection-hosts`, `stream-hosts`, `dead-hosts`, `certificates`, `access-lists`, and `port-forwards`.

### Auth endpoints (no token required)

```
POST /api/auth/login
POST /api/auth/refresh
POST /api/auth/logout
GET  /api/auth/me
```

### Example: create a proxy host via API

```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "domain_names": ["app.example.com"],
    "forward_scheme": "http",
    "forward_host": "192.168.1.10",
    "forward_port": 3000,
    "ssl_forced": true,
    "enabled": true
  }' \
  http://your-server:81/api/proxy-hosts
```

## Notes

- Tokens do not expire. Rotate them regularly or delete and recreate as part of your secret rotation policy.
- All API requests made with a token are recorded in the [Audit Log]({{< relref "/audit-log" >}}) with the associated user.
- There is no rate limit per token beyond the management API rate limit (120 req/min per IP).
