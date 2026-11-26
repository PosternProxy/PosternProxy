#!/bin/sh
set -e

export POSTERNPROXY_CERT_DIR=${POSTERNPROXY_CERT_DIR:-/data/certs}
export POSTERNPROXY_DATA_DIR=${POSTERNPROXY_DATA_DIR:-/data}
export POSTERNPROXY_CADDY_API=${POSTERNPROXY_CADDY_API:-http://localhost:2019}

mkdir -p "$POSTERNPROXY_CERT_DIR"

exec /usr/local/lib/posternproxy/supervise.sh posternproxy-agent
