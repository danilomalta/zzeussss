package onlinecatalog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPurchaseStrictInputExactMoneyAndDecisionStates(t *testing.T) {
	v, r := purchaseFixture49()
	b, _ := json.Marshal(v)
	if got, e := DecodePurchaseDraft("application/json", b); e != nil || got != v {
		t.Fatal(got, e)
	}
	for _, body := range []string{`{}`, string(b) + `{}`, strings.Replace(string(b), `"expected_offer_version":1`, `"expected_offer_version":0`, 1), strings.Replace(string(b), `"reason":"Compra interna"`, `"reason":null`, 1), strings.Replace(string(b), `"reason":"Compra interna"`, `"reason":"Compra interna","reason":"x"`, 1), strings.Replace(string(b), `"reason":"Compra interna"`, `"reason":"Compra interna","total_cents":1`, 1)} {
		if _, e := DecodePurchaseDraft("application/json", []byte(body)); e != ErrInput {
			t.Fatal(body, e)
		}
	}
	if !validPurchaseReceipt(r) || r.After.TotalCents != 1250 {
		t.Fatal(r)
	}
	offer := r.After.Offer
	offer.UnitPriceCents = MaxPrice
	if _, e := purchaseTotal(MaxStockUnits, offer); e != ErrInput {
		t.Fatal("overflow", e)
	}
	if _, e := purchaseTotal(1, offer); e != ErrConflict {
		t.Fatal("minimum", e)
	}
	offer.UnitPriceCents = 0
	if total, e := purchaseTotal(2, offer); e != nil || total != 0 {
		t.Fatal(total, e)
	}
	decision := ReplenishmentDecisionInput{OperationID: testOp, ExpectedRevision: 1, Reason: "Aprovado internamente"}
	approved, e := purchaseTransition(r.After, "approve", decision, fixtureActor.User, r.After.CreatedAt.Add(time.Second))
	if e != nil || approved.State != "approved" {
		t.Fatal(e)
	}
	decision.ExpectedRevision = 2
	if _, e = purchaseTransition(approved, "approve", decision, fixtureActor.User, approved.UpdatedAt); e != ErrConflict {
		t.Fatal(e)
	}
	cancelled, e := purchaseTransition(approved, "cancel", decision, fixtureActor.User, approved.UpdatedAt)
	if e != nil || cancelled.State != "cancelled" {
		t.Fatal(e)
	}
	decision.ExpectedRevision = 3
	if _, e = purchaseTransition(cancelled, "cancel", decision, fixtureActor.User, cancelled.UpdatedAt); e != ErrConflict {
		t.Fatal(e)
	}
	r.After.TotalCents++
	if validPurchaseReceipt(r) {
		t.Fatal("total adulterado")
	}
	if MigratePurchaseDrafts(context.Background(), nil) != ErrUnavailable || CheckPurchaseDrafts(context.Background(), nil) != ErrUnavailable {
		t.Fatal("nil database")
	}
}
