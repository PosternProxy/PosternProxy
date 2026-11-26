package platformservice

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func Directories() (string, string, error) {
	programs, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", "", err
	}
	data, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(programs, "PosternProxy"), filepath.Join(data, "PosternProxy"), nil
}

func CheckOwned(service *mgr.Service, executable, definition string) error {
	configuration, err := service.Config()
	if err != nil {
		return err
	}
	want := windows.ComposeCommandLine([]string{executable, "service", "--config", definition})
	if !strings.EqualFold(configuration.BinaryPathName, want) {
		return fmt.Errorf("service %s is not managed by this installer; preserve or remove its existing setup first", service.Name)
	}
	return nil
}

func Stop(ctx context.Context, service *mgr.Service) error {
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		return nil
	}
	if status.State != svc.StopPending {
		if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return err
		}
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err = service.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Stopped {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("stop %s: %w", service.Name, ctx.Err())
		case <-ticker.C:
		}
	}
}

func uninstall(name string) error {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("service removal requires an elevated administrator terminal")
	}
	bin, data, err := Directories()
	if err != nil {
		return err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	other := "posternproxy-agent"
	if name == other {
		other = "posternproxy"
	}
	if service, err := manager.OpenService(other); err == nil {
		service.Close()
		return fmt.Errorf("the other PosternProxy role owns this machine's Caddy service")
	} else if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, serviceName := range []string{name, CaddyName} {
		service, err := manager.OpenService(serviceName)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			continue
		}
		if err != nil {
			return err
		}
		err = CheckOwned(service, filepath.Join(bin, name+".exe"), filepath.Join(data, "services", serviceName+".dat"))
		if err == nil {
			err = Stop(ctx, service)
		}
		if err == nil {
			err = service.Delete()
		}
		service.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
