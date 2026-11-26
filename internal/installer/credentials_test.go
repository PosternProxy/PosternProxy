package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitialAdminPasswordIsPrintedAndRemovedAfterInstall(t *testing.T) {
	i, _, commands := fixtureInstallation(t)
	if err := i.writeConfig(); err != nil {
		t.Fatal(err)
	}
	if err := i.prepareInitialAdmin(); err != nil {
		t.Fatal(err)
	}
	if i.initialAdmin == nil || len(i.initialAdmin.Password) < 20 {
		t.Fatal("initial password was not prepared")
	}
	password := i.initialAdmin.Password
	if runtime.GOOS != "windows" {
		info, err := os.Stat(i.path(initialAdminEnvironment))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("transient password permissions are not private")
		}
	}
	config, err := os.ReadFile(i.path("/etc/posternproxy/config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(config, []byte(password)) {
		t.Fatal("bootstrap password persisted in configuration")
	}
	temporary, err := os.ReadFile(i.path(initialAdminEnvironment))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(temporary, []byte(password)) {
		t.Fatal("first service start cannot receive password")
	}
	for _, command := range *commands {
		if strings.Contains(command, password) {
			t.Fatal("password passed in command arguments")
		}
	}
	var output bytes.Buffer
	if err := i.printInitialAdmin(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), password) || !strings.Contains(output.String(), "admin@posternproxy.local") {
		t.Fatal("initial login omitted from install output")
	}
	var repeated bytes.Buffer
	if err := i.printInitialAdmin(&repeated); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(repeated.String(), password) {
		t.Fatal("initial password was displayed twice")
	}
	if err := i.removeInitialAdminEnvironment(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(i.path(initialAdminEnvironment)); !os.IsNotExist(err) {
		t.Fatal("transient password file remains")
	}
}

func TestInitialAdminExistingDatabaseIsPreserved(t *testing.T) {
	i, _, _ := fixtureInstallation(t)
	if err := i.writeConfig(); err != nil {
		t.Fatal(err)
	}
	path := i.path("/var/lib/posternproxy/posternproxy.db")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("existing database sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := i.prepareInitialAdmin(); err != nil {
		t.Fatal(err)
	}
	if i.initialAdmin != nil {
		t.Fatal("reinstall prepared a replacement password")
	}
	if _, err := os.Stat(i.path(initialAdminEnvironment)); !os.IsNotExist(err) {
		t.Fatal("reinstall overrides existing admin credentials")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing database sentinel" {
		t.Fatal("existing database changed")
	}
}

func TestInitialAdminUsesConfiguredCredentialsOnlyOnFreshDatabase(t *testing.T) {
	i, _, _ := fixtureInstallation(t)
	if err := os.WriteFile(i.path("/etc/posternproxy/config.env"), []byte("POSTERNPROXY_DB_PATH=/var/lib/posternproxy/custom.db\nPOSTERNPROXY_ADMIN_EMAIL=owner@example.test\nPOSTERNPROXY_ADMIN_PASSWORD=\"configured password\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := i.prepareInitialAdmin(); err != nil {
		t.Fatal(err)
	}
	if i.initialAdmin == nil || i.initialAdmin.Email != "owner@example.test" || i.initialAdmin.Password != "configured password" {
		t.Fatal("configured login was not preserved")
	}
	if _, err := os.Stat(i.path(initialAdminEnvironment)); !os.IsNotExist(err) {
		t.Fatal("configured password was unnecessarily staged")
	}
}

func TestInitialAdminStagingCanBeCleanedAfterApplyFailure(t *testing.T) {
	i, stage, _ := fixtureInstallation(t)
	original := i.run
	i.run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" {
			return nil, errors.New("fixture service failure")
		}
		return original(ctx, name, args...)
	}
	if err := i.apply(context.Background(), stage); err == nil {
		t.Fatal("service failure was ignored")
	}
	if i.initialAdmin == nil {
		t.Fatal("fixture did not reach credential preparation")
	}
	if err := i.removeInitialAdminEnvironment(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(i.path(initialAdminEnvironment)); !os.IsNotExist(err) {
		t.Fatal("failed installation retained a transient password file")
	}
}
