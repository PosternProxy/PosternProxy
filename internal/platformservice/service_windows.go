package platformservice

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func Dispatch(args []string, name string, run func(context.Context) error) (bool, error) {
	if len(args) > 0 && args[0] == "uninstall" {
		if len(args) != 1 {
			return true, fmt.Errorf("uninstall takes no arguments and preserves data")
		}
		if err := uninstall(name); err != nil {
			return true, err
		}
		log.Info().Str("action", "service_uninstall").Str("actor", "administrator").Str("service", name).Msg("services removed; binaries and data retained")
		return true, nil
	}
	if len(args) == 0 || args[0] != "service" {
		return false, nil
	}
	if len(args) != 3 || args[1] != "--config" {
		return true, fmt.Errorf("service requires --config with an absolute protected configuration path")
	}
	d, err := ReadDefinition(args[2])
	if err != nil {
		return true, err
	}
	if d.Name != name && d.Name != CaddyName {
		return true, fmt.Errorf("service role does not match the executable")
	}
	isService, err := svc.IsWindowsService()
	if err != nil {
		return true, err
	}
	if !isService {
		return true, fmt.Errorf("service mode must be started through Windows Service Control Manager")
	}
	for _, variable := range os.Environ() {
		key, _, _ := strings.Cut(variable, "=")
		if strings.HasPrefix(key, "POSTERNPROXY_") {
			if err := os.Unsetenv(key); err != nil {
				return true, err
			}
		}
	}
	for key, value := range d.Environment {
		if err := os.Setenv(key, value); err != nil {
			return true, err
		}
	}
	if d.BootstrapPath != "" {
		bootstrap, err := ReadDefinition(d.BootstrapPath)
		if err == nil {
			if bootstrap.Name != d.Name {
				return true, fmt.Errorf("bootstrap service role mismatch")
			}
			for key, value := range bootstrap.Environment {
				if err := os.Setenv(key, value); err != nil {
					return true, err
				}
			}
		} else if !os.IsNotExist(err) {
			return true, err
		}
	}
	file, err := os.OpenFile(d.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return true, err
	}
	defer file.Close()
	log.Logger = zerolog.New(file).With().Timestamp().Logger()
	if d.Name == CaddyName {
		run = func(ctx context.Context) error { return runCaddy(ctx, d, file) }
	}
	return true, svc.Run(d.Name, &serviceHandler{run: run})
}

type serviceHandler struct{ run func(context.Context) error }

func (h *serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, updates chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates <- svc.Status{State: svc.StartPending, WaitHint: 15000}
	done := make(chan error, 1)
	go func() { done <- h.run(ctx) }()
	status := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	updates <- status
	for {
		select {
		case err := <-done:
			if err != nil {
				log.Error().Err(err).Msg("service stopped unexpectedly")
			}
			return true, 1
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				updates <- status
			case svc.Stop, svc.Shutdown:
				updates <- svc.Status{State: svc.StopPending, WaitHint: 15000}
				cancel()
				select {
				case err := <-done:
					if err != nil && err != context.Canceled {
						log.Error().Err(err).Msg("service shutdown failed")
					}
					return false, 0
				case <-time.After(15 * time.Second):
					log.Error().Msg("service shutdown timed out")
					return false, 0
				}
			}
		}
	}
}

func runCaddy(ctx context.Context, d Definition, output *os.File) error {
	command := exec.Command(d.CaddyExecutable, "run", "--config", d.CaddyConfig) // #nosec G204 -- launches the project's own pinned Caddy binary with fixed arguments
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		return err
	}
	job, err := caddyJob(command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		command.Wait()
		return err
	}
	defer windows.CloseHandle(job)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Post("http://127.0.0.1:2019/stop", "application/json", nil)
		if err == nil {
			response.Body.Close()
		}
		select {
		case <-done:
			return nil
		case <-time.After(3 * time.Second):
			if err := command.Process.Kill(); err != nil {
				return err
			}
			<-done
			return fmt.Errorf("caddy did not stop gracefully")
		}
	}
}

func caddyJob(pid int) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil { // #nosec G103 -- required Win32 job-object syscall
		windows.CloseHandle(job)
		return 0, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid)) // #nosec G115 -- a process ID fits in uint32 on Windows
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}
