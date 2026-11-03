package localapi

import (
	"testing"
)

func TestHTTPAccountingKeepsCashReversalSeparateFromUnconfirmedCheckoutEvidence(t *testing.T) {
	f, manager := proofSetup(t)
	proofResult(t, f, f.token, "POST", submitProofPath, submitProofBody, 201)
	proofResult(t, f, manager, "POST", proofsPath+"/proof-one/accept", reviewProofBody, 200)
	// A separate, actual all-cash sale is cancelled through the existing API.
	// The mixed checkout allocation and its documentary evidence stay pending.
	in := salesInput(f)
	in.Payments[0].AmountCents = 31500
	salesRequest(t, f, in, 201)
	cancelRequest(t, f, cancelBody, 200)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-reversed","declared_cents":1000}`, 200)
	setAccountingClosedAt(t, f, "turn-one", closedFirst)
	token := periodAccountant(t, f)
	r := accountingReport(t, f, token, accountingQuery(closedFirst, periodEnd), 200)
	if r.Summary["gross_sales_cents"] != 31500 || r.Summary["cancelled_sales_cents"] != 31500 || r.Summary["net_sales_cents"] != 0 || r.Summary["sum_expected_at_closures_cents"] != 1000 || r.Summary["journal_entry_count"] != 3 || r.Payments[0].ReversedCents != 31500 || r.Payments[0].ConfirmedCents != 0 || r.Payments[1].ConfirmedCents != 0 || r.Payments[2].ConfirmedCents != 0 {
		t.Fatal(r)
	}
	if p := planRequest(t, f, "GET", planPath+"/stable-cart", "", 200); p.CanFinalize || p.State != "prepared" {
		t.Fatal(p)
	}
	if p := readProof(t, f, f.token, "proof-one", 200); p.ProviderConfirmed || p.State != "accepted" {
		t.Fatal(p)
	}
}
