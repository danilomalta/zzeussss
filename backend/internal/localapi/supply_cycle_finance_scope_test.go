package localapi

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

func financeOnlyToken(t *testing.T, f *httpContractFixture, permissions ...string) string {
	t.Helper()
	const person = "finance-only-qa"
	policyPerson(t, f.db, f.owner, person, "employee")
	postPolicy(t, f.app, f.token, policyInput("dept-finance-qa", "department", "finance-qa", "", "", "active", 0), 200)
	postPolicy(t, f.app, f.token, policyInput("member-finance-qa", "member", "finance-qa", person, "", "assigned", 0), 200)
	for _, p := range permissions {
		postPolicy(t, f.app, f.token, policyInput("grant-finance-"+p, "rule", "finance-qa", person, p, "allow", 0), 200)
	}
	return loginToken(t, f.app, person)
}
func financeOnlyCall(t *testing.T, f *httpContractFixture, token string, want int) []byte {
	t.Helper()
	raw, e := json.Marshal(cycleFinanceInput())
	if e != nil {
		t.Fatal(e)
	}
	code, body := request(t, f.app, "POST", cycleDelivery+"/payable", string(raw), token)
	if code != want {
		t.Fatalf("finance-only request: got %d want %d: %s", code, want, body)
	}
	return body
}
func TestSupplyCycleFinanceOnlyCreatesPendingWithoutStockAuthority(t *testing.T) {
	f, _ := cycleFixture(t)
	received := cycleReceived(t, f)
	token := financeOnlyToken(t, f, "view_accounting", "manage_cash")
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "finance-only-qa"}
	for _, p := range []identity.Permission{identity.ManageStock, identity.ManageReplenishment, identity.ViewOrders} {
		if e := identity.Can(context.Background(), f.db, actor, p); e == nil {
			t.Fatalf("unexpected grant %s", p)
		}
	}
	before := cycleCounts(t, f)
	raw := financeOnlyCall(t, f, token, 200)
	var result map[string]json.RawMessage
	if e := json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e)
	}
	var payable struct {
		State     string `json:"state"`
		Creator   string `json:"creator_id"`
		Execution bool   `json:"payment_execution_available"`
	}
	if e := json.Unmarshal(result["payable"], &payable); e != nil {
		t.Fatal(e)
	}
	if payable.State != "pending" || payable.Creator != "finance-only-qa" || payable.Execution {
		t.Fatal(string(raw))
	}
	for _, table := range []string{"purchase_receipts", "stock_operations", "stock_movements", "cash_movements"} {
		var n int64
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		index := map[string]int{"purchase_receipts": 8, "stock_operations": 9, "stock_movements": 10, "cash_movements": 11}[table]
		if n != before[index] {
			t.Fatalf("finance changed %s", table)
		}
	}
	code, origin := request(t, f.app, "GET", "/local/v1/finance/receiving-origins/invoice-one", "", token)
	if code != 200 {
		t.Fatal(code, string(origin))
	}
	var o struct {
		ReceiptID string `json:"receipt_id"`
		Review    bool   `json:"review_required"`
	}
	if e := json.Unmarshal(origin, &o); e != nil {
		t.Fatal(e)
	}
	if o.ReceiptID != received.Receipt.ID || o.Review {
		t.Fatal(string(origin))
	}
	after := cycleCounts(t, f)
	financeOnlyCall(t, f, token, 200)
	if !reflect.DeepEqual(after, cycleCounts(t, f)) {
		t.Fatal("retry duplicated effects")
	}
	code, _ = request(t, f.app, "GET", "/local/v1/purchase-cycle-operations/finance-one", "", f.token)
	if code != 404 {
		t.Fatalf("different author recovered operation: %d", code)
	}
}
func TestSupplyCycleFinanceOnlyMissingPermissionMakesNoWrites(t *testing.T) {
	for _, permission := range []string{"view_accounting", "manage_cash"} {
		t.Run(permission, func(t *testing.T) {
			f, _ := cycleFixture(t)
			cycleReceived(t, f)
			token := financeOnlyToken(t, f, permission)
			before := cycleCounts(t, f)
			financeOnlyCall(t, f, token, 403)
			if !reflect.DeepEqual(before, cycleCounts(t, f)) {
				t.Fatal("unauthorized write")
			}
		})
	}
}
func TestSupplyCycleFinanceOnlyRejectsVoidedOriginAtomically(t *testing.T) {
	f, _ := cycleFixture(t)
	r := cycleReceived(t, f)
	token := financeOnlyToken(t, f, "view_accounting", "manage_cash")
	raw := `{"operation_id":"void-finance-qa","reason":"QA receiving reversal"}`
	code, body := request(t, f.app, "POST", cycleOrder+"/receipts/"+r.Receipt.ID+"/void", raw, f.token)
	if code != 200 {
		t.Fatal(code, string(body))
	}
	before := cycleCounts(t, f)
	financeOnlyCall(t, f, token, 409)
	if !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("voided source created obligation")
	}
}

func TestSupplyCycleFinanceEvidenceRetainsTenantStoreAndDeviceIsolation(t *testing.T) {
	f, _ := cycleFixture(t)
	r := cycleReceived(t, f)
	financeOnlyToken(t, f, "view_accounting", "manage_cash")
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "finance-only-qa"}
	tx, e := f.db.BeginTx(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	evidence, e := purchases.ReceiptForPayableTx(context.Background(), tx, actor, f.device, r.Receipt.ID)
	if e != nil || evidence.ID != r.Receipt.ID {
		t.Fatal("authorized financial evidence", e)
	}
	for _, field := range []string{"tenant", "store", "identity", "device"} {
		a, d := actor, f.device
		switch field {
		case "tenant":
			a.TenantID = "other-tenant"
		case "store":
			a.StoreID = "other-store"
		case "identity":
			a.IdentityID = "other-identity"
		case "device":
			d.DeviceID = "other-device"
		}
		if _, e := purchases.ReceiptForPayableTx(context.Background(), tx, a, d, r.Receipt.ID); e == nil {
			t.Fatalf("foreign %s read evidence", field)
		}
	}
}
func TestSupplyCycleZeroAcceptanceContractAndAPI(t *testing.T) {
	f, _ := cycleFixture(t)
	cycleAtArrival(t, f)
	in := cycleReceiveInput()
	in.AcceptedMilli = 0
	before := cycleCounts(t, f)
	cycleCall(t, f, "POST", cycleDelivery+"/receive", in, 400)
	if !reflect.DeepEqual(before, cycleCounts(t, f)) {
		t.Fatal("zero acceptance created effect")
	}
	raw, e := os.ReadFile("../../../docs/api/purchase-supply-cycle.openapi.json")
	if e != nil {
		t.Fatal(e)
	}
	var spec struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Minimum int `json:"minimum"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if e = json.Unmarshal(raw, &spec); e != nil {
		t.Fatal(e)
	}
	if spec.Components.Schemas["ReceiveInput"].Properties["accepted_milli"].Minimum != 1 {
		t.Fatal("contract falsely permits complete refusal through receiving")
	}
}
