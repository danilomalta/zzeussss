package fiscalproduction

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/sqlitewrite"
)

// Explicit maintenance must neither request the rotation's writer lock nor
// classify its in-flight export as an orphan, even before the reference commits.
func TestVaultMaintenanceDoesNotAcquireRotationWriter(t *testing.T) {
	dir := t.TempDir()
	vault, keyFile := filepath.Join(dir, "vault.sqlite"), filepath.Join(dir, "active.key")
	if err := CreateCustomerFiscalVault(vault, keyFile, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	v, err := OpenCustomerFiscalVault(vault, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	makeCandidate := func(name string, pending bool) string {
		t.Helper()
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			t.Fatal(err)
		}
		defer clear(raw)
		encoded := []byte(base64.StdEncoding.EncodeToString(raw))
		defer clear(encoded)
		path := filepath.Join(dir, name)
		if _, err := recordKeyCandidateState(vault, path, encoded, pending); err != nil {
			t.Fatal(err)
		}
		if err := privateNew(path, encoded); err != nil {
			t.Fatal(err)
		}
		return path
	}
	orphan := makeCandidate("completed-rollback.key", false)
	err = sqlitewrite.Run(context.Background(), v.db, fiscal.ErrConflict, func(tx *sql.Tx) error {
		pending := makeCandidate("rotation-in-progress.key", true)
		// BEGIN IMMEDIATE is already held. A maintenance writer would contend here.
		items, err := ListVaultOrphans(vault, keyFile, dir)
		if err != nil {
			return err
		}
		if len(items) != 1 {
			t.Fatalf("expected only completed orphan, got %d", len(items))
		}
		if err := RemoveVaultOrphan(vault, keyFile, dir, items[0].ID, "DELETE "+items[0].ID); err != nil {
			return err
		}
		if _, err := os.Stat(pending); err != nil {
			t.Fatal("in-flight key removed")
		}
		if _, err := os.Stat(keyFile); err != nil {
			t.Fatal("active key removed")
		}
		if _, err := os.Stat(orphan); !os.IsNotExist(err) {
			t.Fatal("explicit removal did not occur")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
