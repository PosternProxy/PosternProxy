package portforward

import (
	"fmt"
	"os/exec"
	"sync"
)

// socatManager tracks running socat child processes keyed by incoming port.
type socatManager struct {
	mu    sync.Mutex
	procs map[int]*exec.Cmd
}

var socatMgr = &socatManager{procs: make(map[int]*exec.Cmd)}

// StartSocat launches a socat process that listens on incomingPort and
// proxies traffic to forwardHost:forwardPort.
// proto must be "tcp" or "udp".
func StartSocat(proto string, incomingPort int, forwardHost string, forwardPort int) error {
	socatMgr.mu.Lock()
	defer socatMgr.mu.Unlock()

	if _, running := socatMgr.procs[incomingPort]; running {
		return nil // already running on this port
	}

	var listenAddr, connectAddr string
	switch proto {
	case "udp":
		listenAddr = fmt.Sprintf("UDP-LISTEN:%d,fork,reuseaddr", incomingPort)
		connectAddr = fmt.Sprintf("UDP:%s:%d", forwardHost, forwardPort)
	default: // tcp
		listenAddr = fmt.Sprintf("TCP-LISTEN:%d,fork,reuseaddr", incomingPort)
		connectAddr = fmt.Sprintf("TCP:%s:%d", forwardHost, forwardPort)
	}

	cmd := exec.Command("socat", listenAddr, connectAddr)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start socat on port %d: %w", incomingPort, err)
	}

	socatMgr.procs[incomingPort] = cmd

	// Reap the process when it exits so we don't accumulate zombies.
	// Wait's error is the exit status — we don't care what it was, we just
	// need to unblock the kernel's bookkeeping.
	go func() {
		_ = cmd.Wait()
		socatMgr.mu.Lock()
		delete(socatMgr.procs, incomingPort)
		socatMgr.mu.Unlock()
	}()

	return nil
}

// StopSocat terminates the socat process listening on incomingPort.
func StopSocat(incomingPort int) {
	socatMgr.mu.Lock()
	defer socatMgr.mu.Unlock()

	if cmd, ok := socatMgr.procs[incomingPort]; ok {
		// Kill returns an error if the process has already exited; that's
		// fine — the goroutine above will reap it either way.
		_ = cmd.Process.Kill()
		delete(socatMgr.procs, incomingPort)
	}
}
