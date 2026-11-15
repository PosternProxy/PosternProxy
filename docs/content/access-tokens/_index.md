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
4. Choose the scopes and expiration, then click **Create**
5. **Copy the token immediately**. It is displayed only once.

<!-- TODO: Screenshot — Create Token dialog with the token value shown -->

## Using a token

Include the token as a Bearer token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer <your-token>" \
     http://your-server:81/api/proxy-hosts
```

Management API routes accept API tokens subject to scope and role checks.
Creating an API token requires a signed-in user session.

## Token permissions

The `read` scope allows read requests, and `write` allows mutating requests.
The creator's current role still applies. Admin-only operations require an
admin creator, and a token with `write` cannot grant a viewer permission to
change resources.

## Managing tokens

The Access Tokens page lists your tokens. Administrators can see and revoke
other users' tokens.

| Column | Description |
|---|---|
| Name | The label you gave the token |
| Created | When the token was created |
| Scopes | The operations the token can authorize |
| Expires | When the token expires, if configured |
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

Redirection, stream, and dead hosts use the same host route pattern. Traffic
Forwarding rules use the `/api/stream-hosts` resource.

### User session endpoints

```
POST /api/auth/login    # Username and password
POST /api/auth/refresh  # Refresh cookie
POST /api/auth/logout   # Session access token
GET  /api/auth/me       # Session access token
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

- Tokens can have an expiration date. You can revoke a token before it expires.
- Audited operations made with an API token are attributed to its creator in the [Audit Log]({{< relref "/audit-log" >}}).
- There is no rate limit per token beyond the management API rate limit (120 req/min per IP).
