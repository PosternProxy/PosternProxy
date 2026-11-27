package installer

import (
	"bufio"
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/pe"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/posternproxy/posternproxy/internal/caddybuild"
)

const releaseBaseURL = "https://github.com/PosternProxy/PosternProxy/releases/download/"

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)
var developmentRevision = regexp.MustCompile(`-[0-9]+-g[0-9a-f]+$`)

func readChecksums(r io.Reader) (map[string]string, error) {
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(io.LimitReader(r, 1<<20))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 || len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid checksum entry")
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return nil, fmt.Errorf("invalid checksum: %w", err)
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
			return nil, fmt.Errorf("invalid checksum filename %q", name)
		}
		if _, exists := checksums[name]; exists {
			return nil, fmt.Errorf("duplicate checksum for %s", name)
		}
		checksums[name] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}
	if len(checksums) == 0 {
		return nil, fmt.Errorf("empty checksum manifest")
	}
	return checksums, nil
}

func download(ctx context.Context, client *http.Client, url, dest string, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", filepath.Base(dest), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d; use a release with installer assets or --assets-dir", filepath.Base(dest), resp.StatusCode)
	}
	return copyBounded(dest, resp.Body, limit)
}

func copyBounded(dest string, src io.Reader, limit int64) error {
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- dest is an install staging path, not request input
	if err != nil {
		return fmt.Errorf("create staged asset: %w", err)
	}
	n, copyErr := io.Copy(f, io.LimitReader(src, limit+1))
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("copy asset: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close staged asset: %w", closeErr)
	}
	if n > limit {
		return fmt.Errorf("asset exceeds size limit")
	}
	return nil
}

func verifyChecksum(path, expected string) error {
	f, err := os.Open(path) // #nosec G304 -- path is an install artifact, not request input
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return fmt.Errorf("hash asset: %w", err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("SHA-256 mismatch for %s", filepath.Base(path))
	}
	return nil
}

func verifyArchitecture(path, arch string) error {
	f, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("%s is not a Linux ELF binary: %w", filepath.Base(path), err)
	}
	defer f.Close()
	want := elf.EM_X86_64
	if arch == "arm64" {
		want = elf.EM_AARCH64
	}
	if f.Machine != want || f.Class != elf.ELFCLASS64 || f.Data != elf.ELFDATA2LSB || (f.Type != elf.ET_EXEC && f.Type != elf.ET_DYN) {
		return fmt.Errorf("%s is not a linux/%s executable", filepath.Base(path), arch)
	}
	return nil
}

func VerifyArchitecture(path, arch string) error { return verifyArchitecture(path, arch) }

func stageArtifacts(ctx context.Context, opts Options, version, arch, dest string) error {
	return stagePlatformArtifacts(ctx, opts, version, "linux", arch, dest)
}

// StageLinuxAgentAssets supplies a controller on another OS with its release's
// verified Linux agent/Caddy pair for native SSH provisioning.
func StageLinuxAgentAssets(ctx context.Context, dest, version, arch string) error {
	if !releaseTag.MatchString(version) || strings.HasSuffix(version, "-dirty") || developmentRevision.MatchString(version) {
		return fmt.Errorf("development controllers require matching Linux provisioning assets supplied locally")
	}
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported Linux agent architecture")
	}
	if err := stagePlatformArtifacts(ctx, Options{Release: version}, version, "linux", arch, dest); err != nil {
		return err
	}
	return caddybuild.VerifyArtifact(filepath.Join(dest, "caddy"))
}

func stagePlatformArtifacts(ctx context.Context, opts Options, version, platform, arch, dest string) error {
	client := &http.Client{
		Timeout: 10 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 || req.URL.Scheme != "https" {
				return fmt.Errorf("unsafe asset redirect")
			}
			return nil
		},
	}
	tag := opts.Release
	if tag == "" {
		tag = version
	}
	manifestPath := filepath.Join(dest, "checksums.txt")
	if opts.AssetsDir != "" {
		manifestPath = filepath.Join(opts.AssetsDir, "checksums.txt")
	} else if err := download(ctx, client, releaseBaseURL+tag+"/checksums.txt", manifestPath, 1<<20); err != nil {
		return err
	}
	manifest, err := os.Open(manifestPath) // #nosec G304 -- manifest path is install-controlled, not request input
	if err != nil {
		return fmt.Errorf("open checksums.txt: %w", err)
	}
	checksums, readErr := readChecksums(manifest)
	closeErr := manifest.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, binary := range []string{"caddy", "posternproxy-agent"} {
		suffix := ""
		if platform == "windows" {
			suffix = ".exe"
		}
		name := binary + "-" + platform + "-" + arch + suffix
		if opts.AssetsDir != "" && checksums[name] == "" {
			name = binary + suffix
		}
		expected := checksums[name]
		if expected == "" {
			return fmt.Errorf("missing checksum for %s", name)
		}
		path := filepath.Join(dest, binary+suffix)
		if opts.AssetsDir != "" {
			src, err := os.Open(filepath.Join(opts.AssetsDir, name)) // #nosec G304 -- assets dir is an operator-provided install path, not a network request
			if err != nil {
				return fmt.Errorf("open %s: %w", name, err)
			}
			copyErr := copyBounded(path, src, 512<<20)
			closeErr := src.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		} else if err := download(ctx, client, releaseBaseURL+tag+"/"+name, path, 512<<20); err != nil {
			return err
		}
		if err := verifyChecksum(path, expected); err != nil {
			return err
		}
		if err := verifyPlatformArchitecture(path, platform, arch); err != nil {
			return err
		}
		if err := os.Chmod(path, 0700); err != nil { // #nosec G302 -- installed binary needs the owner-execute bit (0700)
			return err
		}
	}
	return nil
}

func verifyPlatformArchitecture(path, platform, arch string) error {
	if platform == "linux" {
		return verifyArchitecture(path, arch)
	}
	if platform != "windows" || arch != "amd64" {
		return fmt.Errorf("unsupported installation platform")
	}
	f, err := pe.Open(path)
	if err != nil {
		return fmt.Errorf("asset is not a Windows PE executable: %w", err)
	}
	defer f.Close()
	if f.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || f.FileHeader.Characteristics&pe.IMAGE_FILE_EXECUTABLE_IMAGE == 0 || f.FileHeader.Characteristics&pe.IMAGE_FILE_DLL != 0 {
		return fmt.Errorf("asset is not a Windows amd64 executable")
	}
	if _, ok := f.OptionalHeader.(*pe.OptionalHeader64); !ok {
		return fmt.Errorf("asset is not a 64-bit Windows executable")
	}
	return nil
}
