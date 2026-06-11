package portforward

import (
	"fmt"
	"os/exec"
)

// AddIPTablesRule adds DNAT + MASQUERADE rules so traffic arriving on
// incomingPort is forwarded to forwardHost:forwardPort.
// proto must be "tcp" or "udp" (not "tcp+udp" — the caller expands that).
func AddIPTablesRule(proto, iface string, incomingPort int, forwardHost string, forwardPort int) error {
	dest := fmt.Sprintf("%s:%d", forwardHost, forwardPort)

	// PREROUTING DNAT
	dnatArgs := buildPreRoutingArgs("-A", proto, iface, incomingPort, dest)
	if err := iptables(dnatArgs...); err != nil {
		return fmt.Errorf("iptables DNAT: %w", err)
	}

	// POSTROUTING MASQUERADE so replies reach the original client
	masqArgs := []string{
		"-t", "nat", "-A", "POSTROUTING",
		"-p", proto,
		"-d", forwardHost,
		"--dport", fmt.Sprintf("%d", forwardPort),
		"-j", "MASQUERADE",
	}
	if err := iptables(masqArgs...); err != nil {
		return fmt.Errorf("iptables MASQUERADE: %w", err)
	}

	return nil
}

// RemoveIPTablesRule removes the DNAT + MASQUERADE rules added by AddIPTablesRule.
// Errors are swallowed — rules may not exist if the system was restarted.
func RemoveIPTablesRule(proto, iface string, incomingPort int, forwardHost string, forwardPort int) {
	dest := fmt.Sprintf("%s:%d", forwardHost, forwardPort)

	// Both deletes are best-effort: the rule may not exist if the system was
	// restarted or the rule was manually removed. iptables prints "Bad rule"
	// but returns the same exit code as success, so the error is uninteresting.
	dnatArgs := buildPreRoutingArgs("-D", proto, iface, incomingPort, dest)
	_ = iptables(dnatArgs...)

	masqArgs := []string{
		"-t", "nat", "-D", "POSTROUTING",
		"-p", proto,
		"-d", forwardHost,
		"--dport", fmt.Sprintf("%d", forwardPort),
		"-j", "MASQUERADE",
	}
	_ = iptables(masqArgs...)
}

// buildPreRoutingArgs constructs the PREROUTING DNAT argument list,
// optionally including an input interface filter.
func buildPreRoutingArgs(action, proto, iface string, incomingPort int, dest string) []string {
	args := []string{"-t", "nat", action, "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args,
		"-p", proto,
		"--dport", fmt.Sprintf("%d", incomingPort),
		"-j", "DNAT",
		"--to-destination", dest,
	)
	return args
}

func iptables(args ...string) error {
	out, err := exec.Command("iptables", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
