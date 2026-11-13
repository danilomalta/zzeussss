package fiscalproduction

import (
	"database/sql"
	"errors"
	"modernc.org/sqlite"
	"os"
	"path/filepath"
	"testing"
	"titansystem-backend/internal/privatefile"
	"titansystem-backend/internal/sqlitepath"
)

// Separate OS-path proof: never include a DSN, path or credential in diagnostics.
func TestWindowsVaultSQLiteURIIdentity(t *testing.T) {
	dir, err := privatefile.TempDir(t.TempDir(), "uri-")
	if err != nil {
		t.Fatal("private vault URI fixture unavailable")
	}
	path := filepath.Join(dir, "vault.sqlite")
	f, err := privatefile.OpenNew(path, os.O_RDWR)
	if err != nil {
		t.Fatal("private vault URI file unavailable")
	}
	f.Close()
	u, err := sqlitepath.URL(path)
	if err != nil {
		t.Fatal("vault URI construction refused")
	}
	db, err := sql.Open("sqlite", u.String())
	if err == nil {
		defer db.Close()
		err = db.Ping()
	}
	if err != nil {
		code := -1
		var cause *sqlite.Error
		if errors.As(err, &cause) {
			code = cause.Code()
		}
		t.Fatalf("vault sqlite code: %d", code)
	}
	var file string
	if err = db.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&file); err != nil {
		t.Fatal("vault URI identity unavailable")
	}
	if filepath.Clean(file) != filepath.Clean(path) {
		t.Fatal("vault URI opened a different path")
	}
}
