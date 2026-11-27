package caddybuild

import (
	"debug/buildinfo"
	"fmt"
	"strings"
)

func VerifyArtifact(path string) error {
	pins := make(map[string]string)
	for _, line := range strings.Split(Script, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			pins[key] = strings.TrimSpace(value)
		}
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Caddy build information: %w", err)
	}
	if info.GoVersion != pins["CADDY_GO_TOOLCHAIN"] {
		return fmt.Errorf("caddy artifact does not use the pinned patched Go toolchain")
	}
	want := map[string]string{"github.com/caddyserver/caddy/v2": pins["CADDY_VERSION"], "github.com/mholt/caddy-l4": pins["CADDY_L4_VERSION"], "github.com/mholt/caddy-ratelimit": pins["CADDY_RATELIMIT_VERSION"], "github.com/PosternProxy/ntlm-transport": pins["CADDY_NTLM_VERSION"]}
	for _, dep := range info.Deps {
		if version, ok := want[dep.Path]; ok && dep.Replace == nil && dep.Version == version && version != "" {
			delete(want, dep.Path)
		}
	}
	for _, replacement := range strings.Fields(strings.Trim(pins["CADDY_SECURITY_REPLACEMENTS"], "'")) {
		module, target, ok := strings.Cut(replacement, "=")
		if !ok {
			return fmt.Errorf("invalid security dependency contract")
		}
		targetPath, version, ok := strings.Cut(target, "@")
		if !ok {
			return fmt.Errorf("invalid security dependency contract")
		}
		matched := false
		for _, dep := range info.Deps {
			if dep.Path == module && dep.Replace != nil && dep.Replace.Path == targetPath && dep.Replace.Version == version {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("caddy artifact lacks a pinned security dependency update")
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("caddy artifact does not match the pinned core/plugin contract")
	}
	return nil
}
