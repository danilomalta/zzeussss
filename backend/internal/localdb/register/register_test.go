package register

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

func setup(t *testing.T) (*sql.DB, identity.Scope, identity.DeviceContext) {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "cash.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`INSERT INTO tenants VALUES ('market','Mercado','now')`,
		`INSERT INTO stores VALUES ('market','s1','Loja')`,
		`INSERT INTO devices VALUES ('market','s1','d1','Caixa')`,
		`INSERT INTO identities VALUES ('op','Operador','now')`,
		`INSERT INTO memberships VALUES ('market','op','cashier','active','now')`,
		`INSERT INTO membership_stores VALUES ('market','op','s1')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`INSERT INTO device_pairings
		(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
		VALUES ('market','s1','d1',?,?,9999999999,'approved','op')`, make([]byte, 32), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return db, identity.Scope{TenantID: "market", StoreID: "s1", IdentityID: "op"}, identity.DeviceContext{TenantID: "market", StoreID: "s1", DeviceID: "d1"}
}

func TestOpenCloseAtomicIdempotentAndBlindCount(t *testing.T) {
	db, actor, device := setup(t)
	ctx := context.Background()
	open := OpenInput{SessionID: "sesh-1", OpeningCents: 1000}
	if _, err := Open(ctx, db, actor, device, open); err != nil {
		t.Fatal(err)
	}
	if got, err := Open(ctx, db, actor, device, open); err != nil || !got.Repeated {
		t.Fatalf("repetição: %+v %v", got, err)
	}
	if _, err := Open(ctx, db, actor, device, OpenInput{SessionID: "sesh-2", OpeningCents: 0}); err == nil {
		t.Fatal("segundo turno aberto")
	}
	if _, err := db.Exec(`INSERT INTO cash_movements(id,tenant_id,store_id,cash_session_id,amount_cents,reason,created_at)
		VALUES ('cash-1','market','s1','sesh-1',245,'venda','now')`); err != nil {
		t.Fatal(err)
	}
	close := CloseInput{SessionID: "sesh-1", OperationID: "close-1", DeclaredCents: 1200}
	got, err := Close(ctx, db, actor, device, close)
	if err != nil || got.ExpectedCents != 1245 || got.DifferenceCents != -45 {
		t.Fatalf("fechamento: %+v %v", got, err)
	}
	got, err = Close(ctx, db, actor, device, close)
	if err != nil || !got.Repeated || got.ExpectedCents != 1245 {
		t.Fatalf("repetição fechamento: %+v %v", got, err)
	}
	if _, err = Open(ctx, db, actor, device, open); !errors.Is(err, ErrClosed) {
		t.Fatalf("turno encerrado reapareceu aberto: %v", err)
	}
	close.DeclaredCents = 1250
	if _, err = Close(ctx, db, actor, device, close); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflito: %v", err)
	}
	var sessions, bindings, closures, events int
	for _, pair := range []struct {
		q    string
		dest *int
	}{
		{`SELECT COUNT(*) FROM cash_sessions`, &sessions}, {`SELECT COUNT(*) FROM cash_session_operators`, &bindings},
		{`SELECT COUNT(*) FROM cash_closures`, &closures}, {`SELECT COUNT(*) FROM outbox`, &events},
	} {
		if err = db.QueryRow(pair.q).Scan(pair.dest); err != nil {
			t.Fatal(err)
		}
	}
	if sessions != 1 || bindings != 1 || closures != 1 || events != 2 {
		t.Fatalf("contagens: %d %d %d %d", sessions, bindings, closures, events)
	}
}

func TestUnauthorizedCannotOpenOrClose(t *testing.T) {
	db, actor, device := setup(t)
	ctx := context.Background()
	wrong := actor
	wrong.StoreID = "other"
	if _, err := Open(ctx, db, wrong, device, OpenInput{SessionID: "s", OpeningCents: 0}); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("loja alheia: %v", err)
	}
	if _, err := db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id='market' AND device_id='d1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, db, actor, device, OpenInput{SessionID: "s", OpeningCents: 0}); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho revogado: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cash_sessions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("vazou turno: %d %v", count, err)
	}
}
