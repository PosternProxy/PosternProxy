package platformservice

import (
	"fmt"
	"path/filepath"
	"strings"
)

const CaddyName = "posternproxy-caddy"

type Definition struct {
	Name            string
	Environment     map[string]string
	CaddyExecutable string
	CaddyConfig     string
	LogPath         string
	BootstrapPath   string
}

func (d Definition) validate() error {
	if d.Name != "posternproxy" && d.Name != "posternproxy-agent" && d.Name != CaddyName {
		return fmt.Errorf("unknown service name")
	}
	if !filepath.IsAbs(d.LogPath) {
		return fmt.Errorf("service log path must be absolute")
	}
	for key := range d.Environment {
		if !strings.HasPrefix(key, "POSTERNPROXY_") && key != "XDG_DATA_HOME" && key != "XDG_CONFIG_HOME" {
			return fmt.Errorf("unsupported service environment key")
		}
	}
	if d.BootstrapPath != "" && !filepath.IsAbs(d.BootstrapPath) {
		return fmt.Errorf("bootstrap path must be absolute")
	}
	if d.Name == CaddyName && (!filepath.IsAbs(d.CaddyExecutable) || !filepath.IsAbs(d.CaddyConfig)) {
		return fmt.Errorf("Caddy service executable and configuration paths must be absolute")
	}
	return nil
}
