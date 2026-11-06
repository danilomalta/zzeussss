package fiscalproduction

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"titansystem-backend/internal/deployment"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/localsetup"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/stationfile"
)

// A cold recovery rehearsal in a fresh filesystem root, NOT another physical
// host or a live cross-database snapshot. Only disposable customer fixtures are
// copied. The DB archive never substitutes for the separately held keys/vault.
func TestCustomerRecoveryKitFreshRootPreservesVaultAndBusiness(t *testing.T) {
	ctx := context.Background()
	source, kit, target, keyCustody := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	for _, dir := range []string{source, kit, target, keyCustody} {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	live, err := localdb.Open(ctx, filepath.Join(source, "business.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	owner, err := localsetup.Initialize(ctx, live, localsetup.Input{TenantName: "Synthetic", StoreName: "Store", OwnerName: "Owner", DeviceName: "PC", Password: "disposable-recovery-password", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	a := RequestContext{DB: live, Actor: identity.Scope{TenantID: owner.TenantID, StoreID: owner.StoreID, IdentityID: owner.OwnerID}, Device: identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}}
	session, err := localauth.Login(ctx, live, a.Device, owner.OwnerID, "disposable-recovery-password")
	if err != nil {
		t.Fatal(err)
	}
	a.Token = session.Token
	station, err := json.Marshal(stationfile.Station{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(private)})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(station)
	// Station identity is private customer custody, separate from the DB archive.
	stationCopy := filepath.Join(keyCustody, "station.json")
	if err = privateNew(stationCopy, station); err != nil {
		t.Fatal(err)
	}
	vaultPath, master := filepath.Join(source, "vault.sqlite"), filepath.Join(source, "master.key")
	if err := CreateCustomerFiscalVault(vaultPath, master, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	v, err := OpenCustomerFiscalVault(vaultPath, master)
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("disposable-recovery-token-never-output")
	if err = v.Put(ctx, a.Actor.TenantID, "token", secret); err != nil {
		_ = v.Close()
		t.Fatal(err)
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	// Writers are stopped before copying the vault; never copy a live WAL file.
	copyPrivate := func(from, to string) {
		t.Helper()
		raw, e := os.ReadFile(from)
		if e != nil {
			t.Fatal(e)
		}
		defer clear(raw)
		if e = privateNew(to, raw); e != nil {
			t.Fatal(e)
		}
	}
	copyPrivate(vaultPath, filepath.Join(kit, "vault.sqlite"))
	if err = ExportCustomerMasterKey(master, filepath.Join(keyCustody, "master.key")); err != nil {
		t.Fatal(err)
	}
	backupKey := filepath.Join(keyCustody, "backup.key")
	if err = backup.InitKey(backupKey); err != nil {
		t.Fatal(err)
	}
	key, err := backup.ReadKey(backupKey)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	profile, err := json.Marshal(deployment.Local(8181))
	if err != nil {
		t.Fatal(err)
	}
	if err = privateNew(filepath.Join(kit, "deployment.json"), profile); err != nil {
		t.Fatal(err)
	}
	if _, err = register.Open(ctx, a.DB, a.Actor, a.Device, register.OpenInput{SessionID: "recovery-original", OpeningCents: 1701}); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(kit, "business.tytbak")
	captureStart := time.Now()
	if err = backup.Create(ctx, a.DB, a.Device, key, archive); err != nil {
		t.Fatal(err)
	}
	captured := time.Now()
	before, err := backup.Inspect(ctx, archive, a.Device, key)
	if err != nil {
		t.Fatal(err)
	}
	rehearsal := t.TempDir()
	if err = os.Chmod(rehearsal, 0700); err != nil {
		t.Fatal(err)
	}
	allTables, err := backup.TestRestore(ctx, archive, rehearsal, a.Device, key)
	if err != nil || !allTables.DataMatches || !allTables.SessionsRevoked {
		t.Fatal("full business-table restoration did not match", err)
	}
	// This later commit deliberately falls outside the captured recovery point.
	if _, err = a.DB.Exec(`UPDATE tenants SET name='after-capture' WHERE id=?`, a.Actor.TenantID); err != nil {
		t.Fatal(err)
	}
	failureAt := time.Now()
	restoreStart := time.Now()
	newDB := filepath.Join(target, "business.sqlite")
	if err = backup.Restore(ctx, archive, newDB, a.Device, key); err != nil {
		t.Fatal(err)
	}
	copyPrivate(filepath.Join(kit, "vault.sqlite"), filepath.Join(target, "vault.sqlite"))
	copyPrivate(filepath.Join(kit, "deployment.json"), filepath.Join(target, "deployment.json"))
	if _, err = OpenCustomerFiscalVault(filepath.Join(target, "vault.sqlite"), filepath.Join(target, "missing.key")); err == nil {
		t.Fatal("DB/vault backup substituted for missing master key")
	}
	restoredVault, err := OpenCustomerFiscalVault(filepath.Join(target, "vault.sqlite"), filepath.Join(keyCustody, "master.key"))
	if err != nil {
		t.Fatal(err)
	}
	defer restoredVault.Close()
	plain, err := restoredVault.read(ctx, a.Actor.TenantID, "token")
	if err != nil || !bytes.Equal(plain, secret) {
		t.Fatal("separate customer key did not recover secret", err)
	}
	clear(plain)
	if _, err = restoredVault.read(ctx, "other-customer", "token"); err == nil {
		t.Fatal("foreign tenant read restored secret")
	}
	if _, err = deployment.Load(filepath.Join(target, "deployment.json")); err != nil {
		t.Fatal("deployment config not recoverable", err)
	}
	db, err := localdb.Open(ctx, newDB)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	verified, device, _, err := stationfile.OpenVerified(ctx, newDB, stationCopy)
	if err != nil || device != a.Device {
		t.Fatal("station proof not recoverable from customer copy", err)
	}
	if err = verified.Close(); err != nil {
		t.Fatal(err)
	}
	var sessions, cents int
	if err = db.QueryRow(`SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("restored sessions not revoked", err)
	}
	if err = db.QueryRow(`SELECT opening_cents FROM cash_sessions WHERE tenant_id=? AND id='recovery-original'`, a.Actor.TenantID).Scan(&cents); err != nil || cents != 1701 {
		t.Fatal("cash origin/amount changed", err)
	}
	var name string
	if err = db.QueryRow(`SELECT name FROM tenants WHERE id=?`, a.Actor.TenantID).Scan(&name); err != nil || name == "after-capture" {
		t.Fatal("recovery point falsely includes later write", err)
	}
	if _, err = localauth.Resolve(ctx, db, a.Token); err == nil {
		t.Fatal("old session accepted after restore")
	}
	second := filepath.Join(target, "recovered.tytbak")
	if err = backup.Create(ctx, db, a.Device, key, second); err != nil {
		t.Fatal(err)
	}
	after, err := backup.Inspect(ctx, second, a.Device, key)
	if err != nil || before.BusinessSHA256 != after.BusinessSHA256 || !reflect.DeepEqual(before.Records, after.Records) {
		t.Fatal("business IDs/journals changed", err)
	}
	rto := time.Since(restoreStart)
	for _, path := range []string{filepath.Join(kit, "business.tytbak"), filepath.Join(kit, "vault.sqlite")} {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(raw, secret) || bytes.Contains(raw, key) {
			t.Fatal("backup exposed token or backup key")
		}
		clear(raw)
	}
	t.Logf("local_rehearsal capture_ms=%d recovery_point_age_ms=%d restore_and_verify_ms=%d; not a customer RPO/RTO guarantee or separate-host proof", captured.Sub(captureStart).Milliseconds(), failureAt.Sub(captured).Milliseconds(), rto.Milliseconds())
}
