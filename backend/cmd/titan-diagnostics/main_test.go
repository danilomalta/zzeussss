package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

func TestDiagnosticsLocalOnlyReadOnlyBackupKeyAndTLSEvidence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "db.sqlite")
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	owner, err := localsetup.Initialize(ctx, db, localsetup.Input{TenantName: "Private name must never appear", StoreName: "Store", DeviceName: "Device", OwnerName: "Owner", Password: "disposable-long-private-password", PublicKey: pub})
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
	vault := filepath.Join(dir, "vault.sqlite")
	master := filepath.Join(dir, "master")
	copy := filepath.Join(dir, "copy")
	if fiscalproduction.CreateCustomerFiscalVault(vault, master, &bytes.Buffer{}) != nil || fiscalproduction.ExportCustomerMasterKey(master, copy) != nil {
		t.Fatal("vault fixture")
	}
	keyPath := filepath.Join(dir, "backup.key")
	if backup.InitKey(keyPath) != nil {
		t.Fatal("key")
	}
	key, _ := backup.ReadKey(keyPath)
	defer clear(key)
	keyCopy := filepath.Join(dir, "backup.copy")
	os.WriteFile(keyCopy, key, 0600)
	destination := filepath.Join(dir, "backups")
	os.Mkdir(destination, 0700)
	archive := filepath.Join(destination, "first.tytbak")
	if backup.Create(ctx, db, scope, key, archive) != nil {
		t.Fatal("archive")
	}
	args := []string{"--db", path, "--vault", vault, "--key-file", master, "--key-copy", copy, "--backup-dir", destination, "--backup-key", keyPath, "--backup-key-copy", keyCopy, "--tenant", scope.TenantID, "--store", scope.StoreID, "--device", scope.DeviceID}
	var out bytes.Buffer
	if err = run(ctx, args, &out); err != nil {
		t.Fatal(err)
	}
	var r report
	if json.Unmarshal(out.Bytes(), &r) != nil || r.Backup == 0 || r.Vault != "available" || !r.Exported || r.TLS != "loopback" {
		t.Fatal("diagnostic states", r)
	}
	latest, _ := localdb.LatestSchemaVersion()
	if r.Schema != latest {
		t.Fatal("schema")
	}
	raw, _ := os.ReadFile(master)
	for _, secret := range [][]byte{raw, key, []byte("Private name"), []byte(scope.TenantID), []byte(dir)} {
		if bytes.Contains(out.Bytes(), secret) {
			t.Fatal("private data in diagnostic")
		}
	}
	os.Remove(copy)
	out.Reset()
	if run(ctx, args, &out) != nil {
		t.Fatal("missing key copy should warn")
	}
	json.Unmarshal(out.Bytes(), &r)
	if r.Exported {
		t.Fatal("copy falsely confirmed")
	}
	broken := filepath.Join(destination, "newer.tytbak")
	os.WriteFile(broken, []byte("broken"), 0600)
	past := time.Now().Add(-time.Minute)
	os.Chtimes(archive, past, past)
	out.Reset()
	if run(ctx, args, &out) != nil {
		t.Fatal("corrupt archive should warn")
	}
	json.Unmarshal(out.Bytes(), &r)
	if r.Backup != 0 {
		t.Fatal("invalid newest archive hidden")
	}
	badProfile := filepath.Join(dir, "deployment.json")
	os.WriteFile(badProfile, []byte(`{}`), 0600)
	out.Reset()
	if run(ctx, append(args, "--deployment", badProfile), &out) != nil {
		t.Fatal("TLS diagnostic")
	}
	json.Unmarshal(out.Bytes(), &r)
	if r.TLS != "invalid" {
		t.Fatal("invalid TLS hidden")
	}
	before, _ := os.ReadFile(path)
	out.Reset()
	if run(ctx, args, &out) != nil {
		t.Fatal("repeat")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("diagnostic changed source")
	}
}
func TestDiagnosticsRejectsUnsafeFlagsAndDoesNotCreateMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite")
	for _, args := range [][]string{{}, {"--unknown", "value"}, {"--db", path}, {"--db", "https://provider.invalid/data"}} {
		var out bytes.Buffer
		err := run(context.Background(), args, &out)
		if err == nil || out.Len() != 0 || strings.Contains(err.Error(), path) {
			t.Fatal("unsafe diagnostics")
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("created database")
	}
}
