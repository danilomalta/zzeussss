package localapi

import (
	"encoding/json"
	"testing"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
)

// This crosses existing production, stock, cart, sale and closed-report services
// in one database. Fixtures supply identities/catalog/initial materials only.
func TestBackendLocalProductionStagesToCashCartAndOfficialClosing(t *testing.T) {
	f, stages, reserve := stagesFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 99, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Production}), 200)
	requireMaterials(t, f, "POST", stagePlansPath, stages, 201)
	consumeStages(t, f, reserve)
	requireMaterials(t, f, "POST", resultsPath, stageCompletion(stages), 409)
	finishStages(t, f, stages)
	requireMaterials(t, f, "POST", resultsPath, stageCompletion(stages), 201)
	// Catalog prices here are input fixture data, not a claim of UI integration.
	if _, e := f.db.Exec(`UPDATE products SET price_cents=399 WHERE tenant_id=? AND id='bread'`, f.owner.TenantID); e != nil {
		t.Fatal(e)
	}
	code, b := request(t, f.app, "POST", "/local/v1/locations", `{"kind":"shelf","name":"Gondola do produto fabricado"}`, f.token)
	if code != 201 {
		t.Fatal(code, string(b))
	}
	var loc struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(b, &loc) != nil || loc.ID == "" {
		t.Fatal(string(b))
	}
	sf := &stockHTTPFixture{httpContractFixture: f, product: "bread", shelf: loc.ID}
	stockRequest(t, sf, stock.Input{OperationID: "production-to-shelf", Kind: "transfer", ProductID: "bread", FromLocationID: "production-room", ToLocationID: loc.ID, QuantityMilli: 20000, Reason: "Reposicao do produto fabricado"}, 201)
	cashRequest(t, f, "open", cashOpenBody, 201)
	saved := cartCall(t, sf, cartsPath, sale.CartSaveInput{OperationID: "save-produced", CartID: "produced-cart", Label: "Produto fabricado", Items: []sale.Item{{ProductID: "bread", LocationID: loc.ID, QuantityMilli: 2000}}}, 200)
	if saved.Cart.TotalCents != 798 {
		t.Fatal(saved)
	}
	cartCall(t, sf, cartsPath+"/produced-cart/actions", cartAction("claim", 1), 200)
	final := cartAction("finalize_cash", 2)
	final.ReceivedCents = 1000
	first := cartCall(t, sf, cartsPath+"/produced-cart/actions", final, 200)
	again := cartCall(t, sf, cartsPath+"/produced-cart/actions", final, 200)
	if !again.Repeated || first.Cart.SaleID != saved.Cart.SaleID || first.Cart.ChangeCents != 202 {
		t.Fatal(first, again)
	}
	assertStockBalance(t, sf, loc.ID, 18000)
	cashRequest(t, f, "close", `{"session_id":"turn-one","operation_id":"closing-produced","declared_cents":1798}`, 200)
	b = cartGet(t, sf, "/local/v1/cash/sessions/turn-one/report", 200)
	var report register.ClosedReport
	if json.Unmarshal(b, &report) != nil || !report.Complete || report.SaleCount != 1 || report.NetSalesCents != 798 || report.Cash.ExpectedCents != 1798 || report.Cash.DifferenceCents != 0 {
		t.Fatal(string(b))
	}
	cartGet(t, sf, "/local/v1/cash/sessions/turn-one/journal", 200)
	cartGet(t, sf, "/local/v1/pos/cart-operations/finalize_cash-one", 200)
}
