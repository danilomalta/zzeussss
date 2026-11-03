package localapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
)

func cartReconcile(t *testing.T, f *stockHTTPFixture, want int) sale.CartReconciliation {
	t.Helper()
	b := cartGet(t, f, cartsPath+"/stable-cart/reconcile?cash_session_id=turn-one", want)
	var out sale.CartReconciliation
	if want == 200 {
		if json.Unmarshal(b, &out) != nil {
			t.Fatal(string(b))
		}
		domainSchemaFile(t, "suspended-carts.openapi.json", "CartReconciliation", b)
	}
	return out
}
func TestHTTPCartReconciliationReadyPriceChangeAndNoMutation(t *testing.T) {
	f := salesSetup(t)
	first := cartCall(t, f, cartsPath, cartInput(f), 200)
	before := cartCounts(t, f)
	r := cartReconcile(t, f, 200)
	if r.Status != "ready" || r.CurrentQuote == nil || r.CurrentQuote.TotalCents != 3198 || r.Reserved {
		t.Fatal(r)
	}
	if !equalCartCounts(before, cartCounts(t, f)) {
		t.Fatal("reconciliation changed domain data")
	}
	if _, e := f.db.Exec(`UPDATE products SET price_cents=1600 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); e != nil {
		t.Fatal(e)
	}
	r = cartReconcile(t, f, 200)
	if r.Status != "blocked" || r.CurrentQuote == nil || r.CurrentQuote.TotalCents != 3200 || !hasCartBlocker(r, "saved_prices_or_units_changed") || r.SavedTotalCents != first.Cart.TotalCents {
		t.Fatal(r)
	}
	b := cartGet(t, f, cartsPath+"/stable-cart", 200)
	var current sale.Cart
	if json.Unmarshal(b, &current) != nil || current.Revision != 1 || current.TotalCents != 3198 {
		t.Fatal(string(b))
	}
}
func equalCartCounts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func hasCartBlocker(r sale.CartReconciliation, s string) bool {
	for _, v := range r.Blockers {
		if v == s {
			return true
		}
	}
	return false
}
func TestHTTPCartReconciliationStockClosedShiftAndTerminal(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	stockRequest(t, f, stock.Input{OperationID: "remove-free-stock", Kind: "loss", ProductID: f.product, FromLocationID: f.shelf, QuantityMilli: 4000, Reason: "Movimentacao concorrente"}, 201)
	r := cartReconcile(t, f, 200)
	if !hasCartBlocker(r, "insufficient_stock") || r.CurrentQuote != nil {
		t.Fatal(r)
	}
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	r = cartReconcile(t, f, 200)
	if !hasCartBlocker(r, "cash_closed") {
		t.Fatal(r)
	}
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("cancel", 1), 200)
	r = cartReconcile(t, f, 200)
	if r.Status != "terminal" || r.CurrentQuote != nil {
		t.Fatal(r)
	}
}
func cartReportQuery() string {
	day := time.Now().UTC().Format(time.DateOnly)
	return "?from=" + day + "&to=" + day
}
func TestHTTPCartReportAndCSVCompleteCountsWithoutMoney(t *testing.T) {
	f := salesSetup(t)
	in := cartInput(f)
	cartCall(t, f, cartsPath, in, 200)
	in.CartID = "=unsafe-formula"
	in.OperationID = "save-two"
	cartCall(t, f, cartsPath, in, 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	before := cartCounts(t, f)
	b := cartGet(t, f, cartsPath+"/report"+cartReportQuery(), 200)
	domainSchemaFile(t, "suspended-carts.openapi.json", "CartReport", b)
	var out sale.CartReport
	if json.Unmarshal(b, &out) != nil || !out.SourceComplete || out.FinancialTotalsIncluded || out.Total != 2 || out.Summary["suspended"] != 1 || out.Summary["claimed"] != 1 || bytes.Contains(b, []byte("3198")) {
		t.Fatal(string(b))
	}
	b = cartGet(t, f, cartsPath+"/report.csv"+cartReportQuery(), 200)
	rows, e := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if e != nil || len(rows) != 3 || strings.Contains(string(b), "3198") {
		t.Fatal(e, string(b))
	}
	safe := false
	for _, r := range rows[1:] {
		if r[0] == "'=unsafe-formula" {
			safe = true
		}
	}
	if !safe || !equalCartCounts(before, cartCounts(t, f)) {
		t.Fatal("unsafe CSV or writes")
	}
}
func TestHTTPCartReportRejectsPartialCorruptSourceAndBadRanges(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	for _, q := range []string{"?from=bad&to=bad", "?from=2026-01-01&to=2027-01-02", "?from=2026-02-01&to=2026-01-01", cartReportQuery() + "&tenant_id=other", cartReportQuery() + "&from=2026-01-01"} {
		cartGet(t, f, cartsPath+"/report"+q, 400)
	}
	if _, e := f.db.Exec(`UPDATE pos_carts SET revision=revision+1 WHERE id='stable-cart'`); e != nil {
		t.Fatal(e)
	}
	for _, suffix := range []string{"/report", "/report.csv"} {
		b := cartGet(t, f, cartsPath+suffix+cartReportQuery(), 409)
		if bytes.Contains(b, []byte("source_complete")) || bytes.Contains(b, []byte("cart_id,state")) {
			t.Fatal("partial export", string(b))
		}
	}
}
func TestHTTPCartReportSourceBoundCheckedBeforeExport(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	_, e := f.db.Exec(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<1000) INSERT INTO pos_carts SELECT tenant_id,store_id,device_id,'overflow-'||n.i,actor_id,state,revision,'overflow-sale-'||n.i,payload_json,updated_at FROM pos_carts,n WHERE id='stable-cart'`)
	if e != nil {
		t.Fatal(e)
	}
	for _, suffix := range []string{"/report", "/report.csv"} {
		b := cartGet(t, f, cartsPath+suffix+cartReportQuery(), 422)
		if bytes.Contains(b, []byte("source_complete")) {
			t.Fatal("partial report")
		}
	}
}
func TestHTTPCartNewReadsRemainPrivateToActor(t *testing.T) {
	f := salesSetup(t)
	cartCall(t, f, cartsPath, cartInput(f), 200)
	policyPerson(t, f.db, f.owner, "private-carts", "cashier")
	token := loginToken(t, f.app, "private-carts")
	code, b := request(t, f.app, "GET", cartsPath+"/stable-cart/reconcile?cash_session_id=turn-one", "", token)
	if code != 404 {
		t.Fatal(code, string(b))
	}
	code, b = request(t, f.app, "GET", cartsPath+"/report"+cartReportQuery(), "", token)
	var out sale.CartReport
	if code != 200 || json.Unmarshal(b, &out) != nil || out.Total != 0 {
		t.Fatal(code, string(b))
	}
}
