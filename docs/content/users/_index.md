---
title: Users
weight: 120
---

# Users

PosternProxy supports multiple user accounts with role-based access control. User management is available to administrators only.

## Roles

| Role | Capabilities |
|---|---|
| **Admin** | Full access — manage all resources, users, settings, servers |
| **Viewer** | Read-only access — can view all resources but cannot create, edit, or delete |

## Managing users

Go to **Users** in the sidebar (admin only).

### Creating a user

1. Click **+ Add User**
2. Fill in:

| Field | Description |
|---|---|
| **Name** | Display name |
| **Email** | Login email (must be unique) |
| **Password** | Initial password (minimum 8 characters) |
| **Role** | `admin` or `viewer` |
| **Active** | Uncheck to create a disabled account |

3. Click **Create**

<!-- TODO: Screenshot — Add User modal -->

### Editing a user

Click the **Edit** icon on any user row to change their name, email, role, or active status. You cannot change another user's password from the edit form — use **Change Password** for that.

### Changing a password

Click **Change Password** on any user row (admin can reset any user's password; users can change their own from the profile menu).

Passwords must be at least 8 characters. PosternProxy hashes them with bcrypt at cost factor 12.

### Deactivating a user

Toggle the **Active** switch on a user to disable their account. Deactivated users cannot log in and any existing sessions are invalidated. Their API tokens continue to exist but will be rejected.

### Deleting a user

Click **Delete** to permanently remove a user account. This also deletes all of their API tokens.

{{< hint warning >}}
**You cannot delete your own account.** To remove the admin account, first promote another user to admin, log in as that user, then delete the original admin.
{{< /hint >}}

## Default admin account

On first boot, PosternProxy creates an admin account and prints a **randomly generated 20-character password** to stderr. Check the service logs to retrieve it:

```bash
journalctl -u posternproxy --no-pager | grep -A5 "INITIAL SETUP"
```

The default email is `admin@posternproxy.local` (override with `POSTERNPROXY_ADMIN_EMAIL`). To set a specific password instead of a generated one, set `POSTERNPROXY_ADMIN_PASSWORD` before first boot.

Change the password immediately after installation. See [Quick Start]({{< relref "/getting-started/quick-start" >}}).

### Recovering a lost password

If you lose access to the admin account, use the `reset-password` subcommand directly on the server — the service does not need to be running:

```bash
# Reset the first admin account found
posternproxy reset-password

# Reset a specific account by email
posternproxy reset-password --email admin@posternproxy.local
```

A new random password is printed to the terminal. In Docker:

```bash
docker exec posternproxy posternproxy reset-password
```

## Session management

Sessions use JWT access tokens (15-minute TTL) and refresh tokens (7-day TTL) stored in HttpOnly, Secure, SameSite=Strict cookies. Sessions expire automatically and are not tracked server-side (stateless JWT).

## Notes

- Email addresses are case-insensitive for login but stored as-entered.
- Failed login attempts are logged in the [Audit Log]({{< relref "/audit-log" >}}) and trigger fail2ban after the configured threshold.
- There is no password complexity requirement beyond the 8-character minimum. Encourage users to use a password manager.
