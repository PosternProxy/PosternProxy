#!/usr/bin/env bash
# Compatibility entry point; installation is implemented by the controller.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for binary in "${SCRIPT_DIR}/posternproxy" "${SCRIPT_DIR}/../posternproxy" "${PWD}/posternproxy" /usr/local/bin/posternproxy; do
    if [[ -x "$binary" ]]; then
        exec "$binary" install "$@"
    fi
done
echo '[ERROR] Download the controller binary and run: sudo ./posternproxy install' >&2
exit 1
