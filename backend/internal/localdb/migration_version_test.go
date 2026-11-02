package localdb

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLatestSchemaVersionMatchesEveryEmbeddedMigration(t *testing.T) {
	paths, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	latest, err := LatestSchemaVersion()
	if err != nil || latest != len(paths) {
		t.Fatal("schema ceiling differs from embedded files", latest, err)
	}
	seen := make(map[int]bool)
	for _, path := range paths {
		prefix, _, ok := strings.Cut(filepath.Base(path), "_")
		version, err := strconv.Atoi(prefix)
		if !ok || err != nil || version < 1 || version > latest || seen[version] {
			t.Fatal("invalid migration version")
		}
		seen[version] = true
	}
}
