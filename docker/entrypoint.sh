#!/bin/sh
set -e

export POSTERNPROXY_DB_PATH=${POSTERNPROXY_DB_PATH:-/data/posternproxy.db}
export POSTERNPROXY_CERT_DIR=${POSTERNPROXY_CERT_DIR:-/data/certs}
export POSTERNPROXY_CERT_RUNTIME_DIR=${POSTERNPROXY_CERT_RUNTIME_DIR:-/run/posternproxy-certs}
export POSTERNPROXY_CADDY_API=${POSTERNPROXY_CADDY_API:-http://localhost:2019}

mkdir -p "$(dirname "$POSTERNPROXY_DB_PATH")" "$POSTERNPROXY_CERT_DIR"

caddy run &

# Wait for Caddy admin API to be ready before starting PosternProxy.
# This ensures the initial config sync succeeds on first boot.
until wget -q --spider http://localhost:2019/config/ 2>/dev/null; do
    sleep 0.2
done

exec posternproxy
