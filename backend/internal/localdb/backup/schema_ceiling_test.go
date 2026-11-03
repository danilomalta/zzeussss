package backup

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
)

func TestCurrentSchemaBackupRestoresDataAndRevokesSessions(t *testing.T) {
	db, scope, key, dir := fixture(t)
	ctx := context.Background()
	latest, err := localdb.LatestSchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != latest {
		t.Fatal("fixture not at current schema", count, err)
	}
	archive := filepath.Join(dir, "current.tytbak")
	if err = Create(ctx, db, scope, key, archive); err != nil {
		t.Fatal(err)
	}
	info, err := Inspect(ctx, archive, scope, key)
	if err != nil || info.SchemaVersion != latest {
		t.Fatal("current schema not authenticated", err)
	}
	destination := filepath.Join(dir, "restored-current.sqlite")
	if err = Restore(ctx, archive, destination, scope, key); err != nil {
		t.Fatal(err)
	}
	restored, err := localdb.Open(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var name string
	if err = restored.QueryRow(`SELECT name FROM tenants WHERE id=?`, scope.TenantID).Scan(&name); err != nil || name != "Private tenant" {
		t.Fatal("tenant data not preserved", err)
	}
	var originalSessions, restoredSessions int
	if err = db.QueryRow(`SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&originalSessions); err != nil || originalSessions == 0 {
		t.Fatal("source session changed", err)
	}
	if err = restored.QueryRow(`SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&restoredSessions); err != nil || restoredSessions != 0 {
		t.Fatal("restored session still active", err)
	}
	if err = restored.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != latest {
		t.Fatal("restored schema changed", count, err)
	}
}

func TestAuthenticatedFutureSchemaSnapshotIsRefused(t *testing.T) {
	db, scope, key, dir := fixture(t)
	latest, err := localdb.LatestSchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO schema_migrations VALUES (?,'future-schema-checksum','now')`, latest+1); err != nil {
		t.Fatal(err)
	}
	// Simulate an authentic archive from a newer executable. Authentication alone
	// must never allow Restore to publish an unsupported schema.
	snapshot := filepath.Join(dir, "future.sqlite")
	if _, err = db.Exec(`VACUUM INTO ?`, snapshot); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(raw)
	crypt, err := aead(key)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, crypt.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	header := append(append([]byte{}, magic...), nonce...)
	archive := filepath.Join(dir, "authentic-future.tytbak")
	if err = publish(archive, crypt.Seal(header, nonce, raw, magic)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err = Verify(ctx, archive, scope, key); !errors.Is(err, ErrBackup) {
		t.Fatal("future schema passed Verify", err)
	}
	destination := filepath.Join(dir, "must-not-exist.sqlite")
	if err = Restore(ctx, archive, destination, scope, key); !errors.Is(err, ErrBackup) {
		t.Fatal("future schema passed Restore", err)
	}
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatal("future schema was published")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != latest+1 {
		t.Fatal("future source modified", count, err)
	}
}
