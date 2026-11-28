//go:build !windows

package certstore

import "os"

func runtimeGroup(path string) error { return os.Chown(path, -1, os.Getegid()) }

func syncDirectory(path string) error {
	f, err := os.Open(path) // #nosec G304 -- path is a certificate-store directory being fsynced, not request input
	if err != nil {
		return err
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
