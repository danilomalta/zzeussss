package sale

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
)

func fixture(t *testing.T) (*sql.DB, identity.Scope, identity.DeviceContext) {
	t.Helper()
	ctx := context.Background()
	db, err := localdb.Open(ctx, filepath.Join(t.TempDir(), "pos.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`INSERT INTO tenants VALUES ('a','Mercado','now')`,
		`INSERT INTO tenants VALUES ('b','Outro','now')`,
		`INSERT INTO stores VALUES ('a','s','Loja')`,
		`INSERT INTO devices VALUES ('a','s','d','Caixa')`,
		`INSERT INTO identities VALUES ('op','Operador','now')`,
		`INSERT INTO memberships VALUES ('a','op','cashier','active','now')`,
		`INSERT INTO membership_stores VALUES ('a','op','s')`,
		`INSERT INTO stock_locations VALUES ('a','s','shelf','shelf','Gôndola')`,
		`INSERT INTO products (tenant_id,id,sku,name,price_cents,unit) VALUES ('a','p','SKU','Produto',299,'kg')`,
		`INSERT INTO products (tenant_id,id,sku,name,price_cents) VALUES ('b','foreign','F','Alheio',1)`,
	} {
		if _, err = db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.ExecContext(ctx, `INSERT INTO device_pairings
		(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
		VALUES ('a','s','d',?,?,9999999999,'approved','op')`, make([]byte, 32), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	actor := identity.Scope{TenantID: "a", StoreID: "s", IdentityID: "op"}
	device := identity.DeviceContext{TenantID: "a", StoreID: "s", DeviceID: "d"}
	if _, err = register.Open(ctx, db, actor, device, register.OpenInput{SessionID: "shift", OpeningCents: 0}); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO stock_movements VALUES ('entry','a','s','d','p','shelf',5000,'entrada','now')`)
	if err != nil {
		t.Fatal(err)
	}
	return db, actor, device
}

func input() Input {
	return Input{OperationID: "op-1", SaleID: "sale-1", CashSessionID: "shift",
		Items:    []Item{{ProductID: "p", LocationID: "shelf", QuantityMilli: 1500}},
		Payments: []Payment{{Method: "cash", AmountCents: 449}}}
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSalePersistsPaymentsStockAndOutboxOnce(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	in := input()
	got, err := Complete(ctx, db, actor, device, in)
	if err != nil || got.TotalCents != 449 || got.Repeated {
		t.Fatalf("venda: %+v %v", got, err)
	}
	got, err = Complete(ctx, db, actor, device, in)
	if err != nil || !got.Repeated || got.TotalCents != 449 {
		t.Fatalf("repetição: %+v %v", got, err)
	}
	if count(t, db, "sales") != 1 || count(t, db, "sale_items") != 1 || count(t, db, "sale_payments") != 1 || count(t, db, "sale_item_stock") != 1 || count(t, db, "cash_movements") != 1 || count(t, db, "outbox") != 2 {
		t.Fatal("venda ou outbox duplicada")
	}
	var balance int64
	if err = db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements WHERE tenant_id='a' AND product_id='p'`).Scan(&balance); err != nil || balance != 3500 {
		t.Fatalf("estoque: %d %v", balance, err)
	}
	in.Items[0].QuantityMilli = 1000
	if _, err = Complete(ctx, db, actor, device, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("repetição alterada: %v", err)
	}
}

func TestSaleFailuresDoNotLeavePartialWrites(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	for _, change := range []func(*Input){
		func(in *Input) { in.Items[0].QuantityMilli = 6000 },
		func(in *Input) { in.Items[0].ProductID = "foreign" },
		func(in *Input) { in.Payments[0].AmountCents = 450 },
		func(in *Input) { in.Payments[0].Method = "pix" },
		func(in *Input) { in.CashSessionID = "unknown" },
	} {
		in := input()
		change(&in)
		if _, err := Complete(ctx, db, actor, device, in); err == nil {
			t.Fatal("venda inválida aceita")
		}
		if count(t, db, "sales") != 0 || count(t, db, "sale_operations") != 0 || count(t, db, "cash_movements") != 0 || count(t, db, "outbox") != 1 {
			t.Fatal("falha deixou operação parcial")
		}
	}
	// A mesma gôndola em duas linhas não pode gastar o mesmo saldo duas vezes.
	in := input()
	in.Items = []Item{{ProductID: "p", LocationID: "shelf", QuantityMilli: 3000}, {ProductID: "p", LocationID: "shelf", QuantityMilli: 3000}}
	in.Payments[0].AmountCents = 1794
	if _, err := Complete(ctx, db, actor, device, in); !errors.Is(err, ErrNoStock) {
		t.Fatalf("saldo duplicado: %v", err)
	}
}

func TestRevokedDeviceAndClosedRegisterBlockSale(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	if _, err := register.Close(ctx, db, actor, device, register.CloseInput{SessionID: "shift", OperationID: "end", DeclaredCents: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := Complete(ctx, db, actor, device, input()); !errors.Is(err, ErrCashClosed) {
		t.Fatalf("caixa fechado: %v", err)
	}
	if _, err := db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id='a' AND device_id='d'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Complete(ctx, db, actor, device, input()); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho revogado: %v", err)
	}
}

func TestRoundedCents(t *testing.T) {
	for _, tc := range []struct{ price, quantity, want int64 }{{299, 1500, 449}, {199, 500, 100}, {1, 1, 0}} {
		got, err := roundedCents(tc.price, tc.quantity)
		if err != nil || got != tc.want {
			t.Fatalf("arredondamento: %d %v, esperado %d", got, err, tc.want)
		}
	}
}
