package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
)

func writeSyncJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSyncCLIOnceProvesDeviceAndDeliversWithoutPrintingSecrets(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.sqlite")
	source, err := localdb.Open(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	receiver, err := localdb.Open(ctx, filepath.Join(dir, "receiver.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	_, sourceKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, receiverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	from := identity.DeviceContext{TenantID: "company", StoreID: "store", DeviceID: "source"}
	to := from
	to.DeviceID = "receiver"
	actor := identity.Scope{TenantID: "company", StoreID: "store", IdentityID: "owner"}
	// Two independently seeded disposable installations, not provisioning code.
	for _, db := range []*sql.DB{source, receiver} {
		for _, query := range []string{
			"INSERT INTO tenants VALUES ('company','Empresa','now')",
			"INSERT INTO stores VALUES ('company','store','Loja')",
			"INSERT INTO identities VALUES ('owner','Dono','now')",
			"INSERT INTO memberships VALUES ('company','owner','owner','active','now')",
			"INSERT INTO membership_stores VALUES ('company','owner','store')",
			"INSERT INTO devices VALUES ('company','store','source','Origem')",
			"INSERT INTO devices VALUES ('company','store','receiver','Destino')",
		} {
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
		}
		for _, pair := range []struct {
			id  string
			key ed25519.PrivateKey
		}{{"source", sourceKey}, {"receiver", receiverKey}} {
			_, err := db.Exec(`INSERT INTO device_pairings (tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
				VALUES ('company','store',?,?,?,9999999999,'approved','owner')`, pair.id, []byte(pair.key.Public().(ed25519.PublicKey)), make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := incoming.Grant(ctx, receiver, actor, to, from, "sale.committed"); err != nil {
		t.Fatal(err)
	}
	xkey, err := incoming.CreateEncryptionKey(filepath.Join(dir, "encryption.json"), to)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := incoming.SignEncryptionBinding(to, 1, xkey, receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := incoming.TrustEncryptionBinding(ctx, source, actor, from, binding); err != nil {
		t.Fatal(err)
	}
	_, err = source.Exec(`INSERT INTO outbox (event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
		VALUES ('event-one','company','store','source','operation-one','sale-one','sale.committed',1,'{"sale_id":"sale-one","total_cents":100}','now')`)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := incoming.NewSealedReceiverHTTP(ctx, receiver, to, receiverKey, xkey)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	stationPath := filepath.Join(dir, "station.json")
	encoded := base64.RawURLEncoding.EncodeToString(sourceKey)
	writeSyncJSON(t, stationPath, syncStation{"company", "store", "source", encoded})
	configPath := filepath.Join(dir, "sync.json")
	writeSyncJSON(t, configPath, syncConfig{1, "receiver", server.URL + incoming.SealedReceiverPath})
	args := []string{"send", "--db", sourcePath, "--station", stationPath, "--config", configPath, "--once"}
	var output bytes.Buffer
	if err := runSync(ctx, args, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "confirmada") || strings.Contains(output.String(), encoded) || strings.Contains(output.String(), "private_key") || strings.Contains(output.String(), "total_cents") {
		t.Fatal("saída incorreta ou expôs segredo/conteúdo")
	}
	var count int
	if err := source.QueryRow("SELECT COUNT(*) FROM outbox WHERE status='acked'").Scan(&count); err != nil || count != 1 {
		t.Fatal("comando não confirmou evento")
	}
	if err := source.QueryRow("SELECT COUNT(*) FROM device_auth_challenges WHERE consumed_unix IS NOT NULL").Scan(&count); err != nil || count != 1 {
		t.Fatal("comando não provou aparelho")
	}
	output.Reset()
	if err := runSync(ctx, args, &output); err != nil || !strings.Contains(output.String(), "sem eventos") {
		t.Fatalf("fila vazia: %v", err)
	}
	if err := receiver.QueryRow("SELECT COUNT(*) FROM incoming_events").Scan(&count); err != nil || count != 1 {
		t.Fatal("comando repetiu evento")
	}
	// Revocation prevents even reporting an empty queue as an authorized run.
	if _, err := source.Exec("UPDATE device_pairings SET status='revoked' WHERE device_id='source'"); err != nil {
		t.Fatal(err)
	}
	if err := runSync(ctx, args, &output); err == nil {
		t.Fatal("comando aceitou aparelho revogado")
	}
}

func TestSyncCLIRejectsBadConfigBeforeCreatingDatabase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.sqlite")
	config := filepath.Join(dir, "config.json")
	for _, body := range []string{
		`{"version":1,"destination_device_id":"receiver","endpoint":"http://192.168.1.10/sync/v1/events"}`,
		`{"version":1,"version":1,"destination_device_id":"receiver","endpoint":"http://127.0.0.1/sync/v1/events"}`,
		`{"version":1,"destination_device_id":"receiver","endpoint":"https://user:secret@example.invalid/sync/v1/events"}`,
		`{"version":1,"destination_device_id":"receiver","endpoint":"https://example.invalid/sync/v1/events","extra":true}`,
		`{"version":1,"destination_device_id":null,"endpoint":"https://example.invalid/sync/v1/events"}`,
	} {
		if err := os.WriteFile(config, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := runSync(context.Background(), []string{"send", "--db", path, "--station", filepath.Join(dir, "absent.station"), "--config", config, "--once"}, &output); err == nil {
			t.Fatal("configuração inválida aceita")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("criou banco sem configuração válida")
		}
		if output.Len() != 0 {
			t.Fatal("imprimiu estado de sucesso inválido")
		}
	}
}

func TestSyncPrivateReadersRejectPermissionsLinksAndMalformedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "station.json")
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	writeSyncJSON(t, path, syncStation{"company", "store", "source", base64.RawURLEncoding.EncodeToString(key)})
	if _, _, err := loadSyncStation(path); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadSyncStation(link); err == nil {
		t.Fatal("link aceito")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadSyncStation(path); err == nil {
		t.Fatal("arquivo legível por outros aceito")
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`{"tenant_id":"company","store_id":"store","device_id":"source","private_key":"bad"}`,
		`{"tenant_id":"company","tenant_id":"foreign","store_id":"store","device_id":"source","private_key":"bad"}`,
		`{"tenant_id":"company","store_id":"store","device_id":"source","private_key":"bad"} {}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadSyncStation(path); err == nil {
			t.Fatal("chave/JSON inválidos aceitos")
		}
	}
}

func TestSyncCLIRejectsUnknownCommandAndFlags(t *testing.T) {
	for _, args := range [][]string{nil, {"serve"}, {"send"}, {"send", "--unknown"}, {"send", "extra"}} {
		var output bytes.Buffer
		if err := runSync(context.Background(), args, &output); err == nil {
			t.Fatal("comando inválido aceito")
		}
	}
}
