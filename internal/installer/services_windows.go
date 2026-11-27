package installer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/posternproxy/posternproxy/internal/platformservice"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func windowsServices(manager *mgr.Mgr, role, bin, definitions string) ([]*mgr.Service, error) {
	var result []*mgr.Service
	for _, name := range []string{platformservice.CaddyName, role} {
		path := filepath.Join(definitions, name+".dat")
		executable := filepath.Join(bin, role+".exe")
		configuration := mgr.Config{ServiceType: windows.SERVICE_WIN32_OWN_PROCESS, DisplayName: name, StartType: mgr.StartAutomatic,
			ServiceStartName: `NT SERVICE\` + name, SidType: windows.SERVICE_SID_TYPE_UNRESTRICTED,
			Description: "PosternProxy managed service"}
		if name == role {
			configuration.Dependencies = []string{platformservice.CaddyName}
		}
		service, err := manager.OpenService(name)
		if err == nil {
			if err := platformservice.CheckOwned(service, executable, path); err != nil {
				service.Close()
				return result, err
			}
			configuration.BinaryPathName = windows.ComposeCommandLine([]string{executable, "service", "--config", path})
			if err := service.UpdateConfig(configuration); err != nil {
				service.Close()
				return result, err
			}
		} else if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			service, err = manager.CreateService(name, executable, configuration, "service", "--config", path)
			if err != nil {
				return result, fmt.Errorf("create Windows service %s: %w", name, err)
			}
		} else {
			return result, err
		}
		result = append(result, service)
		if err := service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 5 * time.Second}}, 86400); err != nil {
			return result, err
		}
		if err := service.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
			return result, err
		}
	}
	return result, nil
}

func waitWindowsReady(ctx context.Context, services []*mgr.Service, urls ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for {
		ready := true
		for _, service := range services {
			status, err := service.Query()
			if err != nil || status.State != svc.Running {
				ready = false
			}
		}
		for _, url := range urls {
			if !probe(ctx, client, url) {
				ready = false
			}
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("windows services did not become ready: %w", ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func stopWindowsServices(ctx context.Context, services []*mgr.Service) error {
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for index := len(services) - 1; index >= 0; index-- {
		if err := platformservice.Stop(stopCtx, services[index]); err != nil {
			return err
		}
	}
	return nil
}
