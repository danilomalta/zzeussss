package identity

import (
	"context"
	"errors"
	"testing"
)

func TestCanOperateCombinesActorStoreAndDevice(t *testing.T) {
	device, _, db := approvedDevice(t)
	ctx := context.Background()
	cashier := Scope{IdentityID: "caixa", TenantID: "market", StoreID: "m1"}
	manager := Scope{IdentityID: "gerente", TenantID: "market", StoreID: "m1"}
	if err := CanOperate(ctx, db, cashier, device, Sell); err != nil {
		t.Fatalf("caixa autorizado não conseguiu vender: %v", err)
	}
	if err := CanOperate(ctx, db, cashier, device, ManageStock); !errors.Is(err, ErrDenied) {
		t.Fatalf("caixa alterou estoque: %v", err)
	}
	if err := CanOperate(ctx, db, manager, device, ManageStock); err != nil {
		t.Fatalf("gerente não conseguiu gerenciar estoque: %v", err)
	}
	if err := CanOperateReview(ctx, db, manager, device, "gerente"); !errors.Is(err, ErrDenied) {
		t.Fatalf("autorrevisão foi aceita: %v", err)
	}
	if err := CanOperateReview(ctx, db, manager, device, "caixa"); err != nil {
		t.Fatalf("revisão alheia foi negada: %v", err)
	}
	wrongStore := Scope{IdentityID: "caixa", TenantID: "market", StoreID: "m2"}
	if err := CanOperate(ctx, db, wrongStore, device, Sell); !errors.Is(err, ErrDenied) {
		t.Fatalf("loja incorreta aceita: %v", err)
	}
	wrongTenant := DeviceContext{TenantID: "supplier", StoreID: "f1", DeviceID: device.DeviceID}
	if err := CanOperate(ctx, db, cashier, wrongTenant, Sell); !errors.Is(err, ErrDenied) {
		t.Fatalf("empresa incorreta aceita: %v", err)
	}
	if err := RevokeDevice(ctx, db, manager, device.DeviceID); err != nil {
		t.Fatal(err)
	}
	if err := CanOperate(ctx, db, cashier, device, Sell); !errors.Is(err, ErrDenied) {
		t.Fatalf("aparelho revogado autorizou venda: %v", err)
	}
}

func TestCanOperateRejectsRevokedMembership(t *testing.T) {
	device, _, db := approvedDevice(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "UPDATE memberships SET status = 'revoked' WHERE tenant_id = ? AND identity_id = ?",
		"market", "caixa"); err != nil {
		t.Fatal(err)
	}
	if err := CanOperate(ctx, db, Scope{IdentityID: "caixa", TenantID: "market", StoreID: "m1"}, device, Sell); !errors.Is(err, ErrDenied) {
		t.Fatalf("vínculo revogado autorizou venda: %v", err)
	}
}
