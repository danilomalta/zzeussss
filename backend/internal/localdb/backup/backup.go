// Package backup produces authenticated encrypted SQLite snapshots. It is an
// installation-level tool: the OS operator holding the backup key can recover
// the entire installation. It is not a tenant-scoped HTTP download endpoint.
package backup

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

const MaxBytes = 128 << 20

var magic = []byte("TYTBKP01")
var ErrBackup = errors.New("backup inválido, incompatível ou não autorizado")

func aead(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, ErrBackup
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrBackup
	}
	return cipher.NewGCM(block)
}

// ReadKey accepts only a private regular 32-byte key, never a symlink.
func ReadKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() != 32 {
		return nil, ErrBackup
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrBackup
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrBackup
	}
	key, err := io.ReadAll(io.LimitReader(f, 33))
	if err != nil || len(key) != 32 {
		return nil, ErrBackup
	}
	return key, nil
}

func InitKey(path string) error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	return publish(path, key)
}

// Create uses SQLite VACUUM INTO rather than copying a live WAL database.
// No private station or encryption key files are placed in the archive.
func Create(ctx context.Context, db *sql.DB, device identity.DeviceContext, key []byte, destination string) error {
	if db == nil {
		return ErrBackup
	}
	crypt, err := aead(key)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "titan-backup-")
	if err != nil {
		return err
	}
	snapshot := filepath.Join(dir, "snapshot.sqlite")
	defer cleanup(snapshot, dir)
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, snapshot); err != nil {
		return ErrBackup
	}
	if err := os.Chmod(snapshot, 0600); err != nil {
		return err
	}
	if err := validate(ctx, snapshot, device); err != nil {
		return err
	}
	body, err := readLimited(snapshot, MaxBytes)
	if err != nil {
		return err
	}
	nonce := make([]byte, crypt.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	header := append(append([]byte{}, magic...), nonce...)
	sealed := crypt.Seal(header, nonce, body, magic)
	return publish(destination, sealed)
}

// Verify authenticates the archive and performs full integrity, foreign-key,
// migration-history and expected-device checks on a disposable copy.
func Verify(ctx context.Context, archive string, device identity.DeviceContext, key []byte) error {
	_, err := withSnapshot(ctx, archive, device, key, "")
	return err
}

// Restore only publishes to a NEW path. It never touches a running database or
// silently replaces data. Every restored human session is revoked.
func Restore(ctx context.Context, archive, destination string, device identity.DeviceContext, key []byte) error {
	if destination == "" {
		return ErrBackup
	}
	// Old sidecars can belong to a different database. Never reuse such a path.
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(destination + suffix); !errors.Is(err, os.ErrNotExist) {
			return ErrBackup
		}
	}
	_, err := withSnapshot(ctx, archive, device, key, destination)
	return err
}

func withSnapshot(ctx context.Context, archive string, device identity.DeviceContext, key []byte, destination string) (bool, error) {
	crypt, err := aead(key)
	if err != nil {
		return false, err
	}
	sealed, err := readLimited(archive, MaxBytes+64)
	if err != nil || len(sealed) < len(magic)+crypt.NonceSize()+crypt.Overhead() || !bytes.Equal(sealed[:len(magic)], magic) {
		return false, ErrBackup
	}
	body, err := crypt.Open(nil, sealed[len(magic):len(magic)+crypt.NonceSize()], sealed[len(magic)+crypt.NonceSize():], magic)
	if err != nil || len(body) > MaxBytes {
		return false, ErrBackup
	}
	dir, err := os.MkdirTemp("", "titan-restore-")
	if err != nil {
		return false, err
	}
	snapshot := filepath.Join(dir, "snapshot.sqlite")
	defer cleanup(snapshot, dir)
	if err := os.WriteFile(snapshot, body, 0600); err != nil {
		return false, err
	}
	if err := validate(ctx, snapshot, device); err != nil {
		return false, err
	}
	if destination == "" {
		return true, nil
	}
	// Migrate a validated older snapshot only in this disposable recovery copy.
	db, err := localdb.Open(ctx, snapshot)
	if err != nil {
		return false, ErrBackup
	}
	_, err = db.ExecContext(ctx, `UPDATE local_sessions SET revoked_unix=? WHERE revoked_unix IS NULL`, time.Now().Unix())
	if err == nil {
		_, err = db.ExecContext(ctx, `UPDATE owner_recovery_keys SET consumed_unix=? WHERE consumed_unix IS NULL`, time.Now().Unix())
	}
	var remaining int
	if err == nil {
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&remaining)
	}
	var remainingKeys int
	if err == nil {
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM owner_recovery_keys WHERE consumed_unix IS NULL`).Scan(&remainingKeys)
	}
	closeErr := db.Close()
	if err != nil || closeErr != nil || remaining != 0 || remainingKeys != 0 {
		return false, ErrBackup
	}
	body, err = readLimited(snapshot, MaxBytes)
	if err != nil {
		return false, err
	}
	return true, publish(destination, body)
}

func validate(ctx context.Context, path string, device identity.DeviceContext) error {
	if device.TenantID == "" || device.StoreID == "" || device.DeviceID == "" {
		return ErrBackup
	}
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", u.String()+"?mode=ro")
	if err != nil {
		return ErrBackup
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil || result != "ok" {
		return ErrBackup
	}
	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return ErrBackup
	}
	violated := rows.Next()
	err = rows.Err()
	rows.Close()
	if violated || err != nil {
		return ErrBackup
	}
	if localdb.ValidateSchema(ctx, db) != nil {
		return ErrBackup
	}
	var count int
	// This release explicitly supports snapshots at schemas 25 through 40.
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || (count != 25 && count != 26 && count != 27 && count != 28 && count != 29 && count != 30 && count != 31 && count != 32 && count != 33 && count != 34 && count != 35 && count != 36 && count != 37 && count != 38 && count != 39 && count != 40) {
		return ErrBackup
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM tenants`).Scan(&count); err != nil || count != 1 {
		return ErrBackup
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, device.TenantID, device.StoreID, device.DeviceID).Scan(&count); err != nil || count != 1 {
		return ErrBackup
	}
	return nil
}

func readLimited(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, ErrBackup
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrBackup
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrBackup
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, ErrBackup
	}
	return body, nil
}

// publish atomically links a flushed private temporary file into the chosen
// destination. Link fails if ANY destination already exists, including symlinks.
func publish(path string, body []byte) error {
	if path == "" {
		return ErrBackup
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(absolute)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrBackup
	}
	file, err := os.CreateTemp(dir, ".titan-publish-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(body); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Link(file.Name(), absolute); err != nil {
		return err
	}
	parent, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}

func cleanup(path, dir string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	_ = os.Remove(path + "-journal")
	_ = os.Remove(dir)
}
