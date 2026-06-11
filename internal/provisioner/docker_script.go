package provisioner

// dockerInstallScript is executed on the remote server for Docker-mode provisioning.
// It installs Docker if absent, pulls the posternproxy-agent image, and runs it.
//
// The script expects these environment variables set before execution:
//
//	CONTROLLER_URL — wss://controller-host:81
//	AGENT_TOKEN    — pre-shared authentication token
//	SERVER_ID      — numeric server ID
const dockerInstallScript = `#!/usr/bin/env bash
set -euo pipefail

log() { echo "[posternproxy] $*"; }

log "Starting PosternProxy Docker agent provisioning"

### 1. Install Docker if not already present ###
if ! command -v docker &>/dev/null; then
    log "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    log "Docker installed."
else
    log "Docker already installed: $(docker --version)"
fi

### 2. Pull the agent image ###
log "Pulling posternproxy-agent image..."
docker pull ghcr.io/posternproxy/posternproxy-agent:latest

### 3. Create data directory ###
mkdir -p /opt/posternproxy-agent/data/certs

### 4. Write env file ###
cat > /opt/posternproxy-agent/.env <<EOF
POSTERNPROXY_CONTROLLER_URL=${CONTROLLER_URL}
POSTERNPROXY_AGENT_TOKEN=${AGENT_TOKEN}
POSTERNPROXY_SERVER_ID=${SERVER_ID}
EOF
chmod 600 /opt/posternproxy-agent/.env
log "Agent config written."

### 5. Remove existing container if present ###
docker rm -f posternproxy-agent 2>/dev/null || true

### 6. Start agent container ###
log "Starting posternproxy-agent container..."
docker run -d \
    --name posternproxy-agent \
    --restart unless-stopped \
    --env-file /opt/posternproxy-agent/.env \
    -v /opt/posternproxy-agent/data:/data \
    -p 80:80 \
    -p 443:443 \
    --cap-add NET_ADMIN \
    --cap-add NET_RAW \
    --cap-add NET_BIND_SERVICE \
    --sysctl net.ipv4.ip_forward=1 \
    ghcr.io/posternproxy/posternproxy-agent:latest

log "Agent container started successfully."
`
