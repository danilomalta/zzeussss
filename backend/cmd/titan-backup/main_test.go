package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/localsetup"
	"titansystem-backend/internal/localdb/stationfile"
)

func TestBackupCLIKeyInitDoesNotPrintSecretOrOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.key")
	var output bytes.Buffer
	if err := run([]string{"key-init", "--key", path}, &output); err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(path)
	if err != nil || len(key) != 32 {
		t.Fatal("key not created")
	}
	if bytes.Contains(output.Bytes(), key) {
		t.Fatal("secret printed")
	}
	if err := run([]string{"key-init", "--key", path}, &output); err == nil {
		t.Fatal("overwritten")
	}
}

type cancelOutput struct{ cancel context.CancelFunc }

func (o cancelOutput) Write(body []byte) (int, error) { o.cancel(); return len(body), nil }

func TestBackupCLIAutomaticRunStopsOnCancellationAfterOneVerifiedSnapshot(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "live.sqlite")
	db, err := localdb.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := localsetup.Initialize(context.Background(), db, localsetup.Input{TenantName: "Test", StoreName: "Store", OwnerName: "Owner", DeviceName: "Device", Password: "test-backup-password-long", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	stationPath := filepath.Join(dir, "station.json")
	body, err := json.Marshal(stationfile.Station{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(private)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stationPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "backup.key")
	if err := run([]string{"key-init", "--key", key}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	backups := filepath.Join(dir, "backups")
	if err := os.Mkdir(backups, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = runContext(ctx, []string{"watch", "--key", key, "--db", dbPath, "--station", stationPath, "--out", backups, "--interval", "1m"}, cancelOutput{cancel})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("worker result: %v", err)
	}
	entries, err := os.ReadDir(backups)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected backups: %d %v", len(entries), err)
	}
	archive := filepath.Join(backups, entries[0].Name())
	if err := run([]string{"verify", "--key", key, "--archive", archive, "--tenant", owner.TenantID, "--store", owner.StoreID, "--device", owner.DeviceID}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

func TestBackupCLIRejectsUnknownCommandsAndFlags(t *testing.T) {
	for _, args := range [][]string{{}, {"invalid"}, {"create", "--unknown", "x"}, {"restore", "--key", "missing"}, {"key-init", "--key", "x", "extra"}} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
