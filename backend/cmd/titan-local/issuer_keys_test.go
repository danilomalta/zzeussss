package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestIssuerConfigurationIsOptionalButInvalidFileFails(t *testing.T) {
	verifier, err := readIssuerVerifier("")
	if err != nil || verifier != nil {
		t.Fatalf("sem configuracao: %v %v", verifier, err)
	}
	dir := t.TempDir()
	for _, path := range []string{dir, filepath.Join(dir, "missing.json")} {
		if _, err := readIssuerVerifier(path); err == nil {
			t.Fatal("arquivo invalido aceito")
		}
	}
	path := filepath.Join(dir, "keys.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readIssuerVerifier(path); err == nil {
		t.Fatal("configuracao vazia aceita")
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"issuer":"`+base64.StdEncoding.EncodeToString(public)+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if verifier, err := readIssuerVerifier(path); err != nil || verifier == nil {
		t.Fatalf("configuracao valida: %v", err)
	}
}

func TestServeRejectsBadIssuerConfigurationBeforeOpeningDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "absent.sqlite")
	err := serveStation([]string{"--db", dbPath, "--station", filepath.Join(dir, "absent.station"), "--issuer-keys", filepath.Join(dir, "missing-keys.json")})
	if err == nil || err.Error() != "nao foi possivel abrir configuracao de chaves emissoras" {
		t.Fatalf("configuracao nao validada primeiro: %v", err)
	}
	if _, err := os.Lstat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("banco foi criado: %v", err)
	}
}
