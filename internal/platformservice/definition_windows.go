package platformservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protect(data []byte, decrypt bool) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty protected service configuration")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]} // #nosec G115 -- service configuration length fits in uint32
	var out windows.DataBlob
	var err error
	if decrypt {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_LOCAL_MACHINE|windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, fmt.Errorf("protect service configuration: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) // #nosec G103 -- required Win32 DPAPI output-buffer handling
	return bytes.Clone(unsafe.Slice(out.Data, int(out.Size))), nil    // #nosec G103 -- required Win32 DPAPI output-buffer handling
}

func WriteDefinition(path string, d Definition) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("service definition path must be absolute")
	}
	if err := d.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if len(data) > 64<<10 {
		return fmt.Errorf("service configuration is too large")
	}
	data, err = protect(data, false)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".service-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func ReadDefinition(path string) (Definition, error) {
	var d Definition
	if !filepath.IsAbs(path) {
		return d, fmt.Errorf("service definition path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return d, err
	}
	if !info.Mode().IsRegular() || info.Size() > 72<<10 {
		return d, fmt.Errorf("unsafe service configuration file")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is a service-definition file chosen by the installer, not request input
	if err != nil {
		return d, err
	}
	data, err = protect(data, true)
	if err != nil {
		return d, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return d, fmt.Errorf("invalid service configuration")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return d, fmt.Errorf("trailing service configuration")
	}
	return d, d.validate()
}
