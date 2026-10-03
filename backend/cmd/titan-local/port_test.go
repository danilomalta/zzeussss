package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServeRejectsInvalidPortBeforeOpeningFiles(t *testing.T) {
	for _, port := range []string{"0", "-1", "65536"} {
		t.Run(port, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "absent.sqlite")
			err := serveStation([]string{"--db", dbPath, "--station", filepath.Join(dir, "absent.station"), "--port", port})
			if err == nil || err.Error() != "porta local deve estar entre 1 e 65535" {
				t.Fatalf("porta nao validada antes dos arquivos: %v", err)
			}
			if _, err := os.Lstat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("banco foi criado: %v", err)
			}
		})
	}
}
