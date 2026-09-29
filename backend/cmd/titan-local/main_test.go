package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb"
)

func TestInitCreatesPrivateKeyAndRejectsSecondRun(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "titan.sqlite")
	stationPath := filepath.Join(dir, "private", "station.json")
	args := []string{"--db", dbPath, "--station", stationPath, "--empresa", "Mercado", "--loja", "Matriz", "--dono", "Dono"}
	var output bytes.Buffer
	if err := initStation(args, strings.NewReader("senha-bem-forte-2026\n"), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "senha-bem-forte") || strings.Contains(output.String(), "private_key") {
		t.Fatal("saída imprimiu segredo")
	}
	info, err := os.Stat(stationPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissões: %v %v", info, err)
	}
	reopened, err := localdb.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
	verified, device, err := openVerified(context.Background(), dbPath, stationPath)
	if err != nil || device.TenantID == "" {
		t.Fatalf("prova do aparelho: %+v %v", device, err)
	}
	if err = verified.Close(); err != nil {
		t.Fatal(err)
	}
	if err := initStation(args, strings.NewReader("senha-bem-forte-2026"), &output); err == nil {
		t.Fatal("sobrescreveu instalação")
	}
}
