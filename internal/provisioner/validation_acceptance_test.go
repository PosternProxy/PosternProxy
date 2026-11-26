package provisioner

import (
	"strings"
	"testing"
)

func validD1ProvisionInput() ProvisionInput {
	return ProvisionInput{Host: "node.example.com", SSHPort: 2222, SSHUser: "root", SSHAuthMethod: "password", SSHCredentials: "disposable-test-password", ControllerURL: "https://controller.example.com", AgentToken: "disposable-agent-token", ServerID: 2, DeploymentType: "systemd"}
}

func TestValidateInputRejectsInvalidProvisioningBoundary(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ProvisionInput)
	}{
		{"negative server ID", func(in *ProvisionInput) { in.ServerID = -1 }},
		{"missing server ID", func(in *ProvisionInput) { in.ServerID = 0 }},
		{"local server ID", func(in *ProvisionInput) { in.ServerID = 1 }},
		{"missing controller URL", func(in *ProvisionInput) { in.ControllerURL = "" }},
		{"relative controller URL", func(in *ProvisionInput) { in.ControllerURL = "controller.example.com" }},
		{"missing controller host", func(in *ProvisionInput) { in.ControllerURL = "https://" }},
		{"unsupported controller scheme", func(in *ProvisionInput) { in.ControllerURL = "ftp://controller.example.com" }},
		{"controller URL credentials", func(in *ProvisionInput) { in.ControllerURL = "https://user:secret@controller.example.com" }},
		{"controller URL query", func(in *ProvisionInput) { in.ControllerURL = "https://controller.example.com?token=secret" }},
		{"controller URL fragment", func(in *ProvisionInput) { in.ControllerURL = "https://controller.example.com#fragment" }},
		{"missing agent token", func(in *ProvisionInput) { in.AgentToken = "" }},
		{"negative SSH port", func(in *ProvisionInput) { in.SSHPort = -1 }},
		{"missing SSH port", func(in *ProvisionInput) { in.SSHPort = 0 }},
		{"SSH port exceeds range", func(in *ProvisionInput) { in.SSHPort = 65536 }},
		{"missing hostname", func(in *ProvisionInput) { in.Host = "" }},
		{"hostname newline", func(in *ProvisionInput) { in.Host = "node.example.com\nother" }},
		{"hostname carriage return", func(in *ProvisionInput) { in.Host = "node.example.com\rother" }},
		{"hostname tab", func(in *ProvisionInput) { in.Host = "node.example.com\tother" }},
		{"hostname NUL", func(in *ProvisionInput) { in.Host = "node.example.com\x00other" }},
		{"missing SSH user", func(in *ProvisionInput) { in.SSHUser = "" }},
		{"SSH user newline", func(in *ProvisionInput) { in.SSHUser = "root\nother" }},
		{"SSH user tab", func(in *ProvisionInput) { in.SSHUser = "root\tother" }},
		{"SSH user NUL", func(in *ProvisionInput) { in.SSHUser = "root\x00other" }},
		{"unsupported deployment", func(in *ProvisionInput) { in.DeploymentType = "unknown" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := validD1ProvisionInput()
			tc.change(&input)
			if err := ValidateInput(input); err == nil {
				t.Fatal("invalid provisioning input accepted")
			}
		})
	}
}

func TestValidateInputAcceptsSupportedProvisioningInputs(t *testing.T) {
	for _, scheme := range []string{"http", "https", "ws", "wss"} {
		for _, deployment := range []string{"systemd", "docker"} {
			t.Run(scheme+"/"+deployment, func(t *testing.T) {
				input := validD1ProvisionInput()
				input.ControllerURL = scheme + "://controller.example.com:81"
				input.DeploymentType = deployment
				if err := ValidateInput(input); err != nil {
					t.Fatalf("valid input rejected: %v", err)
				}
			})
		}
	}
}

func TestValidateInputAcceptsIPv6Addresses(t *testing.T) {
	input := validD1ProvisionInput()
	input.Host = "2001:db8::1"
	input.ControllerURL = "https://[2001:db8::2]:81"
	if err := ValidateInput(input); err != nil {
		t.Fatal(err)
	}
}
func TestValidateInputTreatsCredentialShellCharactersAsData(t *testing.T) {
	input := validD1ProvisionInput()
	input.AgentToken = "token-'\";$() `literal`"
	input.SSHCredentials = "password-'\";$() `literal`"
	if err := ValidateInput(input); err != nil {
		t.Fatal(err)
	}
}
func TestValidateInputErrorsDoNotExposeSecrets(t *testing.T) {
	for _, url := range []string{"https://user:URL_PASSWORD_SENTINEL@controller.example.com", "https://controller.example.com?token=URL_QUERY_SENTINEL"} {
		input := validD1ProvisionInput()
		input.ControllerURL = url
		input.AgentToken = "AGENT_TOKEN_SENTINEL"
		input.SSHCredentials = "SSH_CREDENTIAL_SENTINEL"
		err := ValidateInput(input)
		if err == nil {
			t.Fatal("invalid controller URL accepted")
		}
		for _, secret := range []string{"URL_PASSWORD_SENTINEL", "URL_QUERY_SENTINEL", "AGENT_TOKEN_SENTINEL", "SSH_CREDENTIAL_SENTINEL"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("validation disclosed %s", secret)
			}
		}
	}
}
