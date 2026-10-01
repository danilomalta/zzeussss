package sale

import (
	"context"
	"errors"
	"testing"

	"titansystem-backend/internal/localdb/entitlementstore"
)

func TestNilSaleContractCannotUseLegacyPermissionOnlyPath(t *testing.T) {
	db, actor, device := fixture(t)
	if _, err := CompleteWithContract(context.Background(), db, nil, actor, device, input()); !errors.Is(err, entitlementstore.ErrNotInstalled) {
		t.Fatalf("contrato nil registrou venda: %v", err)
	}
	if count(t, db, "sales") != 0 || count(t, db, "sale_operations") != 0 || count(t, db, "outbox") != 1 {
		t.Fatal("contrato ausente deixou gravacao parcial")
	}
}
