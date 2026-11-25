package provisioner

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sshClient wraps an active SSH connection and provides helpers.
type sshClient struct {
	client     *ssh.Client
	acceptedFP string // non-empty when a TOFU key was accepted (knownFingerprint was empty)
}

// dialSSH opens an SSH connection to host:port using either password or private-key auth.
//
// knownFingerprint is the SHA256 fingerprint stored from a previous connection.
// If empty, TOFU applies: the remote key is accepted unconditionally on first use and
// its fingerprint is stored in sshClient.acceptedFP for the caller to persist.
// If non-empty, the remote key must match exactly — a mismatch is a fatal error.
func dialSSH(host string, port int, user, authMethod, credentials, knownFingerprint string) (*sshClient, error) {
	var auth ssh.AuthMethod
	switch authMethod {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(credentials))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	default: // "password"
		auth = ssh.Password(credentials)
	}

	var acceptedFP string
	hostKeyCallback := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		if knownFingerprint == "" {
			// First connection — trust and record.
			acceptedFP = fp
			return nil
		}
		if fp != knownFingerprint {
			return fmt.Errorf("SSH host key mismatch: got %s, expected %s — possible MITM attack; clear the stored key in the server settings to re-trust", fp, knownFingerprint)
		}
		return nil
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	return &sshClient{client: client, acceptedFP: acceptedFP}, nil
}

// run executes a command, streaming combined stdout+stderr to out.
// Returns the exit error if the command fails.
func (c *sshClient) run(cmd string, out io.Writer) error {
	sess, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	sess.Stdout = out
	sess.Stderr = out
	return sess.Run(cmd)
}

// output keeps remote diagnostics separate from machine-readable stdout.
func (c *sshClient) output(cmd string) ([]byte, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()
	return sess.Output(cmd)
}

// upload copies the contents of src to remotePath on the server via SFTP.
func (c *sshClient) upload(remotePath string, src io.Reader) (err error) {
	sftp, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("sftp client: %w", err)
	}
	defer sftp.Close()

	f, err := sftp.Create(remotePath)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", remotePath, err)
	}
	defer func() {
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close remote upload: %w", closeErr)
		}
	}()
	if err := f.Chmod(0600); err != nil {
		return fmt.Errorf("protect remote upload: %w", err)
	}

	if _, err := io.Copy(f, src); err != nil {
		return fmt.Errorf("sftp write: %w", err)
	}
	return nil
}

func (c *sshClient) close() {
	c.client.Close()
}
