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
	"sync"
	"testing"
)

func consolidationFixture(t *testing.T, fork bool) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.sqlite")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		n, _ := strconv.Atoi(prefix)
		if n > 85 && (fork || n > 87) {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err = applyMigration(context.Background(), db, n, fmt.Sprintf("%x", sha256.Sum256(body)), string(body)); err != nil {
			t.Fatal(err)
		}
	}
	if fork {
		body, _ := migrations.ReadFile("migrations/0088_fiscal_nfe_origins.sql")
		if err = applyMigration(context.Background(), db, 86, fmt.Sprintf("%x", sha256.Sum256(body)), string(body)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO tenants(id,name,created_at) VALUES('preserved','Synthetic','original-date')`); err != nil {
		t.Fatal(err)
	}
	if fork {
		for _, query := range []string{
			`INSERT INTO stores(tenant_id,id,name) VALUES('preserved','store','Synthetic')`,
			`INSERT INTO devices(tenant_id,store_id,id,name) VALUES('preserved','store','device','Synthetic')`,
			`INSERT INTO users(tenant_id,id,display_name) VALUES('preserved','actor','Synthetic')`,
			`INSERT INTO fiscal_nfe_origins VALUES('preserved','store','origin','inbound_receipt','receipt','11111111111111111111111111111111111111111111','device','actor','operation','{}','{}','0000000000000000000000000000000000000000000000000000000000000000','original-date')`,
		} {
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
		}
	}
	return db, path
}

func TestConsolidationBothHistoriesPreserveDataAndChecksums(t *testing.T) {
	for _, fork := range []bool{false, true} {
		t.Run(fmt.Sprint(fork), func(t *testing.T) {
			db, path := consolidationFixture(t, fork)
			var original string
			if fork {
				if err := db.QueryRow(`SELECT applied_at FROM schema_migrations WHERE version=86`).Scan(&original); err != nil {
					t.Fatal(err)
				}
			}
			if err := ValidateSchema(context.Background(), db); err != nil {
				t.Fatal(err)
			}
			db.Close()
			reopened, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if err = Migrate(context.Background(), reopened); err != nil {
				t.Fatal(err)
			}
			var count int
			var name string
			if err = reopened.QueryRow(`SELECT name FROM tenants WHERE id='preserved'`).Scan(&name); err != nil || name != "Synthetic" {
				t.Fatal("tenant data changed", err)
			}
			latest, err := LatestSchemaVersion()
			if err != nil {
				t.Fatal(err)
			}
			if err = reopened.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil || count != latest {
				t.Fatal("incomplete schema", count, err)
			}
			if fork {
				var source string
				if err = reopened.QueryRow(`SELECT source_id FROM fiscal_nfe_origins WHERE origin_id='origin' AND tenant_id='preserved'`).Scan(&source); err != nil || source != "receipt" {
					t.Fatal("fiscal origin lost", err)
				}
				var date string
				if err = reopened.QueryRow(`SELECT applied_at FROM schema_migrations WHERE version=88`).Scan(&date); err != nil || date != original {
					t.Fatal("origin date changed", err)
				}
			}
			for _, table := range []string{"fiscal_nfe_origins", "auth_attempt_budgets", "supplier_contact_policies"} {
				if err = reopened.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
					t.Fatal("missing table", table, err)
				}
			}
		})
	}
}

func TestConsolidationRollbackAndUnknownHistoryFailClosed(t *testing.T) {
	db, _ := consolidationFixture(t, true)
	if _, err := db.Exec(`CREATE TRIGGER refuse_consolidation BEFORE INSERT ON schema_migrations WHEN NEW.version=87 BEGIN SELECT RAISE(ABORT,'synthetic interruption'); END`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err == nil {
		t.Fatal("interruption accepted")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='auth_attempt_budgets'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial DDL", err)
	}
	if err := db.QueryRow(`SELECT max(version) FROM schema_migrations`).Scan(&count); err != nil || count != 86 {
		t.Fatal("history partially moved", err)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET checksum='unknown' WHERE version=86`); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchema(context.Background(), db); err == nil {
		t.Fatal("unknown fork accepted")
	}
	if err := Migrate(context.Background(), db); err == nil {
		t.Fatal("unknown history mutated")
	}
}

func TestConsolidationConcurrentUpgradeConsistent(t *testing.T) {
	db, _ := consolidationFixture(t, true)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- consolidateOriginsFork(context.Background(), db) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version IN (86,87,88)`).Scan(&count); err != nil || count != 3 {
		t.Fatal("inconsistent upgrade", err)
	}
}
