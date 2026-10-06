package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckExistingStationAndRefusesMissingForeignOrRevoked(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "store.sqlite")
	station := filepath.Join(dir, "station.json")
	var out bytes.Buffer
	args := []string{"--db", db, "--station", station}
	if err := checkStation(args, &out); err == nil {
		t.Fatal("created missing installation")
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatal("created database")
	}
	if err := initStation(append(args, "--empresa", "Teste", "--loja", "Teste", "--dono", "Teste"), strings.NewReader("senha-descartavel-2026"), &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := checkStation(args, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "private_key") || strings.Contains(out.String(), "senha") {
		t.Fatal("secret")
	}
	database, device, err := openVerified(t.Context(), db, station)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec("UPDATE device_pairings SET status='revoked' WHERE device_id=?", device.DeviceID); err != nil {
		t.Fatal(err)
	}
	database.Close()
	if err := checkStation(args, &out); err == nil {
		t.Fatal("accepted revoked station")
	}
	if err := checkStation(append(args, "--unknown"), &out); err == nil {
		t.Fatal("accepted flags")
	}
}
