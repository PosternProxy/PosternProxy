#!/bin/sh
set -e

export POSTERNPROXY_CERT_DIR=${POSTERNPROXY_CERT_DIR:-/data/certs}
export POSTERNPROXY_CADDY_API=${POSTERNPROXY_CADDY_API:-http://localhost:2019}

mkdir -p "$POSTERNPROXY_CERT_DIR"

caddy run &

exec posternproxy-agent
