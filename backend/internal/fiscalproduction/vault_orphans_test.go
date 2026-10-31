package fiscalproduction

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestVaultOrphanDetectionAndExplicitCleanup(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault.sqlite")
	old := filepath.Join(dir, "old.key")
	active := filepath.Join(dir, "active.key")
	var out bytes.Buffer
	if err := CreateCustomerFiscalVault(vault, old, &out); err != nil {
		t.Fatal(err)
	}
	v, err := OpenCustomerFiscalVault(vault, old)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range concurrentVaultCalls(3, func(int) error { return v.initializeKeyControl() }) {
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, e := range concurrentVaultCalls(3, func(int) error { return v.bindTenant(context.Background(), "client") }) {
		if e != nil {
			t.Fatal(e)
		}
	}
	v.Close()
	if err = RotateCustomerMasterKey(vault, old, active, &out); err != nil {
		t.Fatal(err)
	}
	// A copied historical key is still referenced: backup copies must not be offered for deletion.
	backup := filepath.Join(dir, "backup.key")
	if err = ExportCustomerMasterKey(old, backup); err != nil {
		t.Fatal(err)
	}
	items, err := ListVaultOrphans(vault, active, dir)
	if err != nil || len(items) != 0 {
		t.Fatal("referenced key offered as orphan", err)
	}
	// Synthetic file simulates an unreferenced key left by an interrupted old installation.
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(dir, "abandoned.key")
	if err = recordKeyCandidate(vault, orphan, []byte(base64.StdEncoding.EncodeToString(raw))); err != nil {
		t.Fatal(err)
	}
	if err = privateNew(orphan, []byte(base64.StdEncoding.EncodeToString(raw)+"\n")); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(dir, "unregistered-other-client.key")
	if err = privateNew(unknown, []byte(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{95}, 32)))); err != nil {
		t.Fatal(err)
	}
	items, err = ListVaultOrphans(vault, active, dir)
	if err != nil || len(items) != 1 {
		t.Fatal("orphan not detected", err)
	}
	body, _ := json.Marshal(items)
	if bytes.Contains(body, []byte(dir)) || bytes.Contains(body, []byte(base64.StdEncoding.EncodeToString(raw))) {
		t.Fatal("orphan listing exposed path or key")
	}
	if RemoveVaultOrphan(vault, active, dir, items[0].ID, "") == nil {
		t.Fatal("cleanup without confirmation")
	}
	if _, err = os.Stat(orphan); err != nil {
		t.Fatal("automatic cleanup occurred")
	}
	if err = RemoveVaultOrphan(vault, active, dir, items[0].ID, "DELETE "+items[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(orphan); !os.IsNotExist(err) {
		t.Fatal("explicit cleanup failed")
	}
	for _, file := range []string{old, active, backup, unknown} {
		if _, err = os.Stat(file); err != nil {
			t.Fatal("referenced key deleted")
		}
	}
}
