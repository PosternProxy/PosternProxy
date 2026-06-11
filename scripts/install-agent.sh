#!/usr/bin/env bash
# PosternProxy Agent Install Script
# Supports: Debian 11/12, Ubuntu 22.04/24.04
# Run as root: sudo bash install-agent.sh
set -euo pipefail

###############################################################################
# CONFIGURATION
###############################################################################
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/posternproxy-agent"
CONF_DIR="/etc/posternproxy-agent"
LOG_DIR="/var/log/posternproxy-agent"
SERVICE_USER="posternproxy-agent"
CADDY_USER="caddy"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_BINARY="${SCRIPT_DIR}/../posternproxy-agent"

###############################################################################
# HELPERS
###############################################################################
info()    { echo -e "\033[0;32m[INFO]\033[0m  $*"; }
warn()    { echo -e "\033[0;33m[WARN]\033[0m  $*"; }
die()     { echo -e "\033[0;31m[ERROR]\033[0m $*" >&2; exit 1; }

###############################################################################
# PRE-FLIGHT
###############################################################################
[[ $EUID -eq 0 ]] || die "This script must be run as root."
[[ -f /etc/debian_version ]] || die "This script only supports Debian/Ubuntu."
systemctl --version &>/dev/null || die "systemd is required."

info "PosternProxy Agent Installer"
echo "-----------------------------"

###############################################################################
# DEPENDENCIES
###############################################################################
info "Updating package index..."
apt-get update -qq

info "Installing dependencies..."
apt-get install -y -qq \
    curl wget gnupg2 lsb-release \
    ufw fail2ban unattended-upgrades \
    iptables iptables-persistent socat \
    ca-certificates

###############################################################################
# INSTALL CADDY (with L4 module via xcaddy)
###############################################################################
if ! command -v caddy &>/dev/null; then
    info "Building Caddy with L4 module..."

    if ! command -v go &>/dev/null; then
        info "Installing Go..."
        GO_VERSION="1.22.3"
        ARCH=$(dpkg --print-architecture)
        case "${ARCH}" in
            amd64) GO_ARCH="amd64" ;;
            arm64) GO_ARCH="arm64" ;;
            *) die "Unsupported architecture: ${ARCH}" ;;
        esac
        curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -o /tmp/go.tar.gz
        tar -C /usr/local -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
        export PATH="/usr/local/go/bin:$PATH"
        echo 'export PATH="/usr/local/go/bin:$PATH"' >> /etc/profile.d/go.sh
    fi

    go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
    XCADDY_BIN="$(go env GOPATH)/bin/xcaddy"

    cd /tmp
    "${XCADDY_BIN}" build \
        --with github.com/mholt/caddy-l4 \
        --output /usr/bin/caddy

    chmod +x /usr/bin/caddy
    info "Caddy built: $(/usr/bin/caddy version)"
else
    info "Caddy already installed: $(caddy version)"
fi

###############################################################################
# SYSTEM USERS
###############################################################################
if ! id "${CADDY_USER}" &>/dev/null; then
    info "Creating system user: ${CADDY_USER}"
    useradd --system --no-create-home --shell /usr/sbin/nologin "${CADDY_USER}"
fi

if ! id "${SERVICE_USER}" &>/dev/null; then
    info "Creating system user: ${SERVICE_USER}"
    useradd --system --no-create-home --shell /usr/sbin/nologin "${SERVICE_USER}"
fi

###############################################################################
# DIRECTORIES & PERMISSIONS
###############################################################################
info "Creating directories..."
mkdir -p "${DATA_DIR}" "${CONF_DIR}" "${LOG_DIR}"
chown -R "${SERVICE_USER}:${SERVICE_USER}" "${DATA_DIR}" "${LOG_DIR}"
chmod 750 "${DATA_DIR}" "${LOG_DIR}"
chown "${SERVICE_USER}:${SERVICE_USER}" "${CONF_DIR}"
chmod 750 "${CONF_DIR}"

###############################################################################
# INSTALL AGENT BINARY
###############################################################################
if [[ -f "${AGENT_BINARY}" ]]; then
    info "Installing PosternProxy Agent binary from ${AGENT_BINARY}..."
    install -m 755 -o root -g root "${AGENT_BINARY}" "${INSTALL_DIR}/posternproxy-agent"
else
    die "Agent binary not found at ${AGENT_BINARY}. Build the project first, or let the controller provision automatically."
fi

###############################################################################
# PROMPT FOR CONFIG
###############################################################################
CONF_FILE="${CONF_DIR}/config.env"

if [[ ! -f "${CONF_FILE}" ]]; then
    echo ""
    read -r -p "Controller URL (e.g. http://controller-ip:81): " CONTROLLER_URL
    read -r -p "Agent Token (provided by the controller): " AGENT_TOKEN
    read -r -p "Server ID (provided by the controller): " SERVER_ID

    [[ -n "${CONTROLLER_URL}" ]] || die "Controller URL cannot be empty."
    [[ -n "${AGENT_TOKEN}" ]]    || die "Agent token cannot be empty."
    [[ -n "${SERVER_ID}" ]]      || die "Server ID cannot be empty."

    # Convert HTTP to WS
    WS_URL="${CONTROLLER_URL/http:\/\//ws://}"
    WS_URL="${WS_URL/https:\/\//wss://}"

    cat > "${CONF_FILE}" <<EOF
POSTERNPROXY_CONTROLLER_URL=${WS_URL}/ws/agent
POSTERNPROXY_AGENT_TOKEN=${AGENT_TOKEN}
POSTERNPROXY_SERVER_ID=${SERVER_ID}
EOF
    chown "${SERVICE_USER}:${SERVICE_USER}" "${CONF_FILE}"
    chmod 600 "${CONF_FILE}"
    info "Config written to ${CONF_FILE}."
else
    info "Config already exists at ${CONF_FILE}, skipping."
fi

###############################################################################
# CADDY SYSTEMD SERVICE
###############################################################################
info "Installing Caddy systemd service..."
cat > /etc/systemd/system/caddy.service <<'EOF'
[Unit]
Description=Caddy Web Server
Documentation=https://caddyserver.com/docs/
After=network.target network-online.target
Requires=network-online.target

[Service]
Type=notify
User=caddy
Group=caddy
ExecStart=/usr/bin/caddy run --environ
ExecReload=/usr/bin/caddy reload --force
TimeoutStopSec=5s
LimitNOFILE=1048576
PrivateTmp=true
ProtectSystem=full
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

###############################################################################
# AGENT SYSTEMD SERVICE
###############################################################################
info "Installing Agent systemd service..."
cat > /etc/systemd/system/posternproxy-agent.service <<EOF
[Unit]
Description=PosternProxy Agent
After=network-online.target caddy.service
Wants=network-online.target caddy.service

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
EnvironmentFile=${CONF_FILE}
ExecStart=${INSTALL_DIR}/posternproxy-agent
Restart=always
RestartSec=10

# Hardening
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR}
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
SystemCallFilter=@system-service
MemoryDenyWriteExecute=true
RestrictRealtime=true
RestrictSUIDSGID=true
PrivateDevices=true

StandardOutput=append:${LOG_DIR}/agent.log
StandardError=append:${LOG_DIR}/agent.log

[Install]
WantedBy=multi-user.target
EOF

###############################################################################
# SYSCTL HARDENING
###############################################################################
info "Applying sysctl hardening..."
cat > /etc/sysctl.d/99-posternproxy-agent.conf <<'EOF'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.tcp_syncookies = 1
net.ipv4.icmp_echo_ignore_broadcasts = 1
EOF
sysctl --system -q

###############################################################################
# UFW
###############################################################################
if command -v ufw &>/dev/null; then
    info "Configuring UFW..."
    ufw --force reset
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow ssh
    ufw allow 80/tcp
    ufw allow 443/tcp
    ufw --force enable
    info "UFW enabled. Open ports: SSH, 80, 443."
fi

###############################################################################
# UNATTENDED UPGRADES
###############################################################################
info "Configuring automatic security updates..."
systemctl enable unattended-upgrades
systemctl start unattended-upgrades

###############################################################################
# ENABLE & START SERVICES
###############################################################################
info "Enabling and starting services..."
systemctl daemon-reload
systemctl enable caddy
systemctl start caddy || warn "Caddy failed to start"
systemctl enable posternproxy-agent
systemctl start posternproxy-agent

###############################################################################
# DONE
###############################################################################
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  PosternProxy Agent installed successfully!"
echo ""
echo "  The agent will connect to the controller shortly."
echo "  Check status: systemctl status posternproxy-agent"
echo "  View logs:    journalctl -u posternproxy-agent -f"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
