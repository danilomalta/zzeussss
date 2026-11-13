package privatefile

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows chmod does not implement POSIX 0600. Inspect the DACL of the actual
// opened object instead. Local SYSTEM/Administrators are OS authorities, not
// vendor identities. Everyone, Users, Authenticated Users and other principals
// are refused, including inherited grants. No ACL is silently rewritten.
func private(path string, info os.FileInfo, directory bool) bool {
	if info == nil || info.Mode()&os.ModeSymlink != 0 || info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return false
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false
	}
	trusted := func(sid *windows.SID) bool {
		return sid != nil && sid.IsValid() && (sid.Equals(user.User.Sid) || sid.IsWellKnown(windows.WinLocalSystemSid) || sid.IsWellKnown(windows.WinBuiltinAdministratorsSid))
	}
	owner, _, err := sd.Owner()
	if err != nil || !trusted(owner) {
		return false
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil || acl.AceCount == 0 || acl.AceCount > 1024 {
		return false
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(acl, i, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceSize < uint16(unsafe.Sizeof(*ace)) {
			return false
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !trusted(sid) || sid.Len() > int(ace.Header.AceSize)-8 {
			return false
		}
	}
	return true
}

func Regular(path string, info os.FileInfo) bool { return private(path, info, false) }

// SQLite WAL/SHM and newly created keys inherit from this directory. Reject a
// broadly accessible parent before creating any new private content there.
func Parent(path string) bool {
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	return err == nil && Directory(parent, info)
}

func Directory(path string, info os.FileInfo) bool { return private(path, info, true) }

// No replacement or cross-volume copy is permitted. The caller has already
// flushed/closed the private temporary file. Windows does not support POSIX
// directory fsync; use its write-through move with no REPLACE_EXISTING flag.
func PublishNew(source, destination string) error {
	if filepath.Dir(source) != filepath.Dir(destination) || !Parent(destination) {
		return ErrPrivate
	}
	info, err := os.Lstat(source)
	if err != nil || !Regular(source, info) {
		return ErrPrivate
	}
	src, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return ErrPrivate
	}
	dst, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return ErrPrivate
	}
	return windows.MoveFileEx(src, dst, windows.MOVEFILE_WRITE_THROUGH)
}

// TempDir creates an empty directory with a protected DACL atomically, before
// any key/database/profile can be written. It never rewrites an existing ACL.
func TempDir(parent, prefix string) (string, error) {
	if !filepath.IsAbs(parent) || strings.ContainsAny(prefix, "/\\") {
		return "", ErrPrivate
	}
	info, e := os.Lstat(parent)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrPrivate
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", ErrPrivate
	}
	sd, err := windows.SecurityDescriptorFromString("O:" + user.User.Sid.String() + "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	if err != nil {
		return "", ErrPrivate
	}
	attrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	for i := 0; i < 4; i++ {
		var suffix [16]byte
		if _, err = rand.Read(suffix[:]); err != nil {
			return "", ErrPrivate
		}
		name := filepath.Join(parent, prefix+hex.EncodeToString(suffix[:]))
		utf, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return "", ErrPrivate
		}
		err = windows.CreateDirectory(utf, &attrs)
		if err == windows.ERROR_ALREADY_EXISTS {
			continue
		}
		if err != nil {
			return "", ErrPrivate
		}
		return name, nil
	}
	return "", ErrPrivate
}
