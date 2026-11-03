package localdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This base already contains supply-cycle/returns migrations57/58. Adding Fleet
// must append59/60 and leave historical checksums, tables and receipts intact.
func TestFleetBase58UpgradePreservesSupplyCycleAndReturnSchema(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "base58.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA foreign_keys=ON; CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT`); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	expected := map[int]string{}
	for _, entry := range entries {
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		n, e := strconv.Atoi(prefix)
		if e != nil {
			t.Fatal(e)
		}
		if n > 58 {
			continue
		}
		body, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(body))
		if e = applyMigration(ctx, db, n, checksum, string(body)); e != nil {
			t.Fatal(entry.Name(), e)
		}
		expected[n] = checksum
	}
	if len(expected) != 58 {
		t.Fatalf("base has %d migrations", len(expected))
	}
	const setup = `INSERT INTO tenants VALUES('T','Discardable','now');INSERT INTO stores VALUES('T','S','Test');INSERT INTO devices VALUES('T','S','D','Test');INSERT INTO identities VALUES('U','Test','now');INSERT INTO memberships VALUES('T','U','manager','active','now');INSERT INTO purchase_cycle_operations VALUES('T','S','D','ORIGINAL','U','delivery','DELIVERY',1,'{"fixture":"original request"}','{"fixture":"original receipt"}','now');INSERT INTO purchase_cycle_events VALUES('T','S','D','ORIGINAL','delivery','{"fixture":"original event"}','now');`
	if _, err = db.Exec(setup); err != nil {
		t.Fatal(err)
	}
	names := []string{"purchase_supplier_decisions", "purchase_deliveries", "purchase_cycle_operations", "purchase_cycle_events", "payable_receiving_origins", "purchase_supplier_returns", "purchase_supplier_return_operations", "purchase_supplier_return_events"}
	schemas := map[string]string{}
	for _, name := range names {
		var value string
		if err = db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&value); err != nil {
			t.Fatal(name, err)
		}
		schemas[name] = value
	}
	for range 2 {
		if err = Migrate(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	for n, want := range expected {
		var got string
		if err = db.QueryRow(`SELECT checksum FROM schema_migrations WHERE version=?`, n).Scan(&got); err != nil || got != want {
			t.Fatalf("historical migration%d changed: %q %v", n, got, err)
		}
	}
	for name, want := range schemas {
		var got string
		if err = db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got); err != nil || got != want {
			t.Fatalf("schema changed for %s: %v", name, err)
		}
	}
	var result, event string
	if err = db.QueryRow(`SELECT result_json FROM purchase_cycle_operations WHERE operation_id='ORIGINAL'`).Scan(&result); err != nil || result != `{"fixture":"original receipt"}` {
		t.Fatal(result, err)
	}
	if err = db.QueryRow(`SELECT payload_json FROM purchase_cycle_events WHERE operation_id='ORIGINAL'`).Scan(&event); err != nil || event != `{"fixture":"original event"}` {
		t.Fatal(event, err)
	}
	if _, err = db.Exec(`UPDATE purchase_cycle_operations SET result_json='{}' WHERE operation_id='ORIGINAL'`); err == nil {
		t.Fatal("historical receipt lost immutable constraint")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != len(entries) {
		t.Fatal(count, err)
	}
	for _, name := range []string{"fleet_records", "fleet_operations", "fleet_workshop_records", "fleet_workshop_operations"} {
		if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&count); err != nil || count != 1 {
			t.Fatal(name, count, err)
		}
	}
}
