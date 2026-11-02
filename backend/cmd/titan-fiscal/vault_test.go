package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type privateTestTTY struct{ bytes.Buffer }

func (*privateTestTTY) Close() error { return nil }
func TestVaultCommandsKeepMasterKeyOutOfStandardOutput(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "key")
	vault := filepath.Join(dir, "vault")
	tty := &privateTestTTY{}
	var output bytes.Buffer
	opener := func() (io.WriteCloser, error) { return tty, nil }
	if err := vaultCommand([]string{"vault-init", "--vault", vault, "--key-file", key}, &output, opener); err != nil {
		t.Fatal(err)
	}
	secret, _ := os.ReadFile(key)
	if bytes.Contains(output.Bytes(), bytes.TrimSpace(secret)) {
		t.Fatal("key in stdout")
	}
	if !bytes.Contains(tty.Bytes(), bytes.TrimSpace(secret)) {
		t.Fatal("not shown privately")
	}
	old := tty.Len()
	if err := vaultCommand([]string{"vault-export-key", "--key-file", key, "--out", filepath.Join(dir, "backup")}, &output, opener); err != nil {
		t.Fatal(err)
	}
	if tty.Len() != old {
		t.Fatal("backup displayed key")
	}
	if err := vaultCommand([]string{"vault-rotate", "--vault", vault, "--key-file", key, "--out", filepath.Join(dir, "new")}, &output, opener); err == nil {
		t.Fatal("rotation without backup confirmation")
	}
}
