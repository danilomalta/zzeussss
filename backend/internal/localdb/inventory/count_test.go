package inventory

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

func fixture(t *testing.T) (*sql.DB, identity.Scope, identity.DeviceContext) {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "inventory.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, query := range []string{
		`INSERT INTO tenants VALUES ('a','Mercado','now')`,
		`INSERT INTO tenants VALUES ('b','Outro','now')`,
		`INSERT INTO stores VALUES ('a','s','Loja')`,
		`INSERT INTO devices VALUES ('a','s','d','Caixa')`,
		`INSERT INTO identities VALUES ('op','Estoquista','now')`,
		`INSERT INTO memberships VALUES ('a','op','stock','active','now')`,
		`INSERT INTO membership_stores VALUES ('a','op','s')`,
		`INSERT INTO stock_locations VALUES ('a','s','shelf','shelf','Gôndola')`,
		`INSERT INTO products(tenant_id,id,sku,name,price_cents) VALUES ('a','p','P','Produto',200)`,
		`INSERT INTO products(tenant_id,id,sku,name,price_cents) VALUES ('b','foreign','F','Outro',200)`,
		`INSERT INTO stock_movements VALUES ('entry','a','s','d','p','shelf',5000,'entrada','now')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
		VALUES ('a','s','d',?,?,9999999999,'approved','op')`, make([]byte, 32), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return db, identity.Scope{TenantID: "a", StoreID: "s", IdentityID: "op"}, identity.DeviceContext{TenantID: "a", StoreID: "s", DeviceID: "d"}
}

func TestCountAtomicAdjustmentAndReplay(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	in := Input{OperationID: "count-1", ProductID: "p", LocationID: "shelf", CountedMilli: 3000}
	result, err := Count(ctx, db, actor, device, in)
	if err != nil || result.PreviousMilli != 5000 || result.DifferenceMilli != -2000 || result.Repeated {
		t.Fatalf("primeira contagem: %+v %v", result, err)
	}
	result, err = Count(ctx, db, actor, device, in)
	if err != nil || !result.Repeated || result.DifferenceMilli != -2000 {
		t.Fatalf("repetição: %+v %v", result, err)
	}
	in.CountedMilli = 4000
	if _, err = Count(ctx, db, actor, device, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("ID alterado: %v", err)
	}
	var balance int64
	if err = db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements WHERE tenant_id='a' AND product_id='p'`).Scan(&balance); err != nil || balance != 3000 {
		t.Fatalf("saldo: %d %v", balance, err)
	}
	var counts, movements, outbox int
	for _, pair := range []struct {
		q    string
		dest *int
	}{
		{`SELECT COUNT(*) FROM inventory_counts`, &counts},
		{`SELECT COUNT(*) FROM stock_movements`, &movements},
		{`SELECT COUNT(*) FROM outbox`, &outbox},
	} {
		if err = db.QueryRow(pair.q).Scan(pair.dest); err != nil {
			t.Fatal(err)
		}
	}
	if counts != 1 || movements != 2 || outbox != 1 {
		t.Fatalf("contagens: %d %d %d", counts, movements, outbox)
	}
	if _, err = Count(ctx, db, actor, device, Input{OperationID: "count-2", ProductID: "p", LocationID: "shelf", CountedMilli: 3000}); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM stock_movements`).Scan(&movements); err != nil || movements != 2 {
		t.Fatalf("contagem sem diferença alterou saldo: %d %v", movements, err)
	}
}

func TestInvalidCountDoesNotWrite(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	for _, in := range []Input{
		{OperationID: "foreign", ProductID: "foreign", LocationID: "shelf", CountedMilli: 0},
		{OperationID: "negative", ProductID: "p", LocationID: "shelf", CountedMilli: -1},
	} {
		if _, err := Count(ctx, db, actor, device, in); err == nil {
			t.Fatalf("contagem inválida aceita: %+v", in)
		}
	}
	if _, err := db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id='a' AND device_id='d'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Count(ctx, db, actor, device, Input{OperationID: "revoked", ProductID: "p", LocationID: "shelf", CountedMilli: 0}); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho revogado: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inventory_counts`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("falha deixou contagem: %d %v", n, err)
	}
}
