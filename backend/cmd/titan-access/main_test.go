package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/localsetup"
	"titansystem-backend/internal/localdb/stationfile"
)

func TestAccessCLIPreparesAndRecoversWithoutPrintingSecretOrOverwriting(t *testing.T) {
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
	owner, err := localsetup.Initialize(context.Background(), db, localsetup.Input{TenantName: "Test", StoreName: "Store", OwnerName: "Owner", DeviceName: "Device", Password: "current-owner-password", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	stationPath := filepath.Join(dir, "station.json")
	station, _ := json.Marshal(stationfile.Station{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(private)})
	if err := os.WriteFile(stationPath, station, 0600); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(dir, "current.password")
	if err := os.WriteFile(current, []byte("current-owner-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "emergency.json")
	initArgs := []string{"recovery-init", "--db", dbPath, "--station", stationPath, "--owner", owner.OwnerID, "--password-file", current, "--out", destination}
	var output bytes.Buffer
	if err := run(initArgs, &output); err != nil {
		t.Fatal(err)
	}
	body, err := privateRead(destination, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var file recoveryFile
	if err := parseRecovery(body, &file); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output.Bytes(), []byte(file.Key)) || bytes.Contains(output.Bytes(), []byte("current-owner-password")) || bytes.Contains(output.Bytes(), []byte(base64.RawURLEncoding.EncodeToString(private))) {
		t.Fatal("secret printed")
	}
	if err := run(initArgs, &output); err == nil {
		t.Fatal("overwrote key")
	}
	newPassword := filepath.Join(dir, "new.password")
	if err := os.WriteFile(newPassword, []byte("recovered-owner-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"recover", "--db", dbPath, "--station", stationPath, "--recovery-file", destination, "--password-file", newPassword}
	if err := run(args, &output); err != nil {
		t.Fatal(err)
	}
	if err := run(args, &output); err == nil {
		t.Fatal("reused key")
	}
	device := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
	if _, err := localauth.Login(context.Background(), db, device, owner.OwnerID, "recovered-owner-password"); err != nil {
		t.Fatal(err)
	}
}

func TestAccessCLIPrivateReadersRejectPublicFilesLinksAndAmbiguousJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credential")
	if err := os.WriteFile(path, []byte("current-owner-password"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := privateRead(path, 73); err == nil {
		t.Fatal("public credential accepted")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := privateRead(link, 73); err == nil {
		t.Fatal("symlink accepted")
	}
	var file recoveryFile
	if err := parseRecovery([]byte(`{"version":1,"version":1}`), &file); err == nil {
		t.Fatal("duplicate fields accepted")
	}
	for _, args := range [][]string{{}, {"unknown"}, {"recovery-init", "--password", "secret"}, {"recover", "--db", "missing"}} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatal("invalid CLI accepted")
		}
	}
}
