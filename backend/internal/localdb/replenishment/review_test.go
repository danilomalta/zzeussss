package replenishment

import (
	"context"
	"errors"
	"testing"

	"titansystem-backend/internal/localdb/identity"
)

func TestManagerApprovalAndReplay(t *testing.T) {
	db, manager, device := fixture(t)
	ctx := context.Background()
	if _, err := SetPolicy(ctx, db, manager, device, PolicyInput{OperationID: "rule", ProductID: "p", MinimumMilli: 4000, TargetMilli: 10000}); err != nil {
		t.Fatal(err)
	}
	stock := manager
	stock.IdentityID = "stock"
	s, err := Suggest(ctx, db, stock, device, SuggestInput{OperationID: "suggest", ProductID: "p"})
	if err != nil || !s.Needed {
		t.Fatalf("preparar sugestão: %+v %v", s, err)
	}
	in := ReviewInput{OperationID: "review", SuggestionID: s.SuggestionID, Decision: "approved", Reason: "repor gôndola"}
	if _, err = Review(ctx, db, stock, device, in); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("estoquista aprovou: %v", err)
	}
	r, err := Review(ctx, db, manager, device, in)
	if err != nil || r.Repeated || r.Decision != "approved" {
		t.Fatalf("aprovação: %+v %v", r, err)
	}
	r, err = Review(ctx, db, manager, device, in)
	if err != nil || !r.Repeated {
		t.Fatalf("repetição: %+v %v", r, err)
	}
	in.Reason = "outro motivo"
	if _, err = Review(ctx, db, manager, device, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("ID com motivo alterado: %v", err)
	}
	var reviews, events int
	if err = db.QueryRow(`SELECT COUNT(*) FROM restock_reviews`).Scan(&reviews); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if reviews != 1 || events != 3 {
		t.Fatalf("decisões/eventos duplicados: %d %d", reviews, events)
	}
	if _, err = Suggest(ctx, db, stock, device, SuggestInput{OperationID: "another", ProductID: "p"}); err != nil {
		t.Fatal(err)
	}
	var second string
	if err = db.QueryRow(`SELECT id FROM restock_suggestions WHERE operation_id='another'`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	if _, err = Review(ctx, db, manager, device, ReviewInput{OperationID: "second-review", SuggestionID: second, Decision: "approved", Reason: "repetir"}); !errors.Is(err, ErrPendingApproval) {
		t.Fatalf("dupla aprovação: %v", err)
	}
}

func TestStaleProposalRejectableButNotApprovable(t *testing.T) {
	db, manager, device := fixture(t)
	ctx := context.Background()
	if _, err := SetPolicy(ctx, db, manager, device, PolicyInput{OperationID: "rule", ProductID: "p", MinimumMilli: 4000, TargetMilli: 10000}); err != nil {
		t.Fatal(err)
	}
	s, err := Suggest(ctx, db, manager, device, SuggestInput{OperationID: "suggest", ProductID: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO stock_movements VALUES ('new-stock','a','s','d','p','back',2000,'entrada','now')`); err != nil {
		t.Fatal(err)
	}
	in := ReviewInput{OperationID: "review", SuggestionID: s.SuggestionID, Decision: "approved", Reason: "repor"}
	if _, err = Review(ctx, db, manager, device, in); !errors.Is(err, ErrStale) {
		t.Fatalf("estoque mudou: %v", err)
	}
	in.Decision = "rejected"
	in.Reason = "saldo já atualizado"
	if _, err = Review(ctx, db, manager, device, in); err != nil {
		t.Fatalf("rejeição: %v", err)
	}
	var status string
	if err = db.QueryRow(`SELECT status FROM restock_suggestions WHERE id=?`, s.SuggestionID).Scan(&status); err != nil || status != "rejected" {
		t.Fatalf("status: %s %v", status, err)
	}
}

func TestPolicyChangeInvalidatesApprovalAndCrossTenantBlocked(t *testing.T) {
	db, manager, device := fixture(t)
	ctx := context.Background()
	if _, err := SetPolicy(ctx, db, manager, device, PolicyInput{OperationID: "rule", ProductID: "p", MinimumMilli: 4000, TargetMilli: 10000}); err != nil {
		t.Fatal(err)
	}
	s, err := Suggest(ctx, db, manager, device, SuggestInput{OperationID: "suggest", ProductID: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SetPolicy(ctx, db, manager, device, PolicyInput{OperationID: "new-rule", ProductID: "p", MinimumMilli: 5000, TargetMilli: 12000}); err != nil {
		t.Fatal(err)
	}
	in := ReviewInput{OperationID: "review", SuggestionID: s.SuggestionID, Decision: "approved", Reason: "repor"}
	if _, err = Review(ctx, db, manager, device, in); !errors.Is(err, ErrStale) {
		t.Fatalf("regra mudou: %v", err)
	}
	foreign := manager
	foreign.TenantID = "b"
	if _, err = Review(ctx, db, foreign, device, in); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("outro tenant: %v", err)
	}
}
