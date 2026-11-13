package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/stationfile"
	"titansystem-backend/internal/privatefile"
)

func TestWindowsCustomerNativeInitialization(t *testing.T) {
	binary := os.Getenv("TITAN_INSTALLED_LOCAL")
	if !filepath.IsAbs(binary) {
		t.Fatal("explicit native Windows CLI required")
	}
	dir, err := privatefile.TempDir(t.TempDir(), "customer-")
	if err != nil {
		t.Fatal(err)
	}
	dbPath, station := filepath.Join(dir, "customer.sqlite"), filepath.Join(dir, "station.json")
	args := []string{"init", "--db", dbPath, "--station", station, "--empresa", "Synthetic customer", "--loja", "Synthetic store", "--dono", "Synthetic owner"}
	const password = "synthetic-owner-password-for-native-test"
	run := func(args []string, want bool) []byte {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Stdin = strings.NewReader(password + "\n")
		raw, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatal("native customer initialization/check result mismatch; output omitted")
		}
		if bytes.Contains(raw, []byte(password)) || bytes.Contains(raw, []byte("private_key")) {
			t.Fatal("private value in native output")
		}
		return raw
	}
	raw := run(args, true)
	if !bytes.Contains(raw, []byte("Dono (ID para login): ")) {
		t.Fatal("owner identifier missing")
	}
	run(args, false)
	run([]string{"check", "--db", dbPath, "--station", station}, true)
	db, err := localdb.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM memberships WHERE role='owner'").Scan(&count); err != nil || count != 1 {
		t.Fatal("owner duplicated or lost")
	}
	db.Close()
	verified, device, key, err := stationfile.OpenVerified(context.Background(), dbPath, station)
	if err != nil || device.TenantID == "" || len(key) == 0 {
		t.Fatal("native station proof failed")
	}
	clear(key)
	verified.Close()
	info, err := os.Lstat(station)
	if err != nil || !privatefile.Regular(station, info) {
		t.Fatal("station key ACL is not private")
	}
}
