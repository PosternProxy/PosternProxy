package installer

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const initialAdminEnvironment = "/run/posternproxy-install/admin.env"

type initialAdminCredentials struct{ Email, Password string }

func (i *installation) prepareInitialAdmin() error {
	if i.agentConfig != nil {
		return nil
	}
	data, err := os.ReadFile(i.path("/etc/posternproxy/config.env"))
	if err != nil {
		return err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.HasPrefix(name, "#") {
			continue
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, `"`) {
			value, err = strconv.Unquote(value)
			if err != nil {
				return fmt.Errorf("invalid quoted installer environment value")
			}
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = strings.Trim(value, "'")
		}
		values[strings.TrimSpace(name)] = value
	}
	databasePath := values["POSTERNPROXY_DB_PATH"]
	if databasePath == "" {
		databasePath = "/var/lib/posternproxy/data/posternproxy.db"
	}
	if _, err := os.Stat(i.path(databasePath)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing controller database: %w", err)
	}
	email := values["POSTERNPROXY_ADMIN_EMAIL"]
	if email == "" {
		email = "admin@posternproxy.local"
	}
	password := values["POSTERNPROXY_ADMIN_PASSWORD"]
	if password == "" {
		password = rand.Text()
		directory := filepath.Dir(i.path(initialAdminEnvironment))
		if err := os.MkdirAll(directory, 0700); err != nil {
			return err
		}
		info, err := os.Lstat(directory)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("initial admin staging must be a private directory")
		}
		if err := os.Chmod(directory, 0700); err != nil { // #nosec G302 -- 0700 on the private credential directory is intended
			return err
		}
		if err := atomicWrite(i.path(initialAdminEnvironment), strings.NewReader("POSTERNPROXY_ADMIN_PASSWORD="+strconv.Quote(password)+"\n"), 0600); err != nil {
			return err
		}
		i.initialAdminEnvironmentCreated = true
	}
	i.initialAdmin = &initialAdminCredentials{Email: email, Password: password}
	return nil
}

func (i *installation) removeInitialAdminEnvironment() error {
	if !i.initialAdminEnvironmentCreated {
		return nil
	}
	if err := os.Remove(i.path(initialAdminEnvironment)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temporary initial password: %w", err)
	}
	i.initialAdminEnvironmentCreated = false
	if err := os.Remove(filepath.Dir(i.path(initialAdminEnvironment))); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove initial password staging directory: %w", err)
	}
	return nil
}

func (i *installation) printInitialAdmin(output io.Writer) error {
	if i.initialAdmin == nil {
		_, err := io.WriteString(output, "Existing admin credentials are unchanged.\n")
		return err
	}
	credentials := i.initialAdmin
	_, err := io.WriteString(output, fmt.Sprintf("\nInitial administrator login\nEmail: %s\nPassword: %s\nChange this password after your first login.\n", credentials.Email, credentials.Password))
	if err == nil {
		i.initialAdmin = nil
	}
	return err
}
