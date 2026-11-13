package privatefile

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func setTestACL(t *testing.T, path, extra string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;FA;;;SY)" + extra)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsPrivacyRejectsBroadGrantsAndFileReplacement(t *testing.T) {
	dir := t.TempDir()
	setTestACL(t, dir, "")
	p := filepath.Join(dir, "private")
	if !Parent(p) {
		t.Fatal("private parent refused")
	}
	if err := os.WriteFile(p, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(p)
	if !Regular(p, info) {
		t.Fatal("private ACL refused")
	}
	// The POSIX bits are deliberately not the authority on Windows.
	t.Logf("native POSIX compatibility bits: %04o; actual private ACL accepted", info.Mode().Perm())
	if err := os.Rename(p, p+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if Regular(p, info) {
		t.Fatal("replaced file accepted with stale identity")
	}
	for _, sid := range []string{"WD", "AU", "BU"} {
		setTestACL(t, p, "(A;;FR;;;"+sid+")")
		info, _ = os.Lstat(p)
		if Regular(p, info) {
			t.Fatal("broad file grant accepted", sid)
		}
	}
	setTestACL(t, dir, "(A;OICI;FR;;;WD)")
	if Parent(filepath.Join(dir, "new-secret")) {
		t.Fatal("broadly accessible parent accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "new-secret")); !os.IsNotExist(err) {
		t.Fatal("privacy check created a file")
	}
}

func TestWindowsDefaultTempAndNoOverwritePublication(t *testing.T) {
	dir, err := TempDir(t.TempDir(), "private-")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "source")
	if !Parent(source) {
		t.Fatal("explicit user-private temporary directory refused")
	}
	f, err := OpenNew(source, os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("synthetic")); err != nil {
		t.Fatal(err)
	}
	if err = f.Sync(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "published")
	if err = PublishNew(source, dest); err != nil {
		t.Fatal(err)
	}
	f, err = OpenNew(source, os.O_RDWR)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write([]byte("replacement")); err != nil {
		t.Fatal(err)
	}
	f.Sync()
	f.Close()
	if PublishNew(source, dest) == nil {
		t.Fatal("existing destination overwritten")
	}
	raw, err := os.ReadFile(dest)
	if err != nil || string(raw) != "synthetic" {
		t.Fatal("original publication changed")
	}
}
