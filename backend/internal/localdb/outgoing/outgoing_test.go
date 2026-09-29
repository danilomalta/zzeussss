package outgoing

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

type checkedProof struct{}

func (checkedProof) Verify(_ context.Context, e Event, r Receipt) error {
	if string(r.Proof) != "accepted-by-peer" || r.EventID != e.EventID {
		return ErrInvalid
	}
	return nil
}

func setup(t *testing.T) (*sql.DB, identity.DeviceContext) {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "queue.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`INSERT INTO tenants VALUES ('market','Mercado','now')`,
		`INSERT INTO tenants VALUES ('other','Outro','now')`,
		`INSERT INTO stores VALUES ('market','s','Loja')`,
		`INSERT INTO stores VALUES ('other','s','Loja')`,
		`INSERT INTO devices VALUES ('market','s','d','Caixa')`,
		`INSERT INTO devices VALUES ('other','s','d','Caixa')`,
		`INSERT INTO identities VALUES ('owner','Operador','now')`,
		`INSERT INTO memberships VALUES ('market','owner','owner','active','now')`,
		`INSERT INTO memberships VALUES ('other','owner','owner','active','now')`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for _, tenant := range []string{"market", "other"} {
		_, err = db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
			VALUES (?,'s','d',?,?,9999999999,'approved','owner')`, tenant, make([]byte, 32), make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at)
			VALUES (? ,?,'s','d','operation','item','stock.operation',1,'{"quantity":1}','now')`, tenant+"-event", tenant)
		if err != nil {
			t.Fatal(err)
		}
	}
	return db, identity.DeviceContext{TenantID: "market", StoreID: "s", DeviceID: "d"}
}

func TestOutboxStaysPendingUntilAuthenticatedReceipt(t *testing.T) {
	db, device := setup(t)
	ctx := context.Background()
	events, err := Pending(ctx, db, device, 10)
	if err != nil || len(events) != 1 || events[0].EventID != "market-event" {
		t.Fatalf("fila: %+v %v", events, err)
	}
	r := Receipt{ReceiptID: "receipt-1", EventID: events[0].EventID, TenantID: device.TenantID, StoreID: device.StoreID, DeviceID: device.DeviceID, PayloadSHA256: events[0].PayloadSHA256()}
	if err := Confirm(ctx, db, device, r, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("sem verificador: %v", err)
	}
	if err := Confirm(ctx, db, device, r, checkedProof{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("sem prova: %v", err)
	}
	wrong := r
	wrong.PayloadSHA256 = "not-a-hash"
	wrong.Proof = []byte("accepted-by-peer")
	if err := Confirm(ctx, db, device, wrong, checkedProof{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("hash errado: %v", err)
	}
	if err := FailedAttempt(ctx, db, device, r.EventID); err != nil {
		t.Fatal(err)
	}
	events, err = Pending(ctx, db, device, 10)
	if err != nil || len(events) != 1 || events[0].Attempts != 1 {
		t.Fatalf("tentativa não registrada: %+v %v", events, err)
	}
	r.Proof = []byte("accepted-by-peer")
	if err := Confirm(ctx, db, device, r, checkedProof{}); err != nil {
		t.Fatal(err)
	}
	if err := Confirm(ctx, db, device, r, checkedProof{}); err != nil {
		t.Fatalf("mesmo recibo: %v", err)
	}
	events, err = Pending(ctx, db, device, 10)
	if err != nil || len(events) != 0 {
		t.Fatalf("evento confirmado reapareceu: %+v %v", events, err)
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM outbox WHERE event_id='market-event'`).Scan(&status); err != nil || status != "acked" {
		t.Fatalf("status: %s %v", status, err)
	}
	var others int
	if err = db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE tenant_id='other' AND status='pending'`).Scan(&others); err != nil || others != 1 {
		t.Fatalf("outra empresa afetada: %d %v", others, err)
	}
	changed := r
	changed.ReceiptID = "receipt-2"
	if err := Confirm(ctx, db, device, changed, checkedProof{}); !errors.Is(err, ErrReceiptConflict) {
		t.Fatalf("recibo substituído: %v", err)
	}
}

func TestOutboxChecksDeviceScopeAndRevocation(t *testing.T) {
	db, device := setup(t)
	ctx := context.Background()
	other := identity.DeviceContext{TenantID: "other", StoreID: "s", DeviceID: "d"}
	if err := FailedAttempt(ctx, db, other, "market-event"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("acesso cruzado: %v", err)
	}
	if _, err := Pending(ctx, db, device, 101); !errors.Is(err, ErrInvalid) {
		t.Fatalf("limite ignorado: %v", err)
	}
	if _, err := db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id='market' AND device_id='d'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Pending(ctx, db, device, 10); !errors.Is(err, ErrDenied) {
		t.Fatalf("aparelho revogado: %v", err)
	}
	if err := FailedAttempt(ctx, db, device, "market-event"); !errors.Is(err, ErrDenied) {
		t.Fatalf("falha escrita por aparelho revogado: %v", err)
	}
}
