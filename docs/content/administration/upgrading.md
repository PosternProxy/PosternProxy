---
title: Upgrading
weight: 2
---

# Upgrading PosternProxy

## Upgrading the controller

1. **Build the new binary** on your development machine:

   ```bash
   bash scripts/build.sh
   ```

2. **Copy the binary to the server:**

   ```bash
   scp posternproxy user@your-server:/tmp/
   ```

3. **Stop the service, replace the binary, restart:**

   ```bash
   ssh root@your-server
   systemctl stop posternproxy
   cp /tmp/posternproxy /usr/local/bin/posternproxy
   systemctl start posternproxy
   ```

4. **Verify the service started:**

   ```bash
   systemctl status posternproxy
   journalctl -u posternproxy -n 20
   ```

### Database migrations

PosternProxy runs embedded migrations automatically on startup. If a new version adds database columns or tables, the migration runs before the service starts serving requests. No manual intervention is needed.

If a migration fails, the service will log the error and exit. Check the journal:

```bash
journalctl -u posternproxy -n 50
```

### Rollback

If the new binary does not start, roll back:

```bash
systemctl stop posternproxy
cp /usr/local/bin/posternproxy.bak /usr/local/bin/posternproxy  # if you kept a backup
systemctl start posternproxy
```

Alternatively, restore a database backup taken before the upgrade.

## Upgrading Caddy

Caddy is built from source with required plugins during installation. To upgrade Caddy:

1. Install or update `xcaddy` on the server (or build on your dev machine and SCP):

   ```bash
   go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
   ```

2. Build the new Caddy binary:

   ```bash
   xcaddy build \
     --with github.com/mholt/caddy-l4 \
     --with github.com/mholt/caddy-ratelimit \
     --output /tmp/caddy
   ```

3. Replace and restart:

   ```bash
   systemctl stop caddy
   cp /tmp/caddy /usr/bin/caddy
   systemctl start caddy
   ```

4. Verify:

   ```bash
   caddy version
   systemctl status caddy
   ```

## Upgrading agents (multi-server)

When upgrading in a multi-server deployment, upgrade the controller first. Then use the **Update Agent** button in the Servers page to push the new agent binary to each remote server:

1. Go to **Servers** in the sidebar
2. Click **⋮ → Update Agent** on each server
3. PosternProxy will SCP the embedded agent binary to the remote server and restart the agent service

Alternatively, use the **Re-Provision** option to fully re-run the provisioning script on the remote server, which will update both Caddy and the agent.

## Version compatibility

PosternProxy follows semantic versioning. Minor and patch versions are backward-compatible. Major version bumps may require manual migration steps documented in the release notes.

The agent must be at the same version as the controller. Running mismatched versions will result in the agent rejecting commands from the controller.
