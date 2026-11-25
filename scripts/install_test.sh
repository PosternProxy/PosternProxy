#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT
cp "$SCRIPT_DIR/install.sh" "$TEST_DIR/install.sh"
cat > "$TEST_DIR/posternproxy" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$@"
exit 42
STUB
chmod +x "$TEST_DIR/posternproxy"
status=0
output="$(bash "$TEST_DIR/install.sh" --assets-dir 'a path with spaces')" || status=$?
if [[ "$status" -ne 42 || "$output" != $'install\n--assets-dir\na path with spaces' ]]; then
    echo 'Controller installer wrapper did not preserve arguments or exit status' >&2
    exit 1
fi
echo 'Controller installer wrapper checks passed'

cp "$SCRIPT_DIR/install-agent.sh" "$TEST_DIR/install-agent.sh"
cp "$TEST_DIR/posternproxy" "$TEST_DIR/posternproxy-agent"
status=0
output="$(bash "$TEST_DIR/install-agent.sh" --config-file 'a path with spaces')" || status=$?
if [[ "$status" -ne 42 || "$output" != $'install\n--config-file\na path with spaces' ]]; then
    echo 'Agent installer wrapper did not preserve arguments or exit status' >&2
    exit 1
fi
echo 'Agent installer wrapper checks passed'
