package replenishment

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
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "replenishment.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		`INSERT INTO tenants VALUES ('a','Mercado','now')`,
		`INSERT INTO tenants VALUES ('b','Outro','now')`,
		`INSERT INTO stores VALUES ('a','s','Loja')`,
		`INSERT INTO devices VALUES ('a','s','d','Caixa')`,
		`INSERT INTO identities VALUES ('manager','Gerente','now')`,
		`INSERT INTO identities VALUES ('stock','Estoquista','now')`,
		`INSERT INTO memberships VALUES ('a','manager','manager','active','now')`,
		`INSERT INTO memberships VALUES ('a','stock','stock','active','now')`,
		`INSERT INTO membership_stores VALUES ('a','manager','s')`,
		`INSERT INTO membership_stores VALUES ('a','stock','s')`,
		`INSERT INTO products (tenant_id,id,sku,name,price_cents) VALUES ('a','p','SKU','Produto',199)`,
		`INSERT INTO products (tenant_id,id,sku,name,price_cents) VALUES ('b','foreign','SKU','Outro',199)`,
		`INSERT INTO stock_locations VALUES ('a','s','shelf','shelf','Gôndola')`,
		`INSERT INTO stock_locations VALUES ('a','s','back','backroom','Depósito')`,
		`INSERT INTO stock_movements VALUES ('entry','a','s','d','p','shelf',3000,'entrada','now')`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by)
		VALUES ('a','s','d',?,?,9999999999,'approved','manager')`, make([]byte, 32), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return db, identity.Scope{TenantID: "a", StoreID: "s", IdentityID: "manager"}, identity.DeviceContext{TenantID: "a", StoreID: "s", DeviceID: "d"}
}

func TestPolicyRevisionReplayAndScope(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	in := PolicyInput{OperationID: "set-1", ProductID: "p", MinimumMilli: 4000, TargetMilli: 10000}
	got, err := SetPolicy(ctx, db, actor, device, in)
	if err != nil || got.Revision != 1 || got.Repeated {
		t.Fatalf("política: %+v %v", got, err)
	}
	got, err = SetPolicy(ctx, db, actor, device, in)
	if err != nil || !got.Repeated || got.Revision != 1 {
		t.Fatalf("repetição: %+v %v", got, err)
	}
	in.MinimumMilli = 5000
	if _, err = SetPolicy(ctx, db, actor, device, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("alteração com mesmo ID: %v", err)
	}
	in.OperationID = "set-2"
	got, err = SetPolicy(ctx, db, actor, device, in)
	if err != nil || got.Revision != 2 {
		t.Fatalf("revisão: %+v %v", got, err)
	}
	var changes, events int
	if err = db.QueryRow(`SELECT COUNT(*) FROM restock_policy_changes`).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if changes != 2 || events != 2 {
		t.Fatalf("auditoria: %d %d", changes, events)
	}
	actor.IdentityID = "stock"
	if _, err = SetPolicy(ctx, db, actor, device, PolicyInput{OperationID: "forbidden", ProductID: "p", MinimumMilli: 1, TargetMilli: 5}); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("estoquista mudou política: %v", err)
	}
}

func TestPolicyRejectsForeignProductAndRevokedDevice(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	if _, err := SetPolicy(ctx, db, actor, device, PolicyInput{OperationID: "foreign", ProductID: "foreign", MinimumMilli: 0, TargetMilli: 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("produto externo: %v", err)
	}
	if _, err := db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id='a' AND device_id='d'`); err != nil {
		t.Fatal(err)
	}
	if _, err := SetPolicy(ctx, db, actor, device, PolicyInput{OperationID: "revoke", ProductID: "p", MinimumMilli: 1, TargetMilli: 2}); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho revogado: %v", err)
	}
}
