//go:build windows

package platformservice

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func TestServiceStopCancelsWorkWithoutRequestingRestart(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"canceled", context.Canceled},
		{"shutdown error", errors.New("fixture shutdown failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			canceled := make(chan struct{})
			h := serviceHandler{run: func(ctx context.Context) error {
				<-ctx.Done()
				close(canceled)
				if tc.err == context.Canceled {
					return ctx.Err()
				}
				return tc.err
			}}
			requests := make(chan svc.ChangeRequest, 1)
			updates := make(chan svc.Status, 8)
			type result struct {
				specific bool
				code     uint32
			}
			done := make(chan result, 1)
			go func() {
				specific, code := h.Execute(nil, requests, updates)
				done <- result{specific, code}
			}()
			t.Cleanup(func() {
				select {
				case requests <- svc.ChangeRequest{Cmd: svc.Stop}:
				default:
				}
			})
			deadline := time.After(2 * time.Second)
			for {
				select {
				case status := <-updates:
					if status.State == svc.Running {
						goto running
					}
				case <-deadline:
					t.Fatal("service did not reach Running")
				}
			}
		running:
			requests <- svc.ChangeRequest{Cmd: svc.Stop}
			select {
			case got := <-done:
				if got.specific || got.code != 0 {
					t.Fatalf("intentional stop requested restart: specific=%v code=%d", got.specific, got.code)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("service did not finish after Stop")
			}
			select {
			case <-canceled:
			default:
				t.Fatal("Stop did not cancel the worker context")
			}
		})
	}
}

func TestCaddyJobKillsChildWhenClosed(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-test.run=^TestServiceJobChild$")
	command.Env = append(os.Environ(), "POSTERNPROXY_TEST_SERVICE_JOB_CHILD=disposable-job-fixture")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		done <- command.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		select {
		case <-exited:
			return
		default:
		}
		if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Logf("child cleanup: %v", err)
		}
	})
	job, err := caddyJob(command.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CloseHandle(job); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("child survived closing the job handle")
	}
}

func TestServiceJobChild(t *testing.T) {
	if os.Getenv("POSTERNPROXY_TEST_SERVICE_JOB_CHILD") != "disposable-job-fixture" {
		return
	}
	time.Sleep(30 * time.Second)
}
