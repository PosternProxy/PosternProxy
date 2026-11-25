// Package agentconfig validates the identity shared by enrollment and installers.
package agentconfig

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

type Config struct {
	ControllerURL string `json:"controller_url"`
	Token         string `json:"agent_token"`
	ServerID      int64  `json:"server_id"`
}

func (c Config) Validate() error {
	if c.ServerID <= 1 {
		return fmt.Errorf("a remote server ID greater than 1 is required")
	}
	if err := ValidateURL(c.ControllerURL); err != nil {
		return err
	}
	if len(c.Token) > 4096 || strings.TrimSpace(c.Token) == "" || strings.IndexFunc(c.Token, unicode.IsControl) >= 0 {
		return fmt.Errorf("a valid agent token is required")
	}
	return nil
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("controller URL must be an absolute base URL without credentials, query or fragment")
	}
	switch u.Scheme {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("controller URL must use http, https, ws or wss")
	}
	return nil
}
