package localapi

import (
	"encoding/json"
	"testing"
)

func TestHTTPCustomerReturnLaterShiftRequiresOwnExplicitDrawer(t *testing.T) {
	f := cancellationSetup(t)
	ownerToken := f.token
	in := customerReturnInput(t, f, "manager-refund66", 1000, 1599, "restock")
	in.RefundCashSessionID = "turn-two"
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-original","declared_cents":4198}`, 200)
	policyPerson(t, f.db, f.owner, "refund-manager66", "manager")
	managerToken := loginToken(t, f.app, "refund-manager66")
	f.token = managerToken
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"turn-two","opening_cents":2000}`, 201)
	f.token = ownerToken
	returnShiftRequest(t, f, in, 409)
	assertCustomerReturnCount(t, f, 0)
	// Management permission does not let the owner silently debit another
	// operator's open drawer. The drawer's actual manager authorizes the refund.
	f.token = managerToken
	r := returnShiftRequest(t, f, in, 201)
	if r.ActorID != "refund-manager66" || r.OriginalCashSessionID != "turn-one" {
		t.Fatal(r)
	}
	f.token = ownerToken
	returnShiftRequest(t, f, in, 409)
	assertCustomerReturnCount(t, f, 1)
	f.token = managerToken
	again := returnShiftRequest(t, f, in, 200)
	if !again.Repeated || again.ID != r.ID {
		t.Fatal(again)
	}
}

func TestHTTPCustomerReturnLaterShiftRechecksAuthorizationAfterQuote(t *testing.T) {
	f := returnShiftSetup(t, 2000)
	in := customerReturnInput(t, f, "revoked-after-quote66", 1000, 1599, "restock")
	in.RefundCashSessionID = "turn-two"
	raw, _ := json.Marshal(map[string]any{"items": in.Items, "refund_cash_session_id": in.RefundCashSessionID})
	status, b := request(t, f.app, "POST", returnShiftPath+"/quote", string(raw), f.token)
	if status != 200 {
		t.Fatalf("quote %d %s", status, b)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id=?`, f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	returnShiftRequest(t, f, in, 401)
	assertCustomerReturnCount(t, f, 0)
}

func TestHTTPCustomerReturnLaterShiftRequiresOfficialOriginalClosure(t *testing.T) {
	f := returnShiftSetup(t, 2000)
	in := customerReturnInput(t, f, "unproven-close66", 1000, 1599, "restock")
	in.RefundCashSessionID = "turn-two"
	// Corrupt only a disposable fixture to model incomplete legacy closure.
	if _, e := f.db.Exec(`DELETE FROM cash_closures WHERE session_id='turn-one'`); e != nil {
		t.Fatal(e)
	}
	returnShiftRequest(t, f, in, 409)
	assertCustomerReturnCount(t, f, 0)
}
