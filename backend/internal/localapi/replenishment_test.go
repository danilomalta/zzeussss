package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/stock"
)

func restockFixture(t *testing.T) (*httpContractFixture, string) {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory, modules.Orders}), 200)
	status, b := request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 201 {
		t.Fatalf("product %d %s", status, b)
	}
	var v struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	return f, v.ID
}
func restockCall(t *testing.T, f *httpContractFixture, path string, v any, want int) map[string]any {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	status, out := request(t, f.app, "POST", "/local/v1/replenishment/"+path, string(b), f.token)
	if status != want {
		t.Fatalf("%s %d want %d: %s", path, status, want, out)
	}
	var result map[string]any
	_ = json.Unmarshal(out, &result)
	return result
}
func restockPolicyInput(product, op string) map[string]any {
	return map[string]any{"operation_id": op, "product_id": product, "minimum_milli": 2000, "target_milli": 5000}
}
func restockSuggestInput(product, op string) map[string]any {
	return map[string]any{"operation_id": op, "product_id": product}
}
func restockReviewInput(sid, op, decision string) map[string]any {
	return map[string]any{"operation_id": op, "suggestion_id": sid, "decision": decision, "reason": "conferido pelo gerente"}
}
func TestHTTPReplenishmentCompleteFlowAndDurableOperation(t *testing.T) {
	f, p := restockFixture(t)
	policy := restockPolicyInput(p, "policy-http")
	restockCall(t, f, "policies", policy, 200)
	restockCall(t, f, "policies", policy, 200)
	suggest := restockCall(t, f, "suggestions", restockSuggestInput(p, "suggest-http"), 200)
	if suggest["recommended_milli"] != float64(5000) {
		t.Fatal(suggest)
	}
	sid := suggest["suggestion_id"].(string)
	restockCall(t, f, "reviews", restockReviewInput(sid, "review-http", "approved"), 200)
	for _, kind := range []string{"policy", "suggest", "review"} {
		status, body := request(t, f.app, "GET", "/local/v1/replenishment/operations/"+kind+"/"+kind+"-http", "", f.token)
		if status != 200 {
			t.Fatalf("operation %d %s", status, body)
		}
	}
	status, b := request(t, f.app, "GET", "/local/v1/purchase-approvals", "", f.token)
	if status != 200 {
		t.Fatal(status)
	}
	var approvals struct {
		Items []struct {
			ID string `json:"suggestion_id"`
		}
	}
	_ = json.Unmarshal(b, &approvals)
	if len(approvals.Items) != 1 || approvals.Items[0].ID != sid {
		t.Fatal(string(b))
	}
	status, b = request(t, f.app, "POST", "/local/v1/purchase-suppliers", `{"operation_id":"supplier-op","id":"supplier","name":"Padaria","status":"active"}`, f.token)
	if status != 201 {
		t.Fatalf("supplier %d %s", status, b)
	}
	status, b = request(t, f.app, "POST", "/local/v1/purchase-orders", purchaseBody(sid), f.token)
	if status != 201 {
		t.Fatalf("order %d %s", status, b)
	}
	purchaseCounts(t, f, 1, 1, 2)
	status, b = request(t, f.app, "GET", "/local/v1/replenishment/suggestions", "", f.token)
	if status != 200 {
		t.Fatalf("list %d %s", status, b)
	}
	var list struct {
		Items []struct {
			Order  string `json:"order_id"`
			Status string `json:"status"`
		}
	}
	_ = json.Unmarshal(b, &list)
	if len(list.Items) != 1 || list.Items[0].Order != "order" || list.Items[0].Status != "approved" {
		t.Fatal(string(b))
	}
	status, b = request(t, f.app, "GET", "/local/v1/replenishment/products?offset=1", "", f.token)
	if status != 200 || string(b) != "{\"items\":[]}" {
		t.Fatalf("pagination %d %s", status, b)
	}
}
func TestHTTPReplenishmentStaleRejectAndRecalculate(t *testing.T) {
	f, p := restockFixture(t)
	restockCall(t, f, "policies", restockPolicyInput(p, "p1"), 200)
	s := restockCall(t, f, "suggestions", restockSuggestInput(p, "s1"), 200)["suggestion_id"].(string)
	v := restockPolicyInput(p, "p2")
	v["target_milli"] = 6000
	restockCall(t, f, "policies", v, 200)
	restockCall(t, f, "reviews", restockReviewInput(s, "r1", "approved"), 409)
	status, b := request(t, f.app, "GET", "/local/v1/replenishment/suggestions", "", f.token)
	var list struct {
		Items []struct {
			Stale bool `json:"stale"`
		}
	}
	_ = json.Unmarshal(b, &list)
	if status != 200 || len(list.Items) != 1 || !list.Items[0].Stale {
		t.Fatalf("stale %d %s", status, b)
	}
	restockCall(t, f, "reviews", restockReviewInput(s, "r2", "rejected"), 200)
	s2 := restockCall(t, f, "suggestions", restockSuggestInput(p, "s2"), 200)["suggestion_id"].(string)
	restockCall(t, f, "reviews", restockReviewInput(s2, "r3", "approved"), 200)
	s3 := restockCall(t, f, "suggestions", restockSuggestInput(p, "s3"), 200)["suggestion_id"].(string)
	restockCall(t, f, "reviews", restockReviewInput(s3, "r4", "approved"), 409)
}
func TestHTTPReplenishmentRolesContractsAndRevocation(t *testing.T) {
	for _, kind := range []string{"stock", "cashier", "module", "revoked", "device", "verifier", "anonymous"} {
		t.Run(kind, func(t *testing.T) {
			f, p := restockFixture(t)
			restockCall(t, f, "policies", restockPolicyInput(p, "p1"), 200)
			want := 403
			token := f.token
			app := f.app
			switch kind {
			case "stock", "cashier":
				if _, e := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				if _, e := f.db.Exec(`UPDATE memberships SET role=?`, kind); e != nil {
					t.Fatal(e)
				}
			case "module":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
			case "revoked":
				if _, e := f.db.Exec(`UPDATE memberships SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "device":
				if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); e != nil {
					t.Fatal(e)
				}
				want = 401
			case "verifier":
				var e error
				app, e = New(f.db, f.device)
				if e != nil {
					t.Fatal(e)
				}
				want = 503
			case "anonymous":
				token = ""
				want = 401
			}
			b, _ := json.Marshal(restockPolicyInput(p, "p2"))
			status, out := request(t, app, "POST", "/local/v1/replenishment/policies", string(b), token)
			if status != want {
				t.Fatalf("policy %d want %d %s", status, want, out)
			}
			if kind == "stock" {
				restockCall(t, f, "suggestions", restockSuggestInput(p, "stock-s"), 200)
				status, _ = request(t, f.app, "GET", "/local/v1/replenishment/operations/policy/p1", "", f.token)
				if status != 403 {
					t.Fatal(status)
				}
			}
		})
	}
}
func TestHTTPReplenishmentRollbackIncludesIgnoredWrites(t *testing.T) {
	for _, table := range []string{"restock_policies", "restock_policy_changes", "outbox"} {
		for _, failure := range []string{"IGNORE", "ABORT,'test'"} {
			t.Run(table+failure, func(t *testing.T) {
				f, p := restockFixture(t)
				if _, e := f.db.Exec(fmt.Sprintf(`CREATE TRIGGER fail BEFORE INSERT ON %s BEGIN SELECT RAISE(%s); END`, table, failure)); e != nil {
					t.Fatal(e)
				}
				b, _ := json.Marshal(restockPolicyInput(p, "p"))
				status, _ := request(t, f.app, "POST", "/local/v1/replenishment/policies", string(b), f.token)
				if status == 200 {
					t.Fatal("false success")
				}
				for _, name := range []string{"restock_policies", "restock_policy_changes", "outbox"} {
					var n int
					if e := f.db.QueryRow("SELECT COUNT(*) FROM " + name).Scan(&n); e != nil || n != 0 {
						t.Fatalf("partial %s %d %v", name, n, e)
					}
				}
			})
		}
	}
	for _, table := range []string{"restock_suggestions", "restock_reviews"} {
		t.Run(table, func(t *testing.T) {
			f, p := restockFixture(t)
			restockCall(t, f, "policies", restockPolicyInput(p, "p"), 200)
			sid := ""
			if table == "restock_reviews" {
				sid = restockCall(t, f, "suggestions", restockSuggestInput(p, "s"), 200)["suggestion_id"].(string)
			}
			if _, e := f.db.Exec(fmt.Sprintf(`CREATE TRIGGER fail BEFORE INSERT ON %s BEGIN SELECT RAISE(IGNORE); END`, table)); e != nil {
				t.Fatal(e)
			}
			path := "suggestions"
			in := restockSuggestInput(p, "s2")
			if sid != "" {
				path = "reviews"
				in = restockReviewInput(sid, "r", "approved")
			}
			restockCall(t, f, path, in, 409)
			if sid != "" {
				var status string
				if e := f.db.QueryRow(`SELECT status FROM restock_suggestions WHERE id=?`, sid).Scan(&status); e != nil || status != "suggested" {
					t.Fatal(status, e)
				}
			}
		})
	}
}
func TestHTTPReplenishmentConcurrentReviewAndConflict(t *testing.T) {
	f, p := restockFixture(t)
	restockCall(t, f, "policies", restockPolicyInput(p, "p"), 200)
	sid := restockCall(t, f, "suggestions", restockSuggestInput(p, "s"), 200)["suggestion_id"].(string)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, d := range []string{"approved", "rejected"} {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			b, _ := json.Marshal(restockReviewInput(sid, "r-"+d, d))
			status, _ := request(t, f.app, "POST", "/local/v1/replenishment/reviews", string(b), f.token)
			statuses <- status
		}(d)
	}
	wg.Wait()
	close(statuses)
	success := 0
	for s := range statuses {
		if s == 200 {
			success++
		} else if s != 409 {
			t.Fatal(s)
		}
	}
	if success != 1 {
		t.Fatal(success)
	}
	v := restockPolicyInput(p, "p")
	v["target_milli"] = 7000
	restockCall(t, f, "policies", v, 409)
}
func TestHTTPReplenishmentRejectsUnsafeAndForeignInputs(t *testing.T) {
	f, p := restockFixture(t)
	for _, body := range []string{fmt.Sprintf(`{"operation_id":"p","product_id":%q,"minimum_milli":1000,"target_milli":2000,"tenant_id":"forged"}`, p), fmt.Sprintf(`{"operation_id":"p","product_id":%q,"minimum_milli":1,"target_milli":2000}`, p), fmt.Sprintf(`{"operation_id":"p","product_id":%q,"minimum_milli":null,"target_milli":2000}`, p), fmt.Sprintf(`{"operation_id":"p","operation_id":"other","product_id":%q,"minimum_milli":0,"target_milli":2000}`, p), fmt.Sprintf(`{"operation_id":"p","product_id":%q,"minimum_milli":0,"target_milli":9007199254740992}`, p), `{"operation_id":"p","product_id":"foreign","minimum_milli":0,"target_milli":2000}`} {
		status, b := request(t, f.app, "POST", "/local/v1/replenishment/policies", body, f.token)
		if status != 400 {
			t.Fatalf("unsafe %d %s", status, b)
		}
	}
	status, _ := request(t, f.app, "GET", "/local/v1/replenishment/operations/policy/missing", "", f.token)
	if status != 404 {
		t.Fatal(status)
	}
}
func TestHTTPReplenishmentStockChangeMakesApprovalStale(t *testing.T) {
	f := stockHTTPSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.Orders}), 200)
	restockCall(t, f.httpContractFixture, "policies", restockPolicyInput(f.product, "p"), 200)
	sid := restockCall(t, f.httpContractFixture, "suggestions", restockSuggestInput(f.product, "s"), 200)["suggestion_id"].(string)
	stockRequest(t, f, stock.Input{OperationID: "entry", Kind: "entry", ProductID: f.product, ToLocationID: f.back, QuantityMilli: 1000, Reason: "entrada"}, 201)
	restockCall(t, f.httpContractFixture, "reviews", restockReviewInput(sid, "r", "approved"), 409)
	status, b := request(t, f.app, "GET", "/local/v1/replenishment/products", "", f.token)
	var list struct {
		Items []struct {
			Balance int64 `json:"balance_milli"`
		}
	}
	_ = json.Unmarshal(b, &list)
	if status != 200 || len(list.Items) != 1 || list.Items[0].Balance != 1000 {
		t.Fatalf("balance %d %s", status, b)
	}
}
func TestHTTPReplenishmentExpiryPreservesReadsAndClockRollsBackOnFailure(t *testing.T) {
	f, p := restockFixture(t)
	restockCall(t, f, "policies", restockPolicyInput(p, "p"), 200)
	var before int64
	if e := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&before); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=?`, before-10); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`CREATE TRIGGER fail BEFORE INSERT ON restock_policy_changes BEGIN SELECT RAISE(ABORT,'test'); END`); e != nil {
		t.Fatal(e)
	}
	restockCall(t, f, "policies", restockPolicyInput(p, "p-fail"), 500)
	var after int64
	if e := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&after); e != nil || after != before-10 {
		t.Fatalf("clock %d %v", after, e)
	}
	now := time.Now().Unix()
	payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Orders}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))); e != nil {
		t.Fatal(e)
	}
	restockCall(t, f, "policies", restockPolicyInput(p, "p"), 403)
	for _, path := range []string{"products", "suggestions", "operations/policy/p"} {
		status, b := request(t, f.app, "GET", "/local/v1/replenishment/"+path, "", f.token)
		if status != 200 {
			t.Fatalf("read %s %d %s", path, status, b)
		}
	}
}
func TestHTTPReplenishmentOperationSurvivesReopen(t *testing.T) {
	f, p := restockFixture(t)
	restockCall(t, f, "policies", restockPolicyInput(p, "p"), 200)
	restockCall(t, f, "suggestions", restockSuggestInput(p, "s"), 200)
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
	defer db.Close()
	verifier, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	app, e := NewWithVerifier(db, f.device, verifier)
	if e != nil {
		t.Fatal(e)
	}
	for _, op := range []string{"policy/p", "suggest/s"} {
		status, b := request(t, app, "GET", "/local/v1/replenishment/operations/"+op, "", f.token)
		if status != 200 {
			t.Fatalf("reopen %d %s", status, b)
		}
	}
}
