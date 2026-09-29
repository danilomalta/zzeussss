package stock

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

func stockDB(t *testing.T) (*sql.DB, identity.Scope, identity.DeviceContext) {
	t.Helper()
	ctx := context.Background()
	db, err := localdb.Open(ctx, filepath.Join(t.TempDir(), "store.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	statements := []struct {
		query string
		args []any
	}{
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"market", "Mercado", "now"}},
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"other", "Outra", "now"}},
		{"INSERT INTO stores VALUES (?, ?, ?)", []any{"market", "s1", "Loja"}},
		{"INSERT INTO devices VALUES (?, ?, ?, ?)", []any{"market", "s1", "d1", "Caixa"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"operator", "Operador", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"market", "operator", "stock", "active", "now"}},
		{"INSERT INTO membership_stores VALUES (?, ?, ?)", []any{"market", "operator", "s1"}},
		{`INSERT INTO device_pairings
			(tenant_id, store_id, device_id, public_key, challenge, challenge_expires_unix, status, requested_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, []any{"market", "s1", "d1", make([]byte, 32), make([]byte, 32), 9999999999, "approved", "operator"}},
		{"INSERT INTO stock_locations VALUES (?, ?, ?, ?, ?)", []any{"market", "s1", "back", "backroom", "Depósito"}},
		{"INSERT INTO stock_locations VALUES (?, ?, ?, ?, ?)", []any{"market", "s1", "shelf", "shelf", "Gôndola"}},
		{"INSERT INTO products (tenant_id, id, sku, name, price_cents) VALUES (?, ?, ?, ?, ?)", []any{"market", "p1", "SKU1", "Produto", 499}},
		{"INSERT INTO products (tenant_id, id, sku, name, price_cents) VALUES (?, ?, ?, ?, ?)", []any{"other", "p2", "SKU2", "Outro", 799}},
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("preparar cenário: %v", err)
		}
	}
	return db, identity.Scope{IdentityID: "operator", TenantID: "market", StoreID: "s1"},
		identity.DeviceContext{TenantID: "market", StoreID: "s1", DeviceID: "d1"}
}

func TestMovementsAndOutboxRemainAtomicAndIdempotent(t *testing.T) {
	db, actor, device := stockDB(t)
	ctx := context.Background()
	operations := []Input{
		{OperationID: "entry-one", Kind: "entry", ProductID: "p1", ToLocationID: "back", QuantityMilli: 10000, Reason: "recebimento"},
		{OperationID: "transfer-one", Kind: "transfer", ProductID: "p1", FromLocationID: "back", ToLocationID: "shelf", QuantityMilli: 3000, Reason: "repor gôndola"},
		{OperationID: "loss-one", Kind: "loss", ProductID: "p1", FromLocationID: "shelf", QuantityMilli: 1000, Reason: "quebra"},
	}
	for _, op := range operations {
		if result, err := Record(ctx, db, actor, device, op); err != nil || result.Repeated {
			t.Fatalf("registrar %s: %+v %v", op.OperationID, result, err)
		}
	}
	result, err := Record(ctx, db, actor, device, operations[1])
	if err != nil || !result.Repeated {
		t.Fatalf("repetição mudou resultado: %+v %v", result, err)
	}
	altered := operations[1]
	altered.QuantityMilli = 2000
	if _, err := Record(ctx, db, actor, device, altered); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("ID reutilizado com conteúdo alterado: %v", err)
	}
	for _, tc := range []struct { location string; want int64 }{{"back", 7000}, {"shelf", 2000}} {
		balance, err := Balance(ctx, db, actor, device, "p1", tc.location)
		if err != nil || balance != tc.want {
			t.Fatalf("saldo %s: %d erro=%v esperado=%d", tc.location, balance, err, tc.want)
		}
	}
	var movements, events, operationsCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_movements").Scan(&movements); err != nil { t.Fatal(err) }
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM outbox").Scan(&events); err != nil { t.Fatal(err) }
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_operations").Scan(&operationsCount); err != nil { t.Fatal(err) }
	if movements != 4 || events != 3 || operationsCount != 3 {
		t.Fatalf("contagens inesperadas: movimentos=%d outbox=%d operações=%d", movements, events, operationsCount)
	}
}

func TestFailureRollsBackAndCannotCrossTenant(t *testing.T) {
	db, actor, device := stockDB(t)
	ctx := context.Background()
	insufficient := Input{OperationID: "too-much", Kind: "transfer", ProductID: "p1", FromLocationID: "back", ToLocationID: "shelf", QuantityMilli: 1000, Reason: "transferir"}
	if _, err := Record(ctx, db, actor, device, insufficient); !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("saída sem saldo aceita: %v", err)
	}
	foreign := Input{OperationID: "foreign", Kind: "entry", ProductID: "p2", ToLocationID: "back", QuantityMilli: 1000, Reason: "entrada"}
	if _, err := Record(ctx, db, actor, device, foreign); err == nil {
		t.Fatal("produto de outro tenant foi aceito")
	}
	var movements, events, operationsCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_movements").Scan(&movements); err != nil { t.Fatal(err) }
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM outbox").Scan(&events); err != nil { t.Fatal(err) }
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_operations").Scan(&operationsCount); err != nil { t.Fatal(err) }
	if movements != 0 || events != 0 || operationsCount != 0 {
		t.Fatalf("falha deixou registros: movimentos=%d outbox=%d operações=%d", movements, events, operationsCount)
	}
	if _, err := db.ExecContext(ctx, "UPDATE device_pairings SET status = 'revoked' WHERE tenant_id = ? AND device_id = ?", "market", "d1"); err != nil {
		t.Fatal(err)
	}
	valid := Input{OperationID: "after-revoke", Kind: "entry", ProductID: "p1", ToLocationID: "back", QuantityMilli: 1000, Reason: "entrada"}
	if _, err := Record(ctx, db, actor, device, valid); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho revogado registrou movimento: %v", err)
	}
}
