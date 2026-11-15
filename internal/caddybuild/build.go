// Package caddybuild holds the shared Caddy build contract.
package caddybuild

import _ "embed"

// Script is prepended to remote provisioning so it uses the same pins and
// module verification as local installers and container builds.
//
//go:embed build.sh
var Script string
