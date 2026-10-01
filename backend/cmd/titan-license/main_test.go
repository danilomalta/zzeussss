package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
)

var testNow = time.Unix(2000000000, 0).UTC()

func testIssuerKey(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "issuer.private.json")
	if err := run([]string{"init", "--private-key", path, "--key-id", "issuer"}, &bytes.Buffer{}, testNow); err != nil {
		t.Fatal(err)
	}
	return path
}

func signArgs(key, out string) []string {
	return []string{"sign", "--private-key", key, "--out", out, "--tenant", "company-a",
		"--revision", "1", "--expires", testNow.Add(time.Hour).Format(time.RFC3339), "--modules", "pos"}
}

func TestIssuerInitCreatesPrivateFileWithoutPrintingSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issuer.json")
	var output bytes.Buffer
	if err := run([]string{"init", "--private-key", path, "--key-id", "issuer"}, &output, testNow); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissoes: %v %v", info, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var encoded struct {
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(body, &encoded); err != nil || encoded.PrivateKey == "" {
		t.Fatalf("chave gerada invalida: %v", err)
	}
	if strings.Contains(output.String(), encoded.PrivateKey) || strings.Contains(output.String(), "private_key") {
		t.Fatal("saida expos segredo")
	}
	if _, err := readKey(path); err != nil {
		t.Fatal(err)
	}
}

func TestIssuerPublicExportAndSignedContractUseExistingVerifier(t *testing.T) {
	key := testIssuerKey(t)
	dir := t.TempDir()
	publicPath := filepath.Join(dir, "public.json")
	contractPath := filepath.Join(dir, "contract.json")
	if err := run([]string{"public", "--private-key", key, "--out", publicPath}, &bytes.Buffer{}, testNow); err != nil {
		t.Fatal(err)
	}
	public, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(public, []byte("private_key")) {
		t.Fatal("exportacao publica inclui chave privada")
	}
	verifier, err := entitlements.ReadTrustedKeys(bytes.NewReader(public))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(signArgs(key, contractPath), &output, testNow); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	var envelope entitlements.Envelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.Verify(envelope, "company-a", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Revision != 1 || claims.ExpiresAt != testNow.Add(time.Hour).Unix() {
		t.Fatalf("claims: %+v", claims)
	}
	for _, id := range []modules.ID{modules.Core, modules.Inventory, modules.POS} {
		if err := modules.Require(id, claims.Modules); err != nil {
			t.Fatalf("dependencia %s: %v", id, err)
		}
	}
	if err := modules.Require(modules.Staff, claims.Modules); err == nil {
		t.Fatal("modulo nao solicitado concedido")
	}
	if _, err := verifier.Verify(envelope, "company-b", testNow); err == nil {
		t.Fatal("outra empresa aceitou contrato")
	}
	envelope.Payload[0] ^= 1
	if _, err := verifier.Verify(envelope, "company-a", testNow); err == nil {
		t.Fatal("payload adulterado aceito")
	}
}

func TestIssuerNeverOverwritesKeyPublicOrContractFiles(t *testing.T) {
	key := testIssuerKey(t)
	before, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"init", "--private-key", key, "--key-id", "issuer"}, &bytes.Buffer{}, testNow); err == nil {
		t.Fatal("chave sobrescrita")
	}
	after, err := os.ReadFile(key)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("chave alterada")
	}
	out := filepath.Join(t.TempDir(), "existing.json")
	if err := os.WriteFile(out, []byte("preservar"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"public", "--private-key", key, "--out", out}, signArgs(key, out),
	} {
		if err := run(args, &bytes.Buffer{}, testNow); err == nil {
			t.Fatal("arquivo existente sobrescrito")
		}
		body, err := os.ReadFile(out)
		if err != nil || string(body) != "preservar" {
			t.Fatal("arquivo existente alterado")
		}
	}
}

func TestIssuerRejectsInvalidSigningInputsWithoutCreatingContract(t *testing.T) {
	key := testIssuerKey(t)
	for _, change := range []struct{ flag, value string }{
		{"--tenant", ""}, {"--tenant", " company-a"}, {"--revision", "0"}, {"--revision", "-1"},
		{"--expires", "bad"}, {"--expires", testNow.Format(time.RFC3339)},
		{"--modules", ""}, {"--modules", "unknown"}, {"--modules", "pos,pos"}, {"--modules", "pos,"},
	} {
		out := filepath.Join(t.TempDir(), "contract.json")
		args := signArgs(key, out)
		for i := 0; i < len(args)-1; i++ {
			if args[i] == change.flag {
				args[i+1] = change.value
				break
			}
		}
		if err := run(args, &bytes.Buffer{}, testNow); err == nil {
			t.Fatalf("argumento invalido aceito: %s", change.flag)
		}
		if _, err := os.Lstat(out); !os.IsNotExist(err) {
			t.Fatalf("saida criada em falha: %v", err)
		}
	}
}

func TestIssuerRejectsUnsafePrivateFilesAndAmbiguousJSON(t *testing.T) {
	key := testIssuerKey(t)
	body, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(key, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readKey(key); err == nil {
		t.Fatal("chave com permissoes abertas aceita")
	}
	if err := os.Chmod(key, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link.json")
	if err := os.Symlink(key, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readKey(link); err == nil {
		t.Fatal("symlink aceito")
	}
	var decoded issuerKey
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	decoded.PrivateKey[ed25519.SeedSize] ^= 1
	corrupt, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{
		[]byte(`{}`), []byte(`null`), []byte(`{"version":1,"version":1,"key_id":"issuer","private_key":""}`),
		append(append([]byte{}, body...), []byte(` {}`)...), []byte(strings.Repeat(" ", 8193)), corrupt,
	} {
		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, bad, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readKey(path); err == nil {
			t.Fatal("arquivo privado invalido aceito")
		}
	}
}

func TestIssuerRejectsUnknownCommandFlagsAndInvalidKeyID(t *testing.T) {
	for _, args := range [][]string{
		nil, {"unknown"}, {"init", "--bad", "x"}, {"public"},
		{"init", "--private-key", filepath.Join(t.TempDir(), "key.json"), "--key-id", " issuer"},
	} {
		if err := run(args, &bytes.Buffer{}, testNow); err == nil {
			t.Fatal("comando invalido aceito")
		}
	}
}
