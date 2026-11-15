package provisioner

// installScript is the shell script executed on the remote server during provisioning.
// It installs Caddy (with L4 module), the caddyproxy-agent binary, and configures
// systemd + OS hardening.
//
// The script expects these environment variables set before execution:
//
//	CONTROLLER_URL  — wss://controller-host:81
//	AGENT_TOKEN     — pre-shared authentication token
//
// The agent binary is SCP'd to /tmp/caddyproxy-agent before this script runs.
const installScript = `#!/usr/bin/env bash
set -euo pipefail

log() { echo "[posternproxy] $*"; }

log "Starting PosternProxy agent provisioning"

### 1. System prerequisites ###
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl wget git socat iptables ufw fail2ban unattended-upgrades ca-certificates

### 2. Install Go (needed to build xcaddy) ###
if ! command -v go &>/dev/null; then
  log "Installing Go..."
  # Resolve the current stable release dynamically so this script doesn't go
  # stale when the Go CDN drops old archives. The endpoint returns e.g. "go1.22.6"
  # on the first line.
  GO_VERSION="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
  if [ -z "$GO_VERSION" ]; then
    log "ERROR: failed to resolve latest Go version"; exit 1
  fi
  log "Latest Go: ${GO_VERSION}"
  curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
fi
log "Go: $(go version)"

### 3. Build Caddy with L4 module ###
if ! command -v caddy &>/dev/null; then
  log "Building Caddy with layer4 module..."
  build_caddy /usr/local/bin/caddy
  chmod 755 /usr/local/bin/caddy
fi
verify_caddy "$(command -v caddy)"
log "Caddy: $(caddy version)"

### 4. Create users ###
id -u caddy    &>/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin caddy
id -u posternproxy-agent &>/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin posternproxy-agent

### 5. Create directories ###
mkdir -p /etc/caddy /var/lib/caddy /etc/posternproxy-agent
chown caddy:caddy /var/lib/caddy

### 6. Install agent binary ###
log "Installing agent binary..."
install -m 0755 -o root -g root /tmp/caddyproxy-agent /usr/local/bin/caddyproxy-agent

### 7. Write agent config ###
cat > /etc/posternproxy-agent/config.env <<EOF
POSTERNPROXY_CONTROLLER_URL=${CONTROLLER_URL}
POSTERNPROXY_AGENT_TOKEN=${AGENT_TOKEN}
EOF
chmod 0600 /etc/posternproxy-agent/config.env

### 8. Caddy systemd service ###
cat > /etc/systemd/system/caddy.service <<'UNIT'
[Unit]
Description=Caddy HTTP/2 web server
After=network.target

[Service]
User=caddy
Group=caddy
ExecStart=/usr/local/bin/caddy run --environ --config /etc/caddy/config.json
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/config.json
TimeoutStopSec=5s
LimitNOFILE=1048576
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true
Restart=on-failure

[Install]
WantedBy=multi-user.target
UNIT

### 9. Agent systemd service ###
cat > /etc/systemd/system/caddyproxy-agent.service <<'UNIT'
[Unit]
Description=PosternProxy Agent
After=network.target caddy.service

[Service]
User=posternproxy-agent
Group=posternproxy-agent
EnvironmentFile=/etc/posternproxy-agent/config.env
ExecStart=/usr/local/bin/caddyproxy-agent
Restart=on-failure
RestartSec=5s
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
# AmbientCapabilities grants iptables and socat the netlink/bind privileges
# they need without setuid, so we can safely deny privilege escalation here.
NoNewPrivileges=true
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
ReadWritePaths=/etc/caddy /var/lib/caddy /etc/posternproxy-agent

[Install]
WantedBy=multi-user.target
UNIT

### 10. OS hardening ###
log "Hardening OS..."

# IP forwarding (needed for port forwarding)
sysctl -w net.ipv4.ip_forward=1
grep -q 'net.ipv4.ip_forward' /etc/sysctl.conf \
  && sed -i 's/.*net.ipv4.ip_forward.*/net.ipv4.ip_forward=1/' /etc/sysctl.conf \
  || echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf

# rp_filter + SYN cookies
echo 'net.ipv4.conf.all.rp_filter=1' >> /etc/sysctl.conf || true
echo 'net.ipv4.tcp_syncookies=1'     >> /etc/sysctl.conf || true
sysctl -p || true

# UFW
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

# Fail2ban
systemctl enable fail2ban || true
systemctl start  fail2ban || true

# Unattended upgrades
echo 'APT::Periodic::Update-Package-Lists "1";'  > /etc/apt/apt.conf.d/20auto-upgrades
echo 'APT::Periodic::Unattended-Upgrade "1";'   >> /etc/apt/apt.conf.d/20auto-upgrades

### 11. Enable + start services ###
log "Starting services..."
systemctl daemon-reload
systemctl enable caddy
systemctl start  caddy   || true
systemctl enable caddyproxy-agent
systemctl start  caddyproxy-agent

log "Provisioning complete!"
`
