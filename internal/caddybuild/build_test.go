package caddybuild

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestEmbeddedScriptUsesUnixLineEndings(t *testing.T) {
	if strings.ContainsRune(Script, '\r') {
		t.Fatal("embedded Caddy script contains carriage returns and cannot run in Linux Bash")
	}
}

func TestNormalizeScript(t *testing.T) {
	for _, input := range []string{"one\ntwo\n", "one\r\ntwo\r\n", "one\rtwo\r", "one\r\ntwo\n"} {
		if got := normalizeScript(input); got != "one\ntwo\n" {
			t.Errorf("normalizeScript(%q) = %q", input, got)
		}
	}
}

func TestEmbeddedScriptParsesAsBash(t *testing.T) {
	bash := os.Getenv("POSTERNPROXY_TEST_BASH")
	if bash == "" {
		if runtime.GOOS == "windows" {
			t.Skip("set POSTERNPROXY_TEST_BASH to a native Bash executable")
		}
		var err error
		bash, err = exec.LookPath("bash")
		if err != nil {
			t.Fatal(err)
		}
	}
	if output, err := exec.Command(bash, "-n", "-c", Script).CombinedOutput(); err != nil {
		t.Fatalf("embedded script does not parse: %v: %s", err, output)
	}
	fixture := `
fixture_caddy() {
    case "$1" in
        version) printf '%s\n' "$CADDY_VERSION" ;;
        list-modules) printf 'layer4 %s\nhttp.handlers.rate_limit %s\nhttp.reverse_proxy.transport.http_ntlm %s\n' "$CADDY_L4_VERSION" "$CADDY_RATELIMIT_VERSION" "$CADDY_NTLM_VERSION" ;;
        *) return 1 ;;
    esac
}
verify_caddy fixture_caddy
`
	if output, err := exec.Command(bash, "-c", Script+fixture).CombinedOutput(); err != nil {
		t.Fatalf("embedded verification failed: %v: %s", err, output)
	}
}
