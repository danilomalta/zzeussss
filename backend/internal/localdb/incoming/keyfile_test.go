package incoming

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb/identity"
)

func TestEncryptionPrivateFileSurvivesReloadAndCannotBeOverwritten(t *testing.T) {
	device := identity.DeviceContext{TenantID: "company", StoreID: "store", DeviceID: "receiver"}
	path := filepath.Join(t.TempDir(), "private", "encryption.json")
	created, err := CreateEncryptionKey(path, device)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissões: %v %v", info, err)
	}
	loaded, err := LoadEncryptionKey(path, device)
	if err != nil || !bytes.Equal(created.Bytes(), loaded.Bytes()) {
		t.Fatal("chave não persistiu")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateEncryptionKey(path, device); err == nil {
		t.Fatal("substituiu chave existente")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("arquivo existente foi alterado")
	}
	device.DeviceID = "other"
	if _, err := LoadEncryptionKey(path, device); err == nil {
		t.Fatal("chave de outro aparelho aceita")
	}
}

func TestEncryptionPrivateFileRejectsUnsafeOrAmbiguousFiles(t *testing.T) {
	device := identity.DeviceContext{TenantID: "company", StoreID: "store", DeviceID: "receiver"}
	dir := t.TempDir()
	path := filepath.Join(dir, "key.json")
	if _, err := CreateEncryptionKey(path, device); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEncryptionKey(link, device); err == nil {
		t.Fatal("link simbólico aceito")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEncryptionKey(path, device); err == nil {
		t.Fatal("arquivo legível por outros aceito")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"version":1,"version":1,"device":{},"private_key":"AA=="}`,
		`{"version":1,"device":{"TenantID":"company","StoreID":"store","DeviceID":"receiver","DeviceID":"receiver"},"private_key":"AA=="}`,
		`{"version":1,"device":null,"private_key":"AA=="}`,
		`{"version":1,"device":{},"private_key":"AA==","extra":1}`,
		`{"version":1,"device":{},"private_key":"AA=="} {}`,
		`{`,
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadEncryptionKey(path, device); err == nil {
			t.Fatal("arquivo inválido aceito")
		}
	}
}
