package platformservice

const CaddyName = "posternproxy-caddy"

type Definition struct {
	Name            string
	Environment     map[string]string
	CaddyExecutable string
	CaddyConfig     string
	LogPath         string
	BootstrapPath   string
}
