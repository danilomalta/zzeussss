package localdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Exercise the actual driver parser, not a hand-split approximation. Trigger
// bodies and semicolons inside text must survive; failures roll back all DDL.
func TestMigrationCompleteScriptSupportsTriggersAndAtomicRetry(t *testing.T) {
	db, _ := temporaryDB(t)
	script := `CREATE TABLE journal_probe(id INTEGER PRIMARY KEY, message TEXT);
	CREATE TRIGGER journal_probe_guard BEFORE INSERT ON journal_probe WHEN NEW.id=2
	BEGIN SELECT RAISE(ABORT,'blocked; still one string'); END;
	INSERT INTO journal_probe VALUES (1,'valid; literal');`
	if err := applyMigration(context.Background(), db, 51, "probe", script+`INSERT INTO journal_probe VALUES(2,'reject');`); err == nil {
		t.Fatal("accepted failing trigger script")
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name LIKE 'journal_probe%'`).Scan(&n); err != nil || n != 0 {
		t.Fatal("partial DDL/trigger after failure", n, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=51`).Scan(&n); err != nil || n != 0 {
		t.Fatal("failed script recorded", n, err)
	}
	if err := applyMigration(context.Background(), db, 51, "probe", script); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO journal_probe VALUES(2,'reject')`); err == nil {
		t.Fatal("trigger body lost")
	}
	var text string
	if err := db.QueryRow(`SELECT message FROM journal_probe WHERE id=1`).Scan(&text); err != nil || text != "valid; literal" {
		t.Fatal(text, err)
	}
	if err := applyMigration(context.Background(), db, 51, "probe", script); err != nil {
		t.Fatal("idempotent script replay", err)
	}
}

// Construct an actual pre-47 file using its unchanged embedded migrations.
// Historical cash without an authoritative audit retains unknown provenance.
func TestJournalMigrationBackfillsKnownAndUnknownSourcesWithoutGuessing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema46.sqlite")
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
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		prefix, _, _ := strings.Cut(e.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if err != nil {
			t.Fatal(err)
		}
		if version > 46 {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := applyMigration(context.Background(), db, version, fmt.Sprintf("%x", sha256.Sum256(body)), string(body)); err != nil {
			t.Fatal(e.Name(), err)
		}
	}
	for _, q := range []string{
		`INSERT INTO tenants VALUES('t','Historical','now')`,
		`INSERT INTO stores VALUES('t','s','Store')`,
		`INSERT INTO devices VALUES('t','s','d','Device')`,
		`INSERT INTO users VALUES('t','person','Operator')`,
		`INSERT INTO identities VALUES('person','Operator','now')`,
		`INSERT INTO memberships VALUES('t','person','owner','active','now')`,
		`INSERT INTO cash_sessions VALUES('t','s','d','historical','person','2026-10-01T10:00:00Z','2026-10-01T11:00:00Z',1000)`,
		`INSERT INTO cash_session_operators VALUES('t','s','historical','person')`,
		`INSERT INTO cash_movements VALUES('old-supply','t','s','historical',500,'supply:documented','2026-10-01T10:10:00Z')`,
		`INSERT INTO cash_adjustments VALUES('t','s','d','supply-op','historical','person','supply',500,'documented','old-supply','2026-10-01T10:10:00Z')`,
		// A suggestive reason without its authoritative operation is NOT a sale.
		`INSERT INTO cash_movements VALUES('old-unknown','t','s','historical',200,'venda:missing-sale','2026-10-01T10:20:00Z')`,
		`INSERT INTO cash_sessions VALUES('t','s','d','unbound','person','2026-10-01T09:00:00Z','2026-10-01T09:30:00Z',0)`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var category, counter string
	var actor, op sql.NullString
	if err := db.QueryRow(`SELECT category,counter_account,actor_identity_id,operation_id FROM cash_journal_entries WHERE entry_id='movement:old-unknown'`).Scan(&category, &counter, &actor, &op); err != nil || category != "unclassified" || counter != "unclassified_control" || actor.Valid || op.Valid {
		t.Fatal("invented unknown provenance", category, counter, actor, op, err)
	}
	if err := db.QueryRow(`SELECT category,counter_account,actor_identity_id,operation_id FROM cash_journal_entries WHERE entry_id='movement:old-supply'`).Scan(&category, &counter, &actor, &op); err != nil || category != "supply" || counter != "supply_control" || !actor.Valid || actor.String != "person" || !op.Valid || op.String != "supply-op" {
		t.Fatal("lost historical audit", category, counter, actor, op, err)
	}
	if err := db.QueryRow(`SELECT actor_identity_id,operation_id FROM cash_journal_entries WHERE entry_id='opening:unbound'`).Scan(&actor, &op); err != nil || actor.Valid || op.Valid {
		t.Fatal("guessed unbound operator", actor, op, err)
	}
	var before, after string
	if err := db.QueryRow(`SELECT group_concat(entry_id||':'||sequence,'|') FROM (SELECT entry_id,sequence FROM cash_journal_entries ORDER BY sequence)`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT group_concat(entry_id||':'||sequence,'|') FROM (SELECT entry_id,sequence FROM cash_journal_entries ORDER BY sequence)`).Scan(&after); err != nil || before != after {
		t.Fatal("migration changed historical IDs or order", err)
	}
	if _, err = db.Exec(`UPDATE cash_journal_entries SET cash_delta_cents=1`); err == nil {
		t.Fatal("backfill editable")
	}
}
