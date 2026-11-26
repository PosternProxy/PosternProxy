package provisioner

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
)

func ValidateInput(input ProvisionInput) error {
	if err := (agentconfig.Config{ControllerURL: input.ControllerURL, Token: input.AgentToken, ServerID: input.ServerID}).Validate(); err != nil {
		return err
	}
	if input.SSHPort < 1 || input.SSHPort > 65535 {
		return fmt.Errorf("SSH port must be between 1 and 65535")
	}
	for _, field := range []struct{ name, value string }{{"hostname", input.Host}, {"SSH user", input.SSHUser}} {
		if strings.TrimSpace(field.value) == "" || len(field.value) > 255 || strings.IndexFunc(field.value, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0 {
			return fmt.Errorf("invalid %s", field.name)
		}
	}
	if input.SSHAuthMethod != "password" && input.SSHAuthMethod != "key" {
		return fmt.Errorf("unsupported SSH authentication method")
	}
	if input.SSHCredentials == "" || len(input.SSHCredentials) > 1<<20 {
		return fmt.Errorf("SSH credentials are required and must be within size limits")
	}
	switch input.DeploymentType {
	case "", "systemd", "docker":
	default:
		return fmt.Errorf("unsupported deployment type")
	}
	return nil
}
