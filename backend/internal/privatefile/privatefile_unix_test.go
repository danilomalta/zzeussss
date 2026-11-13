//go:build !windows

package privatefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPOSIXPrivacyUnchanged(t *testing.T) {
	p := filepath.Join(t.TempDir(), "private")
	if err := os.WriteFile(p, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(p)
	if !Regular(p, info) {
		t.Fatal("private file refused")
	}
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	info, _ = os.Lstat(p)
	if Regular(p, info) {
		t.Fatal("public file accepted")
	}
	link := p + "-link"
	if err := os.Symlink(p, link); err != nil {
		t.Fatal(err)
	}
	info, _ = os.Lstat(link)
	if Regular(link, info) {
		t.Fatal("symlink accepted")
	}
}
