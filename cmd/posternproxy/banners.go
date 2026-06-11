package main

import (
	"fmt"
	"os"
)

// Banner output goes to stderr so it is always visible regardless of log
// level or whether stdout is being captured.

func printSetupBanner(email, password string) {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════════════════╗")
	fmt.Fprintln(os.Stderr, "║           POSTERNPROXY — INITIAL SETUP               ║")
	fmt.Fprintln(os.Stderr, "╟──────────────────────────────────────────────────────╢")
	fmt.Fprintf(os.Stderr, "║  Email:    %-41s║\n", email)
	fmt.Fprintf(os.Stderr, "║  Password: %-41s║\n", password)
	fmt.Fprintln(os.Stderr, "╟──────────────────────────────────────────────────────╢")
	fmt.Fprintln(os.Stderr, "║  Change this password immediately after first login. ║")
	fmt.Fprintln(os.Stderr, "║  Set POSTERNPROXY_ADMIN_PASSWORD to silence this.    ║")
	fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════════════════╝")
	fmt.Fprintln(os.Stderr, "")
}

func printResetBanner(email, password string) {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════════════════╗")
	fmt.Fprintln(os.Stderr, "║           POSTERNPROXY — PASSWORD RESET              ║")
	fmt.Fprintln(os.Stderr, "╟──────────────────────────────────────────────────────╢")
	fmt.Fprintf(os.Stderr, "║  Email:    %-41s║\n", email)
	fmt.Fprintf(os.Stderr, "║  Password: %-41s║\n", password)
	fmt.Fprintln(os.Stderr, "╟──────────────────────────────────────────────────────╢")
	fmt.Fprintln(os.Stderr, "║  Change this password after logging in.              ║")
	fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════════════════╝")
	fmt.Fprintln(os.Stderr, "")
}
