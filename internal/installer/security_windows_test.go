package installer

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsRootCreationAndExistingTreeRefusal(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("administrator ownership requires an elevated test runner")
	}
	root := filepath.Join(t.TempDir(), "managed")
	if err := ensureWindowsRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := ensureWindowsRoot(root); err != nil {
		t.Fatalf("trusted root could not be reused: %v", err)
	}
	unsafeRoot := filepath.Join(t.TempDir(), "untrusted")
	if err := os.Mkdir(unsafeRoot, 0700); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;BA)(A;;FA;;;SY)(A;;GW;;;BU)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(unsafeRoot, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if err := ensureWindowsRoot(unsafeRoot); err == nil {
		t.Fatal("accepted an existing ordinary-user-writable tree")
	}
	if err := trustedWindowsPath(unsafeRoot, false); err == nil {
		t.Fatal("refusal changed the untrusted directory's permissions")
	}
}

func TestWindowsServiceDirectoryACLs(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("administrator ownership requires an elevated test runner")
	}
	for _, access := range []string{"FA", "RX", "TRAVERSE"} {
		t.Run(access, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "service")
			if err := makeWindowsDirectory(path, map[string]string{expectedServiceSID("posternproxy"): access}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestServiceSIDMatchesWindowsMapping(t *testing.T) {
	want := "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	if got := expectedServiceSID("TrustedInstaller"); got != want {
		t.Fatalf("service SID = %s", got)
	}
}

func TestRetainedConfigurationRequiresTrustedOwnerAndAccess(t *testing.T) {
	app := expectedServiceSID("posternproxy")
	for _, tc := range []struct {
		name, sddl             string
		confidential, rejected bool
	}{
		{"private administrator owned", "O:BAG:BAD:P(A;;FA;;;BA)(A;;FA;;;SY)(A;;GR;;;" + app + ")", true, false},
		{"ordinary owner", "O:BUG:BUD:P(A;;FA;;;BA)", true, true},
		{"ordinary writer", "O:BAG:BAD:P(A;;FA;;;BA)(A;;GW;;;BU)", false, true},
		{"ordinary reader of secret", "O:BAG:BAD:P(A;;FA;;;BA)(A;;GR;;;BU)", true, true},
		{"ordinary reader of directory", "O:BAG:BAD:P(A;;FA;;;BA)(A;;GR;;;BU)", false, false},
		{"service writer of definition", "O:BAG:BAD:P(A;;FA;;;BA)(A;;GW;;;" + app + ")", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			descriptor, err := windows.SecurityDescriptorFromString(tc.sddl)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateWindowsSecurity(descriptor, tc.confidential); (err != nil) != tc.rejected {
				t.Fatalf("security check: %v", err)
			}
		})
	}
}
