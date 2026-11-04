package sqlitewrite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	sqlite "modernc.org/sqlite"
)

func TestRunPreservesLongerSourceWaitAndReleasesConnection(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "source.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE writes(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout(5000)`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = Run(ctx, db, errors.New("revision conflict"), func(tx *sql.Tx) error {
		// This hangs if metadata keeps the sole source connection checked out.
		var value int
		if err := db.QueryRowContext(ctx, `SELECT 1`).Scan(&value); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO writes VALUES(1)`)
		return err
	})
	if err != nil {
		t.Fatal("source pool was retained or writer failed", err)
	}
	var timeout, count int
	if err := db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil || timeout != 5000 {
		t.Fatal("a longer source wait was changed")
	}
	if err := db.QueryRow(`SELECT count(*) FROM writes`).Scan(&count); err != nil || count != 1 {
		t.Fatal("write was lost or duplicated")
	}
}

func TestRunExclusiveLockExhaustionReturnsConflictAndSQLiteCause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.sqlite")
	holder, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if _, err := holder.Exec(`CREATE TABLE writes(id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	conn, err := holder.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), `BEGIN EXCLUSIVE`); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(context.Background(), `ROLLBACK`)
	source, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conflict := errors.New("revision conflict")
	called := false
	err = Run(ctx, source, conflict, func(tx *sql.Tx) error { called = true; return nil })
	var cause *sqlite.Error
	if !errors.Is(err, conflict) || !errors.As(err, &cause) || cause.Code()&255 != 5 {
		t.Fatal("bounded contention lost its conflict classification or SQLite cause", err)
	}
	if called {
		t.Fatal("business callback ran without acquiring writer rights")
	}
}

func TestRunRejectsMemorySourceWithoutCallingBusinessCode(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	called := false
	err = Run(context.Background(), db, errors.New("revision conflict"), func(tx *sql.Tx) error { called = true; return nil })
	if err == nil || called {
		t.Fatal("memory source accepted or business callback ran")
	}
}
