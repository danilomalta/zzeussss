package incoming

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/outgoing"
)

func deliveryFixture(t *testing.T) (*fixture, *ecdh.PrivateKey, http.Handler) {
	t.Helper()
	f := setup(t)
	f.grant(t)
	key := encryptionKey(t)
	binding, err := SignEncryptionBinding(f.receiver, 1, key, f.receiverKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TrustEncryptionBinding(context.Background(), f.senderDB, f.actor, f.sender, binding); err != nil {
		t.Fatal(err)
	}
	handler, err := NewSealedReceiverHTTP(context.Background(), f.receiverDB, f.receiver, f.receiverKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return f, key, handler
}

func TestPendingDeliveryCommitsReceiptAndBecomesEmpty(t *testing.T) {
	f, _, handler := deliveryFixture(t)
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	ctx := context.Background()
	result, err := DeliverPendingOnce(ctx, f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, server.Client())
	if err != nil || result.Empty || result.EventID != f.event.EventID || result.ReceiptID == "" {
		t.Fatalf("entrega: %+v %v", result, err)
	}
	result, err = DeliverPendingOnce(ctx, f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, server.Client())
	if err != nil || !result.Empty {
		t.Fatalf("fila vazia: %+v %v", result, err)
	}
	receivedCount(t, f, 1)
	var receipts int
	if err := f.senderDB.QueryRow("SELECT COUNT(*) FROM outbox_receipts").Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("recibos=%d %v", receipts, err)
	}
}

func TestPendingDeliveryRecordsFailureAndRetriesSameEvent(t *testing.T) {
	f, _, handler := deliveryFixture(t)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	_, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, failing.URL+SealedReceiverPath, nil)
	failing.Close()
	if err == nil {
		t.Fatal("falha remota ignorada")
	}
	pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 || pending[0].EventID != f.event.EventID {
		t.Fatalf("pendência: %+v %v", pending, err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	if _, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, nil); err != nil {
		t.Fatal(err)
	}
	receivedCount(t, f, 1)
}

func TestPendingDeliveryAckFailureRollsBackAndRetryUsesDurableReceipt(t *testing.T) {
	f, _, handler := deliveryFixture(t)
	server := httptest.NewServer(handler)
	defer server.Close()
	if _, err := f.senderDB.Exec(`CREATE TRIGGER fail_delivery_ack BEFORE UPDATE OF status ON outbox WHEN NEW.status='acked' BEGIN SELECT RAISE(ABORT,'falha de teste'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, nil); err == nil {
		t.Fatal("falha na confirmação ignorada")
	}
	var receipts int
	if err := f.senderDB.QueryRow("SELECT COUNT(*) FROM outbox_receipts").Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal("recibo não sofreu rollback")
	}
	pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 {
		t.Fatal("evento foi perdido")
	}
	receivedCount(t, f, 1)
	var seq int
	var name, path string
	if err := f.senderDB.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.senderDB.Close(); err != nil {
		t.Fatal(err)
	}
	f.senderDB, err = localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	// Only this disposable test database is modified.
	if _, err := f.senderDB.Exec("DROP TRIGGER fail_delivery_ack"); err != nil {
		t.Fatal(err)
	}
	result, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, nil)
	if err != nil || !result.Repeated {
		t.Fatalf("repetição: %+v %v", result, err)
	}
	receivedCount(t, f, 1)
}

func TestPendingDeliveryRevocationDuringHTTPBlocksLocalAck(t *testing.T) {
	for _, query := range []string{
		"UPDATE memberships SET status='revoked' WHERE identity_id='owner'",
		"UPDATE device_pairings SET status='revoked' WHERE device_id='sender'",
		"UPDATE device_pairings SET status='revoked' WHERE device_id='receiver'",
		"UPDATE device_encryption_keys SET signature=zeroblob(64)",
	} {
		t.Run(query, func(t *testing.T) {
			f, _, handler := deliveryFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				buffer := httptest.NewRecorder()
				handler.ServeHTTP(buffer, r)
				if _, err := f.senderDB.Exec(query); err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				for name, values := range buffer.Header() {
					for _, value := range values {
						w.Header().Add(name, value)
					}
				}
				w.WriteHeader(buffer.Code)
				_, _ = w.Write(buffer.Body.Bytes())
			}))
			defer server.Close()
			if _, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, nil); err == nil {
				t.Fatal("revogação não bloqueou confirmação")
			}
			var status string
			if err := f.senderDB.QueryRow("SELECT status FROM outbox WHERE event_id=?", f.event.EventID).Scan(&status); err != nil || status != "pending" {
				t.Fatalf("estado=%s %v", status, err)
			}
			receivedCount(t, f, 1)
		})
	}
}

func TestPendingDeliveryMissingTrustOrWrongPrivateKeyDoesNotSend(t *testing.T) {
	f := setup(t)
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer server.Close()
	if _, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, nil); err == nil {
		t.Fatal("chave pública não aprovada aceita")
	}
	approvedEncryption(t, f)
	_, wrong, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, wrong, f.receiver, server.URL+SealedReceiverPath, nil); err == nil {
		t.Fatal("chave privada estrangeira aceita")
	}
	if calls != 0 {
		t.Fatal("enviou antes de validar confiança")
	}
}

func TestPendingDeliveryConcurrentAttemptsDoNotDuplicate(t *testing.T) {
	f, _, handler := deliveryFixture(t)
	server := httptest.NewServer(handler)
	defer server.Close()
	var group sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, nil)
			errors <- err
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	receivedCount(t, f, 1)
	var count int
	if err := f.senderDB.QueryRow("SELECT COUNT(*) FROM outbox_receipts").Scan(&count); err != nil || count != 1 {
		t.Fatalf("recibos duplicados: %d %v", count, err)
	}
}
