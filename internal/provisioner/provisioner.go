// Package provisioner handles automated remote server setup via SSH.
// It connects to the target, SFTPs the agent binary, and executes the
// provisioning script, streaming output line-by-line via a channel.
package provisioner

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Result is returned when provisioning finishes (success or failure).
type Result struct {
	Success bool
	Error   string
	Log     string
}

// lineWriter buffers writes and sends complete lines to ch.
type lineWriter struct {
	mu     sync.Mutex
	onLine func(string)
	ch     chan<- string
	buf    strings.Builder
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	for _, b := range p {
		if b == '\n' {
			if lw.onLine != nil {
				lw.onLine(lw.buf.String())
			} else {
				lw.ch <- lw.buf.String()
			}
			lw.buf.Reset()
		} else if lw.buf.Len() < 64<<10 {
			lw.buf.WriteByte(b)
		}
	}
	return len(p), nil
}

func (lw *lineWriter) flush() {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	if s := lw.buf.String(); s != "" {
		if lw.onLine != nil {
			lw.onLine(s)
		} else {
			lw.ch <- s
		}
		lw.buf.Reset()
	}
}

// ProvisionInput carries all inputs needed to provision a server.
type ProvisionInput struct {
	Host           string
	SSHPort        int
	SSHUser        string
	SSHAuthMethod  string
	SSHCredentials string

	ControllerURL    string // wss://host:81 — sent to agent config
	AgentToken       string // pre-shared token
	ServerID         int64  // numeric server ID — required for Docker mode
	AgentBinPath     string // local path to the caddyproxy-agent binary to SCP (systemd mode only)
	ArtifactCacheDir string
	CaddyBinPath     string
	Version          string
	DeploymentType   string // "systemd" (default) | "docker"

	// TOFU: SSHHostKey is the previously stored SHA256 fingerprint.
	// Empty means first connection — accept any key and call OnHostKeyAccepted.
	// Non-empty means verify the key matches; reject if it doesn't.
	SSHHostKey        string
	OnHostKeyAccepted func(fingerprint string)
}

// Provision connects to the server, uploads the agent binary, and runs the
// install script.  Progress lines are sent to logCh as they arrive.
// The returned Result is sent to resultCh when done.
//
// Callers should start this in a goroutine.
func Provision(input ProvisionInput, logCh chan<- string, resultCh chan<- Result) {
	var fullLog strings.Builder
	log := func(msg string) {
		if input.AgentToken != "" {
			msg = strings.ReplaceAll(msg, input.AgentToken, "[redacted]")
		}
		if fullLog.Len() < 1<<20 {
			fullLog.WriteString(msg + "\n")
		}
		logCh <- msg
	}

	finish := func(err error) {
		if err != nil {
			log("ERROR: " + err.Error())
			resultCh <- Result{Success: false, Error: err.Error(), Log: fullLog.String()}
		} else {
			resultCh <- Result{Success: true, Log: fullLog.String()}
		}
	}
	if err := ValidateInput(input); err != nil {
		finish(err)
		return
	}

	if input.DeploymentType == "docker" && !regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$`).MatchString(input.Version) {
		finish(fmt.Errorf("Docker provisioning requires a released controller version and its matching agent image"))
		return
	}
	log(fmt.Sprintf("[%s] Connecting to %s:%d...", timestamp(), input.Host, input.SSHPort))

	c, err := dialSSH(input.Host, input.SSHPort, input.SSHUser, input.SSHAuthMethod, input.SSHCredentials, input.SSHHostKey)
	if err != nil {
		finish(fmt.Errorf("SSH connect: %w", err))
		return
	}
	defer c.close()
	timeout := time.AfterFunc(15*time.Minute, func() { c.close() })
	defer timeout.Stop()

	if c.acceptedFP != "" {
		log(fmt.Sprintf("[%s] Host key trusted (TOFU): %s", timestamp(), c.acceptedFP))
		if input.OnHostKeyAccepted != nil {
			input.OnHostKeyAccepted(c.acceptedFP)
		}
	} else {
		log(fmt.Sprintf("[%s] Host key verified: %s", timestamp(), input.SSHHostKey))
	}

	if input.DeploymentType == "docker" {
		provisionDocker(input, c, log, finish, logCh)
	} else {
		provisionSystemd(input, c, log, finish, logCh)
	}
}

func provisionSystemd(input ProvisionInput, c *sshClient, log func(string), finish func(error), logCh chan<- string) {
	finish(installSystemdAgent(input, c, log))
}

func provisionDocker(input ProvisionInput, c *sshClient, log func(string), finish func(error), logCh chan<- string) {
	finish(installDockerAgent(input, c, log))
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
