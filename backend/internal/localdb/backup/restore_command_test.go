package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/localdb"
)

func TestRestoreCommandChecksAllBusinessTablesAndCleansPrivateCopies(t *testing.T) {
	db, d, key, dir := fixture(t)
	ctx := context.Background()
	archive := filepath.Join(dir, "customer.tytbak")
	if err := Create(ctx, db, d, key, archive); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "external-mounted-destination")
	if os.Mkdir(root, 0700) != nil {
		t.Fatal("mkdir")
	}
	report, err := TestRestore(ctx, archive, root, d, key)
	if err != nil {
		t.Fatal(err)
	}
	latest, _ := localdb.LatestSchemaVersion()
	if !report.DataMatches || !report.SessionsRevoked || report.SchemaVersion != latest || report.TablesChecked <= len(pilotTables) {
		t.Fatal(report)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("plaintext left behind")
	}
	wrong := append([]byte{}, key...)
	wrong[0] ^= 1
	if _, err = TestRestore(ctx, archive, root, d, wrong); err == nil {
		t.Fatal("wrong key")
	}
	entries, _ = os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("failed validation left files")
	}
	foreign := d
	foreign.TenantID = "another-customer"
	if _, err = TestRestore(ctx, archive, root, foreign, key); err == nil {
		t.Fatal("foreign tenant")
	}
	var live int
	db.QueryRow(`SELECT count(*) FROM local_sessions WHERE revoked_unix IS NULL`).Scan(&live)
	if live != 1 {
		t.Fatal("live installation modified")
	}
	link := filepath.Join(dir, "alias")
	os.Symlink(root, link)
	if _, err = TestRestore(ctx, archive, link, d, key); err == nil {
		t.Fatal("symlink destination")
	}
}
