package localapi

import (
	"encoding/json"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/sale"
)

// Exercise the actual production reservation service and the B01 shared price
// checker together, not a second reservation model invented for checkout.
func TestHTTPB02QuoteAndSaleHonorProductionReservations(t *testing.T) {
	f, reservation := materialsFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2,
		[]modules.ID{modules.Core, modules.Inventory, modules.Production, modules.POS}), 200)
	requireMaterials(t, f, "POST", materialsPath, reservation, 201)
	if _, err := f.db.Exec(`UPDATE stock_locations SET kind='shelf' WHERE tenant_id=? AND store_id=? AND id='production-room'`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE products SET price_cents=1000 WHERE tenant_id=? AND id='flour'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "open", `{"session_id":"reserved-shift","opening_cents":0}`, 201)
	quote := sale.QuoteInput{CashSessionID: "reserved-shift", Items: []sale.Item{{ProductID: "flour", LocationID: "production-room", QuantityMilli: 500000}}}
	callQuote := func(want int) sale.Quote {
		t.Helper()
		body, _ := json.Marshal(quote)
		status, result := request(t, f.app, "POST", "/local/v1/sales/quote", string(body), f.token)
		if status != want {
			t.Fatalf("quote %d want %d: %s", status, want, result)
		}
		var out sale.Quote
		if want == 200 {
			if err := json.Unmarshal(result, &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	q := callQuote(200)
	if q.Items[0].AvailableMilli != 500000 {
		t.Fatalf("physical stock was exposed as free: %+v", q)
	}
	quote.Items[0].QuantityMilli = 500001
	callQuote(409)
	input := sale.Input{OperationID: "reserved-sale", SaleID: "reserved-sale", CashSessionID: "reserved-shift", Items: quote.Items, Payments: []sale.Payment{{Method: "cash", AmountCents: 500001}}}
	status, body := request(t, f.app, "POST", "/local/v1/sales", orderBody(t, input), f.token)
	if status != 409 {
		t.Fatalf("reserved stock was sold: %d %s", status, body)
	}
	materialCount(t, f, "sales", 0)
	materialCount(t, f, "cash_movements", 0)
	quote.Items[0].QuantityMilli = 500000
	input.Items = quote.Items
	input.Payments[0].AmountCents = 500000
	for _, want := range []int{201, 200} {
		status, body = request(t, f.app, "POST", "/local/v1/sales", orderBody(t, input), f.token)
		if status != want {
			t.Fatalf("sale %d want %d: %s", status, want, body)
		}
	}
	materialCount(t, f, "sales", 1)
	materialCount(t, f, "cash_movements", 1)
	quote.Items[0].QuantityMilli = 1
	callQuote(409) // 1,500,000 physically remain; every unit is reserved.
	requireMaterials(t, f, "POST", materialsPath+"/state", production.MaterialChangeInput{OperationID: "release-for-checkout", ReservationID: reservation.ReservationID, Action: "release", Reason: "Replanejar"}, 200)
	q = callQuote(200)
	if q.Items[0].AvailableMilli != 1500000 {
		t.Fatalf("release did not restore free balance: %+v", q)
	}
}

func TestHTTPB02InactiveProductCannotBeQuotedOrSoldButPriorSaleRecovers(t *testing.T) {
	f := salesSetup(t)
	in := salesInput(f)
	salesRequest(t, f, in, 201)
	if _, err := f.db.Exec(`INSERT INTO catalog_product_states(tenant_id,product_id,status,revision) VALUES(?,?,'inactive',1)`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	readQuote(t, f, quoteBody(t, f), 409)
	in.OperationID = "new-inactive"
	in.SaleID = "new-inactive"
	salesRequest(t, f, in, 409)
	salesRequest(t, f, salesInput(f), 200) // durable replay is not a new sale
	if readReceipt(t, f, salesInput(f).SaleID).Status != "committed" {
		t.Fatal("historical sale hidden")
	}
	salesCounts(t, f, 1)
}
