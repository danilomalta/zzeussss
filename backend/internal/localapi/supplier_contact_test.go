package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/replenishment"
	"titansystem-backend/internal/localdb/supplycycle"
)

const contactPath = "/local/v1/purchase-suppliers/supplier/contact-alert"

func contactInput(op string, threshold int64) supplycycle.ContactPolicyInput {
	return supplycycle.ContactPolicyInput{OperationID: op, Enabled: true, Threshold: threshold, Since: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), CountMode: "current_declined_orders"}
}
func contactGet(t *testing.T, f *httpContractFixture) supplycycle.ContactAlert {
	t.Helper()
	code, raw := request(t, f.app, "GET", contactPath, "", f.token)
	if code != 200 {
		t.Fatalf("contact read: %d", code)
	}
	var out supplycycle.ContactAlert
	if json.Unmarshal(raw, &out) != nil {
		t.Fatal("contact JSON")
	}
	domainSchemaFile(t, "purchase-supply-cycle.openapi.json", "ContactAlert", raw)
	return out
}
func contactRefuse(t *testing.T, f *httpContractFixture, i int) string {
	t.Helper()
	var product map[string]any
	if e := json.Unmarshal([]byte(testProductBody), &product); e != nil {
		t.Fatal(e)
	}
	product["sku"] = fmt.Sprintf("P14-%d", i)
	raw, _ := json.Marshal(product)
	code, b := request(t, f.app, "POST", "/local/v1/products", string(raw), f.token)
	if code != 201 {
		t.Fatalf("product fixture %d", code)
	}
	var p struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	s := retailServiceFor(f)
	ctx := context.Background()
	if _, e := replenishment.SetPolicy(ctx, f.db, s.Actor, f.device, replenishment.PolicyInput{OperationID: fmt.Sprintf("policy-%d", i), ProductID: p.ID, MinimumMilli: 1000, TargetMilli: 5000}); e != nil {
		t.Fatal(e)
	}
	sug, e := replenishment.Suggest(ctx, f.db, s.Actor, f.device, replenishment.SuggestInput{OperationID: fmt.Sprintf("suggest-%d", i), ProductID: p.ID})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := replenishment.Review(ctx, f.db, s.Actor, f.device, replenishment.ReviewInput{OperationID: fmt.Sprintf("review-%d", i), SuggestionID: sug.SuggestionID, Decision: "approved", Reason: "Human approval"}); e != nil {
		t.Fatal(e)
	}
	id := fmt.Sprintf("contact-order-%d", i)
	raw, _ = json.Marshal(purchases.Input{OperationID: id, OrderID: id, SupplierID: "supplier", SuggestionID: sug.SuggestionID})
	if code, _ := request(t, f.app, "POST", "/local/v1/purchase-orders", string(raw), f.token); code != 201 {
		t.Fatalf("approved order fixture %d", code)
	}
	in := cycleDecisionInput()
	in.OperationID = "decline-" + id
	in.Decision = "declined"
	in.AcceptedMilli = 0
	in.PromisedAt = ""
	cycleCall(t, f, "POST", "/local/v1/purchase-orders/"+id+"/supplier-decisions", in, 200)
	cycleCall(t, f, "POST", "/local/v1/purchase-orders/"+id+"/supplier-decisions", in, 200)
	return id
}
func TestSupplierContactPolicyManualRestartAndNoAutomaticOrder(t *testing.T) {
	f, manager := cycleFixture(t)
	if out := contactGet(t, f); out.Policy.Enabled || out.Recommended || out.SourceComplete {
		t.Fatal("default fabricated active/completed alert")
	}
	in := contactInput("contact-config", 3)
	raw, _ := json.Marshal(in)
	if code, _ := request(t, f.app, "POST", contactPath, string(raw), manager); code != 403 {
		t.Fatal("non-owner configured", code)
	}
	cycleCall(t, f, "POST", contactPath, in, 200)
	for i := 1; i <= 3; i++ {
		contactRefuse(t, f, i)
		before := cycleCounts(t, f)
		out := contactGet(t, f)
		if out.ManualDeclinedOrders != int64(i) || out.AuthenticatedDeclinedOrders != 0 || !out.SourceComplete || out.Recommended != (i >= 3) {
			t.Fatal("incorrect mode or deduplication", out)
		}
		after := cycleCounts(t, f)
		if fmt.Sprint(before) != fmt.Sprint(after) {
			t.Fatal("advisory created business effect")
		}
	}
	replay := cycleCall(t, f, "POST", contactPath, in, 200)
	if !replay.Repeated {
		t.Fatal("config replay not recovered")
	}
	in.Threshold = 2
	cycleCall(t, f, "POST", contactPath, in, 409)
	var seq int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	if e := f.db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	f.db = db
	defer db.Close()
	v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	f.app, e = NewWithVerifier(db, f.device, v)
	if e != nil {
		t.Fatal(e)
	}
	if out := contactGet(t, f); out.Policy.Revision != 1 || !out.Recommended || out.ManualDeclinedOrders != 3 {
		t.Fatal("restart lost policy/evidence", out)
	}
	confirmation := cycleDecisionInput()
	confirmation.OperationID = "confirm-after-decline"
	confirmation.ExpectedRevision = 1
	confirmation.PromisedAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	cycleCall(t, f, "POST", "/local/v1/purchase-orders/contact-order-1/supplier-decisions", confirmation, 200)
	if out := contactGet(t, f); out.Recommended || out.ManualDeclinedOrders != 2 {
		t.Fatal("superseded refusal counted", out)
	}
	foreign, _ := cycleFixture(t)
	if code, _ := request(t, foreign.app, "GET", "/local/v1/purchase-suppliers/foreign/contact-alert", "", foreign.token); code != 404 {
		t.Fatal("unknown supplier accessible", code)
	}
}
func TestSupplierContactPolicyConcurrentAndAuditRollback(t *testing.T) {
	f, _ := cycleFixture(t)
	in := contactInput("contact-rollback", 3)
	if _, e := f.db.Exec(`CREATE TRIGGER contact_ignore BEFORE INSERT ON supplier_contact_policy_operations BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	cycleCall(t, f, "POST", contactPath, in, 409)
	if out := contactGet(t, f); out.Policy.Revision != 0 {
		t.Fatal("policy survived missing audit")
	}
	if _, e := f.db.Exec(`DROP TRIGGER contact_ignore`); e != nil {
		t.Fatal(e)
	}
	const n = 4
	var wg sync.WaitGroup
	codes := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			x := in
			x.OperationID = fmt.Sprintf("concurrent-contact-%d", i)
			b, _ := json.Marshal(x)
			code, _ := request(t, f.app, "POST", contactPath, string(b), f.token)
			codes <- code
		}(i)
	}
	wg.Wait()
	close(codes)
	winners := 0
	for code := range codes {
		if code == 200 {
			winners++
		} else if code != 409 {
			t.Fatal("unexpected revision result", code)
		}
	}
	if winners != 1 {
		t.Fatal("policy winners", winners)
	}
	if out := contactGet(t, f); out.Policy.Revision != 1 || out.Policy.Threshold != 3 {
		t.Fatal("mixed policy", out)
	}
}
func TestSupplierContactPolicyAuthenticatedRefusalUsesDistinctAuthority(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	offer := tradeOffer(t, b, s)
	var decision supplycycle.TradeResult
	s.p.call("POST", "/local/v1/company-trade/orders/"+offer.Message.ID+"/decision", supplycycle.TradeDecisionInput{OperationID: "contact-auth-decline", Decision: "declined", Reason: "Explicit refusal"}, 200, &decision)
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 200, nil)
	b.p.call("POST", contactPath, contactInput("contact-auth-config", 1), 200, nil)
	var out supplycycle.ContactAlert
	b.p.call("GET", contactPath, nil, 200, &out)
	if !out.Recommended || !out.SourceComplete || out.AuthenticatedDeclinedOrders != 1 || out.ManualDeclinedOrders != 0 {
		t.Fatal("authenticated refusal mislabeled", out)
	}
}

func TestSupplierContactPolicyInvalidScopeAndRevokedSession(t *testing.T) {
	f, _ := cycleFixture(t)
	valid := contactInput("invalid-input", 3)
	for _, change := range []func(*supplycycle.ContactPolicyInput){func(x *supplycycle.ContactPolicyInput) { x.Threshold = 0 }, func(x *supplycycle.ContactPolicyInput) { x.Since = "" }, func(x *supplycycle.ContactPolicyInput) { x.CountMode = "assumed" }, func(x *supplycycle.ContactPolicyInput) { x.ExpectedRevision = -1 }} {
		in := valid
		change(&in)
		cycleCall(t, f, "POST", contactPath, in, 400)
	}
	s := retailServiceFor(f)
	foreign := s.Actor
	foreign.StoreID = "foreign-store"
	if _, e := supplycycle.ConfigureContactPolicy(context.Background(), f.db, s.Contracts, foreign, f.device, f.token, "supplier", valid); !errors.Is(e, localauth.ErrDenied) {
		t.Fatal("foreign context accepted", e)
	}
	if e := localauth.Logout(context.Background(), f.db, f.token); e != nil {
		t.Fatal(e)
	}
	if _, e := supplycycle.ConfigureContactPolicy(context.Background(), f.db, s.Contracts, s.Actor, f.device, f.token, "supplier", valid); !errors.Is(e, localauth.ErrDenied) {
		t.Fatal("revoked session accepted", e)
	}
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM supplier_contact_policies`).Scan(&n); e != nil || n != 0 {
		t.Fatal("denied configuration persisted", n, e)
	}
}
