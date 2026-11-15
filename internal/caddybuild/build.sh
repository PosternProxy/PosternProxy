# Shared by repository builds, installers, containers, and embedded provisioning.
CADDY_VERSION=v2.11.4
CADDY_L4_VERSION=v0.1.1
CADDY_RATELIMIT_VERSION=v0.1.0
XCADDY_VERSION=v0.4.6

verify_caddy() {
    caddy_binary="$1"
    caddy_actual_version="$("$caddy_binary" version)" || return 1
    case "$caddy_actual_version" in
        "$CADDY_VERSION"|"$CADDY_VERSION "*) ;;
        *) echo "Expected Caddy $CADDY_VERSION; found $caddy_actual_version" >&2; return 1 ;;
    esac
    caddy_modules="$("$caddy_binary" list-modules --versions)" || return 1
    for caddy_required_module in "layer4 $CADDY_L4_VERSION" "http.handlers.rate_limit $CADDY_RATELIMIT_VERSION"; do
        if ! printf '%s\n' "$caddy_modules" | awk -v required="$caddy_required_module" '$1 " " $2 == required { found=1 } END { exit !found }'; then
            echo "Missing required Caddy module/version: $caddy_required_module" >&2
            return 1
        fi
    done
}

build_caddy() {
    caddy_output="$1"
    # xcaddy must execute on the build host even when the output is cross-compiled.
    env GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" \
        go install "github.com/caddyserver/xcaddy/cmd/xcaddy@$XCADDY_VERSION" || return 1
    "$(go env GOPATH)/bin/xcaddy" build "$CADDY_VERSION" \
        --with "github.com/mholt/caddy-l4@$CADDY_L4_VERSION" \
        --with "github.com/mholt/caddy-ratelimit@$CADDY_RATELIMIT_VERSION" \
        --output "$caddy_output"
}
