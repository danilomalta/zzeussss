package incoming

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/outgoing"
)

func sealedHTTPRequest(t *testing.T, handler http.Handler, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, SealedReceiverPath, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestSealedHTTPReceptionRetriesAndConfirmsOutbox(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	var first HTTPReceiptResult
	for attempt := 0; attempt < 2; attempt++ {
		envelope, err := Seal(f.message(t), key.PublicKey())
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Post(server.URL+SealedReceiverPath, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		want := http.StatusCreated
		if attempt == 1 {
			want = http.StatusOK
		}
		if response.StatusCode != want {
			t.Fatalf("status=%d body=%s", response.StatusCode, raw)
		}
		if bytes.Contains(raw, []byte("sale_id")) || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("resposta expôs conteúdo ou permitiu cache")
		}
		var result HTTPReceiptResult
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		if attempt == 0 {
			first = result
			if result.Repeated {
				t.Fatal("primeiro recebimento marcado repetido")
			}
		} else if !result.Repeated || result.Receipt.ReceiptID != first.Receipt.ReceiptID || !bytes.Equal(result.Receipt.Proof, first.Receipt.Proof) {
			t.Fatal("repetição não devolveu recibo persistido")
		}
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	if err := outgoing.Confirm(context.Background(), f.senderDB, f.sender, first.Receipt, verifier); err != nil {
		t.Fatal(err)
	}
	pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 0 {
		t.Fatal("recibo HTTP não confirmou fila")
	}
	receivedCount(t, f, 1)
}

func TestSealedHTTPRejectsAmbiguousAndPlaintextRequests(t *testing.T) {
	f := setup(t)
	key := encryptionKey(t)
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	valid, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := json.Marshal(f.message(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{
		plain,
		append(append([]byte(nil), valid...), []byte(" {}")...),
		[]byte(strings.Replace(string(valid), `"version":1`, `"version":1,"version":1`, 1)),
		[]byte(strings.Replace(string(valid), `"DeviceID":"receiver"`, `"DeviceID":"receiver","DeviceID":"receiver"`, 1)),
		[]byte(strings.Replace(string(valid), `"version":1`, `"version":1,"extra":true`, 1)),
		[]byte(strings.Replace(string(valid), `"destination":{`, `"destination":null,"ignored":{`, 1)),
	} {
		w := sealedHTTPRequest(t, handler, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("JSON inválido: %d", w.Code)
		}
		if bytes.Contains(w.Body.Bytes(), []byte("receipt")) {
			t.Fatal("requisição inválida recebeu recibo")
		}
	}
	receivedCount(t, f, 0)
}

func TestSealedHTTPChecksMethodMediaAndBothBodyLimits(t *testing.T) {
	f := setup(t)
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, encryptionKey(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		method, path, media, encoding string
		status                        int
	}{
		{"GET", SealedReceiverPath, "application/json", "", 405},
		{"POST", "/other", "application/json", "", 404},
		{"POST", SealedReceiverPath + "?key=x", "application/json", "", 404},
		{"POST", SealedReceiverPath, "text/plain", "", 415},
		{"POST", SealedReceiverPath, "application/json", "gzip", 415},
	} {
		r := httptest.NewRequest(test.method, test.path, strings.NewReader("{}"))
		r.Header.Set("Content-Type", test.media)
		r.Header.Set("Content-Encoding", test.encoding)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("protocolo: %d esperado=%d", w.Code, test.status)
		}
	}
	for _, unknownLength := range []bool{false, true} {
		r := httptest.NewRequest(http.MethodPost, SealedReceiverPath, strings.NewReader(strings.Repeat("x", sealedHTTPBodyLimit+1)))
		r.Header.Set("Content-Type", "application/json")
		if unknownLength {
			r.ContentLength = -1
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("limite: %d", w.Code)
		}
	}
	receivedCount(t, f, 0)
}

func TestSealedHTTPRechecksGrantAndRevocation(t *testing.T) {
	f := setup(t)
	key := encryptionKey(t)
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if w := sealedHTTPRequest(t, handler, body); w.Code != 403 {
		t.Fatalf("sem autorização: %d", w.Code)
	}
	f.grant(t)
	if _, err := f.receiverDB.Exec("UPDATE device_pairings SET status='revoked' WHERE device_id='sender'"); err != nil {
		t.Fatal(err)
	}
	if w := sealedHTTPRequest(t, handler, body); w.Code != 403 {
		t.Fatalf("origem revogada: %d", w.Code)
	}
	receivedCount(t, f, 0)
}

func TestSealedHTTPFailureCannotReturnSuccessfulReceipt(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.receiverDB.Exec(`CREATE TRIGGER fail_http_inbox BEFORE INSERT ON incoming_events BEGIN SELECT RAISE(ABORT,'conteudo-interno-secreto'); END`); err != nil {
		t.Fatal(err)
	}
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	w := sealedHTTPRequest(t, handler, body)
	if w.Code != 500 || bytes.Contains(w.Body.Bytes(), []byte("receipt")) || bytes.Contains(w.Body.Bytes(), []byte("conteudo-interno-secreto")) {
		t.Fatal("falha expôs detalhe ou confirmou recebimento")
	}
	receivedCount(t, f, 0)
	pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 {
		t.Fatal("falha confirmou origem")
	}
}

func TestSealedHTTPConflictDoesNotReplaceFirstMessage(t *testing.T) {
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Seal(f.message(t), key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if w := sealedHTTPRequest(t, handler, body); w.Code != 201 {
		t.Fatalf("primeiro: %d", w.Code)
	}
	event := f.event
	event.Payload = []byte(`{"total_cents":1}`)
	message, err := Sign(event, f.receiver, f.senderKey)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err = Seal(message, key.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	body, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if w := sealedHTTPRequest(t, handler, body); w.Code != 409 {
		t.Fatalf("conflito: %d", w.Code)
	}
	receivedCount(t, f, 1)
}

func TestSealedHTTPConstructorRejectsMissingOrForeignPrivateKeys(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	key := encryptionKey(t)
	_, wrong, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSealedReceiverHTTP(ctx, nil, f.receiver, f.receiverKey, key); err == nil {
		t.Fatal("banco ausente aceito")
	}
	if _, err := NewSealedReceiverHTTP(ctx, f.receiverDB, f.receiver, f.receiverKey, nil); err == nil {
		t.Fatal("chave de criptografia ausente aceita")
	}
	if _, err := NewSealedReceiverHTTP(ctx, f.receiverDB, f.receiver, wrong, key); err == nil {
		t.Fatal("chave de assinatura estrangeira aceita")
	}
}
