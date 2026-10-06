package backup

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/localsetup"
)

func fixture(t *testing.T) (*sql.DB, identity.DeviceContext, []byte, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := localdb.Open(context.Background(), filepath.Join(dir, "live.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := localsetup.Initialize(context.Background(), db, localsetup.Input{TenantName: "Private tenant", StoreName: "Store", DeviceName: "Device", OwnerName: "Owner", Password: "strong-private-password", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	scope := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
	if _, err := localauth.Login(context.Background(), db, scope, owner.OwnerID, "strong-private-password"); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return db, scope, key, dir
}

func TestEncryptedLiveSnapshotRestoreKeepsDataAndRevokesSessions(t *testing.T) {
	db, scope, key, dir := fixture(t)
	archive := filepath.Join(dir, "copy.tytbak")
	ctx := context.Background()
	// The source stays open; the snapshot must include committed WAL data.
	if err := Create(ctx, db, scope, key, archive); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(body, []byte("Private tenant")) || bytes.Contains(body, []byte("SQLite format")) {
		t.Fatal("plaintext leaked")
	}
	info, _ := os.Stat(archive)
	if info.Mode().Perm() != 0600 {
		t.Fatal("public archive")
	}
	if err := Verify(ctx, archive, scope, key); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(dir, "recovered.sqlite")
	if err := Restore(ctx, archive, restored, scope, key); err != nil {
		t.Fatal(err)
	}
	recovery, err := localdb.Open(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	defer recovery.Close()
	var name string
	if err := recovery.QueryRow(`SELECT name FROM tenants WHERE id=?`, scope.TenantID).Scan(&name); err != nil || name != "Private tenant" {
		t.Fatalf("data: %q %v", name, err)
	}
	var live, restoredSessions int
	db.QueryRow(`SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&live)
	recovery.QueryRow(`SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&restoredSessions)
	if live != 1 || restoredSessions != 0 {
		t.Fatalf("sessions: live=%d restored=%d", live, restoredSessions)
	}
	if err := Restore(ctx, archive, restored, scope, key); err == nil {
		t.Fatal("overwrote recovery")
	}
	if err := Create(ctx, db, scope, key, archive); err == nil {
		t.Fatal("overwrote archive")
	}
}

func TestWrongKeyScopeTamperingAndSymlinkNeverPublish(t *testing.T) {
	db, scope, key, dir := fixture(t)
	archive := filepath.Join(dir, "copy.tytbak")
	if err := Create(context.Background(), db, scope, key, archive); err != nil {
		t.Fatal(err)
	}
	wrongKey := bytes.Repeat([]byte{1}, 32)
	if err := Verify(context.Background(), archive, scope, wrongKey); err == nil {
		t.Fatal("wrong key")
	}
	foreign := scope
	foreign.TenantID = "foreign"
	destination := filepath.Join(dir, "must-not-exist.sqlite")
	if err := Restore(context.Background(), archive, destination, foreign, key); err == nil {
		t.Fatal("foreign scope")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("published invalid recovery")
	}
	body, _ := os.ReadFile(archive)
	body[len(body)-1] ^= 1
	altered := filepath.Join(dir, "altered.tytbak")
	if err := os.WriteFile(altered, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(context.Background(), altered, destination, scope, key); err == nil {
		t.Fatal("tampering")
	}
	link := filepath.Join(dir, "link.tytbak")
	if err := os.Symlink(archive, link); err != nil {
		t.Fatal(err)
	}
	if err := Verify(context.Background(), link, scope, key); err == nil {
		t.Fatal("read symlink")
	}
	if err := Create(context.Background(), db, scope, key, link); err == nil {
		t.Fatal("replaced symlink")
	}
}

func TestPrivateKeyAndFutureSchemaAreRejected(t *testing.T) {
	db, scope, _, dir := fixture(t)
	path := filepath.Join(dir, "backup.key")
	if err := InitKey(path); err != nil {
		t.Fatal(err)
	}
	key, err := ReadKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := InitKey(path); err == nil {
		t.Fatal("replaced key")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadKey(path); err == nil {
		t.Fatal("public key file accepted")
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations VALUES (9999,'future','now')`); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "invalid.tytbak")
	if err := Create(context.Background(), db, scope, key, archive); err == nil {
		t.Fatal("future schema accepted")
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("invalid backup published")
	}
}

func TestRestoreRejectsIgnoredSessionRevocationAndStaleSidecars(t *testing.T) {
	db, scope, key, dir := fixture(t)
	if _, err := db.Exec(`CREATE TRIGGER ignore_restored_session BEFORE UPDATE ON local_sessions BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "with-trigger.tytbak")
	if err := Create(context.Background(), db, scope, key, archive); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "recovered.sqlite")
	if err := Restore(context.Background(), archive, destination, scope, key); err == nil {
		t.Fatal("published unrevoked sessions")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatal("invalid recovery published")
	}
	if _, err := db.Exec(`DROP TRIGGER ignore_restored_session`); err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(dir, "clean.tytbak")
	if err := Create(context.Background(), db, scope, key, clean); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination+"-wal", []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(context.Background(), clean, destination, scope, key); err == nil {
		t.Fatal("stale WAL accepted")
	}
}

func TestRestoreConsumesRecoveryKeysOnlyInRecoveredCopy(t *testing.T) {
	db, device, key, dir := fixture(t)
	var owner string
	if err := db.QueryRow(`SELECT identity_id FROM memberships WHERE role='owner'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err := localauth.IssueOwnerRecovery(context.Background(), db, device, owner, "strong-private-password", make([]byte, 32), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "recovery.tytbak")
	if err := Create(context.Background(), db, device, key, archive); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "recovered.sqlite")
	if err := Restore(context.Background(), archive, destination, device, key); err != nil {
		t.Fatal(err)
	}
	recovered, err := localdb.Open(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	var original, restored int
	if err := db.QueryRow(`SELECT count(*) FROM owner_recovery_keys WHERE consumed_unix IS NULL`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if err := recovered.QueryRow(`SELECT count(*) FROM owner_recovery_keys WHERE consumed_unix IS NULL`).Scan(&restored); err != nil {
		t.Fatal(err)
	}
	if original != 1 || restored != 0 {
		t.Fatalf("keys: original=%d recovered=%d", original, restored)
	}
}
