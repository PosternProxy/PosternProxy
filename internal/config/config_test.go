package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistedSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "secret.key")
	first := loadOrCreateSecret(path)
	if len(first) != 64 || loadOrCreateSecret(path) != first {
		t.Fatal("secret changed across startup")
	}
	for _, contents := range []string{"", "  \n"} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if recover() == nil {
					t.Error("empty persisted secret was silently replaced")
				}
			}()
			loadOrCreateSecret(path)
		}()
		data, err := os.ReadFile(path)
		if err != nil || string(data) != contents {
			t.Fatal("damaged secret file was overwritten")
		}
	}
}

func TestUnreadableSecretIsNotReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Error("unreadable persisted secret did not stop startup")
		}
	}()
	loadOrCreateSecret(path)
}
