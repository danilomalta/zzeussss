package localdb

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func TestMigrationProcessExitRollsBackAndReopens(t *testing.T) {
	db, path := temporaryDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestMigrationCrashChild$")
	command.Env = append(os.Environ(), "TITAN_TEST_MIGRATION_CRASH_PATH="+path)
	err := command.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 77 {
		t.Fatalf("unexpected child result: %v", err)
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err := reopened.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='crash_probe'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial migration after process exit: %d %v", count, err)
	}
}

func TestMigrationCrashChild(t *testing.T) {
	path := os.Getenv("TITAN_TEST_MIGRATION_CRASH_PATH")
	if path == "" {
		t.Skip("subprocess only")
	}
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`CREATE TABLE crash_probe(id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations VALUES (35,'interrupted','now')`); err != nil {
		t.Fatal(err)
	}
	// Simulate abrupt termination without Rollback, Close or deferred cleanup.
	os.Exit(77)
}

func TestFutureSchemaAndHistoryDamageRefuseAllMigrationWrites(t *testing.T) {
	for _, damage := range []string{
		`INSERT INTO schema_migrations VALUES (9999,'future','now')`,
		`UPDATE schema_migrations SET checksum='changed' WHERE version=1`,
		`DELETE FROM schema_migrations WHERE version=2`,
	} {
		t.Run(damage, func(t *testing.T) {
			db, _ := temporaryDB(t)
			if _, err := db.Exec(damage); err != nil {
				t.Fatal(err)
			}
			var before, after int
			db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&before)
			if err := Migrate(context.Background(), db); err == nil {
				t.Fatal("accepted damaged or future history")
			}
			db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&after)
			if before != after {
				t.Fatal("modified rejected history")
			}
		})
	}
}

func TestFailedMigrationRollsBackDDLAndCanRetry(t *testing.T) {
	db, _ := temporaryDB(t)
	if err := applyMigration(context.Background(), db, 35, "test", `CREATE TABLE migration_probe(id INTEGER); INSERT INTO missing_table VALUES (1);`); err == nil {
		t.Fatal("accepted broken migration")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='migration_probe'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial DDL persisted: %d %v", count, err)
	}
	if err := applyMigration(context.Background(), db, 35, "test", `CREATE TABLE migration_probe(id INTEGER);`); err != nil {
		t.Fatal(err)
	}
}
