package localapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/sale"
)

func TestHTTPCustomerReturnBackupRestoresPopulatedLedger(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
	original := customerReturnRequest(t, f, in, 201)
	dir := t.TempDir()
	archive := filepath.Join(dir, "refund.tytbak")
	path := filepath.Join(dir, "restored.sqlite")
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	if e := backup.Create(context.Background(), f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(context.Background(), archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	r, e := sale.ReadCustomerReturnOperation(context.Background(), db, a, f.device, "return-one")
	if e != nil || r.ID != original.ID || r.RefundCents != 1599 || r.Items[0].ReturnMovementID == nil {
		t.Fatal(r, e)
	}
	a.TenantID = "foreign"
	if _, e = sale.ReadCustomerReturnOperation(context.Background(), db, a, f.device, "return-one"); e == nil {
		t.Fatal("cross-tenant read")
	}
}

func TestHTTPCustomerReturnIntegratesCartSaleAndAccountingCSV(t *testing.T) {
	f := salesSetup(t)
	saved := cartCall(t, f, cartsPath, cartInput(f), 200)
	cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("claim", 1), 200)
	final := cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200)
	receipt := readReceipt(t, f, saved.Cart.SaleID)
	in := sale.CustomerReturnInput{OperationID: "return-cart", SaleID: saved.Cart.SaleID, Reason: "Devolucao parcial conferida", ExpectedRefundCents: 1599, Items: []sale.ReturnItemInput{{SaleItemID: receipt.Items[0].ItemID, QuantityMilli: 1000, Disposition: "restock"}}}
	customerReturnRequest(t, f, in, 201)
	if after := cartCall(t, f, cartsPath+"/stable-cart/actions", cartAction("finalize_cash", 2), 200); !after.Repeated || after.Cart.SaleID != final.Cart.SaleID || after.Cart.State != "completed" {
		t.Fatal(after)
	}
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-return-cart","declared_cents":2599}`, 200)
	setAccountingClosedAt(t, f, "turn-one", closedFirst)
	token := periodAccountant(t, f)
	r := accountingReport(t, f, token, accountingQuery(closedFirst, periodEnd), 200)
	if r.Summary["returned_sales_cents"] != 1599 || r.Summary["gross_sales_cents"] != 3198 || r.Summary["net_sales_cents"] != 1599 || r.Summary["sum_expected_at_closures_cents"] != 2599 || r.Summary["unclassified_count"] != 0 {
		t.Fatal(r)
	}
	// The export uses the same validated service snapshot and includes the
	// distinct return metric, without falsifying the original payment status.
	status, b := request(t, f.app, "GET", "/local/v1/accounting/cash/report.csv"+accountingQuery(closedFirst, periodEnd), "", token)
	if status != 200 || !strings.Contains(string(b), "returned_sales_cents") {
		t.Fatalf("CSV %d %s", status, b)
	}
}

func TestHTTPCustomerReturnHistorySurvivesLicenseExpiryButWritesStop(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
	customerReturnRequest(t, f, in, 201)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	h := customerReturnHistory(t, f, 200)
	if len(h.Returns) != 1 {
		t.Fatal(h)
	}
	status, _ := request(t, f.app, "GET", "/local/v1/sale-return-operations/return-one", "", f.token)
	if status != 200 {
		t.Fatal(status)
	}
	customerReturnRequest(t, f, in, 403)
}

func TestHTTPCustomerReturnHistoryDetectsDamagedMovement(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
	r := customerReturnRequest(t, f, in, 201)
	if _, e := f.db.Exec(`UPDATE stock_movements SET quantity_milli=999 WHERE id=?`, *r.Items[0].ReturnMovementID); e != nil {
		t.Fatal(e)
	}
	customerReturnHistory(t, f, 409)
	customerReturnRequest(t, f, in, 409)
}

func TestHTTPCustomerReturnFractionalUnitAndDuplicateLinesRejected(t *testing.T) {
	f := cancellationSetup(t)
	in := customerReturnInput(t, f, "return-one", 500, 799, "restock")
	if _, e := f.db.Exec(`UPDATE products SET unit='unit' WHERE id=?`, f.product); e != nil {
		t.Fatal(e)
	}
	customerReturnRequest(t, f, in, 400)
	in.Items[0].QuantityMilli = 1000
	in.ExpectedRefundCents = 1599
	in.Items = append(in.Items, in.Items[0])
	customerReturnRequest(t, f, in, 400)
	assertCustomerReturnCount(t, f, 0)
}

func TestHTTPCustomerReturnUnknownQueriesAndMissingOperation(t *testing.T) {
	f := cancellationSetup(t)
	for _, path := range []string{"/local/v1/sales/sale-one/returns?tenant_id=other", "/local/v1/sale-return-operations/missing?offset=1"} {
		status, _ := request(t, f.app, "GET", path, "", f.token)
		if status != 400 {
			t.Fatal(path, status)
		}
	}
	status, _ := request(t, f.app, "GET", "/local/v1/sale-return-operations/missing", "", f.token)
	if status != 404 {
		t.Fatal(status)
	}
	in := customerReturnInput(t, f, "return-one", 1000, 1599, "restock")
	in.SaleID = "missing"
	customerReturnRequest(t, f, in, 404)
	quote, _ := json.Marshal(map[string]any{"items": in.Items})
	status, _ = request(t, f.app, "POST", "/local/v1/sales/sale-one/returns/quote?extra=1", string(quote), f.token)
	if status != 400 {
		t.Fatal(status)
	}
}
