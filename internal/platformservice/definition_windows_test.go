//go:build windows

package platformservice

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefinitionEncryptsEnrollmentAndRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.enc")
	want := Definition{
		Name: "posternproxy-agent",
		Environment: map[string]string{
			"POSTERNPROXY_AGENT_TOKEN": "disposable-sentinel",
		},
		LogPath: filepath.Join(dir, "agent.log"),
	}
	if err := WriteDefinition(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || bytes.Contains(data, []byte("disposable-sentinel")) {
		t.Fatal("service definition did not protect the enrollment token at rest")
	}
	got, err := ReadDefinition(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("service definition did not round trip")
	}
	data[len(data)/2] ^= 0xff
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDefinition(path); err == nil {
		t.Fatal("tampered service definition accepted")
	}
}

func TestDefinitionRejectsInvalidServiceAndEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name        string
		serviceName string
		environment map[string]string
	}{
		{"unknown service", "other-service", nil},
		{"service name case", "PosternProxy", nil},
		{"empty service", "", nil},
		{"process search path", "posternproxy-agent", map[string]string{"PATH": "C:\\untrusted"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			d := Definition{
				Name:        tc.serviceName,
				Environment: tc.environment,
				LogPath:     filepath.Join(dir, "agent.log"),
			}
			if err := WriteDefinition(filepath.Join(dir, "service.enc"), d); err == nil {
				t.Fatal("invalid service definition accepted")
			}
		})
	}
}

func TestDefinitionRequiresAbsolutePath(t *testing.T) {
	d := Definition{
		Name:    "posternproxy-agent",
		LogPath: filepath.Join(t.TempDir(), "agent.log"),
	}
	if err := WriteDefinition("relative-service.enc", d); err == nil {
		t.Fatal("relative definition write path accepted")
	}
	if _, err := ReadDefinition("relative-service.enc"); err == nil {
		t.Fatal("relative definition read path accepted")
	}
}
