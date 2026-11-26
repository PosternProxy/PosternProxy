package caddy

import (
	_ "embed"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/branding"
)

//go:embed pages.css
var defaultPageStyles string

// Inline images keep error pages independent of the controller and upstream.
var defaultPageLogo = fmt.Sprintf(`<picture>
<source media="(prefers-color-scheme: dark)" srcset="%s">
<img src="%s" alt="PosternProxy" width="128" height="128">
</picture>`, branding.DarkDataURI, branding.LightDataURI)
