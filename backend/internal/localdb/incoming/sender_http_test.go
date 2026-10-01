package incoming

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb/outgoing"
)

func TestHTTPPostingUsesApprovedEncryptionAndVerifiesReceipt(t *testing.T) {
	f := setup(t)
	f.grant(t)
	ctx := context.Background()
	key := encryptionKey(t)
	binding, err := SignEncryptionBinding(f.receiver, 1, key, f.receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(ctx, f.senderDB, f.actor, f.sender, binding); err != nil {
		t.Fatal(err)
	}
	public, err := TrustedEncryptionPublic(ctx, f.senderDB, f.sender, f.receiver)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewSealedReceiverHTTP(ctx, f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	var first HTTPReceiptResult
	for attempt := 0; attempt < 2; attempt++ {
		envelope, err := Seal(f.message(t), public)
		if err != nil {
			t.Fatal(err)
		}
		result, err := PostSealedHTTP(ctx, server.Client(), server.URL+SealedReceiverPath, envelope, f.event, verifier)
		if err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			first = result
		} else if !result.Repeated || result.Receipt.ReceiptID != first.Receipt.ReceiptID {
			t.Fatal("recibo repetido divergiu")
		}
		pending, err := outgoing.Pending(ctx, f.senderDB, f.sender, 10)
		if err != nil || len(pending) != 1 {
			t.Fatal("cliente confirmou fila sem chamada explícita")
		}
	}
	if err := outgoing.Confirm(ctx, f.senderDB, f.sender, first.Receipt, verifier); err != nil {
		t.Fatal(err)
	}
	pending, err := outgoing.Pending(ctx, f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 0 {
		t.Fatal("recibo verificado não confirmou")
	}
	receivedCount(t, f, 1)
}

func TestHTTPPostingRejectsBadRepliesAndPreservesPendingEvent(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReceiveSealed(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key, envelope)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := json.Marshal(HTTPReceiptResult{Receipt: result.Receipt, Repeated: false})
	if err != nil {
		t.Fatal(err)
	}
	forged := result.Receipt
	forged.Proof = make([]byte, 64)
	badProof, err := json.Marshal(HTTPReceiptResult{Receipt: forged, Repeated: false})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	for _, reply := range []struct {
		status int
		body   []byte
	}{
		{201, badProof},
		{201, []byte("{}")},
		{201, append(append([]byte(nil), valid...), []byte(" {}")...)},
		{201, []byte(strings.Replace(string(valid), `"repeated":false`, `"repeated":false,"repeated":false`, 1))},
		{201, []byte(strings.Replace(string(valid), `"ReceiptID":`, `"extra":true,"ReceiptID":`, 1))},
		{201, []byte(strings.Repeat("x", receiptHTTPBodyLimit+1))},
		{200, valid},
		{500, valid},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(reply.status)
			_, _ = w.Write(reply.body)
		}))
		_, err := PostSealedHTTP(context.Background(), nil, server.URL+SealedReceiverPath, envelope, f.event, verifier)
		server.Close()
		if err == nil {
			t.Fatal("resposta inválida aceita")
		}
		pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
		if err != nil || len(pending) != 1 {
			t.Fatal("resposta inválida confirmou fila")
		}
	}
}

func TestHTTPPostingDoesNotFollowRedirect(t *testing.T) {
	f := setup(t)
	key := encryptionKey(t)
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	var reached bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(500) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL+SealedReceiverPath, 307) }))
	defer redirect.Close()
	if _, err := PostSealedHTTP(context.Background(), nil, redirect.URL+SealedReceiverPath, envelope, f.event, verifier); err == nil {
		t.Fatal("redirect aceito")
	}
	if reached {
		t.Fatal("enviou mensagem ao destino redirecionado")
	}
}

func TestHTTPPostingRejectsUnsafeEndpointsAndInsecureTLS(t *testing.T) {
	f := setup(t)
	key := encryptionKey(t)
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{
		"http://192.168.1.10/sync/v1/events",
		"http://localhost/sync/v1/events",
		"https://user:password@example.invalid/sync/v1/events",
		"https://example.invalid/sync/v1/events?secret=x",
		"https://example.invalid/sync/v1/events#fragment",
		"https://example.invalid/other",
		"file:///sync/v1/events",
	} {
		if _, err := PostSealedHTTP(context.Background(), nil, endpoint, envelope, f.event, verifier); err == nil {
			t.Fatal("endereço inseguro aceito")
		}
	}
	unsafe := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	if _, err := PostSealedHTTP(context.Background(), unsafe, "https://example.invalid/sync/v1/events", envelope, f.event, verifier); err == nil {
		t.Fatal("verificação TLS desativada aceita")
	}
	if _, err := PostSealedHTTP(context.Background(), nil, "http://127.0.0.1/sync/v1/events", envelope, f.event, nil); err == nil {
		t.Fatal("verificador ausente aceito")
	}
}

func TestHTTPPostingCancelledContextAndLostResponseKeepRetryPossible(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PostSealedHTTP(ctx, nil, server.URL+SealedReceiverPath, envelope, f.event, verifier); err == nil {
		t.Fatal("contexto cancelado ignorado")
	}
	receivedCount(t, f, 0)
	// Simulate reception already committed but no response available to sender.
	first, err := ReceiveSealed(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key, envelope)
	if err != nil {
		t.Fatal(err)
	}
	newEnvelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	retry, err := PostSealedHTTP(context.Background(), nil, server.URL+SealedReceiverPath, newEnvelope, f.event, verifier)
	if err != nil || !retry.Repeated || !bytes.Equal(retry.Receipt.Proof, first.Receipt.Proof) {
		t.Fatal("perda de resposta impediu repetição estável")
	}
	receivedCount(t, f, 1)
}
