package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/stationfile"
)

var pilotBackupCLI = flag.String("pilot-backup-cli", "", "Explicit built titan-backup executable; empty uses library acceptance")

func pilotPrepareCLIStation(t *testing.T, f *stockHTTPFixture) string {
	t.Helper()
	if *pilotBackupCLI != "" && !filepath.IsAbs(*pilotBackupCLI) {
		t.Fatal("backup CLI path must be absolute")
	}
	if *pilotBackupCLI != "" {
		info, e := os.Stat(*pilotBackupCLI)
		if e != nil || !info.Mode().IsRegular() {
			t.Fatal("built backup CLI required")
		}
	}
	public, private, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	defer clear(private)
	// Fixture provisioning only: the synthetic installation receives a station
	// whose private key is retained for the real CLI challenge. No business rows
	// are changed by this setup, and no commercial installation is opened.
	result, e := f.db.Exec(`UPDATE device_pairings SET public_key=? WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, []byte(public), f.device.TenantID, f.device.StoreID, f.device.DeviceID)
	if e != nil {
		t.Fatal(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		t.Fatal("fixture station provisioning failed")
	}
	raw, e := json.Marshal(stationfile.Station{TenantID: f.device.TenantID, StoreID: f.device.StoreID, DeviceID: f.device.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(private)})
	if e != nil {
		t.Fatal(e)
	}
	defer clear(raw)
	path := filepath.Join(t.TempDir(), "station.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func pilotRunBackupCLI(t *testing.T, label string, success bool, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, *pilotBackupCLI, args...)
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "SYSTEMROOT"} {
		if v, ok := os.LookupEnv(name); ok {
			cmd.Env = append(cmd.Env, name+"="+v)
		}
	}
	out, e := cmd.CombinedOutput()
	if (e == nil) != success {
		t.Fatalf("backup executable step %s: expected success=%t; error=%v; output omitted", label, success, e)
	}
	if ctx.Err() != nil {
		t.Fatal("backup executable exceeded deadline", label)
	}
	t.Logf("BACKUP CLI %s: expected_success=%t observed_success=%t", label, success, e == nil)
	return out
}

func pilotCLIBackupRoundTrip(t *testing.T, f *stockHTTPFixture, dbPath, station string) string {
	t.Helper()
	dir := t.TempDir()
	key := filepath.Join(dir, "backup.key")
	wrong := filepath.Join(dir, "wrong.key")
	archive := filepath.Join(dir, "pilot.tytbak")
	restored := filepath.Join(dir, "restored.sqlite")
	pilotRunBackupCLI(t, "key-init", true, "key-init", "--key", key)
	pilotRunBackupCLI(t, "create-live", true, "create", "--key", key, "--db", dbPath, "--station", station, "--out", archive)
	scope := []string{"--tenant", f.device.TenantID, "--store", f.device.StoreID, "--device", f.device.DeviceID}
	args := func(command, archivePath string) []string {
		return append([]string{command, "--key", key, "--archive", archivePath}, scope...)
	}
	pilotRunBackupCLI(t, "verify", true, args("verify", archive)...)
	inspect := func(label, path string) backup.Inspection {
		raw := pilotRunBackupCLI(t, label, true, args("inspect", path)...)
		var report backup.Inspection
		if e := json.Unmarshal(raw, &report); e != nil || report.Profile != "pilot_cash_v1" || report.SchemaVersion != 73 || len(report.Unavailable) != 0 {
			t.Fatal("invalid CLI inspection report", e)
		}
		// Aggregate counts and hashes only; never print keys, station or DB rows.
		clean, _ := json.Marshal(report)
		t.Logf("BACKUP REPORT %s: %s", label, clean)
		return report
	}
	before := inspect("before-restore", archive)
	pilotRunBackupCLI(t, "wrong-key-init", true, "key-init", "--key", wrong)
	bad := append([]string{"verify", "--key", wrong, "--archive", archive}, scope...)
	pilotRunBackupCLI(t, "reject-wrong-key", false, bad...)
	bad = args("inspect", archive)
	for i := range bad {
		if bad[i] == "--tenant" {
			bad[i+1] = "wrong-company"
		}
	}
	pilotRunBackupCLI(t, "reject-wrong-company", false, bad...)
	restore := append(args("restore", archive), "--out", restored)
	pilotRunBackupCLI(t, "restore-new-path", true, restore...)
	pilotRunBackupCLI(t, "reject-overwrite", false, restore...)
	second := filepath.Join(dir, "restored-copy.tytbak")
	pilotRunBackupCLI(t, "create-from-restored", true, "create", "--key", key, "--db", restored, "--station", station, "--out", second)
	after := inspect("after-restore", second)
	if before.BusinessSHA256 != after.BusinessSHA256 || !reflect.DeepEqual(before.Records, after.Records) {
		t.Fatal("CLI restore changed pilot business digest/counts")
	}
	if before.ArchiveSHA256 == after.ArchiveSHA256 {
		t.Fatal("new encrypted archive unexpectedly identical")
	}
	t.Log("BACKUP CLI RESTORATION: pilot table counts and business SHA256 identical; encrypted archive SHA256 distinct; source and restored DBs separate")
	return restored
}
