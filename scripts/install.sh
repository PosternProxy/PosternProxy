#!/usr/bin/env bash
# PosternProxy Controller Install Script
# Supports: Debian 11/12, Ubuntu 22.04/24.04
# Run as root: sudo bash install.sh
set -euo pipefail

###############################################################################
# CONFIGURATION
###############################################################################
INSTALL_DIR="/usr/local/bin"
DATA_DIR="/var/lib/posternproxy"
CONF_DIR="/etc/posternproxy"
CERT_DIR="${DATA_DIR}/certs"
LOG_DIR="/var/log/posternproxy"
SERVICE_USER="posternproxy"
CADDY_USER="caddy"
LISTEN_PORT="81"
CADDY_ADMIN="http://localhost:2019"

# Detect binary path (same directory as this script, or download)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="${SCRIPT_DIR}/../posternproxy"

###############################################################################
# HELPERS
###############################################################################
info()    { echo -e "\033[0;32m[INFO]\033[0m  $*"; }
warn()    { echo -e "\033[0;33m[WARN]\033[0m  $*"; }
die()     { echo -e "\033[0;31m[ERROR]\033[0m $*" >&2; exit 1; }
require() { command -v "$1" &>/dev/null || die "$1 is required but not installed."; }

###############################################################################
# PRE-FLIGHT
###############################################################################
[[ $EUID -eq 0 ]] || die "This script must be run as root."
[[ -f /etc/debian_version ]] || die "This script only supports Debian/Ubuntu."
systemctl --version &>/dev/null || die "systemd is required."

info "PosternProxy Controller Installer"
echo "------------------------------------"

###############################################################################
# DEPENDENCIES
###############################################################################
info "Updating package index..."
apt-get update -qq

info "Installing dependencies..."
apt-get install -y -qq \
    curl wget git gnupg2 lsb-release \
    ufw fail2ban unattended-upgrades apt-listchanges \
    iptables iptables-persistent socat \
    ca-certificates

###############################################################################
# INSTALL CADDY (with L4 module via xcaddy)
###############################################################################
if ! command -v caddy &>/dev/null; then
    info "Building Caddy with L4 module..."

    # Install Go if needed
    if ! command -v go &>/dev/null; then
        info "Installing Go..."
        # Resolve the latest stable release dynamically — hardcoded versions
        # rot fast (Go CDN eventually drops old archives).
        GO_VERSION="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)"
        [ -n "${GO_VERSION}" ] || die "Failed to resolve latest Go version"
        ARCH=$(dpkg --print-architecture)
        case "${ARCH}" in
            amd64) GO_ARCH="amd64" ;;
            arm64) GO_ARCH="arm64" ;;
            *) die "Unsupported architecture: ${ARCH}" ;;
        esac
        info "Latest Go: ${GO_VERSION} (${GO_ARCH})"
        curl -fsSL "https://go.dev/dl/${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -o /tmp/go.tar.gz
        tar -C /usr/local -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
        export PATH="/usr/local/go/bin:$PATH"
        echo 'export PATH="/usr/local/go/bin:$PATH"' >> /etc/profile.d/go.sh
    fi

    # Install xcaddy
    go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
    XCADDY_BIN="$(go env GOPATH)/bin/xcaddy"

    # Build Caddy with L4 (stream hosts / TLS passthrough) and rate-limit plugins
    cd /tmp
    "${XCADDY_BIN}" build \
        --with github.com/mholt/caddy-l4 \
        --with github.com/mholt/caddy-ratelimit \
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
mkdir -p "${DATA_DIR}" "${CONF_DIR}" "${CERT_DIR}" "${LOG_DIR}"
chown -R "${SERVICE_USER}:${SERVICE_USER}" "${DATA_DIR}" "${LOG_DIR}"
chmod 750 "${DATA_DIR}" "${LOG_DIR}"
chmod 700 "${CERT_DIR}"
chown "${SERVICE_USER}:${SERVICE_USER}" "${CONF_DIR}"
chmod 750 "${CONF_DIR}"

###############################################################################
# INSTALL POSTERNPROXY BINARY
###############################################################################
if [[ -f "${BINARY}" ]]; then
    info "Installing PosternProxy binary from ${BINARY}..."
    install -m 755 -o root -g root "${BINARY}" "${INSTALL_DIR}/posternproxy"
else
    die "PosternProxy binary not found at ${BINARY}. Build the project first."
fi

###############################################################################
# GENERATE CONFIG
###############################################################################
JWT_SECRET=$(openssl rand -hex 32)
CONF_FILE="${CONF_DIR}/config.env"

if [[ ! -f "${CONF_FILE}" ]]; then
    info "Generating config at ${CONF_FILE}..."
    cat > "${CONF_FILE}" <<EOF
# PosternProxy configuration
POSTERNPROXY_LISTEN_ADDR=:${LISTEN_PORT}
POSTERNPROXY_DB_PATH=${DATA_DIR}/posternproxy.db
POSTERNPROXY_JWT_SECRET=${JWT_SECRET}
POSTERNPROXY_CADDY_API=${CADDY_ADMIN}
POSTERNPROXY_CERT_DIR=${CERT_DIR}
POSTERNPROXY_CONTROLLER_URL=http://$(hostname -I | awk '{print $1}'):${LISTEN_PORT}
POSTERNPROXY_AGENT_BIN_PATH=${INSTALL_DIR}/posternproxy-agent
EOF
    chown "${SERVICE_USER}:${SERVICE_USER}" "${CONF_FILE}"
    chmod 600 "${CONF_FILE}"
    info "Config written. JWT secret generated."
else
    info "Config already exists at ${CONF_FILE}, skipping generation."
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
# POSTERNPROXY SYSTEMD SERVICE
###############################################################################
info "Installing PosternProxy systemd service..."
cat > /etc/systemd/system/posternproxy.service <<EOF
[Unit]
Description=PosternProxy Controller
After=network.target caddy.service
Wants=caddy.service

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
EnvironmentFile=${CONF_FILE}
ExecStart=${INSTALL_DIR}/posternproxy
Restart=always
RestartSec=5

# Hardening
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
NoNewPrivileges=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR}
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE
SystemCallFilter=@system-service
MemoryDenyWriteExecute=true
RestrictRealtime=true
RestrictSUIDSGID=true
PrivateDevices=true

StandardOutput=append:${LOG_DIR}/posternproxy.log
StandardError=append:${LOG_DIR}/posternproxy.log

[Install]
WantedBy=multi-user.target
EOF

###############################################################################
# SYSCTL HARDENING
###############################################################################
info "Applying sysctl hardening..."
cat > /etc/sysctl.d/99-posternproxy.conf <<'EOF'
# Enable IP forwarding (needed for port forwarding feature)
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1

# Prevent IP spoofing
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1

# Disable ICMP redirects
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0

# Enable TCP SYN cookies
net.ipv4.tcp_syncookies = 1

# Ignore ICMP broadcast
net.ipv4.icmp_echo_ignore_broadcasts = 1

# Log martian packets
net.ipv4.conf.all.log_martians = 1
EOF
sysctl --system -q

###############################################################################
# IPTABLES PERSISTENCE
###############################################################################
info "Enabling iptables persistence..."
netfilter-persistent save 2>/dev/null || true

###############################################################################
# UFW FIREWALL
###############################################################################
if command -v ufw &>/dev/null; then
    info "Configuring UFW..."
    ufw --force reset
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow ssh
    ufw allow 80/tcp
    ufw allow 443/tcp
    ufw allow "${LISTEN_PORT}/tcp"
    ufw --force enable
    info "UFW enabled. Open ports: SSH, 80, 443, ${LISTEN_PORT}."
fi

###############################################################################
# FAIL2BAN
###############################################################################
if command -v fail2ban-server &>/dev/null; then
    info "Configuring fail2ban..."
    cat > /etc/fail2ban/filter.d/posternproxy.conf <<'EOF'
[Definition]
failregex = .*"action":"login_failure".*"ip_address":"<HOST>".*
ignoreregex =
EOF
    cat > /etc/fail2ban/jail.d/posternproxy.conf <<EOF
[posternproxy]
enabled  = true
port     = ${LISTEN_PORT}
filter   = posternproxy
logpath  = ${LOG_DIR}/posternproxy.log
maxretry = 5
findtime = 60
bantime  = 3600
EOF
    systemctl enable fail2ban
    systemctl restart fail2ban
fi

###############################################################################
# UNATTENDED UPGRADES
###############################################################################
info "Configuring automatic security updates..."
cat > /etc/apt/apt.conf.d/50unattended-upgrades-posternproxy <<'EOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
};
Unattended-Upgrade::Automatic-Reboot "false";
EOF
systemctl enable unattended-upgrades
systemctl start unattended-upgrades

###############################################################################
# OPTIONAL: SSH HARDENING
###############################################################################
read -r -p "Harden SSH? (disable root login, enforce key-only auth) [y/N] " HARDEN_SSH
if [[ "${HARDEN_SSH,,}" == "y" ]]; then
    SSHD_CONF="/etc/ssh/sshd_config"
    info "Hardening SSH..."
    sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/' "${SSHD_CONF}"
    sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' "${SSHD_CONF}"
    sed -i 's/^#\?ChallengeResponseAuthentication.*/ChallengeResponseAuthentication no/' "${SSHD_CONF}"
    systemctl reload sshd
    warn "SSH hardened: root login disabled, password auth disabled."
    warn "Ensure your SSH key is in ~/.ssh/authorized_keys before logging out!"
fi

###############################################################################
# ENABLE & START SERVICES
###############################################################################
info "Enabling and starting services..."
systemctl daemon-reload
systemctl enable caddy
systemctl start caddy || warn "Caddy failed to start (check config)"
systemctl enable posternproxy
systemctl start posternproxy

###############################################################################
# DONE
###############################################################################
SERVER_IP=$(hostname -I | awk '{print $1}')
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  PosternProxy installed successfully!"
echo ""
echo "  UI:      http://${SERVER_IP}:${LISTEN_PORT}"
echo "  Login:   admin@posternproxy.local"
echo "  Password is printed in the service logs on first boot."
echo "  View it with: journalctl -u posternproxy | grep password"
echo ""
echo "  Change the password immediately after first login."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
