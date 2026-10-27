package localapi

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/sale"
)

func quoteBody(t *testing.T, f *stockHTTPFixture) string {
	t.Helper()
	in := salesInput(f)
	data, err := json.Marshal(sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readQuote(t *testing.T, f *stockHTTPFixture, body string, want int) sale.Quote {
	t.Helper()
	status, response := request(t, f.app, "POST", "/local/v1/sales/quote", body, f.token)
	if status != want {
		t.Fatalf("quote got=%d want=%d body=%s", status, want, response)
	}
	var quote sale.Quote
	if want == 200 {
		if err := json.Unmarshal(response, &quote); err != nil {
			t.Fatal(err)
		}
	}
	return quote
}

func TestHTTPQuoteAuthoritativeExactAndNoCommercialWrites(t *testing.T) {
	f := salesSetup(t)
	before := isolationCounts(t, f.db)
	body := quoteBody(t, f)
	for i := 0; i < 2; i++ {
		q := readQuote(t, f, body, 200)
		if q.TotalCents != 3198 || q.CashSessionID != "turn-one" || len(q.Items) != 1 || q.Items[0].AvailableMilli != 5000 || q.Items[0].UnitPriceCents != 1599 || q.Reserved || q.DiscountsSupported || q.LoyaltySupported || len(q.PaymentMethods) != 1 || q.PaymentMethods[0] != "cash" {
			t.Fatalf("quote: %+v", q)
		}
	}
	after := isolationCounts(t, f.db)
	for i, n := range before {
		if after[i] != n {
			t.Fatalf("quote created business rows: %v -> %v", before, after)
		}
	}
	salesCounts(t, f, 0)
	assertStockBalance(t, f, f.shelf, 5000)
	// Fractional weights use the SAME rounding code as the final sale.
	if _, err := f.db.Exec(`UPDATE products SET unit='kg',price_cents=299 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	body = strings.Replace(body, `"quantity_milli":2000`, `"quantity_milli":1500`, 1)
	q := readQuote(t, f, body, 200)
	if q.TotalCents != 449 {
		t.Fatalf("rounding: %+v", q)
	}
	in := salesInput(f)
	in.Items[0].QuantityMilli = 1500
	in.Payments[0].AmountCents = q.TotalCents
	salesRequest(t, f, in, 201)
	if readReceipt(t, f, in.SaleID).TotalCents != q.TotalCents {
		t.Fatal("quote and sale differ")
	}
}

func TestHTTPQuoteDoesNotReserveOrLockPrice(t *testing.T) {
	f := salesSetup(t)
	body := quoteBody(t, f)
	readQuote(t, f, body, 200)
	if _, err := f.db.Exec(`UPDATE products SET price_cents=1600 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	in := salesInput(f)
	salesRequest(t, f, in, 400) // stale amount cannot confirm at the previous price
	salesCounts(t, f, 0)
	q := readQuote(t, f, body, 200)
	if q.TotalCents != 3200 {
		t.Fatal(q)
	}
	in.Payments[0].AmountCents = q.TotalCents
	salesRequest(t, f, in, 201)
	salesRequest(t, f, in, 200)
	salesCounts(t, f, 1)
	// A subsequent loss/other sale is visible on the next quote; no reservation.
	body = strings.Replace(body, `"quantity_milli":2000`, `"quantity_milli":4000`, 1)
	readQuote(t, f, body, 409)
	assertStockBalance(t, f, f.shelf, 3000)
}

func TestHTTPQuoteRejectsAmbiguousInputAndForgedApproval(t *testing.T) {
	f := salesSetup(t)
	body := quoteBody(t, f)
	for _, invalid := range []string{
		`null`, `{}`, body + ` {}`,
		strings.Replace(body, `"cash_session_id":"turn-one"`, `"cash_session_id":"turn-one","cash_session_id":"other"`, 1),
		strings.Replace(body, `"quantity_milli":2000`, `"quantity_milli":2000,"quantity_milli":1000`, 1),
		strings.Replace(body, `"quantity_milli":2000`, `"quantity_milli":null`, 1),
		strings.Replace(body, `"quantity_milli":2000`, `"quantity_milli":1.5`, 1),
		strings.Replace(body, `"quantity_milli":2000`, `"quantity_milli":2000,"price_cents":1`, 1),
		strings.Replace(body, `"items":`, `"state":"approved","items":`, 1),
		strings.Replace(body, `"items":`, `"tenant_id":"foreign","items":`, 1),
		strings.Replace(body, `"items":`, `"discount_cents":1,"items":`, 1),
	} {
		readQuote(t, f, invalid, 400)
	}
	salesCounts(t, f, 0)
}

func TestHTTPQuoteCumulativeStockAndInvalidLocation(t *testing.T) {
	f := salesSetup(t)
	in := salesInput(f)
	in.Items = []sale.Item{{ProductID: f.product, LocationID: f.shelf, QuantityMilli: 3000}, {ProductID: f.product, LocationID: f.shelf, QuantityMilli: 3000}}
	data, _ := json.Marshal(sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items})
	readQuote(t, f, string(data), 409)
	body := quoteBody(t, f)
	readQuote(t, f, strings.Replace(body, f.shelf, f.back, 1), 400)
	readQuote(t, f, strings.Replace(body, f.product, "foreign-product", 1), 400)
	readQuote(t, f, strings.Replace(body, `"quantity_milli":2000`, `"quantity_milli":1500`, 1), 400)
	readQuote(t, f, strings.Replace(body, "turn-one", "missing", 1), 409)
	salesCounts(t, f, 0)
}

func TestHTTPQuoteRequiresRolePOSSessionAndVerifier(t *testing.T) {
	f := salesSetup(t)
	body := quoteBody(t, f)
	if status, _ := request(t, f.app, "POST", "/local/v1/sales/quote", body, ""); status != 401 {
		t.Fatalf("anonymous=%d", status)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='stock' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	readQuote(t, f, body, 403)
	if _, err := f.db.Exec(`UPDATE memberships SET role='owner' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	readQuote(t, f, body, 403)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	f.app = app
	readQuote(t, f, body, 503)
	salesCounts(t, f, 0)
}

func TestHTTPQuoteRejectsUnsafeWebNumbers(t *testing.T) {
	f := salesSetup(t)
	if _, err := f.db.Exec(`UPDATE products SET price_cents=? WHERE tenant_id=? AND id=?`, math.MaxInt64, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	readQuote(t, f, quoteBody(t, f), 400)
	salesCounts(t, f, 0)
}

func TestHTTPQuoteMixed315IsUnavailableWithoutPaymentConnector(t *testing.T) {
	f := salesSetup(t)
	if _, err := f.db.Exec(`UPDATE products SET price_cents=31500 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	in := salesInput(f)
	in.Items[0].QuantityMilli = 1000
	in.Payments = []sale.Payment{{Method: "cash", AmountCents: 10000}, {Method: "debit", AmountCents: 9600}, {Method: "credit", AmountCents: 11900}}
	data, _ := json.Marshal(sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items})
	q := readQuote(t, f, string(data), 200)
	if q.TotalCents != 31500 || len(q.PaymentMethods) != 1 || q.PaymentMethods[0] != "cash" {
		t.Fatal(q)
	}
	salesRequest(t, f, in, 409)
	salesCounts(t, f, 0)
	assertStockBalance(t, f, f.shelf, 5000)
}
