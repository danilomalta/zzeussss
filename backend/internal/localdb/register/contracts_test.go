package register

import (
	"context"
	"errors"
	"testing"

	"titansystem-backend/internal/localdb/entitlementstore"
)

func TestNilCashContractCannotFallBackToLegacyAuthorization(t *testing.T) {
	db, actor, device := setup(t)
	ctx := context.Background()
	if _, err := OpenWithContract(ctx, db, nil, actor, device, OpenInput{SessionID: "new"}); !errors.Is(err, entitlementstore.ErrNotInstalled) {
		t.Fatalf("contrato ausente abriu turno: %v", err)
	}
	if _, err := Open(ctx, db, actor, device, OpenInput{SessionID: "existing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CloseWithContract(ctx, db, nil, actor, device, CloseInput{SessionID: "existing", OperationID: "close"}); !errors.Is(err, entitlementstore.ErrNotInstalled) {
		t.Fatalf("contrato ausente fechou turno: %v", err)
	}
	current, err := Current(ctx, db, actor, device)
	if err != nil || current == nil || current.SessionID != "existing" {
		t.Fatalf("consulta autorizada: %+v %v", current, err)
	}
	var closures int
	if err := db.QueryRow("SELECT COUNT(*) FROM cash_closures").Scan(&closures); err != nil || closures != 0 {
		t.Fatalf("fechamento indevido: %d %v", closures, err)
	}
}
