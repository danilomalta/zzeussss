package localapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/register"
)

func movementReceipt(t *testing.T, f *httpContractFixture, token, id string, want int) register.MovementResult {
	t.Helper()
	s, b := request(t, f.app, "GET", "/local/v1/cash/movements/operations/"+url.PathEscape(id), "", token)
	if s != want {
		t.Fatalf("movement receipt %d want%d %s", s, want, b)
	}
	var r register.MovementResult
	if want == 200 {
		domainSchemaFile(t, "cash-movement-receipt.openapi.json", "MovementReceipt", b)
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e)
		}
	}
	return r
}
func TestHTTPMovementReceiptBeforeAndAfterLinkCloseAndRestart(t *testing.T) {
	f, reviewer, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	original := movementReceipt(t, f, f.token, "withdraw-one", 200)
	if original.Repeated || original.AmountCents != 6000 || original.OperationID != "withdraw-one" || original.Kind != "withdrawal" {
		t.Fatal(original)
	}
	movementReceipt(t, f, reviewer, "withdraw-one", 404)
	movementReceipt(t, f, f.token, "absent", 404)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	if got := movementReceipt(t, f, f.token, "withdraw-one", 200); got != original {
		t.Fatal("link changed original movement receipt")
	}
	cashRequest(t, f, "close", `{"session_id":"turn-one","operation_id":"close-source","declared_cents":44000}`, 200)
	flowReopen(t, f)
	if got := movementReceipt(t, f, f.token, "withdraw-one", 200); got != original {
		t.Fatal("close/restart changed source receipt")
	}
	counts := pcCounts(t, f)
	movementReceipt(t, f, f.token, "withdraw-one", 200)
	if !reflect.DeepEqual(counts, pcCounts(t, f)) {
		t.Fatal("GET moved money")
	}
}
func TestHTTPMovementReceiptStrictInputScopeAndDamagedAudit(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	for _, suffix := range []string{"?tenant_id=other", "?limit=1", "?id=other"} {
		if s, _ := request(t, f.app, "GET", "/local/v1/cash/movements/operations/withdraw-one"+suffix, "", f.token); s != 400 {
			t.Fatal(s)
		}
	}
	if s, _ := request(t, f.app, "GET", "/local/v1/cash/movements/operations/withdraw-one", `{}`, f.token); s != 400 {
		t.Fatal(s)
	}
	req := httptest.NewRequest("GET", "/local/v1/cash/movements/operations/withdraw-one", nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	r, e := f.app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if r.StatusCode != 200 || r.Header.Get("Cache-Control") != "no-store" {
		t.Fatal(r.StatusCode)
	}
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	d := f.device
	d.DeviceID = "other"
	if _, e := payables.AuditedMovementReceipt(context.Background(), f.db, a, d, "withdraw-one"); e == nil {
		t.Fatal("foreign device authorized")
	}
	if _, e := f.db.Exec(`DROP TRIGGER cash_journal_no_update; UPDATE cash_journal_entries SET cash_delta_cents=-1 WHERE operation_id='withdraw-one'`); e != nil {
		t.Fatal(e)
	}
	movementReceipt(t, f, f.token, "withdraw-one", 409)
}
func TestHTTPMovementReceiptLegacyEscapedIdentifiers(t *testing.T) {
	f, _, _ := pcSetup(t)
	for _, id := range []string{"movimento-ç", "source/part", "percent%20literal"} {
		pcMove(t, f, f.token, id, "supply", 1)
		if r := movementReceipt(t, f, f.token, id, 200); r.OperationID != id {
			t.Fatal(r)
		}
	}
}
