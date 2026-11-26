// Package branding contains the runtime logos shared by the admin UI and Caddy pages.
package branding

import (
	_ "embed"
	"encoding/base64"
)

//go:embed logo.png
var lightLogo []byte

//go:embed logo_dark.png
var darkLogo []byte

var (
	LightDataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(lightLogo)
	DarkDataURI  = "data:image/png;base64," + base64.StdEncoding.EncodeToString(darkLogo)
)
