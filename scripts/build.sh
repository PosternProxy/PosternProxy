#!/usr/bin/env bash
# PosternProxy Build Script
# Produces: ./posternproxy (controller), ./posternproxy-agent (agent),
#           ./caddy (custom Caddy build with the L4 + rate-limit plugins)
# Requires: Go 1.25+, Node 20+ with npm
set -euo pipefail

###############################################################################
# CONFIGURATION
###############################################################################
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
. "${PROJECT_ROOT}/internal/caddybuild/build.sh"
FRONTEND_DIR="${PROJECT_ROOT}/frontend"
OUT_DIR="${PROJECT_ROOT}"

# Build target (defaults to host OS/arch, override with env vars)
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"

CONTROLLER_BIN="${OUT_DIR}/posternproxy"
AGENT_BIN="${OUT_DIR}/posternproxy-agent"
CADDY_BIN="${OUT_DIR}/caddy"

###############################################################################
# HELPERS
###############################################################################
info() { echo -e "\033[0;32m[BUILD]\033[0m $*"; }
die()  { echo -e "\033[0;31m[ERROR]\033[0m $*" >&2; exit 1; }

###############################################################################
# PRE-FLIGHT
###############################################################################
command -v go   &>/dev/null || die "Go is not installed."
command -v node &>/dev/null || die "Node.js is not installed."
command -v npm  &>/dev/null || die "npm is not installed."

GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
NODE_VER=$(node --version | sed 's/v//')
info "Go ${GO_VER}, Node ${NODE_VER}"

###############################################################################
# FRONTEND BUILD
###############################################################################
info "Building frontend..."
cd "${FRONTEND_DIR}"
npm ci --silent
npm run build

DIST_SIZE=$(du -sh dist 2>/dev/null | cut -f1)
info "Frontend built (${DIST_SIZE}): ${FRONTEND_DIR}/dist"

# Copy dist into internal/ui/dist so go:embed can find it.
UI_EMBED_DIR="${PROJECT_ROOT}/internal/ui/dist"
info "Copying dist to ${UI_EMBED_DIR}..."
rm -rf "${UI_EMBED_DIR}"
cp -r "${FRONTEND_DIR}/dist" "${UI_EMBED_DIR}"

###############################################################################
# GO BUILD — CONTROLLER
###############################################################################
info "Building controller binary (GOOS=${GOOS} GOARCH=${GOARCH})..."
cd "${PROJECT_ROOT}"

CGO_ENABLED=0 \
GOOS="${GOOS}" \
GOARCH="${GOARCH}" \
go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o "${CONTROLLER_BIN}" \
    ./cmd/posternproxy

info "Controller: ${CONTROLLER_BIN} ($(du -sh "${CONTROLLER_BIN}" | cut -f1))"

###############################################################################
# GO BUILD — AGENT
###############################################################################
info "Building agent binary (GOOS=${GOOS} GOARCH=${GOARCH})..."

CGO_ENABLED=0 \
GOOS="${GOOS}" \
GOARCH="${GOARCH}" \
go build \
    -trimpath \
    -ldflags="-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o "${AGENT_BIN}" \
    ./cmd/posternproxy-agent

info "Agent: ${AGENT_BIN} ($(du -sh "${AGENT_BIN}" | cut -f1))"

###############################################################################
# CADDY BUILD — custom build with L4 + rate-limit plugins
###############################################################################
# Building Caddy here (on the build/CI host) keeps the Go toolchain off the
# production server. install.sh installs this binary directly and only falls
# back to compiling on-server if it is missing.
info "Building custom Caddy (GOOS=${GOOS} GOARCH=${GOARCH})..."

CGO_ENABLED=0 \
GOOS="${GOOS}" \
GOARCH="${GOARCH}" \
build_caddy "${CADDY_BIN}"

info "Caddy: ${CADDY_BIN} ($(du -sh "${CADDY_BIN}" | cut -f1))"

###############################################################################
# DONE
###############################################################################
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Build complete!"
echo ""
echo "  Controller: ${CONTROLLER_BIN}"
echo "  Agent:      ${AGENT_BIN}"
echo "  Caddy:      ${CADDY_BIN}"
echo ""
echo "  Deploy with: sudo bash scripts/install.sh"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
