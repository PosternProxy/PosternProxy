package config

import (
	"fmt"
	"net"
	"strconv"
)

func ValidateManagementPort(port int) error {
	if port < 1 || port > 65535 || port == 80 || port == 443 || port == 2019 || port == 8443 {
		return fmt.Errorf("management port must be 1–65535 and differ from 80, 443, 2019 (Caddy admin), and 8443 (TLS bridge)")
	}
	return nil
}

func ValidateManagementAddress(address string) error {
	_, value, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid management listen address: %w", err)
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("management listen port must be numeric")
	}
	return ValidateManagementPort(port)
}
