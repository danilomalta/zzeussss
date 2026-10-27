package localdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestSupplierReturnMigration57To58PreservesDataAndChecksums(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "previous.sqlite")
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	f.Close()
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT`); e != nil {
		t.Fatal(e)
	}
	entries, e := migrations.ReadDir("migrations")
	if e != nil {
		t.Fatal(e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	index := -1
	for i, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_supplier_returns.sql") {
			index = i
		}
	}
	if index < 1 {
		t.Fatal("supply cycle migration missing")
	}
	for _, entry := range entries[:index] {
		body, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		v, e := strconv.Atoi(prefix)
		if e != nil {
			t.Fatal(e)
		}
		if e = applyMigration(ctx, db, v, fmt.Sprintf("%x", sha256.Sum256(body)), string(body)); e != nil {
			t.Fatal(entry.Name(), e)
		}
	}
	if _, e = db.Exec(`CREATE TABLE receiving_case_upgrade_sentinel(value TEXT); INSERT INTO receiving_case_upgrade_sentinel VALUES('preserved')`); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	var value string
	var count int
	if e = db.QueryRow(`SELECT value FROM receiving_case_upgrade_sentinel`).Scan(&value); e != nil || value != "preserved" {
		t.Fatal(value, e)
	}
	if e = db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); e != nil || count != len(entries) {
		t.Fatal(count, e)
	}
	for _, name := range []string{"purchase_supplier_returns", "purchase_supplier_return_operations", "purchase_supplier_return_events"} {
		if e = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=? AND type='table'`, name).Scan(&count); e != nil || count != 1 {
			t.Fatal(name, count, e)
		}
	}
}
