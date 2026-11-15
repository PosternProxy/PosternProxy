#!/usr/bin/env bash
set -euo pipefail
. "$(dirname "${BASH_SOURCE[0]}")/build.sh"

fixture_core="$CADDY_VERSION"
fixture_l4="$CADDY_L4_VERSION"
fixture_ratelimit="$CADDY_RATELIMIT_VERSION"
fixture_caddy() {
    case "$1" in
        version) printf '%s test-build\n' "$fixture_core" ;;
        list-modules)
            printf 'layer4 %s\nhttp.handlers.rate_limit %s\n' "$fixture_l4" "$fixture_ratelimit"
            ;;
        *) return 1 ;;
    esac
}

verify_caddy fixture_caddy
fixture_core=v0.0.0
if verify_caddy fixture_caddy 2>/dev/null; then
    echo 'Accepted an incorrect Caddy version' >&2; exit 1
fi
fixture_core="$CADDY_VERSION"
fixture_ratelimit=
if verify_caddy fixture_caddy 2>/dev/null; then
    echo 'Accepted a missing rate-limit module' >&2; exit 1
fi
fixture_ratelimit=v0.0.0
if verify_caddy fixture_caddy 2>/dev/null; then
    echo 'Accepted an incorrect rate-limit module version' >&2; exit 1
fi
fixture_ratelimit="$CADDY_RATELIMIT_VERSION"
fixture_l4=v0.0.0
if verify_caddy fixture_caddy 2>/dev/null; then
    echo 'Accepted an incorrect L4 module version' >&2; exit 1
fi
echo 'Caddy build contract checks passed'
