//go:build !windows

// Package privatefile checks private files using the operating system's model.
package privatefile

import (
	"os"
	"path/filepath"
)

func Regular(path string, info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode().Perm() == 0600
}

// Existing POSIX callers already create directories with 0700; do not change
// their directory policy in the Windows compatibility layer.
func Parent(path string) bool { return true }

func Directory(path string, info os.FileInfo) bool {
	return info != nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm() == 0700
}

// The existing POSIX publication and parent fsync semantics are unchanged.
func PublishNew(source, destination string) error {
	if err := os.Link(source, destination); err != nil {
		return err
	}
	parent, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}

func TempDir(parent, prefix string) (string, error) { return os.MkdirTemp(parent, prefix) }
