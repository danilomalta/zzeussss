package incoming

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/outgoing"
)

type fixture struct {
	senderDB, receiverDB   *sql.DB
	sender, receiver       identity.DeviceContext
	actor                  identity.Scope
	senderKey, receiverKey ed25519.PrivateKey
	event                  outgoing.Event
	receiverPath           string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	_, senderKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, receiverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		sender:    identity.DeviceContext{TenantID: "company", StoreID: "store", DeviceID: "sender"},
		receiver:  identity.DeviceContext{TenantID: "company", StoreID: "store", DeviceID: "receiver"},
		actor:     identity.Scope{TenantID: "company", StoreID: "store", IdentityID: "owner"},
		senderKey: senderKey, receiverKey: receiverKey,
		receiverPath: filepath.Join(t.TempDir(), "receiver.sqlite"),
	}
	ctx := context.Background()
	f.senderDB, err = localdb.Open(ctx, filepath.Join(t.TempDir(), "sender.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.senderDB.Close() })
	f.receiverDB, err = localdb.Open(ctx, f.receiverPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.receiverDB.Close() })
	// Fixture deliberately installs trusted identity/pairing records on two
	// independent DBs; this is not a production provisioning/sync mechanism.
	for _, db := range []*sql.DB{f.senderDB, f.receiverDB} {
		for _, query := range []string{
			`INSERT INTO tenants VALUES ('company','Empresa','now')`,
			`INSERT INTO stores VALUES ('company','store','Loja')`,
			`INSERT INTO identities VALUES ('owner','Dono','now')`,
			`INSERT INTO memberships VALUES ('company','owner','owner','active','now')`,
			`INSERT INTO membership_stores VALUES ('company','owner','store')`,
			`INSERT INTO devices VALUES ('company','store','sender','Origem')`,
			`INSERT INTO devices VALUES ('company','store','receiver','Destino')`,
		} {
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
		}
		for _, pair := range []struct {
			id  string
			key ed25519.PrivateKey
		}{{"sender", senderKey}, {"receiver", receiverKey}} {
			_, err := db.Exec(`INSERT INTO device_pairings
				(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
				VALUES ('company','store',?,?,?,9999999999,'approved','owner')`,
				pair.id, []byte(pair.key.Public().(ed25519.PublicKey)), make([]byte, 32))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err = f.senderDB.Exec(`INSERT INTO outbox
		(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
		VALUES ('event-one','company','store','sender','operation-one','sale-one','sale.committed',1,'{"sale_id":"sale-one","total_cents":100}','now')`)
	if err != nil {
		t.Fatal(err)
	}
	events, err := outgoing.Pending(ctx, f.senderDB, f.sender, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("fila origem: %v %v", events, err)
	}
	f.event = events[0]
	return f
}

func (f *fixture) grant(t *testing.T) {
	t.Helper()
	if err := Grant(context.Background(), f.receiverDB, f.actor, f.receiver, f.sender, f.event.EventType); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) message(t *testing.T) Message {
	t.Helper()
	message, err := Sign(f.event, f.receiver, f.senderKey)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func receivedCount(t *testing.T, f *fixture, want int) {
	t.Helper()
	var got int
	if err := f.receiverDB.QueryRow("SELECT COUNT(*) FROM incoming_events").Scan(&got); err != nil || got != want {
		t.Fatalf("recebidos=%d esperado=%d erro=%v", got, want, err)
	}
}

func TestDurableReceptionAndAuthenticatedOutboxConfirmation(t *testing.T) {
	f := setup(t)
	f.grant(t)
	ctx := context.Background()
	message := f.message(t)
	first, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message)
	if err != nil || first.Repeated {
		t.Fatalf("recepcao: %+v %v", first, err)
	}
	second, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message)
	if err != nil || !second.Repeated || second.Receipt.ReceiptID != first.Receipt.ReceiptID || !bytes.Equal(first.Receipt.Proof, second.Receipt.Proof) {
		t.Fatalf("repeticao: %+v %v", second, err)
	}
	receivedCount(t, f, 1)
	var sales int
	if err := f.receiverDB.QueryRow("SELECT COUNT(*) FROM sales").Scan(&sales); err != nil || sales != 0 {
		t.Fatalf("recepcao aplicou venda: %d %v", sales, err)
	}
	pending, err := outgoing.Pending(ctx, f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 {
		t.Fatal("origem confirmou antes de receber recibo")
	}
	verifier, err := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	if err := outgoing.Confirm(ctx, f.senderDB, f.sender, first.Receipt, verifier); err != nil {
		t.Fatal(err)
	}
	if err := outgoing.Confirm(ctx, f.senderDB, f.sender, second.Receipt, verifier); err != nil {
		t.Fatal(err)
	}
	pending, err = outgoing.Pending(ctx, f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("origem nao confirmou: %v %v", pending, err)
	}
}

func TestUnknownPeerScopeAndTypeAreRejected(t *testing.T) {
	f := setup(t)
	message := f.message(t)
	if _, err := Receive(context.Background(), f.receiverDB, f.receiver, f.receiverKey, message); !errors.Is(err, ErrDenied) {
		t.Fatalf("sem aprovacao: %v", err)
	}
	f.grant(t)
	for _, change := range []func(*outgoing.Event, *identity.DeviceContext){
		func(e *outgoing.Event, d *identity.DeviceContext) { d.TenantID = "foreign" },
		func(e *outgoing.Event, d *identity.DeviceContext) { d.StoreID = "other-store" },
		func(e *outgoing.Event, d *identity.DeviceContext) { d.DeviceID = e.DeviceID },
		func(e *outgoing.Event, d *identity.DeviceContext) { e.EventType = "unknown" },
		func(e *outgoing.Event, d *identity.DeviceContext) { e.SchemaVersion = 2 },
	} {
		e, d := f.event, f.receiver
		change(&e, &d)
		if _, err := Sign(e, d, f.senderKey); err == nil {
			t.Fatal("escopo ou tipo invalido assinado")
		}
	}
	receivedCount(t, f, 0)
}

func TestTamperingAndWrongKeysCannotReceiveOrConfirm(t *testing.T) {
	f := setup(t)
	f.grant(t)
	ctx := context.Background()
	message := f.message(t)
	message.Event.Payload = []byte(`{"sale_id":"sale-one","total_cents":1}`)
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message); !errors.Is(err, ErrDenied) {
		t.Fatalf("conteudo adulterado: %v", err)
	}
	message = f.message(t)
	message.Destination.DeviceID = "another-device"
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message); !errors.Is(err, ErrDenied) {
		t.Fatalf("destino adulterado: %v", err)
	}
	_, wrong, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	message = f.message(t)
	if _, err := Receive(ctx, f.receiverDB, f.receiver, wrong, message); !errors.Is(err, ErrDenied) {
		t.Fatalf("chave receptora errada: %v", err)
	}
	forged, err := Sign(f.event, f.receiver, wrong)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, forged); !errors.Is(err, ErrDenied) {
		t.Fatalf("emissor falso: %v", err)
	}
	result, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message)
	if err != nil {
		t.Fatal(err)
	}
	verifier, _ := NewReceiptVerifier(f.receiver, f.receiverKey.Public().(ed25519.PublicKey))
	result.Receipt.PayloadSHA256 = "forged"
	if err := outgoing.Confirm(ctx, f.senderDB, f.sender, result.Receipt, verifier); err == nil {
		t.Fatal("recibo falso confirmou origem")
	}
	pending, err := outgoing.Pending(ctx, f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 {
		t.Fatal("recibo falso removeu pendencia")
	}
	receivedCount(t, f, 1)
}

func TestConflictingEventOrOperationPreservesFirstRecord(t *testing.T) {
	f := setup(t)
	f.grant(t)
	ctx := context.Background()
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, f.message(t)); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*outgoing.Event){
		func(e *outgoing.Event) { e.Payload = []byte(`{"different":true}`) },
		func(e *outgoing.Event) { e.AggregateID = "different" },
		func(e *outgoing.Event) { e.EventID = "another-event" },
	} {
		e := f.event
		change(&e)
		message, err := Sign(e, f.receiver, f.senderKey)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message); !errors.Is(err, ErrConflict) {
			t.Fatalf("conflito: %v", err)
		}
	}
	receivedCount(t, f, 1)
}

func TestPeerGrantAndRevocationRequireOwnerAndAreAudited(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.receiverDB.Exec("UPDATE memberships SET role='manager'"); err != nil {
		t.Fatal(err)
	}
	if err := Grant(ctx, f.receiverDB, f.actor, f.receiver, f.sender, f.event.EventType); !errors.Is(err, ErrDenied) {
		t.Fatalf("gerente concedeu: %v", err)
	}
	if _, err := f.receiverDB.Exec("UPDATE memberships SET role='owner'"); err != nil {
		t.Fatal(err)
	}
	f.grant(t)
	f.grant(t)
	if err := Revoke(ctx, f.receiverDB, f.actor, f.receiver, f.sender, f.event.EventType); err != nil {
		t.Fatal(err)
	}
	if err := Revoke(ctx, f.receiverDB, f.actor, f.receiver, f.sender, f.event.EventType); err != nil {
		t.Fatal(err)
	}
	if _, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, f.message(t)); !errors.Is(err, ErrDenied) {
		t.Fatalf("vinculo revogado: %v", err)
	}
	var audit int
	if err := f.receiverDB.QueryRow("SELECT COUNT(*) FROM sync_peer_audit").Scan(&audit); err != nil || audit != 2 {
		t.Fatalf("auditoria=%d %v", audit, err)
	}
	receivedCount(t, f, 0)
}

func TestDeviceRevocationOwnerRevocationAndKeyRotationBlockReception(t *testing.T) {
	for _, query := range []string{
		`UPDATE device_pairings SET status='revoked' WHERE device_id='sender'`,
		`UPDATE device_pairings SET status='revoked' WHERE device_id='receiver'`,
		`UPDATE memberships SET status='revoked' WHERE identity_id='owner'`,
		`UPDATE device_pairings SET public_key=zeroblob(32) WHERE device_id='sender'`,
	} {
		t.Run(query, func(t *testing.T) {
			f := setup(t)
			f.grant(t)
			if _, err := f.receiverDB.Exec(query); err != nil {
				t.Fatal(err)
			}
			if _, err := Receive(context.Background(), f.receiverDB, f.receiver, f.receiverKey, f.message(t)); !errors.Is(err, ErrDenied) {
				t.Fatalf("revogacao/rotacao: %v", err)
			}
			receivedCount(t, f, 0)
		})
	}
}

func TestFailedPersistenceCannotEmitSuccessfulReceipt(t *testing.T) {
	f := setup(t)
	f.grant(t)
	if _, err := f.receiverDB.Exec(`CREATE TRIGGER fail_receipt BEFORE INSERT ON incoming_events BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	result, err := Receive(context.Background(), f.receiverDB, f.receiver, f.receiverKey, f.message(t))
	if err == nil || result.Receipt.ReceiptID != "" || len(result.Receipt.Proof) != 0 {
		t.Fatalf("falha confirmou: %+v %v", result, err)
	}
	receivedCount(t, f, 0)
	pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 {
		t.Fatal("falha perdeu evento da origem")
	}
}

func TestReceiptSurvivesReopenAndRetriesRemainStable(t *testing.T) {
	f := setup(t)
	f.grant(t)
	ctx := context.Background()
	first, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, f.message(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.receiverDB.Close(); err != nil {
		t.Fatal(err)
	}
	f.receiverDB, err = localdb.Open(ctx, f.receiverPath)
	if err != nil {
		t.Fatal(err)
	}
	e := f.event
	e.Attempts = 20
	message, err := Sign(e, f.receiver, f.senderKey)
	if err != nil {
		t.Fatal(err)
	}
	after, err := Receive(ctx, f.receiverDB, f.receiver, f.receiverKey, message)
	if err != nil || !after.Repeated || after.Receipt.ReceiptID != first.Receipt.ReceiptID || !bytes.Equal(after.Receipt.Proof, first.Receipt.Proof) {
		t.Fatalf("reinicio: %+v %v", after, err)
	}
	receivedCount(t, f, 1)
}

func TestConcurrentReceptionDoesNotDuplicate(t *testing.T) {
	f := setup(t)
	f.grant(t)
	message := f.message(t)
	type result struct {
		value Result
		err   error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			value, err := Receive(context.Background(), f.receiverDB, f.receiver, f.receiverKey, message)
			results <- result{value, err}
		}()
	}
	var firstID string
	repeated := 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		if firstID == "" {
			firstID = r.value.Receipt.ReceiptID
		}
		if firstID != r.value.Receipt.ReceiptID {
			t.Fatal("recibos divergentes")
		}
		if r.value.Repeated {
			repeated++
		}
	}
	if repeated != 1 {
		t.Fatalf("repeticoes=%d", repeated)
	}
	receivedCount(t, f, 1)
}

func TestInvalidPayloadAndReceiptTrustAreRejected(t *testing.T) {
	f := setup(t)
	for _, payload := range [][]byte{nil, []byte("not-json"), bytes.Repeat([]byte(" "), 128*1024+1)} {
		e := f.event
		e.Payload = payload
		if _, err := Sign(e, f.receiver, f.senderKey); !errors.Is(err, ErrInvalid) {
			t.Fatalf("payload invalido: %v", err)
		}
	}
	key := append(ed25519.PrivateKey(nil), f.senderKey...)
	key[63] ^= 1
	if _, err := Sign(f.event, f.receiver, key); !errors.Is(err, ErrInvalid) {
		t.Fatalf("chave inconsistente: %v", err)
	}
	if _, err := NewReceiptVerifier(f.receiver, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("confianca ausente: %v", err)
	}
}
