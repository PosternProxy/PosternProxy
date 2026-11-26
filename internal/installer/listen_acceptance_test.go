package installer

import (
	"os"
	"testing"
)

func TestExistingConfigRejectsReservedManagementPorts(t *testing.T) {
	for _, address := range []string{":80", "127.0.0.1:443", "localhost:2019", "[::1]:8443"} {
		t.Run(address, func(t *testing.T) {
			i, _, commands := fixtureInstallation(t)
			configPath := i.path("/etc/posternproxy/config.env")
			config := "POSTERNPROXY_JWT_SECRET=existing-secret\nPOSTERNPROXY_LISTEN='" + address + "'\n"
			if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			if err := i.readExistingConfig(); err == nil {
				t.Errorf("reserved management address %q accepted from existing configuration", address)
			}
			got, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != config {
				t.Fatal("existing configuration changed during validation")
			}
			if len(*commands) != 0 {
				t.Fatalf("system commands ran before rejecting existing configuration: %v", *commands)
			}
		})
	}
}
