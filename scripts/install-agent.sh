#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for binary in "$SCRIPT_DIR/posternproxy-agent" "$SCRIPT_DIR/../posternproxy-agent" "$SCRIPT_DIR/../bin/posternproxy-agent"; do
    if [[ -x "$binary" ]]; then
        exec "$binary" install "$@"
    fi
done
echo 'Missing posternproxy-agent. Extract the Linux release archive and run sudo ./posternproxy-agent install --config-file agent.json --assets-dir .' >&2
exit 1
