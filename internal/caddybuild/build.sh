# Shared by repository builds, installers, containers, and embedded provisioning.
CADDY_VERSION=v2.11.9
CADDY_L4_VERSION=v0.1.1
CADDY_RATELIMIT_VERSION=v0.1.0
CADDY_NTLM_VERSION=v0.0.0-20261127162849-66b6501bd51c
XCADDY_VERSION=v0.4.6
CADDY_GO_TOOLCHAIN=go1.26.11
CADDY_SECURITY_REPLACEMENTS='golang.org/x/crypto=golang.org/x/crypto@v0.59.0 golang.org/x/net=golang.org/x/net@v0.63.0 github.com/klauspost/compress=github.com/klauspost/compress@v1.20.5 go.opentelemetry.io/otel=go.opentelemetry.io/otel@v1.47.0 go.opentelemetry.io/otel/sdk=go.opentelemetry.io/otel/sdk@v1.47.0 go.opentelemetry.io/otel/sdk/log=go.opentelemetry.io/otel/sdk/log@v1.47.0 go.opentelemetry.io/otel/trace=go.opentelemetry.io/otel/trace@v1.47.0'

verify_caddy() {
    caddy_binary="$1"
    caddy_actual_version="$("$caddy_binary" version)" || return 1
    case "$caddy_actual_version" in
        "$CADDY_VERSION"|"$CADDY_VERSION "*) ;;
        *) echo "Expected Caddy $CADDY_VERSION; found $caddy_actual_version" >&2; return 1 ;;
    esac
    caddy_modules="$("$caddy_binary" list-modules --versions)" || return 1
    for caddy_required_module in "layer4 $CADDY_L4_VERSION" "http.handlers.rate_limit $CADDY_RATELIMIT_VERSION" "http.reverse_proxy.transport.http_ntlm $CADDY_NTLM_VERSION"; do
        if ! printf '%s\n' "$caddy_modules" | awk -v required="$caddy_required_module" '$1 " " $2 == required { found=1 } END { exit !found }'; then
            echo "Missing required Caddy module/version: $caddy_required_module" >&2
            return 1
        fi
    done
}

build_caddy() {
    caddy_output="$1"
    # xcaddy must execute on the build host even when the output is cross-compiled.
    env GOTOOLCHAIN="$CADDY_GO_TOOLCHAIN" GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" \
        go install "github.com/caddyserver/xcaddy/cmd/xcaddy@$XCADDY_VERSION" || return 1
    set -- build "$CADDY_VERSION" \
        --with "github.com/mholt/caddy-l4@$CADDY_L4_VERSION" \
        --with "github.com/mholt/caddy-ratelimit@$CADDY_RATELIMIT_VERSION" \
        --with "github.com/PosternProxy/ntlm-transport@$CADDY_NTLM_VERSION" \
        --output "$caddy_output"
    for caddy_security_replacement in $CADDY_SECURITY_REPLACEMENTS; do
        set -- "$@" --replace "$caddy_security_replacement"
    done
    GOTOOLCHAIN="$CADDY_GO_TOOLCHAIN" "$(go env GOPATH)/bin/xcaddy" "$@"
}
