package stationfile

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/localsetup"
)

func TestStationFileProvesExistingInstallationAndRejectsRevocation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "store.sqlite")
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	created, err := localsetup.Initialize(ctx, db, localsetup.Input{TenantName: "Empresa", StoreName: "Loja", OwnerName: "Dono", DeviceName: "Caixa", Password: "senha-de-teste-longa", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	station := filepath.Join(dir, "station.json")
	body, err := json.Marshal(Station{created.TenantID, created.StoreID, created.DeviceID, base64.RawURLEncoding.EncodeToString(key)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(station, body, 0600); err != nil {
		t.Fatal(err)
	}
	opened, device, _, err := OpenVerified(ctx, path, station)
	if err != nil {
		t.Fatal(err)
	}
	opened.Close()
	if device.DeviceID != created.DeviceID {
		t.Fatal("aparelho divergente")
	}
	if _, err := db.Exec("UPDATE device_pairings SET status='revoked' WHERE device_id=?", created.DeviceID); err != nil {
		t.Fatal(err)
	}
	if opened, _, _, err := OpenVerified(ctx, path, station); err == nil {
		opened.Close()
		t.Fatal("aparelho revogado aceito")
	}
}

func TestStationFileDoesNotCreateMissingDatabase(t *testing.T) {
	dir := t.TempDir()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	station := filepath.Join(dir, "station.json")
	body, _ := json.Marshal(Station{"company", "store", "device", base64.RawURLEncoding.EncodeToString(key)})
	if err := os.WriteFile(station, body, 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "missing.sqlite")
	if db, _, _, err := OpenVerified(context.Background(), path, station); err == nil {
		db.Close()
		t.Fatal("banco inexistente aceito")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("criou banco por engano")
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(station, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadStation(link); err == nil {
		t.Fatal("symlink aceito")
	}
	if err := os.Chmod(station, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadStation(station); err == nil {
		t.Fatal("arquivo inseguro aceito")
	}
}
