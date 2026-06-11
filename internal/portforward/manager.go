// Package portforward manages OS-level port forwarding via iptables or socat.
// Rules are applied when a PortForward record is enabled and removed when it is
// disabled or deleted. On application startup, RestoreAll re-applies rules for
// every enabled record so they survive a reboot.
package portforward

import (
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
)

// ApplyRule installs the forwarding rule described by pf.
// For "tcp+udp" protocol, rules are applied for both TCP and UDP.
func ApplyRule(pf *models.PortForward) error {
	protos := expandProtocol(pf.Protocol)
	switch pf.Method {
	case "iptables":
		for _, proto := range protos {
			if err := AddIPTablesRule(proto, pf.Interface, pf.IncomingPort, pf.ForwardHost, pf.ForwardPort); err != nil {
				return err
			}
		}
	case "socat":
		for _, proto := range protos {
			if err := StartSocat(proto, pf.IncomingPort, pf.ForwardHost, pf.ForwardPort); err != nil {
				return err
			}
		}
	case "nftables":
		return fmt.Errorf("nftables support is not yet implemented")
	default:
		return fmt.Errorf("unknown method: %s", pf.Method)
	}
	return nil
}

// RemoveRule tears down the forwarding rule described by pf.
func RemoveRule(pf *models.PortForward) {
	protos := expandProtocol(pf.Protocol)
	switch pf.Method {
	case "iptables":
		for _, proto := range protos {
			RemoveIPTablesRule(proto, pf.Interface, pf.IncomingPort, pf.ForwardHost, pf.ForwardPort)
		}
	case "socat":
		StopSocat(pf.IncomingPort)
	}
}

// expandProtocol splits "tcp+udp" into ["tcp","udp"]; other values pass through unchanged.
func expandProtocol(proto string) []string {
	if proto == "tcp+udp" {
		return []string{"tcp", "udp"}
	}
	return []string{proto}
}
