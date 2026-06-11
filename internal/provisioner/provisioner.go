// Package provisioner handles automated remote server setup via SSH.
// It connects to the target, SFTPs the agent binary, and executes the
// provisioning script, streaming output line-by-line via a channel.
package provisioner

import (
	"fmt"
	"io"
	"os"
	"strings"
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
	ch  chan<- string
	buf strings.Builder
}

func (lw *lineWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			lw.ch <- lw.buf.String()
			lw.buf.Reset()
		} else {
			lw.buf.WriteByte(b)
		}
	}
	return len(p), nil
}

func (lw *lineWriter) flush() {
	if s := lw.buf.String(); s != "" {
		lw.ch <- s
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

	ControllerURL  string // wss://host:81 — sent to agent config
	AgentToken     string // pre-shared token
	ServerID       int64  // numeric server ID — required for Docker mode
	AgentBinPath   string // local path to the caddyproxy-agent binary to SCP (systemd mode only)
	DeploymentType string // "systemd" (default) | "docker"

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
		fullLog.WriteString(msg + "\n")
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

	log(fmt.Sprintf("[%s] Connecting to %s:%d...", timestamp(), input.Host, input.SSHPort))

	c, err := dialSSH(input.Host, input.SSHPort, input.SSHUser, input.SSHAuthMethod, input.SSHCredentials, input.SSHHostKey)
	if err != nil {
		finish(fmt.Errorf("SSH connect: %w", err))
		return
	}
	defer c.close()

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
	// Upload agent binary if a local path is provided.
	if input.AgentBinPath != "" {
		log(fmt.Sprintf("[%s] Uploading agent binary...", timestamp()))
		f, err := os.Open(input.AgentBinPath)
		if err != nil {
			finish(fmt.Errorf("open agent binary: %w", err))
			return
		}
		defer f.Close()
		if err := c.upload("/tmp/caddyproxy-agent", f); err != nil {
			finish(fmt.Errorf("upload agent binary: %w", err))
			return
		}
		log(fmt.Sprintf("[%s] Agent binary uploaded.", timestamp()))
	} else {
		// No binary provided — write a stub so the script doesn't fail.
		log(fmt.Sprintf("[%s] No agent binary path provided; skipping upload.", timestamp()))
		if err := c.upload("/tmp/caddyproxy-agent", strings.NewReader("")); err != nil {
			log("[warning] Could not upload placeholder: " + err.Error())
		}
	}

	log(fmt.Sprintf("[%s] Writing provisioning script...", timestamp()))
	scriptContent := fmt.Sprintf("CONTROLLER_URL=%q\nAGENT_TOKEN=%q\n%s",
		input.ControllerURL, input.AgentToken, installScript)

	if err := c.upload("/tmp/posternproxy-install.sh", strings.NewReader(scriptContent)); err != nil {
		finish(fmt.Errorf("upload script: %w", err))
		return
	}

	log(fmt.Sprintf("[%s] Running provisioning script (this may take several minutes)...", timestamp()))
	lw := &lineWriter{ch: logCh}
	var scriptOut io.Writer = lw

	scriptErr := c.run("bash /tmp/posternproxy-install.sh", scriptOut)
	lw.flush()

	if scriptErr != nil {
		finish(fmt.Errorf("provisioning script: %w", scriptErr))
		return
	}

	log(fmt.Sprintf("[%s] Provisioning complete!", timestamp()))
	finish(nil)
}

func provisionDocker(input ProvisionInput, c *sshClient, log func(string), finish func(error), logCh chan<- string) {
	log(fmt.Sprintf("[%s] Writing Docker provisioning script...", timestamp()))
	scriptContent := fmt.Sprintf(
		"CONTROLLER_URL=%q\nAGENT_TOKEN=%q\nSERVER_ID=%q\n%s",
		input.ControllerURL, input.AgentToken,
		fmt.Sprintf("%d", input.ServerID),
		dockerInstallScript,
	)

	if err := c.upload("/tmp/posternproxy-install.sh", strings.NewReader(scriptContent)); err != nil {
		finish(fmt.Errorf("upload script: %w", err))
		return
	}

	log(fmt.Sprintf("[%s] Running Docker provisioning script...", timestamp()))
	lw := &lineWriter{ch: logCh}
	var scriptOut io.Writer = lw

	scriptErr := c.run("bash /tmp/posternproxy-install.sh", scriptOut)
	lw.flush()

	if scriptErr != nil {
		finish(fmt.Errorf("provisioning script: %w", scriptErr))
		return
	}

	log(fmt.Sprintf("[%s] Provisioning complete!", timestamp()))
	finish(nil)
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
