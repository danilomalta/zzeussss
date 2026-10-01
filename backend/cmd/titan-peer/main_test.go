package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/localsetup"
	"titansystem-backend/internal/localdb/stationfile"
)

const peerTestPassword = "senha-de-teste-longa"

type peerFixture struct {
	db                                  *sql.DB
	result                              localsetup.Result
	dbPath, stationPath, encryptionPath string
	key                                 ed25519.PrivateKey
}

func newPeerFixture(t *testing.T) peerFixture {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "store.sqlite")
	db, err := localdb.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	result, err := localsetup.Initialize(context.Background(), db, localsetup.Input{TenantName: "Empresa", StoreName: "Loja", OwnerName: "Dono", DeviceName: "Caixa", Password: peerTestPassword, PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	stationPath := filepath.Join(dir, "station.json")
	raw, err := json.Marshal(stationfile.Station{TenantID: result.TenantID, StoreID: result.StoreID, DeviceID: result.DeviceID, PrivateKey: base64.RawURLEncoding.EncodeToString(key)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stationPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return peerFixture{db: db, result: result, dbPath: dbPath, stationPath: stationPath, encryptionPath: filepath.Join(dir, "encryption.json"), key: key}
}

func (f peerFixture) initArgs() []string {
	return []string{"key-init", "--db", f.dbPath, "--station", f.stationPath, "--encryption", f.encryptionPath, "--owner", f.result.OwnerID}
}
func (f peerFixture) device() identity.DeviceContext {
	return identity.DeviceContext{TenantID: f.result.TenantID, StoreID: f.result.StoreID, DeviceID: f.result.DeviceID}
}
func peerCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestPeerOwnerKeyInitExportInspectWithoutPrivateContent(t *testing.T) {
	f := newPeerFixture(t)
	var output bytes.Buffer
	if err := runPeer(context.Background(), f.initArgs(), strings.NewReader(peerTestPassword+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f.encryptionPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private permissions: %v", err)
	}
	if peerCount(t, f.db, "device_encryption_keys") != 1 || peerCount(t, f.db, "device_encryption_key_audit") != 1 {
		t.Fatal("missing owner approval audit")
	}
	original, err := os.ReadFile(f.encryptionPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := runPeer(context.Background(), f.initArgs(), strings.NewReader(peerTestPassword), &output); err == nil {
		t.Fatal("second initialization accepted")
	}
	current, _ := os.ReadFile(f.encryptionPath)
	if !bytes.Equal(original, current) {
		t.Fatal("private key overwritten")
	}
	other := f
	other.encryptionPath = filepath.Join(filepath.Dir(f.dbPath), "other-key.json")
	if err := runPeer(context.Background(), other.initArgs(), strings.NewReader(peerTestPassword), &output); err == nil {
		t.Fatal("silent key rotation accepted")
	}
	if _, err := os.Stat(other.encryptionPath); !os.IsNotExist(err) {
		t.Fatal("second key created")
	}
	outPath := filepath.Join(filepath.Dir(f.dbPath), "public.json")
	args := []string{"export", "--db", f.dbPath, "--station", f.stationPath, "--encryption", f.encryptionPath, "--out", outPath}
	if err := runPeer(context.Background(), args, nil, &output); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, _, err := incoming.ParsePublicDescriptor(body)
	if err != nil || descriptor.Binding.Device != f.device() {
		t.Fatalf("public export: %v", err)
	}
	encryption, err := incoming.LoadEncryptionKey(f.encryptionPath, f.device())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{peerTestPassword, "private_key", base64.RawURLEncoding.EncodeToString(f.key), base64.StdEncoding.EncodeToString(encryption.Bytes())} {
		if strings.Contains(output.String(), secret) || strings.Contains(string(body), secret) {
			t.Fatal("export exposed secret")
		}
	}
	if err := runPeer(context.Background(), args, nil, &output); err == nil {
		t.Fatal("public descriptor overwritten")
	}
	if err := runPeer(context.Background(), []string{"inspect", "--in", outPath}, nil, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "NÃO aprova") || peerCount(t, f.db, "device_encryption_keys") != 1 || peerCount(t, f.db, "device_encryption_key_audit") != 1 || peerCount(t, f.db, "sync_incoming_peers") != 0 {
		t.Fatal("inspect granted remote trust")
	}
}

func TestPeerWrongPasswordManagerAndRevokedOwnerCannotCreateKey(t *testing.T) {
	for _, scenario := range []string{"password", "manager", "revoked"} {
		t.Run(scenario, func(t *testing.T) {
			f := newPeerFixture(t)
			password := peerTestPassword
			switch scenario {
			case "password":
				password = "senha-incorreta"
			case "manager":
				if _, err := f.db.Exec("UPDATE memberships SET role='manager'"); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec("INSERT INTO membership_stores (tenant_id,identity_id,store_id) VALUES (?,?,?)", f.result.TenantID, f.result.OwnerID, f.result.StoreID); err != nil {
					t.Fatal(err)
				}
			case "revoked":
				if _, err := f.db.Exec("UPDATE memberships SET status='revoked'"); err != nil {
					t.Fatal(err)
				}
			}
			if err := runPeer(context.Background(), f.initArgs(), strings.NewReader(password), &bytes.Buffer{}); err == nil {
				t.Fatal("unauthorized key initialization")
			}
			if _, err := os.Stat(f.encryptionPath); !os.IsNotExist(err) {
				t.Fatal("unauthorized private file created")
			}
			if peerCount(t, f.db, "device_encryption_keys") != 0 {
				t.Fatal("unauthorized public key approved")
			}
		})
	}
}

func TestPeerApprovalFailureKeepsPrivateFileAndRollsBackDatabase(t *testing.T) {
	f := newPeerFixture(t)
	if _, err := f.db.Exec("CREATE TRIGGER reject_key_audit BEFORE INSERT ON device_encryption_key_audit BEGIN SELECT RAISE(ABORT,'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	if err := runPeer(context.Background(), f.initArgs(), strings.NewReader(peerTestPassword), &bytes.Buffer{}); err == nil {
		t.Fatal("approval failure hidden")
	}
	if _, err := incoming.LoadEncryptionKey(f.encryptionPath, f.device()); err != nil {
		t.Fatal("recoverable private file was removed")
	}
	if peerCount(t, f.db, "device_encryption_keys") != 0 || peerCount(t, f.db, "device_encryption_key_audit") != 0 {
		t.Fatal("partial database approval")
	}
}

func TestPeerInvalidCommandDoesNotCreateDatabase(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.sqlite")
	for _, args := range [][]string{{}, {"unknown"}, {"key-init", "--db", missing}, {"export", "--db", missing}, {"inspect", "--in", missing}, {"key-init", "--unknown"}} {
		if err := runPeer(context.Background(), args, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("invalid arguments created a database")
	}
}
