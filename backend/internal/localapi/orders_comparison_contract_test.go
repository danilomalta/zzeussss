package localapi

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOrdersComparisonOpenAPIApprovedPurchaseAndSiteSnapshot(t *testing.T) {
	f, product := restockFixture(t)
	schema := func(name string, body []byte) {
		t.Helper()
		domainSchemaFile(t, "orders-replenishment-comparison.openapi.json", name, body)
	}
	call := func(method, path, input, output string, payload any, want int) []byte {
		t.Helper()
		var body []byte
		if payload != nil {
			var e error
			body, e = json.Marshal(payload)
			if e != nil {
				t.Fatal(e)
			}
			schema(input, body)
		}
		status, result := request(t, f.app, method, "/local/v1/"+path, string(body), f.token)
		if status != want {
			t.Fatalf("%s %s status=%d want=%d", method, path, status, want)
		}
		schema(output, result)
		return result
	}
	reads := []struct{ path, name string }{
		{"purchase-suppliers", "PurchaseSupplierPage"}, {"purchase-approvals", "PurchaseApprovalPage"},
		{"purchase-orders", "PurchaseOrderPage"}, {"replenishment/products", "RestockProductPage"},
		{"replenishment/suggestions", "RestockSuggestionPage"}, {"comparison-sites", "ComparisonPage"},
	}
	for _, r := range reads {
		call("GET", r.path, "", r.name, nil, 200)
	}
	policy := restockPolicyInput(product, "contract-policy")
	for i := 0; i < 2; i++ {
		call("POST", "replenishment/policies", "RestockPolicyInput", "RestockPolicyResult", policy, 200)
	}
	suggest := restockSuggestInput(product, "contract-suggest")
	b := call("POST", "replenishment/suggestions", "RestockSuggestInput", "RestockSuggestResult", suggest, 200)
	var suggestion struct {
		ID string `json:"suggestion_id"`
	}
	if json.Unmarshal(b, &suggestion) != nil || suggestion.ID == "" {
		t.Fatal("missing suggestion")
	}
	call("POST", "replenishment/suggestions", "RestockSuggestInput", "RestockSuggestResult", suggest, 200)
	review := restockReviewInput(suggestion.ID, "contract-review", "approved")
	for i := 0; i < 2; i++ {
		call("POST", "replenishment/reviews", "RestockReviewInput", "RestockReviewResult", review, 200)
	}
	for _, kind := range []string{"policy", "suggest", "review"} {
		b = call("GET", "replenishment/operations/"+kind+"/contract-"+kind, "", "RestockOperation", nil, 200)
		if !bytes.Contains(b, []byte(`"repeated":true`)) {
			t.Fatal("operation lost replay flag")
		}
	}
	supplier := map[string]any{"operation_id": "supplier-op", "id": "supplier", "name": "Padaria", "status": "active"}
	call("POST", "purchase-suppliers", "PurchaseSupplierInput", "PurchaseResult", supplier, 201)
	call("POST", "purchase-suppliers", "PurchaseSupplierInput", "PurchaseResult", supplier, 200)
	call("GET", "purchase-approvals", "", "PurchaseApprovalPage", nil, 200)
	var order map[string]any
	if json.Unmarshal([]byte(purchaseBody(suggestion.ID)), &order) != nil {
		t.Fatal("invalid fixture")
	}
	call("POST", "purchase-orders", "PurchaseInput", "PurchaseResult", order, 201)
	call("POST", "purchase-orders", "PurchaseInput", "PurchaseResult", order, 200)
	b = call("GET", "purchase-orders/order", "", "PurchaseOrder", nil, 200)
	var detail struct {
		Status string `json:"status"`
		Items  []struct {
			Quantity int64 `json:"quantity_milli"`
		} `json:"items"`
	}
	if json.Unmarshal(b, &detail) != nil || detail.Status != "local_not_sent" || len(detail.Items) != 1 || detail.Items[0].Quantity != 5000 {
		t.Fatal("approved snapshot differs")
	}
	b = call("GET", "purchase-orders", "", "PurchaseOrderPage", nil, 200)
	var page struct {
		Items []struct {
			Items []any `json:"items"`
		} `json:"items"`
	}
	if json.Unmarshal(b, &page) != nil || len(page.Items) != 1 || len(page.Items[0].Items) != 0 {
		t.Fatal("order list invented detail lines")
	}
	site := comparisonInput()
	call("POST", "comparison-sites", "ComparisonInput", "ComparisonResult", site, 201)
	call("POST", "comparison-sites", "ComparisonInput", "ComparisonResult", site, 200)
	site.OperationID = "site-update"
	site.ExpectedRevision = 1
	site.Name = "Nome atualizado"
	call("POST", "comparison-sites", "ComparisonInput", "ComparisonResult", site, 200)
	b = call("GET", "comparison-site-operations/site-op", "", "ComparisonResult", nil, 200)
	var receipt struct {
		Site struct {
			Revision int64  `json:"revision"`
			Name     string `json:"name"`
		} `json:"site"`
	}
	if json.Unmarshal(b, &receipt) != nil || receipt.Site.Revision != 1 || receipt.Site.Name != "Netshoes" {
		t.Fatal("original operation snapshot rewritten")
	}
	for _, r := range reads {
		call("GET", r.path, "", r.name, nil, 200)
	}
	for _, r := range reads[:5] {
		call("GET", r.path+"?offset=100", "", r.name, nil, 200)
	}
	purchaseCounts(t, f, 1, 1, 2)
}
