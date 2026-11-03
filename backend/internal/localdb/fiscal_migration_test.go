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

func TestFiscalMigration66To67PreservesDataAndChecksums(t *testing.T) {
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
		if strings.HasSuffix(entry.Name(), "_fiscal_intents.sql") {
			index = i
		}
	}
	if index < 1 {
		t.Fatal("fiscal migration missing")
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
	// Seed prior trust and recovery evidence before upgrading. These are
	// migration fixtures only; authorization is covered by the real API pilots.
	if _, e = db.Exec(`INSERT INTO tenants VALUES('upgrade-tenant','Upgrade','now');
		INSERT INTO stores VALUES('upgrade-tenant','upgrade-store','Store');
		INSERT INTO company_trade_peers VALUES('upgrade-tenant','upgrade-store','upgrade-peer','remote-tenant','remote-store','remote-device','revoked',2,'{"status":"revoked","revision":2}');
		INSERT INTO company_trade_recovery VALUES('upgrade-tenant','upgrade-store','generation-before-upgrade',1,'','','now')`); e != nil {
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
	var status, generation string
	var required int
	if e = db.QueryRow(`SELECT status FROM company_trade_peers WHERE id='upgrade-peer'`).Scan(&status); e != nil || status != "revoked" {
		t.Fatal("revocation lost during migration", e)
	}
	if e = db.QueryRow(`SELECT generation,required FROM company_trade_recovery WHERE tenant_id='upgrade-tenant'`).Scan(&generation, &required); e != nil || generation != "generation-before-upgrade" || required != 1 {
		t.Fatal("recovery block lost during migration", e)
	}
	for _, entry := range entries[:index] {
		body, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		version, e := strconv.Atoi(prefix)
		if e != nil {
			t.Fatal(e)
		}
		var checksum string
		if e = db.QueryRow(`SELECT checksum FROM schema_migrations WHERE version=?`, version).Scan(&checksum); e != nil || checksum != fmt.Sprintf("%x", sha256.Sum256(body)) {
			t.Fatal("historical migration changed", entry.Name(), e)
		}
	}
	for _, name := range []string{"fiscal_profiles", "fiscal_intents"} {
		if e = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name=? AND type='table'`, name).Scan(&count); e != nil || count != 1 {
			t.Fatal(name, count, e)
		}
	}
}
