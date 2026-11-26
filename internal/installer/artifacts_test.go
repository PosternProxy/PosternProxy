package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadChecksums(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, manifest string
		wantErr        bool
	}{
		{"GNU text", hash + "  caddy\n", false},
		{"GNU binary", hash + " *caddy\n", false},
		{"uppercase", strings.ToUpper(hash) + "  caddy\n", false},
		{"empty", "\n", true},
		{"short hash", "abcd  caddy\n", true},
		{"non hex", strings.Repeat("x", 64) + "  caddy\n", true},
		{"extra field", hash + " caddy extra\n", true},
		{"duplicate", hash + " caddy\n" + hash + " caddy\n", true},
		{"parent traversal", hash + " ../caddy\n", true},
		{"Windows traversal", hash + " ..\\caddy\n", true},
		{"absolute path", hash + " /usr/bin/caddy\n", true},
		{"dot", hash + " .\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readChecksums(strings.NewReader(tc.manifest))
			if (err != nil) != tc.wantErr {
				t.Fatalf("readChecksums() error = %v; wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && got["caddy"] != hash {
				t.Fatalf("checksum = %q; want %q", got["caddy"], hash)
			}
		})
	}
}

func TestLinuxProvisioningDownloadRequiresReleaseAndArchitecture(t *testing.T) {
	for _, tc := range []struct{ version, arch string }{
		{"dev", "arm64"}, {"v1.0.0-dirty", "arm64"},
		{"v1.0.0-3-gabcdef", "arm64"}, {"v1.0.0", "386"},
	} {
		t.Run(tc.version+"/"+tc.arch, func(t *testing.T) {
			destination := t.TempDir()
			if err := StageLinuxAgentAssets(context.Background(), destination, tc.version, tc.arch); err == nil {
				t.Fatal("unsupported download accepted")
			}
			files, err := os.ReadDir(destination)
			if err != nil || len(files) != 0 {
				t.Fatalf("download refusal touched staging: files=%d err=%v", len(files), err)
			}
		})
	}
}

func TestVerifyChecksumDetectsChangedAsset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "caddy")
	data := []byte("verified artifact")
	writeTestFile(t, path, data)
	if err := verifyChecksum(path, testSHA256(data)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, []byte("changed artifact"))
	if err := verifyChecksum(path, testSHA256(data)); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("changed asset error = %v", err)
	}
}

func TestDownload(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		limit  int64
		want   string
	}{
		{"success", http.StatusOK, "artifact", 8, ""},
		{"missing release", http.StatusNotFound, "not found", 32, "HTTP 404"},
		{"oversized", http.StatusOK, "artifact", 7, "size limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			dest := filepath.Join(t.TempDir(), "asset")
			err := download(context.Background(), server.Client(), server.URL, dest, tc.limit)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("download error = %v; want %q", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(dest)
			if err != nil || string(got) != tc.body {
				t.Fatalf("downloaded %q, error %v", got, err)
			}
		})
	}
}

func TestLocalAssets(t *testing.T) {
	for _, suffix := range []string{"", "-linux-amd64"} {
		t.Run("names"+suffix, func(t *testing.T) {
			assets := t.TempDir()
			fixture := minimalELF(elf.EM_X86_64)
			manifest := ""
			for _, name := range []string{"caddy", "posternproxy-agent"} {
				writeTestFile(t, filepath.Join(assets, name+suffix), fixture)
				manifest += testSHA256(fixture) + "  " + name + suffix + "\n"
			}
			writeTestFile(t, filepath.Join(assets, "checksums.txt"), []byte(manifest))
			dest := t.TempDir()
			if err := stageArtifacts(context.Background(), Options{AssetsDir: assets}, "dev", "amd64", dest); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"caddy", "posternproxy-agent"} {
				got, err := os.ReadFile(filepath.Join(dest, name))
				if err != nil || !bytes.Equal(got, fixture) {
					t.Fatalf("staged %s differs from verified source: %v", name, err)
				}
			}
		})
	}
}

func TestLocalAssetsRejectInvalidInputs(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"missing manifest", "checksums.txt"},
		{"missing checksum", "missing checksum"},
		{"missing binary", "open caddy"},
		{"corrupt binary", "SHA-256 mismatch"},
		{"wrong architecture", "linux/amd64"},
		{"not ELF", "not a Linux ELF binary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assets := t.TempDir()
			fixture := minimalELF(elf.EM_X86_64)
			if tc.name == "wrong architecture" {
				fixture = minimalELF(elf.EM_AARCH64)
			}
			if tc.name == "not ELF" {
				fixture = []byte("not an executable")
			}
			manifest := fmt.Sprintf("%s  caddy\n%s  posternproxy-agent\n", testSHA256(fixture), testSHA256(fixture))
			if tc.name == "missing checksum" {
				manifest = testSHA256(fixture) + "  unrelated\n"
			}
			if tc.name != "missing manifest" {
				writeTestFile(t, filepath.Join(assets, "checksums.txt"), []byte(manifest))
			}
			if tc.name != "missing binary" {
				writeTestFile(t, filepath.Join(assets, "caddy"), fixture)
			}
			writeTestFile(t, filepath.Join(assets, "posternproxy-agent"), fixture)
			if tc.name == "corrupt binary" {
				writeTestFile(t, filepath.Join(assets, "caddy"), []byte("changed after manifest creation"))
			}
			err := stageArtifacts(context.Background(), Options{AssetsDir: assets}, "dev", "amd64", t.TempDir())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("stageArtifacts() error = %v; want %q", err, tc.want)
			}
		})
	}
}

func TestOptionsValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    Options
		version string
		wantErr bool
	}{
		{"release build", Options{Port: 81}, "v0.2.0-beta", false},
		{"development local", Options{Port: 81, AssetsDir: "/assets"}, "dev", false},
		{"development explicit release", Options{Port: 81, Release: "v0.2.0-beta"}, "dev", false},
		{"development no source", Options{Port: 81}, "dev", true},
		{"dirty version", Options{Port: 81}, "v0.2.0-beta-dirty", true},
		{"implicit latest", Options{Port: 81, Release: "latest"}, "dev", true},
		{"release traversal", Options{Port: 81, Release: "../v0.2.0"}, "dev", true},
		{"zero port", Options{Port: 0}, "v0.2.0", true},
		{"large port", Options{Port: 65536}, "v0.2.0", true},
		{"HTTP conflict", Options{Port: 80}, "v0.2.0", true},
		{"HTTPS conflict", Options{Port: 443}, "v0.2.0", true},
		{"Caddy admin conflict", Options{Port: 2019}, "v0.2.0", true},
		{"passthrough bridge conflict", Options{Port: 8443}, "v0.2.0", true},
		{"firewall missing SSH", Options{Port: 81, ConfigureFirewall: true}, "v0.2.0", true},
		{"firewall invalid SSH", Options{Port: 81, ConfigureFirewall: true, SSHPort: 65536}, "v0.2.0", true},
		{"firewall custom SSH", Options{Port: 81, ConfigureFirewall: true, SSHPort: 2222}, "v0.2.0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.opts.Validate(tc.version); (err != nil) != tc.wantErr {
				t.Fatalf("Validate(%q) error = %v; wantErr %v", tc.version, err, tc.wantErr)
			}
		})
	}
}

func minimalELF(machine elf.Machine) []byte {
	data := make([]byte, 64)
	copy(data, []byte{0x7f, 'E', 'L', 'F', byte(elf.ELFCLASS64), byte(elf.ELFDATA2LSB), 1})
	binary.LittleEndian.PutUint16(data[16:18], uint16(elf.ET_EXEC))
	binary.LittleEndian.PutUint16(data[18:20], uint16(machine))
	binary.LittleEndian.PutUint32(data[20:24], 1)
	binary.LittleEndian.PutUint16(data[52:54], 64)
	return data
}

func testSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
