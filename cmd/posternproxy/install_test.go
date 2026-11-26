package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestInstallHelp(t *testing.T) {
	var output bytes.Buffer
	_, err := parseInstallOptions([]string{"--help"}, &output)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
	for _, option := range []string{"assets-dir", "release", "configure-firewall", "ssh-port", "skip-packages"} {
		if !strings.Contains(output.String(), option) {
			t.Errorf("help missing %s", option)
		}
	}
}

func TestParseInstallOptions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"local defaults", []string{"--assets-dir", "/assets"}, false},
		{"custom options", []string{"--release", "v0.2.0-beta", "--port", "8081", "--configure-firewall", "--ssh-port", "2222", "--skip-packages"}, false},
		{"unexpected argument", []string{"--assets-dir", "/assets", "extra"}, true},
		{"unknown flag", []string{"--not-a-flag"}, true},
		{"missing SSH port", []string{"--assets-dir", "/assets", "--configure-firewall"}, true},
		{"invalid port", []string{"--assets-dir", "/assets", "--port", "80"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			opts, err := parseInstallOptions(tc.args, &output)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseInstallOptions() error = %v; wantErr %v", err, tc.wantErr)
			}
			if tc.name == "local defaults" && (opts.Port != 81 || opts.ConfigureFirewall || opts.AssetsDir != "/assets") {
				t.Fatalf("unexpected defaults: %+v", opts)
			}
			if tc.name == "custom options" && (opts.Port != 8081 || opts.SSHPort != 2222 || !opts.ConfigureFirewall || !opts.SkipPackages || opts.Release != "v0.2.0-beta") {
				t.Fatalf("custom flags were not retained: %+v", opts)
			}
		})
	}
}
