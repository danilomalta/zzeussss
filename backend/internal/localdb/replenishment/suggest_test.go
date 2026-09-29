package replenishment

import (
	"context"
	"errors"
	"testing"
)

func TestSuggestionSnapshotsBalanceAndDoesNotDispatch(t *testing.T) {
	db, manager, device := fixture(t)
	ctx := context.Background()
	if _, err := SetPolicy(ctx, db, manager, device, PolicyInput{OperationID: "policy", ProductID: "p", MinimumMilli: 4000, TargetMilli: 10000}); err != nil {
		t.Fatal(err)
	}
	stock := manager
	stock.IdentityID = "stock"
	in := SuggestInput{OperationID: "suggest-1", ProductID: "p"}
	result, err := Suggest(ctx, db, stock, device, in)
	if err != nil || !result.Needed || result.RecommendedMilli != 7000 || result.ObservedMilli != 3000 || result.PolicyRevision != 1 {
		t.Fatalf("sugestão: %+v %v", result, err)
	}
	if result.SuggestionID == "" {
		t.Fatal("faltou ID local")
	}
	if _, err := db.Exec(`INSERT INTO stock_movements VALUES ('new-stock','a','s','d','p','back',6000,'entrada','now')`); err != nil {
		t.Fatal(err)
	}
	repeated, err := Suggest(ctx, db, stock, device, in)
	if err != nil || !repeated.Repeated || repeated.SuggestionID != result.SuggestionID || repeated.RecommendedMilli != 7000 {
		t.Fatalf("repetição mudou proposta: %+v %v", repeated, err)
	}
	in.OperationID = "suggest-2"
	result, err = Suggest(ctx, db, stock, device, in)
	if err != nil || result.Needed || result.RecommendedMilli != 0 || result.ObservedMilli != 9000 {
		t.Fatalf("sem necessidade: %+v %v", result, err)
	}
	var suggestions, events int
	if err = db.QueryRow(`SELECT COUNT(*) FROM restock_suggestions`).Scan(&suggestions); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if suggestions != 2 || events != 2 {
		t.Fatalf("resultados: %d sugestões, %d eventos (política+sugestão)", suggestions, events)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM restock_suggestions WHERE status='approved'`).Scan(&suggestions); err != nil || suggestions != 0 {
		t.Fatalf("aprovou automaticamente: %d %v", suggestions, err)
	}
}

func TestSuggestionRequiresPolicyAndScope(t *testing.T) {
	db, actor, device := fixture(t)
	ctx := context.Background()
	if _, err := Suggest(ctx, db, actor, device, SuggestInput{OperationID: "no-policy", ProductID: "p"}); !errors.Is(err, ErrNoPolicy) {
		t.Fatalf("sem regra: %v", err)
	}
	if _, err := SetPolicy(ctx, db, actor, device, PolicyInput{OperationID: "policy", ProductID: "p", MinimumMilli: 4000, TargetMilli: 10000}); err != nil {
		t.Fatal(err)
	}
	if _, err := Suggest(ctx, db, actor, device, SuggestInput{OperationID: "foreign", ProductID: "foreign"}); !errors.Is(err, ErrNoPolicy) {
		t.Fatalf("produto de outro tenant: %v", err)
	}
	if _, err := db.Exec(`UPDATE memberships SET status='revoked' WHERE tenant_id='a' AND identity_id='manager'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Suggest(ctx, db, actor, device, SuggestInput{OperationID: "revoked", ProductID: "p"}); err == nil {
		t.Fatal("vínculo revogado calculou sugestão")
	}
}
