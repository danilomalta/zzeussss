package localapi

import (
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/sale"
)

func TestHTTPCheckoutProofExactMaximumAndAggregateOverflowUseRealPlanSources(t *testing.T) {
	f, manager := proofSetup(t)
	if _, err := f.db.Exec(`UPDATE products SET price_cents=9007199254740991 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	in := checkoutInput(f)
	in.OperationID = "prepare-max"
	in.CheckoutID = "max-cart"
	in.ExpectedTotalCents = 9007199254740991
	in.Items[0].QuantityMilli = 1000
	in.Parts = []sale.PlanPartInput{{PartID: "max-debit", Method: "debit", AmountCents: 9007199254740991}}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	proofResult(t, f, f.token, "POST", planPath+"/max-cart/parts/max-debit/proofs", submitProofBody, 201)
	if r := proofReport(t, f, manager, 200); r.ReportedActiveCents != 9007199254740991 || r.Items[0].AmountCents != 9007199254740991 {
		t.Fatal(r)
	}
	if _, err := f.db.Exec(`UPDATE products SET price_cents=1 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	in.OperationID = "prepare-small"
	in.CheckoutID = "small-cart"
	in.ExpectedTotalCents = 1
	in.Parts = []sale.PlanPartInput{{PartID: "small-debit", Method: "debit", AmountCents: 1}}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	body := strings.NewReplacer("submit-proof-one", "submit-small", "proof-one", "proof-small", "reference-one", "reference-small").Replace(submitProofBody)
	proofResult(t, f, f.token, "POST", planPath+"/small-cart/parts/small-debit/proofs", body, 201)
	proofReport(t, f, manager, 422)
	if s, b := request(t, f.app, "GET", proofsPath+"/report.csv", "", manager); s != 422 || strings.Contains(string(b), "format_version") {
		t.Fatal(s, string(b))
	}
}
func TestHTTPCheckoutProofAndAllocationCancellationCannotBothCommit(t *testing.T) {
	f, _ := proofSetup(t)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s, _ := request(t, f.app, "POST", submitProofPath, submitProofBody, f.token)
		statuses <- s
	}()
	go func() {
		defer wg.Done()
		s, _ := request(t, f.app, "POST", planPath+"/stable-cart/cancel", `{"operation_id":"cancel-racing-proof","reason":"Intencao sem evidencia externa"}`, f.token)
		statuses <- s
	}()
	wg.Wait()
	close(statuses)
	ok, conflict := 0, 0
	for s := range statuses {
		if s == 200 || s == 201 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	p := planRequest(t, f, "GET", planPath+"/stable-cart", "", 200)
	n := proofCounts(t, f)
	if p.State == "cancelled" {
		if n[0] != 0 || n[1] != 0 || n[2] != 0 {
			t.Fatal("proof attached after cancellation", n)
		}
	} else if p.State == "prepared" {
		if n[0] != 1 || n[1] != 1 || n[2] != 1 || n[9] != 0 {
			t.Fatal("proof and cancel committed", n)
		}
	} else {
		t.Fatal(p)
	}
	if n[3] != 0 || n[4] != 0 {
		t.Fatal("race created sale", n)
	}
}
func TestHTTPCheckoutProofCashierCanRecoverOwnDocumentButCannotReviewOrReadQueue(t *testing.T) {
	f, manager := proofSetup(t)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	policyPerson(t, f.db, f.owner, "cashier-proof", "cashier")
	token := loginToken(t, f.app, "cashier-proof")
	if s, b := request(t, f.app, "POST", "/local/v1/cash/open", `{"session_id":"cashier-turn","opening_cents":0}`, token); s != 201 {
		t.Fatal(s, string(b))
	}
	in := checkoutInput(f)
	in.OperationID = "cashier-prepare"
	in.CheckoutID = "cashier-cart"
	in.CashSessionID = "cashier-turn"
	if s, b := request(t, f.app, "POST", planPath, planJSON(t, in), token); s != 201 {
		t.Fatal(s, string(b))
	}
	path := planPath + "/cashier-cart/parts/debit-1/proofs"
	proofResult(t, f, token, "POST", path, submitProofBody, 201)
	if p := readProof(t, f, token, "proof-one", 200); p.SubmitterID != "cashier-proof" || p.ProviderConfirmed {
		t.Fatal(p)
	}
	proofResult(t, f, token, "GET", proofsPath+"/operations/submit-proof-one", "", 200)
	proofResult(t, f, token, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 403)
	if s, _ := request(t, f.app, "GET", proofsPath, "", token); s != 403 {
		t.Fatal(s)
	}
	proofResult(t, f, manager, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 200)
	readProof(t, f, token, "proof-one", 200)
}
