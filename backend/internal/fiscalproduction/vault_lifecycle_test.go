package fiscalproduction

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomerMasterKeyGenerationBackupRotationAndStaleWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.sqlite")
	keyFile := filepath.Join(dir, "master.key")
	backup := filepath.Join(dir, "backup.key")
	newKey := filepath.Join(dir, "rotated.key")
	var terminal bytes.Buffer
	if err := CreateCustomerFiscalVault(path, keyFile, &terminal); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(terminal.Bytes(), raw) {
		t.Fatal("not shown exactly once")
	}
	before := terminal.Len()
	if CreateCustomerFiscalVault(path, keyFile, &terminal) == nil || terminal.Len() != before {
		t.Fatal("recreated or reprinted key")
	}
	if err = ExportCustomerMasterKey(keyFile, backup); err != nil {
		t.Fatal(err)
	}
	if ExportCustomerMasterKey(keyFile, backup) == nil {
		t.Fatal("backup overwritten")
	}
	v, err := OpenCustomerFiscalVault(path, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	secret := []byte("customer-owned-disposable-secret")
	if err = v.Put(context.Background(), "client", "token", secret); err != nil {
		t.Fatal(err)
	}
	if v.Put(context.Background(), "another-client", "token", secret) == nil {
		t.Fatal("one vault accepted two clients")
	}
	archived := filepath.Join(dir, "customer-backup.sqlite")
	snapshot, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = privateNew(archived, snapshot); err != nil {
		t.Fatal(err)
	}
	var rotated bytes.Buffer
	if err = RotateCustomerMasterKey(path, keyFile, newKey, &rotated); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(rotated.Bytes(), raw) {
		t.Fatal("key reused")
	}
	if _, err = OpenCustomerFiscalVault(path, keyFile); err == nil {
		t.Fatal("old key opened rotated vault")
	}
	if v.Put(context.Background(), "client", "token", []byte("stale")) == nil {
		t.Fatal("stale writer committed old encryption")
	}
	active, err := OpenCustomerFiscalVault(path, newKey)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	plain, err := active.read(context.Background(), "client", "token")
	if err != nil || !bytes.Equal(plain, secret) {
		t.Fatal("rotation lost secret", err)
	}
	clear(plain)
	restored, err := OpenCustomerFiscalVault(archived, backup)
	if err != nil {
		t.Fatal("backup key restoration", err)
	}
	defer restored.Close()
	oldPlain, err := restored.read(context.Background(), "client", "token")
	if err != nil || !bytes.Equal(oldPlain, secret) {
		t.Fatal("old backup no longer recoverable", err)
	}
	clear(oldPlain)
	saved, _ := os.ReadFile(backup)
	if !bytes.Equal(saved, raw) {
		t.Fatal("old backup changed")
	}
	ciphertext, _ := os.ReadFile(path)
	if bytes.Contains(ciphertext, secret) || bytes.Contains(ciphertext, bytes.TrimSpace(raw)) {
		t.Fatal("plaintext in vault")
	}
	if _, err = OpenCustomerFiscalVault(path, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing key accepted")
	}
}
func TestIndependentCustomerKeysCannotOpenOtherVault(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.sqlite"), filepath.Join(dir, "b.sqlite")
	ka, kb := filepath.Join(dir, "a.key"), filepath.Join(dir, "b.key")
	var display bytes.Buffer
	if CreateCustomerFiscalVault(a, ka, &display) != nil || CreateCustomerFiscalVault(b, kb, &display) != nil {
		t.Fatal("create")
	}
	if _, err := OpenCustomerFiscalVault(a, kb); err == nil {
		t.Fatal("B key opened A vault")
	}
	if _, err := OpenCustomerFiscalVault(b, ka); err == nil {
		t.Fatal("A key opened B vault")
	}
}
