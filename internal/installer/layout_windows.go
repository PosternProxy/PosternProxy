package installer

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/config"
	"github.com/posternproxy/posternproxy/internal/security"
	"golang.org/x/sys/windows"
)

func checkWindowsPath(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("installation paths must not contain links or junctions")
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}

func setWindowsACL(path string, grants map[string]string) error {
	if err := checkWindowsPath(path); err != nil {
		return err
	}
	sddl := "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	for sid, access := range grants {
		inherit := "OICI"
		if access == "TRAVERSE" {
			access, inherit = "0x20", ""
		} else if access == "RX" {
			access = "FRFX"
		}
		sddl += "(A;" + inherit + ";" + access + ";;;" + sid + ")"
	}
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("build Windows permissions for %s: %w", path, err)
	}
	acl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read Windows permissions for %s: %w", path, err)
	}
	owner, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, owner, nil, acl, nil); err != nil {
		return fmt.Errorf("protect Windows installation path %s: %w", path, err)
	}
	return nil
}

func makeWindowsDirectory(path string, grants map[string]string) error {
	if err := checkWindowsPath(path); err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	return setWindowsACL(path, grants)
}

// Seal a fresh managed root at creation, before any retained files or services
// are used. ProgramData permits ordinary users to create new child directories.
func ensureWindowsRoot(path string) error {
	if err := checkWindowsPath(path); err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("O:BAG:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: descriptor}
	if err := windows.CreateDirectory(name, &attributes); err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return fmt.Errorf("create protected Windows installation root: %w", err)
	}
	return trustedWindowsPath(path, false)
}

func serviceSID(name string) (string, error) {
	sid, _, _, err := windows.LookupSID("", `NT SERVICE\`+name)
	if err != nil {
		return "", err
	}
	return sid.String(), nil
}

// Service SIDs are deterministic even after SCM removal. This is Windows' SID
// name mapping, not a password hash; it lets retained files be checked before
// recreating the virtual account.
func expectedServiceSID(name string) string {
	var input []byte
	for _, value := range utf16.Encode([]rune(strings.ToUpper(name))) {
		input = binary.LittleEndian.AppendUint16(input, value)
	}
	hash := sha1.Sum(input)
	result := "S-1-5-80"
	for offset := 0; offset < len(hash); offset += 4 {
		result += fmt.Sprintf("-%d", binary.LittleEndian.Uint32(hash[offset:offset+4]))
	}
	return result
}

func trustedWindowsPath(path string, confidential bool) error {
	if err := checkWindowsPath(path); err != nil {
		return err
	}
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	return validateWindowsSecurity(descriptor, confidential)
}

func validateWindowsSecurity(descriptor *windows.SECURITY_DESCRIPTOR, confidential bool) error {
	owner, _, err := descriptor.Owner()
	if err != nil {
		return err
	}
	if owner.String() != "S-1-5-18" && owner.String() != "S-1-5-32-544" {
		return fmt.Errorf("existing Windows installation path has an untrusted owner; preserve its contents and resolve ownership before installing")
	}
	acl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if acl == nil {
		return fmt.Errorf("existing Windows installation path has unrestricted access")
	}
	services := map[string]bool{}
	for _, name := range []string{"posternproxy", "posternproxy-agent", "posternproxy-caddy"} {
		services[expectedServiceSID(name)] = true
	}
	for index := uint32(0); index < uint32(acl.AceCount); index++ {
		var entry *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, index, &entry); err != nil {
			return err
		}
		header := &entry.Header
		if header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		if header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return fmt.Errorf("unsupported existing installation ACL")
		}
		sid := (*windows.SID)(unsafe.Pointer(&entry.SidStart)).String()
		if sid == "S-1-5-18" || sid == "S-1-5-32-544" {
			continue
		}
		const fileDeleteChild = 0x40
		const writable = windows.GENERIC_ALL | windows.GENERIC_WRITE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.DELETE | windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES | fileDeleteChild
		if services[sid] {
			if entry.Mask&writable == 0 {
				continue
			}
		} else if !confidential && entry.Mask&writable == 0 {
			continue
		}
		return fmt.Errorf("existing Windows installation path has unsafe access permissions")
	}
	return nil
}

func applyWindowsSettings(path string, environment map[string]string) error {
	if path == "" {
		return validateWindowsSettings(environment)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 16385))
	var updates map[string]string
	if err := decoder.Decode(&updates); err != nil {
		return fmt.Errorf("invalid Windows service settings JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("settings file must contain one JSON object")
	}
	allowed := map[string]bool{"POSTERNPROXY_LISTEN": true, "POSTERNPROXY_CONTROLLER_URL": true,
		"POSTERNPROXY_BEHIND_TLS_PROXY": true, "POSTERNPROXY_TRUSTED_PROXIES": true,
		"POSTERNPROXY_CADDY_TRUSTED_PROXIES": true, "POSTERNPROXY_ALLOWED_ORIGINS": true}
	for key, value := range updates {
		if !allowed[key] {
			return fmt.Errorf("unsupported Windows service setting %s", key)
		}
		environment[key] = value
	}
	return validateWindowsSettings(environment)
}

func validateWindowsSettings(environment map[string]string) error {
	if err := config.ValidateManagementAddress(environment["POSTERNPROXY_LISTEN"]); err != nil {
		return err
	}
	if value := environment["POSTERNPROXY_CONTROLLER_URL"]; value != "" {
		if err := agentconfig.ValidateURL(value); err != nil {
			return err
		}
	}
	if value := environment["POSTERNPROXY_BEHIND_TLS_PROXY"]; value != "true" && value != "false" {
		return fmt.Errorf("BEHIND_TLS_PROXY must be true or false")
	}
	var proxies []string
	if value := environment["POSTERNPROXY_TRUSTED_PROXIES"]; value != "" {
		proxies = strings.Split(value, ",")
	}
	if _, err := security.ParseTrustedProxies(proxies); err != nil {
		return err
	}
	_, err := caddy.ParseTrustedProxyPolicies(environment["POSTERNPROXY_CADDY_TRUSTED_PROXIES"])
	return err
}

func replaceWindowsBinary(source, destination string) error {
	if err := checkWindowsPath(destination); err != nil {
		return err
	}
	staged := destination + ".new"
	if err := copyFile(source, staged, 0700); err != nil {
		return err
	}
	defer os.Remove(staged)
	previous := destination + ".previous"
	if _, err := os.Stat(destination); err == nil {
		if err := os.Remove(previous); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(destination, previous); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staged, destination); err != nil {
		if restoreErr := os.Rename(previous, destination); restoreErr != nil {
			return fmt.Errorf("replace binary: %w; restore previous: %v", err, restoreErr)
		}
		return err
	}
	return nil
}
