package provisioner

import "testing"

func TestRemoteArchitecture(t *testing.T) {
	for _, tc := range []struct{ output, want string }{
		{"Linux x86_64\n", "amd64"}, {"Linux aarch64\r\n", "arm64"},
		{"Linux amd64", "amd64"}, {"Linux arm64", "arm64"},
		{"Darwin arm64", ""}, {"Linux i386", ""}, {"banner\nLinux x86_64", ""},
	} {
		t.Run(tc.output, func(t *testing.T) {
			got, err := remoteArchitecture(tc.output)
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}
