package backup

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

// Build an actual schema-25 database from unchanged historical migrations,
// not by deleting tables from a newer database.
func TestOlderBackupRestoresThroughMigration37(t *testing.T) {
	for _, historicalVersion := range []int{25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36} {
		t.Run(strconv.Itoa(historicalVersion), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "older.sqlite")
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			file.Close()
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT`); err != nil {
				t.Fatal(err)
			}
			paths, err := filepath.Glob("../migrations/*.sql")
			if err != nil {
				t.Fatal(err)
			}
			for _, scriptPath := range paths {
				prefix, _, _ := strings.Cut(filepath.Base(scriptPath), "_")
				version, err := strconv.Atoi(prefix)
				if err != nil {
					t.Fatal(err)
				}
				if version > historicalVersion {
					continue
				}
				body, err := os.ReadFile(scriptPath)
				if err != nil {
					t.Fatal(err)
				}
				var lines []string
				for _, line := range strings.Split(string(body), "\n") {
					if !strings.HasPrefix(strings.TrimSpace(line), "--") {
						lines = append(lines, line)
					}
				}
				for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
					if strings.TrimSpace(statement) == "" {
						continue
					}
					if _, err := db.Exec(statement); err != nil {
						t.Fatalf("historical migration %d: %v", version, err)
					}
				}
				checksum := fmt.Sprintf("%x", sha256.Sum256(body))
				if _, err := db.Exec(`INSERT INTO schema_migrations VALUES (?,?,'now')`, version, checksum); err != nil {
					t.Fatal(err)
				}
			}
			public, _, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			owner, err := localsetup.Initialize(context.Background(), db, localsetup.Input{TenantName: "Older", StoreName: "Store", OwnerName: "Owner", DeviceName: "Device", Password: "older-owner-password", PublicKey: public})
			if err != nil {
				t.Fatal(err)
			}
			device := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
			key := make([]byte, 32)
			rand.Read(key)
			archive := filepath.Join(dir, "older.tytbak")
			if err := Create(context.Background(), db, device, key, archive); err != nil {
				t.Fatal(err)
			}
			recovered := filepath.Join(dir, "recovered.sqlite")
			if err := Restore(context.Background(), archive, recovered, device, key); err != nil {
				t.Fatal(err)
			}
			current, err := localdb.Open(context.Background(), recovered)
			if err != nil {
				t.Fatal(err)
			}
			defer current.Close()
			var count int
			if err := current.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != 37 {
				t.Fatalf("migration count %d %v", count, err)
			}
			if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != historicalVersion {
				t.Fatal("modified original database")
			}
		})
	}
}
