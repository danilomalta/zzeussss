package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
)

func salesSetup(t *testing.T) *stockHTTPFixture {
	t.Helper()
	f := stockHTTPSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2,
		[]modules.ID{modules.Core, modules.Inventory, modules.POS}), 200)
	stockRequest(t, f, stockEntry(f, "sales-entry", 10000), 201)
	stockRequest(t, f, stock.Input{OperationID: "sales-transfer", Kind: "transfer", ProductID: f.product,
		FromLocationID: f.back, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Reposicao"}, 201)
	cashRequest(t, f.httpContractFixture, "open", cashOpenBody, 201)
	return f
}

func salesInput(f *stockHTTPFixture) sale.Input {
	return sale.Input{OperationID: "sale-operation-one", SaleID: "sale-one", CashSessionID: "turn-one",
		Items:    []sale.Item{{ProductID: f.product, LocationID: f.shelf, QuantityMilli: 2000}},
		Payments: []sale.Payment{{Method: "cash", AmountCents: 3198}}}
}

func salesJSON(t *testing.T, input sale.Input) string {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func salesRequest(t *testing.T, f *stockHTTPFixture, input sale.Input, want int) []byte {
	t.Helper()
	status, body := request(t, f.app, "POST", "/local/v1/sales", salesJSON(t, input), f.token)
	if status != want {
		t.Fatalf("venda: recebido=%d esperado=%d corpo=%s", status, want, body)
	}
	return body
}

func salesCounts(t *testing.T, f *stockHTTPFixture, count int) {
	t.Helper()
	for _, table := range []string{"sales", "sale_items", "sale_payments", "sale_item_stock", "sale_operations", "cash_movements"} {
		var got int
		if err := f.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil || got != count {
			t.Fatalf("%s=%d esperado=%d erro=%v", table, got, count, err)
		}
	}
	var got int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE event_type='sale.committed'`).Scan(&got); err != nil || got != count {
		t.Fatalf("eventos=%d esperado=%d erro=%v", got, count, err)
	}
}

func readReceipt(t *testing.T, f *stockHTTPFixture, id string) sale.Receipt {
	t.Helper()
	status, body := request(t, f.app, "GET", "/local/v1/sales/"+id, "", f.token)
	var result sale.Receipt
	if status != 200 {
		t.Fatalf("consulta: %d %s", status, body)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHTTPSaleCompleteCashFlowAndRepeatAfterClosing(t *testing.T) {
	f := salesSetup(t)
	input := salesInput(f)
	body := salesRequest(t, f, input, 201)
	if !bytes.Contains(body, []byte(`"total_cents":3198`)) || !bytes.Contains(body, []byte(`"repeated":false`)) {
		t.Fatalf("venda: %s", body)
	}
	salesRequest(t, f, input, 200)
	salesCounts(t, f, 1)
	assertStockBalance(t, f, f.shelf, 3000)
	receipt := readReceipt(t, f, input.SaleID)
	if receipt.TotalCents != 3198 || receipt.Status != "committed" || receipt.FiscalAuthorized || receipt.CommittedAt == "" ||
		len(receipt.Items) != 1 || receipt.Items[0].UnitPriceCents != 1599 || receipt.Items[0].QuantityMilli != 2000 ||
		len(receipt.Payments) != 1 || receipt.Payments[0].Method != "cash" || receipt.Payments[0].Status != "confirmed" {
		t.Fatalf("registro operacional: %+v", receipt)
	}
	closeBody := `{"session_id":"turn-one","operation_id":"end-sale-shift","declared_cents":4198}`
	body = cashRequest(t, f.httpContractFixture, "close", closeBody, 200)
	if !bytes.Contains(body, []byte(`"expected_cents":4198`)) || !bytes.Contains(body, []byte(`"difference_cents":0`)) {
		t.Fatalf("caixa: %s", body)
	}
	// This is recovery of a previously committed operation, not a new sale.
	salesRequest(t, f, input, 200)
	input.OperationID, input.SaleID = "new-after-close", "new-after-close"
	salesRequest(t, f, input, 409)
	salesCounts(t, f, 1)
}

func TestHTTPSaleConflictingOperationAndSaleIDs(t *testing.T) {
	f := salesSetup(t)
	input := salesInput(f)
	salesRequest(t, f, input, 201)
	changed := salesInput(f)
	changed.Payments[0].AmountCents = 1
	salesRequest(t, f, changed, 409)
	changed = salesInput(f)
	changed.OperationID = "different-operation"
	salesRequest(t, f, changed, 409)
	changed = salesInput(f)
	changed.SaleID = "different-sale"
	salesRequest(t, f, changed, 409)
	salesCounts(t, f, 1)
	assertStockBalance(t, f, f.shelf, 3000)
}

func TestHTTPSaleStoredPricesRemainStableAfterCatalogChange(t *testing.T) {
	f := salesSetup(t)
	input := salesInput(f)
	salesRequest(t, f, input, 201)
	if _, err := f.db.Exec(`UPDATE products SET price_cents=9999 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 200)
	receipt := readReceipt(t, f, input.SaleID)
	if receipt.TotalCents != 3198 || receipt.Items[0].UnitPriceCents != 1599 {
		t.Fatalf("preco historico alterado: %+v", receipt)
	}
	salesCounts(t, f, 1)
}

func TestHTTPSaleInsufficientStockPaymentAndInvalidReferences(t *testing.T) {
	f := salesSetup(t)
	for _, change := range []func(*sale.Input){
		func(in *sale.Input) { in.Items[0].QuantityMilli = 6000; in.Payments[0].AmountCents = 9594 },
		func(in *sale.Input) { in.Payments[0].Method = "pix" },
		func(in *sale.Input) { in.Payments[0].Method = "card" },
		func(in *sale.Input) { in.CashSessionID = "missing" },
	} {
		input := salesInput(f)
		change(&input)
		salesRequest(t, f, input, 409)
	}
	for _, change := range []func(*sale.Input){
		func(in *sale.Input) { in.Items[0].ProductID = "missing" },
		func(in *sale.Input) { in.Items[0].LocationID = f.back },
		func(in *sale.Input) { in.Items[0].QuantityMilli = 1500 },
		func(in *sale.Input) { in.Payments[0].AmountCents = 3199 },
	} {
		input := salesInput(f)
		change(&input)
		salesRequest(t, f, input, 400)
	}
	// Duplicate item lines cannot consume the same local balance twice.
	input := salesInput(f)
	input.Items = []sale.Item{{ProductID: f.product, LocationID: f.shelf, QuantityMilli: 3000}, {ProductID: f.product, LocationID: f.shelf, QuantityMilli: 3000}}
	input.Payments[0].AmountCents = 9594
	salesRequest(t, f, input, 409)
	salesCounts(t, f, 0)
	assertStockBalance(t, f, f.shelf, 5000)
}

func TestHTTPSaleRequiresPOSAndVerifierButPreservesReceiptRead(t *testing.T) {
	f := salesSetup(t)
	input := salesInput(f)
	salesRequest(t, f, input, 201)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3,
		[]modules.ID{modules.Core, modules.Inventory}), 200)
	salesRequest(t, f, input, 403)
	if readReceipt(t, f, input.SaleID).TotalCents != 3198 {
		t.Fatal("consulta bloqueada")
	}
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	f.app, f.token = app, loginToken(t, app, f.owner.OwnerID)
	salesRequest(t, f, input, 503)
	readReceipt(t, f, input.SaleID)
	salesCounts(t, f, 1)
}

func TestHTTPSaleRequiresSessionRoleAndActiveDevice(t *testing.T) {
	f := salesSetup(t)
	input := salesInput(f)
	status, _ := request(t, f.app, "POST", "/local/v1/sales", salesJSON(t, input), "")
	if status != 401 {
		t.Fatalf("sem sessao: %d", status)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='stock' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 403)
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 201)
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, f.owner.TenantID, f.owner.DeviceID); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 401)
	status, _ = request(t, f.app, "GET", "/local/v1/sales/"+input.SaleID, "", f.token)
	if status != 401 {
		t.Fatalf("aparelho revogado consultou: %d", status)
	}
	salesCounts(t, f, 1)
}

func TestHTTPSaleRollbackOnPaymentOrOutboxFailure(t *testing.T) {
	for _, failure := range []string{
		`CREATE TRIGGER sale_fail BEFORE INSERT ON sale_payments BEGIN SELECT RAISE(ABORT,'test failure'); END`,
		`CREATE TRIGGER sale_fail BEFORE INSERT ON outbox WHEN NEW.event_type='sale.committed' BEGIN SELECT RAISE(ABORT,'test failure'); END`,
	} {
		t.Run(failure, func(t *testing.T) {
			f := salesSetup(t)
			observed := time.Now().Unix() - 30
			if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, observed, f.owner.TenantID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec(failure); err != nil {
				t.Fatal(err)
			}
			salesRequest(t, f, salesInput(f), 500)
			salesCounts(t, f, 0)
			assertStockBalance(t, f, f.shelf, 5000)
			var got int64
			if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, f.owner.TenantID).Scan(&got); err != nil || got != observed {
				t.Fatalf("rollback do relogio: %d %v", got, err)
			}
			status, _ := request(t, f.app, "GET", "/local/v1/sales/sale-one", "", f.token)
			if status != 404 {
				t.Fatalf("venda parcial consultavel: %d", status)
			}
		})
	}
}

func TestHTTPSalePersistsAndCanRecoverAfterReopen(t *testing.T) {
	f := salesSetup(t)
	input := salesInput(f)
	salesRequest(t, f, input, 201)
	before := readReceipt(t, f, input.SaleID)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil || path == "" {
		t.Fatalf("banco descartavel: %v", err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	f.db, f.app = db, app
	after := readReceipt(t, f, input.SaleID)
	if after.TotalCents != before.TotalCents || after.CommittedAt != before.CommittedAt || after.Items[0].ItemID != before.Items[0].ItemID {
		t.Fatal("registro mudou apos reinicio")
	}
	salesRequest(t, f, input, 200)
	salesCounts(t, f, 1)
	assertStockBalance(t, f, f.shelf, 3000)
}

func TestHTTPSaleRejectsAmbiguousNestedJSONAndInexactMoney(t *testing.T) {
	f := salesSetup(t)
	valid := salesJSON(t, salesInput(f))
	invalid := []string{
		`{}`, `null`, valid + ` {}`,
		strings.Replace(valid, `"sale_id":"sale-one"`, `"sale_id":"sale-one","sale_id":"other"`, 1),
		strings.Replace(valid, `"quantity_milli":2000`, `"quantity_milli":2000,"quantity_milli":1000`, 1),
		strings.Replace(valid, `"amount_cents":3198`, `"amount_cents":3198,"amount_cents":1`, 1),
		strings.Replace(valid, `"amount_cents":3198`, `"amount_cents":1.5`, 1),
		strings.Replace(valid, `"amount_cents":3198`, `"amount_cents":9223372036854775808`, 1),
		strings.Replace(valid, `"amount_cents":3198`, `"amount_cents":null`, 1),
		strings.Replace(valid, `"quantity_milli":2000`, `"quantity_milli":2000,"unit_price_cents":1`, 1),
		strings.Replace(valid, `"sale_id":"sale-one"`, `"sale_id":"sale-one","tenant_id":"foreign"`, 1),
		strings.Replace(valid, `"method":"cash"`, `"method":"cash","confirmed":true`, 1),
	}
	for _, body := range invalid {
		status, _ := request(t, f.app, "POST", "/local/v1/sales", body, f.token)
		if status != 400 {
			t.Fatalf("JSON invalido: %d corpo=%s", status, body)
		}
	}
	salesCounts(t, f, 0)
}

func TestHTTPSaleConcurrentRequestsDoNotDuplicate(t *testing.T) {
	f := salesSetup(t)
	body := salesJSON(t, salesInput(f))
	type response struct {
		status int
		err    error
	}
	results := make(chan response, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/local/v1/sales", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+f.token)
			res, err := f.app.Test(req, -1)
			if err != nil {
				results <- response{err: err}
				return
			}
			_ = res.Body.Close()
			results <- response{status: res.StatusCode}
		}()
	}
	created, replayed := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		switch r.status {
		case 201:
			created++
		case 200:
			replayed++
		default:
			t.Fatalf("concorrencia: %d", r.status)
		}
	}
	if created != 1 || replayed != 1 {
		t.Fatalf("criadas=%d repetidas=%d", created, replayed)
	}
	salesCounts(t, f, 1)
	assertStockBalance(t, f, f.shelf, 3000)
}

func TestHTTPSaleForeignProductStoreAndActorAreIsolated(t *testing.T) {
	f := salesSetup(t)
	for _, stmt := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO tenants VALUES (?,?,?)`, []any{"foreign", "Outra", "now"}},
		{`INSERT INTO products(tenant_id,id,sku,name,price_cents) VALUES (?,?,?,?,?)`, []any{"foreign", "foreign-product", "F", "Outro", 1}},
		{`INSERT INTO stores VALUES (?,?,?)`, []any{f.owner.TenantID, "other-store", "Filial"}},
		{`INSERT INTO stock_locations VALUES (?,?,?,?,?)`, []any{f.owner.TenantID, "other-store", "other-shelf", "shelf", "Filial"}},
		{`INSERT INTO identities VALUES (?,?,?)`, []any{"other-actor", "Outro", "now"}},
		{`INSERT INTO memberships VALUES (?,?,?,?,?)`, []any{f.owner.TenantID, "other-actor", "cashier", "active", "now"}},
	} {
		if _, err := f.db.Exec(stmt.query, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	input := salesInput(f)
	input.Items[0].ProductID = "foreign-product"
	salesRequest(t, f, input, 400)
	input = salesInput(f)
	input.Items[0].LocationID = "other-shelf"
	salesRequest(t, f, input, 400)
	input = salesInput(f)
	salesRequest(t, f, input, 201)
	if _, err := f.db.Exec(`UPDATE sale_operations SET actor_identity_id='other-actor' WHERE tenant_id=? AND sale_id=?`, f.owner.TenantID, input.SaleID); err != nil {
		t.Fatal(err)
	}
	status, _ := request(t, f.app, "GET", "/local/v1/sales/"+input.SaleID, "", f.token)
	if status != 404 {
		t.Fatalf("consultou venda alheia: %d", status)
	}
	salesCounts(t, f, 1)
}

func TestHTTPSaleCashOverflowAndWeightedRounding(t *testing.T) {
	t.Run("overflow", func(t *testing.T) {
		f := salesSetup(t)
		if _, err := f.db.Exec(`UPDATE cash_sessions SET opening_cents=? WHERE tenant_id=? AND id='turn-one'`, math.MaxInt64, f.owner.TenantID); err != nil {
			t.Fatal(err)
		}
		salesRequest(t, f, salesInput(f), 409)
		salesCounts(t, f, 0)
		assertStockBalance(t, f, f.shelf, 5000)
	})
	t.Run("peso", func(t *testing.T) {
		f := salesSetup(t)
		if _, err := f.db.Exec(`UPDATE products SET unit='kg',price_cents=299 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
			t.Fatal(err)
		}
		input := salesInput(f)
		input.Items[0].QuantityMilli = 1500
		input.Payments[0].AmountCents = 449
		body := salesRequest(t, f, input, 201)
		if !bytes.Contains(body, []byte(`"total_cents":449`)) {
			t.Fatalf("arredondamento: %s", body)
		}
		assertStockBalance(t, f, f.shelf, 3500)
	})
}

func TestHTTPSaleExpiredContractStillAllowsReceipt(t *testing.T) {
	f := salesSetup(t)
	input := salesInput(f)
	salesRequest(t, f, input, 201)
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 2,
		IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60,
		Modules: []modules.ID{modules.Core, modules.Inventory, modules.POS}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=? WHERE tenant_id=? AND revision=2`, payload,
		ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload)), f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 403)
	receipt := readReceipt(t, f, input.SaleID)
	if receipt.TotalCents != 3198 {
		t.Fatalf("total incorreto: %+v", receipt)
	}
	salesCounts(t, f, 1)
}
