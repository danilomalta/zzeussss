package localdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReceivableReconciliationUpgradePreservesDocuments(t *testing.T) {
	ctx := context.Background()
	db, e := sql.Open("sqlite", filepath.Join(t.TempDir(), "upgrade.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`PRAGMA foreign_keys=ON;CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT`); e != nil {
		t.Fatal(e)
	}
	entries, e := migrations.ReadDir("migrations")
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_receivable_reconciliation.sql") {
			found = true
			break
		}
		body, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			t.Fatal(e)
		}
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		version, e := strconv.Atoi(prefix)
		if e != nil {
			t.Fatal(e)
		}
		if e = applyMigration(ctx, db, version, fmt.Sprintf("%x", sha256.Sum256(body)), string(body)); e != nil {
			t.Fatal(e)
		}
	}
	if !found {
		t.Fatal("reconciliation migration missing")
	}
	if _, e = db.Exec(`INSERT INTO tenants VALUES('tenant','Synthetic','now');INSERT INTO stores VALUES('tenant','store','Synthetic');INSERT INTO identities VALUES('owner','Synthetic','now');INSERT INTO memberships VALUES('tenant','owner','owner','active','now');
 INSERT INTO receivable_documents VALUES('tenant','store','document','customer','reference','synthetic','2026-11-01',1000,'open','owner','now');INSERT INTO receivable_installments VALUES('tenant','store','document','part','2026-11-10',1000);`); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	var amount int64
	var reference string
	if e = db.QueryRow(`SELECT total_cents,document_reference FROM receivable_documents WHERE id='document'`).Scan(&amount, &reference); e != nil || amount != 1000 || reference != "reference" {
		t.Fatal("existing document changed", amount, reference, e)
	}
	latest, e := LatestSchemaVersion()
	if e != nil {
		t.Fatal(e)
	}
	var applied int
	if e = db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&applied); e != nil || applied != latest {
		t.Fatal("schema", applied, latest, e)
	}
	if _, e = db.Exec(`INSERT INTO receivable_reconciliations VALUES('tenant','store','device','attempt','owner','document','receive_cash','now')`); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO receivable_operations VALUES('tenant','store','device','attempt','owner','document','receive_cash','','hash','{}','now')`); e == nil {
		t.Fatal("fenced operation inserted")
	}
}
