// Command check runs every project check from one place: formatting, vet,
// static analysis, complexity and file-size limits, tests, security scanners,
// and the frontend pipeline. It replaces a task runner so the identical command
// works on Windows, Linux, and CI with nothing but the Go toolchain.
//
// The static analyzers run against every target GOOS (see analysisGOOS), so the
// _windows.go files and the !windows files are both checked no matter which OS
// runs the command. go test still runs only for the host OS.
//
// Usage:
//
//	go run ./scripts/check            # everything
//	go run ./scripts/check -go        # Go checks only
//	go run ./scripts/check -frontend  # frontend checks only
//	go run ./scripts/check -fast      # skip checks that fetch tools from the network
//	go run ./scripts/check -race      # run Go tests with the race detector
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Tool versions, pinned so the check is reproducible in CI (ST-1 / SE-3).
// Bump these deliberately.
const (
	staticcheckVer = "v0.8.2"
	gocycloVer     = "v0.6.0"
	gosecVer       = "v2.30.0"
	gitleaksVer    = "v8.30.1"
	govulncheckVer = "v1.9.0"
)

// Maintainability limits (MT-2). maxComplexity is an adoption ceiling set at
// today's most complex function; lower it toward ~15 as the hotspots in
// installer/ and services/ are refactored, so complexity can only improve.
const (
	maxFileLines  = 700
	maxComplexity = 62
)

// Go code the checks apply to. Excludes the vendored frontend node_modules and
// throwaway tmp/ trees that live under the module root.
var goPkgs = []string{"./cmd/...", "./internal/..."}

// goDirs mirrors goPkgs for tools that take directories instead of packages.
var goDirs = []string{"cmd", "internal"}

// analysisGOOS are the platforms the static analyzers run against so both the
// Windows-only and the !windows files are checked from any host.
var analysisGOOS = []string{"linux", "windows"}

type step struct {
	name  string
	dir   string   // working directory; "" means repo root
	argv  []string // external command to run; empty when fn is set
	fn    func() error
	fetch bool     // reaches the network to fetch a tool or advisory data
	env   []string // extra environment (e.g. "GOOS=linux") that overrides the inherited one
}

func (s step) run() error {
	if s.fn != nil {
		return s.fn()
	}
	cmd := exec.Command(s.argv[0], s.argv[1:]...)
	cmd.Dir = s.dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(s.env) > 0 {
		cmd.Env = withEnv(os.Environ(), s.env)
	}
	return cmd.Run()
}

func main() {
	var goOnly, feOnly, fast, race bool
	flag.BoolVar(&goOnly, "go", false, "run only Go checks")
	flag.BoolVar(&feOnly, "frontend", false, "run only frontend checks")
	flag.BoolVar(&fast, "fast", false, "skip checks that fetch tools or advisory data from the network")
	flag.BoolVar(&race, "race", false, "run Go tests with the race detector (needs a C toolchain)")
	flag.Parse()

	runGo := !feOnly
	runFE := !goOnly

	testArgs := []string{"test", "-count=1"}
	if race {
		testArgs = append(testArgs, "-race")
	}
	testArgs = append(testArgs, goPkgs...)

	var steps []step
	if runGo {
		// Build the external analyzers as host-native binaries so they can be
		// run under any GOOS; `go run tool@ver` would cross-compile the tool.
		var staticcheckBin, gosecBin string
		if !fast {
			var err error
			fmt.Println("== install analysis tools")
			if staticcheckBin, err = installTool("honnef.co/go/tools/cmd/staticcheck", staticcheckVer, "staticcheck"); err != nil {
				fatal(err)
			}
			if gosecBin, err = installTool("github.com/securego/gosec/v2/cmd/gosec", gosecVer, "gosec"); err != nil {
				fatal(err)
			}
		}

		steps = append(steps, step{name: "gofmt", fn: gofmtCheck})
		for _, goos := range analysisGOOS {
			steps = append(steps,
				step{name: "go vet (" + goos + ")", env: []string{"GOOS=" + goos}, argv: concat([]string{"go", "vet"}, goPkgs)},
				step{name: "staticcheck (" + goos + ")", fetch: true, env: []string{"GOOS=" + goos}, argv: concat([]string{staticcheckBin}, goPkgs)},
			)
		}
		steps = append(steps,
			step{name: "gocyclo", fetch: true, argv: []string{"go", "run", "github.com/fzipp/gocyclo/cmd/gocyclo@" + gocycloVer, "-over", strconv.Itoa(maxComplexity), "-ignore", `_test\.go`, "cmd", "internal"}},
			step{name: "file size", fn: fileSizeCheck},
			step{name: "go test", argv: concat([]string{"go"}, testArgs)},
		)
		// G104 (unchecked errors) fires on every deferred Close/io.Copy and
		// drowns the real findings; drop it and keep the rest of the ruleset.
		for _, goos := range analysisGOOS {
			steps = append(steps, step{name: "gosec (" + goos + ")", fetch: true, env: []string{"GOOS=" + goos}, argv: concat([]string{gosecBin, "-quiet", "-exclude=G104"}, goPkgs)})
		}
		steps = append(steps,
			step{name: "govulncheck", fetch: true, argv: concat([]string{"go", "run", "golang.org/x/vuln/cmd/govulncheck@" + govulncheckVer}, goPkgs)},
			step{name: "gitleaks", fetch: true, argv: []string{"go", "run", "github.com/zricethezav/gitleaks/v8@" + gitleaksVer, "detect", "--no-banner", "--redact", "-c", ".gitleaks.toml"}},
		)
	}
	if runFE {
		steps = append(steps,
			step{name: "npm ci", dir: "frontend", fetch: true, argv: []string{"npm", "ci"}},
			step{name: "type-check", dir: "frontend", argv: []string{"npm", "run", "type-check"}},
			step{name: "lint", dir: "frontend", argv: []string{"npm", "run", "lint:check"}},
			step{name: "build", dir: "frontend", argv: []string{"npm", "run", "build"}},
			step{name: "npm audit", dir: "frontend", fetch: true, argv: []string{"npm", "audit", "--audit-level=moderate"}},
		)
	}

	start := time.Now()
	for _, s := range steps {
		if fast && s.fetch {
			fmt.Printf("== %-12s  skipped (-fast)\n", s.name)
			continue
		}
		fmt.Printf("== %s\n", s.name)
		if err := s.run(); err != nil {
			fmt.Fprintf(os.Stderr, "\nFAILED: %s: %v\n", s.name, err)
			os.Exit(1)
		}
	}
	fmt.Printf("\nAll checks passed in %s.\n", time.Since(start).Round(time.Second))
}

// concat flattens a fixed prefix and a slice of arguments into one argv.
func concat(prefix, rest []string) []string {
	return append(append([]string{}, prefix...), rest...)
}

// fatal prints the error and exits nonzero.
func fatal(err error) {
	fmt.Fprintf(os.Stderr, "\nFAILED: %v\n", err)
	os.Exit(1)
}

// installTool builds a Go tool as a host-native binary with `go install` and
// returns its path, so it can analyze any GOOS (plain `go run tool@ver` would
// cross-compile the tool itself for the target GOOS and fail to execute).
func installTool(pkg, version, bin string) (string, error) {
	cmd := exec.Command("go", "install", pkg+"@"+version)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("install %s@%s: %w", pkg, version, err)
	}
	dir := goEnv("GOBIN")
	if dir == "" {
		dir = filepath.Join(goEnv("GOPATH"), "bin")
	}
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return filepath.Join(dir, bin), nil
}

// goEnv returns a single trimmed `go env` value.
func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// withEnv returns base with any keys that overrides sets removed, then the
// overrides appended, so the override values win regardless of how the platform
// handles duplicate keys.
func withEnv(base, overrides []string) []string {
	drop := map[string]bool{}
	for _, kv := range overrides {
		if i := strings.IndexByte(kv, '='); i > 0 {
			drop[kv[:i]] = true
		}
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i > 0 && drop[kv[:i]] {
			continue
		}
		out = append(out, kv)
	}
	return append(out, overrides...)
}

// gofmtCheck fails (without rewriting) when any Go file is not gofmt-clean.
func gofmtCheck() error {
	out, err := exec.Command("gofmt", append([]string{"-l"}, goDirs...)...).Output()
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(out)) > 0 {
		return fmt.Errorf("not gofmt-clean:\n%s\nfix with: gofmt -w %s", out, strings.Join(goDirs, " "))
	}
	return nil
}

// fileSizeCheck fails when a non-test Go file exceeds maxFileLines.
func fileSizeCheck() error {
	var over []string
	for _, root := range goDirs {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if n := bytes.Count(b, []byte{'\n'}); n > maxFileLines {
				over = append(over, fmt.Sprintf("  %s (%d lines)", filepath.ToSlash(p), n))
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(over) > 0 {
		return fmt.Errorf("Go files over %d lines:\n%s", maxFileLines, strings.Join(over, "\n"))
	}
	return nil
}
