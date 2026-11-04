package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

func taxedSaleFixture(t *testing.T, codes ...string) (*productionFixture, fiscal.TaxQuote, sale.Input) {
	t.Helper()
	p := newProductionFixture(t, "55", "1", "America/Sao_Paulo", "required")
	code := "202"
	if len(codes) > 0 {
		code = codes[0]
	}
	policy := productionSimpleSTPolicy(t, p, code, "40")
	// Add FCP-ST as a separately approved component, before publication.
	// Replace through a new policy revision to preserve the original publication.
	policy.OperationID = "st-policy-fcp"
	policy.ExpectedRevision = 1
	policy.Rule.Goods.FCPScope = "substitution"
	policy.Rule.Components = append(policy.Rule.Components, fiscalcore.TaxComponent{Code: "FCPST", Method: "percentage", Rate: "2", MVA: "40", ReductionPercent: "0", AddsToTotal: true})
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
	q := taxQuoteFixture(policy)
	q.PolicyRevision = 2
	q.Context.Model = "55"
	line := p.input.Lines[0]
	q.Item = fiscalcore.TaxItem{ProductID: line.ProductID, Quantity: line.Quantity, UnitPrice: line.UnitPrice, Discount: "0", Freight: "0", Insurance: "0", Other: "0"}
	// The fixture's public binding fixes issuer timezone.
	// Current test dates come from issuer-local time, not UTC substring assumptions.
	q.Context.Date = issuerToday(t, p)
	var quote fiscal.TaxQuote
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, &quote)
	in := salesInput(p.f)
	in.OperationID = "taxed-sale-operation"
	in.SaleID = "taxed-sale"
	in.TaxQuoteOperations = []string{q.OperationID}
	in.Payments[0].AmountCents = 3518
	return p, quote, in
}
func TestHTTPFiscalTaxPricedSaleToIssuedXML(t *testing.T) {
	for _, code := range []string{"201", "202", "203"} {
		t.Run(code, func(t *testing.T) { testFiscalTaxPricedSaleToIssuedXML(t, code) })
	}
}
func testFiscalTaxPricedSaleToIssuedXML(t *testing.T, code string) {
	p, quote, in := taxedSaleFixture(t, code)
	if quote.Result.Total != "35.18" || quote.Result.GoodsXML.Totals["vST"] != "2.30" || quote.Result.GoodsXML.Totals["vFCPST"] != "0.90" {
		t.Fatalf("bad fixture %+v", quote.Result)
	}
	raw, _ := json.Marshal(sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items, TaxQuoteOperations: in.TaxQuoteOperations})
	priced := readQuote(t, p.f, string(raw), 200)
	if priced.TotalCents != 3518 || priced.Items[0].MerchandiseCents == nil || *priced.Items[0].MerchandiseCents != 3198 || priced.Items[0].Tax == nil || priced.Items[0].Tax.AdditionalCents != 320 {
		t.Fatalf("bad price %+v", priced)
	}
	salesRequest(t, p.f, in, 201)
	salesRequest(t, p.f, in, 200)
	receipt := readReceipt(t, p.f, in.SaleID)
	if receipt.TotalCents != 3518 || receipt.Items[0].TotalCents != 3518 || receipt.Items[0].Tax == nil {
		t.Fatalf("receipt lost tax %+v", receipt)
	}
	assertStockBalance(t, p.f, p.f.shelf, 1000)
	p.intent = fiscalIntent(t, p.f, `{"operation_id":"taxed-intent","sale_id":"taxed-sale","profile_id":"issuer-one","profile_revision":2}`, 201)
	if p.intent.Snapshot.Items[0].Tax == nil || p.intent.Snapshot.TotalCents != 3518 {
		t.Fatal("snapshot lost tax")
	}
	p.input.Lines[0].TaxQuoteOperationID = quote.Input.OperationID
	p.input.Lines[0].Taxes = nil
	p.input.Lines[0].NCM = ""
	p.input.Lines[0].CFOP = ""
	p.input.TaxTotals = nil
	p.input.Payments[0].Amount = quote.Result.Total
	prep := p.prepare(t, "taxed-review", 0, 201)
	p.approve(t, prep, 204)
	out := p.emit(t, prep, "taxed-issue", 200)
	if out.State != "authorized" || p.issues.Load() != 1 || !bytes.Contains(out.ProcessedXML, []byte("<vNF>35.18</vNF>")) || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
		t.Fatal("taxed real API / fixture mTLS failed")
	}
	again := p.emit(t, prep, "taxed-issue", 200)
	if p.issues.Load() != 1 || !bytes.Equal(out.ProcessedXML, again.ProcessedXML) {
		t.Fatal("duplicate issue")
	}
	t.Log("Cash3518 = merchandise3198 + ST230 + FCP-ST90; SQLite/API/signature/XSD/mTLS real; authority disposable, no official homologation")
}
func TestHTTPFiscalTaxPricedSaleWrongPaymentAndRollback(t *testing.T) {
	p, _, in := taxedSaleFixture(t)
	in.Payments[0].AmountCents = 3198
	salesRequest(t, p.f, in, 400)
	var n int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM sale_tax_bindings`).Scan(&n); err != nil || n != 0 {
		t.Fatal("partial evidence", n, err)
	}
	assertStockBalance(t, p.f, p.f.shelf, 3000)
	in.Payments[0].AmountCents = 3518
	if _, err := p.f.db.Exec(`CREATE TEMP TRIGGER fail_tax_binding BEFORE INSERT ON sale_tax_bindings BEGIN SELECT RAISE(ABORT,'test rollback'); END`); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(in)
	code, _ := request(t, p.f.app, "POST", "/local/v1/sales", string(body), p.f.token)
	if code < 400 {
		t.Fatal("failure not rejected", code)
	}
	if _, err := p.f.db.Exec(`DROP TRIGGER fail_tax_binding`); err != nil {
		t.Fatal(err)
	}
	assertStockBalance(t, p.f, p.f.shelf, 3000)
	salesRequest(t, p.f, in, 201)
}
func TestHTTPFiscalTaxPricedSaleRefundIncludesTax(t *testing.T) {
	p, _, in := taxedSaleFixture(t)
	salesRequest(t, p.f, in, 201)
	receipt := readReceipt(t, p.f, in.SaleID)
	q, err := sale.QuoteCustomerReturn(context.Background(), p.f.db, identity.Scope{TenantID: p.f.owner.TenantID, StoreID: p.f.device.StoreID, IdentityID: p.f.owner.OwnerID}, p.f.device, in.SaleID, []sale.ReturnItemInput{{SaleItemID: receipt.Items[0].ItemID, QuantityMilli: 1000, Disposition: "restock"}})
	if err != nil || q.RefundCents != 1759 {
		t.Fatal("tax refund not included", q, err)
	}
}

func issuerToday(t *testing.T, p *productionFixture) string {
	t.Helper()
	zone, err := time.LoadLocation(p.binding.Timezone)
	if err != nil {
		t.Fatal(err)
	}
	return time.Now().In(zone).Format("2006-01-02")
}

func TestHTTPFiscalTaxPricedSaleCancellationIncludesTax(t *testing.T) {
	p, _, in := taxedSaleFixture(t)
	salesRequest(t, p.f, in, 201)
	for i := 0; i < 2; i++ {
		code, body := request(t, p.f.app, "POST", "/local/v1/sales/"+in.SaleID+"/cancel", cancelBody, p.f.token)
		var v sale.CancelResult
		if code != 200 || json.Unmarshal(body, &v) != nil || v.RefundedCents != 3518 || v.Repeated != (i == 1) {
			t.Fatalf("refund %d %s", code, body)
		}
	}
	assertStockBalance(t, p.f, p.f.shelf, 3000)
	r := readReceipt(t, p.f, in.SaleID)
	if r.Cancellation == nil || r.Items[0].Tax == nil || r.Cancellation.RefundedCents != 3518 {
		t.Fatal("history lost")
	}
}
func TestHTTPFiscalTaxPricedSalePartialReturnsExhaustExactTotal(t *testing.T) {
	p, _, in := taxedSaleFixture(t)
	salesRequest(t, p.f, in, 201)
	r := readReceipt(t, p.f, in.SaleID)
	ret := sale.CustomerReturnInput{OperationID: "tax-return-one", SaleID: in.SaleID, Reason: "Synthetic return", ExpectedRefundCents: 1759, Items: []sale.ReturnItemInput{{SaleItemID: r.Items[0].ItemID, QuantityMilli: 1000, Disposition: "restock"}}}
	first := customerReturnRequest(t, p.f, ret, 201)
	ret.OperationID = "tax-return-two"
	second := customerReturnRequest(t, p.f, ret, 201)
	if first.RefundCents+second.RefundCents != 3518 {
		t.Fatal("tax/refund rounding lost cents")
	}
	ret.OperationID = "tax-return-extra"
	customerReturnRequest(t, p.f, ret, 409)
	assertStockBalance(t, p.f, p.f.shelf, 3000)
}
func TestHTTPFiscalTaxPricedSaleRejectsStaleSources(t *testing.T) {
	for _, kind := range []string{"missing-tax-binding", "price", "unit", "quantity", "missing-ref", "wrong-device", "aged", "future", "profile", "duplicate-ref"} {
		t.Run(kind, func(t *testing.T) {
			p, _, in := taxedSaleFixture(t)
			expectedBalance := int64(3000)
			switch kind {
			case "missing-tax-binding":
				in.TaxQuoteOperations = nil
				in.Payments[0].AmountCents = 3198
			case "price":
				if _, err := p.f.db.Exec(`UPDATE products SET price_cents=1600 WHERE tenant_id=? AND id=?`, p.f.owner.TenantID, p.f.product); err != nil {
					t.Fatal(err)
				}
			case "unit":
				if _, err := p.f.db.Exec(`UPDATE products SET unit='kg' WHERE tenant_id=? AND id=?`, p.f.owner.TenantID, p.f.product); err != nil {
					t.Fatal(err)
				}
			case "quantity":
				in.Items[0].QuantityMilli = 1000
			case "missing-ref":
				in.TaxQuoteOperations[0] = "absent"
			case "wrong-device", "aged", "future":
				// Isolated database tampering simulates aged/foreign evidence; production
				// immutability triggers are never removed by the application.
				if _, err := p.f.db.Exec(`DROP TRIGGER fiscal_tax_quotes_no_update`); err != nil {
					t.Fatal(err)
				}
				var err error
				if kind == "wrong-device" {
					_, err = p.f.db.Exec(`UPDATE fiscal_tax_quotes SET device_id='foreign'`)
				} else if kind == "aged" {
					_, err = p.f.db.Exec(`UPDATE fiscal_tax_quotes SET created_at='2000-01-01T00:00:00Z'`)
				} else {
					_, err = p.f.db.Exec(`UPDATE fiscal_tax_quotes SET created_at='2999-01-01T00:00:00Z'`)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "profile":
				code, _ := request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", `{"operation_id":"profile-changed","profile_id":"issuer-one","expected_revision":2,"cnpj":"12ABC34501DE35","legal_name":"Synthetic test issuer","state":"SP","municipality_ibge":"3550308","state_registration":"123456789","crt":"1","model":"55","environment":"production","series":1,"timezone":"America/Sao_Paulo","require_tax_pricing":true}`, p.f.token)
				if code != 201 {
					t.Fatal(code)
				}
			case "duplicate-ref":
				if _, err := p.f.db.Exec(`INSERT INTO stock_movements SELECT 'tax-test-topup',tenant_id,store_id,device_id,product_id,location_id,5000,'test top-up',created_at FROM stock_movements WHERE tenant_id=? AND product_id=? AND location_id=? AND quantity_milli>0 LIMIT 1`, p.f.owner.TenantID, p.f.product, p.f.shelf); err != nil {
					t.Fatal(err)
				}
				expectedBalance = 8000
				in.Items = append(in.Items, in.Items[0])
				in.TaxQuoteOperations = append(in.TaxQuoteOperations, in.TaxQuoteOperations[0])
			}
			raw, _ := json.Marshal(in)
			code, _ := request(t, p.f.app, "POST", "/local/v1/sales", string(raw), p.f.token)
			if code != 400 && code != 409 {
				t.Fatal("source mismatch was not refused", code)
			}
			var count int
			if err := p.f.db.QueryRow(`SELECT count(*) FROM sale_tax_bindings`).Scan(&count); err != nil || count != 0 {
				t.Fatal("partial write", count, err)
			}
			assertStockBalance(t, p.f, p.f.shelf, expectedBalance)
		})
	}
}
func TestHTTPFiscalTaxPricedSaleSessionGuardAndRecovery(t *testing.T) {
	p, _, in := taxedSaleFixture(t)
	token := p.f.token
	if err := localauth.Logout(context.Background(), p.f.db, token); err != nil {
		t.Fatal(err)
	}
	a := identity.Scope{TenantID: p.f.owner.TenantID, StoreID: p.f.device.StoreID, IdentityID: p.f.owner.OwnerID}
	// Simulate middleware having resolved the actor before logout.
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": p.f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	contracts, err := entitlementstore.New(p.f.db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sale.CompleteWithContract(context.Background(), p.f.db, contracts, a, p.f.device, in, token)
	if !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("old session was allowed", err)
	}
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	salesRequest(t, p.f, in, 201)
	var seq int
	var name, path string
	if err = p.f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err = p.f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p.f.db = db
	p.installApp(t, p.runtime)
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	salesRequest(t, p.f, in, 200)
	var restored string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, p.f)
		restored = pilotCLIBackupRoundTrip(t, p.f, path, station)
	} else {
		restored = productionTaxBackupRoundTrip(t, p)
	}
	restoredDB, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer restoredDB.Close()
	p.f.db = restoredDB
	p.installApp(t, p.runtime)
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	r := readReceipt(t, p.f, in.SaleID)
	if r.TotalCents != 3518 || r.Items[0].Tax == nil || r.Items[0].Tax.AdditionalCents != 320 {
		t.Fatal("restored taxes changed")
	}
}
func TestHTTPFiscalTaxPricedSaleConcurrentOnce(t *testing.T) {
	p, _, in := taxedSaleFixture(t)
	raw, _ := json.Marshal(in)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(t, p.f.app, "POST", "/local/v1/sales", string(raw), p.f.token)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[200] != 1 {
		t.Fatal(counts)
	}
	var n int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM sale_tax_bindings`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	assertStockBalance(t, p.f, p.f.shelf, 1000)
}

func TestHTTPFiscalTaxPricedSaleNativeLoopback(t *testing.T) {
	p, quote, in := taxedSaleFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.f.app.Listener(listener) }()
	t.Cleanup(func() {
		_ = p.f.app.Shutdown()
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("HTTP server did not stop")
		}
	})
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}
	defer client.CloseIdleConnections()
	steps := 0
	call := func(method, path string, body any, want int) []byte {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		req, err := http.NewRequest(method, "http://"+listener.Addr().String()+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.f.token)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err = io.ReadAll(io.LimitReader(response.Body, 2*fiscalcore.MaxXML))
		if err != nil || response.StatusCode != want {
			t.Fatalf("HTTP %s %s -> %d want %d: %v", method, path, response.StatusCode, want, err)
		}
		steps++
		t.Logf("HTTP %s %s -> %d", method, path, response.StatusCode)
		return raw
	}
	call("POST", "/local/v1/sales/quote", sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items, TaxQuoteOperations: in.TaxQuoteOperations}, 200)
	call("POST", "/local/v1/sales", in, 201)
	call("GET", "/local/v1/sales/"+in.SaleID, nil, 200)
	if err = json.Unmarshal(call("POST", "/local/v1/fiscal/intents", fiscal.IntentInput{OperationID: "native-priced-intent", SaleID: in.SaleID, ProfileID: "issuer-one", ProfileRevision: 2}, 201), &p.intent); err != nil {
		t.Fatal(err)
	}
	p.input.Lines[0].TaxQuoteOperationID = quote.Input.OperationID
	p.input.Lines[0].Taxes = nil
	p.input.Lines[0].NCM = ""
	p.input.Lines[0].CFOP = ""
	p.input.TaxTotals = nil
	p.input.Payments[0].Amount = quote.Result.Total
	var prep fiscal.ProductionPreparation
	if err = json.Unmarshal(call("POST", p.path("/prepare"), map[string]any{"operation_id": "native-priced-review", "expected_revision": 0, "invoice": p.input}, 201), &prep); err != nil {
		t.Fatal(err)
	}
	call("POST", p.path("/approve"), map[string]any{"operation_id": "native-priced-approve", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256, "tax_reference": "DISPOSABLE-REVIEW-NOT-OFFICIAL"}, 204)
	emit := map[string]any{"operation_id": "native-priced-issue", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256}
	first := call("POST", p.path("/emit"), emit, 200)
	second := call("POST", p.path("/emit"), emit, 200)
	if !bytes.Equal(first, second) || p.issues.Load() != 1 {
		t.Fatal("native emission replay changed or duplicated")
	}
	call("GET", p.path(""), nil, 200)
	if steps != 9 {
		t.Fatal(steps)
	}
	t.Log("TAX PRICED SALE NATIVE: 9 real TCP HTTP requests, merchandise3198 + ST230 + FCP-ST90 = cash3518; one signed mTLS POST, disposable authority only, no official homologation")
}

func TestHTTPFiscalTaxPricedSaleIPIIncludedBeforePayment(t *testing.T) {
	p := newProductionFixture(t, "55", "3", "America/Sao_Paulo")
	p.input.FinalConsumer = "0"
	p.input.Recipient.IEDestination = "1"
	p.input.Recipient.IE = "110042490114"
	policy := productionTaxPolicy(t, p, func(in *fiscal.TaxPolicyInput) {
		in.Case.RecipientStatus = "contributor"
		in.Case.FinalConsumer = "no"
		in.Case.Purpose = "resale"
		in.Rule.Goods.IPIScope = "classified"
		in.Rule.Goods.IPI = "50"
		in.Rule.Goods.IPIEnquadramento = "999"
		in.Rule.Components = append(in.Rule.Components, fiscalcore.TaxComponent{Code: "IPI", Method: "percentage", Rate: "5", MVA: "0", ReductionPercent: "0", AddsToTotal: true})
	})
	quote := productionTaxQuote(t, p, policy)
	if quote.Result.Total != "33.58" || quote.Result.GoodsXML.Totals["vIPI"] != "1.60" {
		t.Fatal("bad synthetic IPI fixture")
	}
	in := salesInput(p.f)
	in.OperationID = "ipi-sale-operation"
	in.SaleID = "ipi-sale"
	in.TaxQuoteOperations = []string{quote.Input.OperationID}
	in.Payments[0].AmountCents = 3358
	salesRequest(t, p.f, in, 201)
	p.intent = fiscalIntent(t, p.f, `{"operation_id":"ipi-intent","sale_id":"ipi-sale","profile_id":"issuer-one","profile_revision":2}`, 201)
	p.input.Payments[0].Amount = quote.Result.Total
	prep := p.prepare(t, "ipi-sale-review", 0, 201)
	p.approve(t, prep, 204)
	out := p.emit(t, prep, "ipi-sale-issue", 200)
	if out.State != "authorized" || !bytes.Contains(out.ProcessedXML, []byte("<vIPI>1.60</vIPI>")) || !bytes.Contains(out.ProcessedXML, []byte("<vNF>33.58</vNF>")) || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
		t.Fatal("IPI economic source/XML diverged")
	}
}

func TestHTTPFiscalTaxPricedSaleRequiredAcrossWriters(t *testing.T) {
	p, _, in := taxedSaleFixture(t)
	body, _ := json.Marshal(sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items})
	readQuote(t, p.f, string(body), 409)
	legacy := in
	legacy.Payments = append([]sale.Payment(nil), in.Payments...)
	legacy.TaxQuoteOperations = nil
	legacy.Payments[0].AmountCents = 3198
	salesRequest(t, p.f, legacy, 409)
	plan := sale.PlanInput{OperationID: "tax-plan-create", CheckoutID: "tax-plan", CashSessionID: in.CashSessionID, ExpectedTotalCents: 3198, Items: in.Items, Parts: []sale.PlanPartInput{{PartID: "tax-cash", Method: "cash", AmountCents: 3198}}}
	v := planRequest(t, p.f, "POST", planPath, planJSON(t, plan), 201)
	if v.CanFinalize {
		t.Fatal("legacy plan advertised a blocked operation")
	}
	code, _ := request(t, p.f.app, "POST", planPath+"/tax-plan/finalize-cash", `{"operation_id":"tax-plan-finish","received_cents":3518}`, p.f.token)
	if code != 409 {
		t.Fatal("alternate writer bypassed tax requirement", code)
	}
	assertStockBalance(t, p.f, p.f.shelf, 3000)
	salesRequest(t, p.f, in, 201)
}
func TestHTTPFiscalTaxPricedSaleModeRequiresExplicitOwnerDecision(t *testing.T) {
	p, _, _ := taxedSaleFixture(t)
	omitted := `{"operation_id":"omit-required","profile_id":"issuer-one","expected_revision":2,"cnpj":"12ABC34501DE35","legal_name":"Synthetic test issuer","state":"SP","municipality_ibge":"3550308","state_registration":"123456789","crt":"1","model":"55","environment":"production","series":1,"timezone":"America/Sao_Paulo"}`
	code, _ := request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", omitted, p.f.token)
	if code != 409 {
		t.Fatal("omission disabled tax requirement", code)
	}
	// An old uncertain configuration operation is historical recovery, not a new
	// revision. It must keep its ID/revision without changing the active mode.
	replay := strings.Replace(fiscalProfileBody, `"model":"65"`, `"model":"55"`, 1)
	code, _ = request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", replay, p.f.token)
	if code != 200 {
		t.Fatal("historical profile replay was lost", code)
	}
	off := strings.Replace(omitted, `"timezone":"America/Sao_Paulo"}`, `"timezone":"America/Sao_Paulo","require_tax_pricing":false}`, 1)
	code, _ = request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", off, p.f.token)
	if code != 201 {
		t.Fatal("explicit owner decision refused", code)
	}
}
