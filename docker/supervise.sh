#!/bin/sh
set -eu

export XDG_DATA_HOME=${XDG_DATA_HOME:-/data/caddy}
export XDG_CONFIG_HOME=${XDG_CONFIG_HOME:-/data/caddy}
mkdir -p "$XDG_DATA_HOME" "$XDG_CONFIG_HOME"

app_pid=
caddy_pid=
stop() {
    trap - TERM INT
    [ -z "$app_pid" ] || kill -TERM "$app_pid" 2>/dev/null || true
    [ -z "$caddy_pid" ] || kill -TERM "$caddy_pid" 2>/dev/null || true
    [ -z "$app_pid" ] || wait "$app_pid" 2>/dev/null || true
    [ -z "$caddy_pid" ] || wait "$caddy_pid" 2>/dev/null || true
}
trap 'stop; exit 0' TERM INT
trap stop EXIT
printf '%s\n' '{"admin":{"listen":"127.0.0.1:2019"}}' > /run/posternproxy-caddy.json
caddy run --config /run/posternproxy-caddy.json &
caddy_pid=$!
attempt=0
until wget -q -O /dev/null http://127.0.0.1:2019/config/; do
    kill -0 "$caddy_pid" 2>/dev/null || exit 1
    attempt=$((attempt + 1))
    [ "$attempt" -lt 100 ] || exit 1
    sleep 0.2
done
"$@" &
app_pid=$!
while kill -0 "$app_pid" 2>/dev/null && kill -0 "$caddy_pid" 2>/dev/null; do sleep 0.2; done
exit 1
