package buildinfo

import (
	"github.com/rs/zerolog"
	"io"
	"runtime/debug"
)

// Version returns a version string derived from VCS build info embedded by
// the Go toolchain. If the binary was built outside a VCS checkout (or info
// is unavailable), it returns "dev".
//
// Callers in cmd/ packages declare a package-level Version variable
// initialised to this function's return value; the linker can override it
// via -ldflags "-X main.Version=...".
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var vcs, rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs":
			vcs = s.Value
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if vcs == "git" && rev != "" {
		if len(rev) > 8 {
			rev = rev[:8]
		}
		return rev + dirty
	}
	return "dev"
}

func WriteVersion(w io.Writer, name, version string) {
	logger := zerolog.New(w)
	logger.Info().Str("version", version).Msg(name)
}
