package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/localsetup"
	"titansystem-backend/internal/localdb/outgoing"
	"titansystem-backend/internal/localdb/stationfile"
)

type receiverAddressWriter struct{ ready chan string }

func (w receiverAddressWriter) Write(body []byte) (int, error) {
	line := strings.TrimSpace(string(body))
	if strings.HasPrefix(line, "Receptor iniciado em ") {
		w.ready <- strings.TrimPrefix(line, "Receptor iniciado em ")
	}
	return len(body), nil
}

func TestReceiverCLIReceivesAndStopsWithoutExposingSecret(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "receiver.sqlite")
	db, err := localdb.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	created, err := localsetup.Initialize(ctx, db, localsetup.Input{TenantName: "Empresa", StoreName: "Loja", OwnerName: "Dono", DeviceName: "Receptor", Password: "senha-de-teste-longa", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	device := identity.DeviceContext{TenantID: created.TenantID, StoreID: created.StoreID, DeviceID: created.DeviceID}
	actor := identity.Scope{TenantID: created.TenantID, StoreID: created.StoreID, IdentityID: created.OwnerID}
	sender := device
	sender.DeviceID = "sender"
	senderPublic, senderKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO devices VALUES (?,?,?,'Origem')", sender.TenantID, sender.StoreID, sender.DeviceID); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO device_pairings (tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
		VALUES (?,?,?,?,zeroblob(32),9999999999,'approved',?)`, sender.TenantID, sender.StoreID, sender.DeviceID, []byte(senderPublic), created.OwnerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := incoming.Grant(ctx, db, actor, device, sender, "sale.committed"); err != nil {
		t.Fatal(err)
	}
	stationPath := filepath.Join(dir, "station.json")
	encoded := base64.RawURLEncoding.EncodeToString(key)
	body, err := json.Marshal(stationfile.Station{TenantID: device.TenantID, StoreID: device.StoreID, DeviceID: device.DeviceID, PrivateKey: encoded})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stationPath, body, 0600); err != nil {
		t.Fatal(err)
	}
	encryptionPath := filepath.Join(dir, "encryption.json")
	encryption, err := incoming.CreateEncryptionKey(encryptionPath, device)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan string, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- runReceive(ctx, []string{"serve", "--db", dbPath, "--station", stationPath, "--encryption", encryptionPath, "--listen", "127.0.0.1:0"}, receiverAddressWriter{ready})
	}()
	var endpoint string
	select {
	case endpoint = <-ready:
	case err := <-finished:
		t.Fatalf("início: %v", err)
	case <-ctx.Done():
		t.Fatal("receptor não iniciou")
	}
	if strings.Contains(endpoint, encoded) || strings.Contains(endpoint, "private_key") {
		t.Fatal("saída expôs segredo")
	}
	event := outgoing.Event{EventID: "event", TenantID: sender.TenantID, StoreID: sender.StoreID, DeviceID: sender.DeviceID, OperationID: "operation", AggregateID: "sale", EventType: "sale.committed", SchemaVersion: 1, Payload: json.RawMessage(`{"total_cents":100}`)}
	message, err := incoming.Sign(event, device, senderKey)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := incoming.Seal(message, encryption.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := incoming.NewReceiptVerifier(device, public)
	if err != nil {
		t.Fatal(err)
	}
	result, err := incoming.PostSealedHTTP(ctx, nil, endpoint, envelope, event, verifier)
	if err != nil || result.Receipt.ReceiptID == "" {
		t.Fatalf("recepção: %v", err)
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("receptor não encerrou")
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM incoming_events").Scan(&count); err != nil || count != 1 {
		t.Fatalf("mensagens=%d %v", count, err)
	}
}

func TestReceiverRequiresExplicitIPAndTLSOutsideLoopback(t *testing.T) {
	for _, address := range []string{"192.168.1.10:8282", "0.0.0.0:8282", ":8282", "localhost:8282", "127.0.0.1:99999"} {
		if _, err := receiverTLS(address, "", ""); err == nil {
			t.Fatal("escuta insegura aceita")
		}
	}
	if config, err := receiverTLS("127.0.0.1:0", "", ""); err != nil || config != nil {
		t.Fatal("loopback sem TLS recusado")
	}
	if _, err := receiverTLS("127.0.0.1:8282", "missing.pem", ""); err == nil {
		t.Fatal("TLS parcial aceito")
	}
}

func TestReceiverRejectsBadFlagsWithoutOpeningDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.sqlite")
	for _, args := range [][]string{nil, {"send"}, {"serve"}, {"serve", "--unknown"}, {"serve", "--db", path, "--station", "missing", "--encryption", "missing", "--listen", "192.168.1.10:8282"}} {
		if err := runReceive(context.Background(), args, io.Discard); err == nil {
			t.Fatal("configuração inválida aceita")
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("criou banco por engano")
	}
}

func TestReceiverAdmissionLimitsRequestStarts(t *testing.T) {
	handler := boundReceiver(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	denied := false
	for n := 0; n < 25; n++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", "/sync/v1/events", bytes.NewReader(nil)))
		if w.Code == 429 {
			denied = true
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("sem indicação de espera")
			}
		}
	}
	if !denied {
		t.Fatal("limite não foi aplicado")
	}
}

func TestReceiverLoadsTLSFilesAndServesWithCertificateValidation(t *testing.T) {
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer fixture.Close()
	certificate := fixture.TLS.Certificates[0]
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0644); err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600); err != nil {
		t.Fatal(err)
	}
	tlsConfig, err := receiverTLS("127.0.0.1:0", certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- serveReceiver(ctx, listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), tlsConfig)
	}()
	client := fixture.Client()
	client.Timeout = 3 * time.Second
	response, err := client.Get("https://" + listener.Addr().String() + "/sync/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatal("TLS não serviu requisição")
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("TLS não encerrou")
	}
	if err := os.Chmod(keyPath, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := receiverTLS("127.0.0.1:0", certPath, keyPath); err == nil {
		t.Fatal("chave TLS insegura aceita")
	}
}
