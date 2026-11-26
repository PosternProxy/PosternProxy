package config

import "testing"

func TestValidateManagementAddress(t *testing.T) {
	for _, tc := range []struct {
		address string
		valid   bool
	}{
		{":81", true},
		{"localhost:18181", true},
		{"127.0.0.1:81", true},
		{"[::1]:81", true},
		{":65535", true},
		{":80", false},
		{"127.0.0.1:443", false},
		{"localhost:2019", false},
		{"[::1]:8443", false},
		{":http", false},
		{":65536", false},
		{":-1", false},
		{"", false},
		{"localhost", false},
		{"127.0.0.1:", false},
		{"::1:81", false},
		{"[::1:81", false},
		{"http://localhost:81", false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			err := ValidateManagementAddress(tc.address)
			if tc.valid && err != nil {
				t.Fatalf("valid management address %q rejected: %v", tc.address, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("invalid management address %q accepted", tc.address)
			}
		})
	}
}
