// Package caddybuild holds the shared Caddy build contract.
package caddybuild

import (
	_ "embed"
	"strings"
)

//go:embed build.sh
var embeddedScript string

// Script is prepended to remote provisioning so it uses the same pins and
// module verification as local installers and container builds.
var Script = normalizeScript(embeddedScript)

func normalizeScript(script string) string {
	script = strings.ReplaceAll(script, "\r\n", "\n")
	return strings.ReplaceAll(script, "\r", "\n")
}
