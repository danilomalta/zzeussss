package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/sale"
)

func retainedPolicy(t *testing.T, p *productionFixture) fiscal.TaxPolicyInput {
	t.Helper()
	p.input.FinalConsumer = "0"
	p.input.Recipient.IEDestination = "1"
	p.input.Recipient.IE = "110042490114"
	return productionTaxPolicy(t, p, func(in *fiscal.TaxPolicyInput) {
		in.Case.RecipientStatus = "contributor"
		in.Case.FinalConsumer = "no"
		in.Case.Purpose = "resale"
		in.Case.CEST = "0100100"
		in.Rule.CFOP = "5405"
		in.Rule.Goods.ICMS = "60"
		if in.Rule.CRT == "1" {
			in.Rule.Goods.ICMS = "500"
		}
		in.Rule.Goods.FCPScope = "retained"
		in.Rule.Components[0] = fiscalcore.TaxComponent{Code: "ICMS", Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"}
	})
}
func retainedQuoteInput(t *testing.T, p *productionFixture, policy fiscal.TaxPolicyInput) fiscal.TaxQuoteInput {
	t.Helper()
	q := taxQuoteFixture(policy)
	q.Context.Model = "55"
	q.Context.CRT = policy.Rule.CRT
	q.Context.Date = issuerToday(t, p)
	line := p.input.Lines[0]
	q.Item = fiscalcore.TaxItem{ProductID: line.ProductID, Quantity: line.Quantity, UnitPrice: line.UnitPrice, Discount: "0", Freight: "0", Insurance: "0", Other: "0"}
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), CNPJ: "11222333000181", Model: "55", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	var unit string
	if err = p.f.db.QueryRow(`SELECT unit FROM products WHERE tenant_id=? AND id=?`, p.f.owner.TenantID, line.ProductID).Scan(&unit); err != nil {
		t.Fatal(err)
	}
	q.Item.RetainedST = &fiscalcore.RetainedSTInput{SourceKey: key, SourceItem: 2, ApprovalReference: "Owner reviewed disposable purchase allocation", Quantity: line.Quantity, Unit: unit, Base: "44.77", SupportedRate: "20", SubstituteAmount: "5.76", RetainedAmount: "2.30", FCP: &fiscalcore.RetainedFCPInput{Base: "44.77", Rate: "2", Amount: "0.90"}}
	return q
}
func retainedFixture(t *testing.T, crt string) (*productionFixture, fiscal.TaxPolicyInput, fiscal.TaxQuote, sale.Input) {
	t.Helper()
	p := newProductionFixture(t, "55", crt, "America/Sao_Paulo", "required")
	policy := retainedPolicy(t, p)
	q := retainedQuoteInput(t, p, policy)
	var quote fiscal.TaxQuote
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, &quote)
	in := salesInput(p.f)
	in.OperationID = "retained-sale-operation"
	in.SaleID = "retained-sale"
	in.TaxQuoteOperations = []string{q.OperationID}
	return p, policy, quote, in
}
func TestHTTPFiscalRetainedSTSaleSignedXMLAndRecovery(t *testing.T) {
	for _, crt := range []string{"1", "3"} {
		t.Run(crt, func(t *testing.T) {
			p, _, quote, in := retainedFixture(t, crt)
			if quote.Result.Total != "31.98" || quote.Result.GoodsXML.Totals["vFCPSTRet"] != "0.90" {
				t.Fatal("retention incorrectly charged")
			}
			raw, _ := json.Marshal(sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items, TaxQuoteOperations: in.TaxQuoteOperations})
			price := readQuote(t, p.f, string(raw), 200)
			if price.TotalCents != 3198 || price.Items[0].Tax == nil || price.Items[0].Tax.AdditionalCents != 0 {
				t.Fatal("cash price includes prior retention")
			}
			salesRequest(t, p.f, in, 201)
			salesRequest(t, p.f, in, 200)
			p.intent = fiscalIntent(t, p.f, `{"operation_id":"retained-intent","sale_id":"retained-sale","profile_id":"issuer-one","profile_revision":2}`, 201)
			p.input.Lines[0].TaxQuoteOperationID = quote.Input.OperationID
			p.input.Lines[0].Taxes = nil
			p.input.Lines[0].NCM = ""
			p.input.Lines[0].CFOP = ""
			p.input.Lines[0].CEST = ""
			p.input.TaxTotals = nil
			prep := p.prepare(t, "retained-review", 0, 201)
			p.approve(t, prep, 204)
			out := p.emit(t, prep, "retained-issue", 200)
			if out.State != "authorized" || p.issues.Load() != 1 || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil || !bytes.Contains(out.ProcessedXML, []byte("<vNF>31.98</vNF>")) || !bytes.Contains(out.ProcessedXML, []byte("<vICMSSTRet>2.30</vICMSSTRet>")) {
				t.Fatal("retention emission failed")
			}
			again := p.emit(t, prep, "retained-issue", 200)
			if p.issues.Load() != 1 || !bytes.Equal(out.ProcessedXML, again.ProcessedXML) {
				t.Fatal("duplicated transmission")
			}
			// Reopen the same SQLite, then restore an encrypted copy into another DB.
			var seq int
			var name, path string
			if err := p.f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
				t.Fatal(err)
			}
			if err := p.f.db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := localdb.Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			p.f.db = db
			t.Cleanup(func() { db.Close() })
			p.installApp(t, p.runtime)
			p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
			var reopened fiscal.TaxQuote
			taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &reopened)
			if reopened.ResultSHA256 != quote.ResultSHA256 || reopened.Input.Item.RetainedST.SourceKey != quote.Input.Item.RetainedST.SourceKey {
				t.Fatal("restart lost source")
			}
			restored := productionTaxBackupRoundTrip(t, p)
			copyDB, err := localdb.Open(context.Background(), restored)
			if err != nil {
				t.Fatal(err)
			}
			defer copyDB.Close()
			p.f.db = copyDB
			p.installApp(t, p.runtime)
			p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
			var after fiscal.TaxQuote
			taxRequest(t, p.f, "GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, 200, &after)
			if after.ResultSHA256 != quote.ResultSHA256 {
				t.Fatal("restore changed tax evidence")
			}
			if receipt := readReceipt(t, p.f, in.SaleID); receipt.TotalCents != 3198 || receipt.Items[0].Tax.AdditionalCents != 0 {
				t.Fatal("restored cash changed")
			}
			code, body := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token)
			if code != 200 || !bytes.Equal(body, out.ProcessedXML) {
				t.Fatal("restored XML changed")
			}
			t.Log("Real SQLite/API and signed mTLS to disposable authority: prior ST230/FCP90 remain evidence; cash3198 unchanged; replay/restart/encrypted restore preserve original source/XML.")
		})
	}
}
func TestHTTPFiscalRetainedSTOwnerAndUnitGuard(t *testing.T) {
	p := newProductionFixture(t, "55", "1", "America/Sao_Paulo")
	policy := retainedPolicy(t, p)
	q := retainedQuoteInput(t, p, policy)
	q.Item.RetainedST.Unit = "KG"
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 400, nil)
	q = retainedQuoteInput(t, p, policy)
	policyPerson(t, p.f.db, p.f.owner, "retained-accountant", "accountant")
	owner := p.f.token
	p.f.token = loginToken(t, p.f.app, "retained-accountant")
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 403, nil)
	p.f.token = owner
	var quote fiscal.TaxQuote
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, &quote)
	q.Item.RetainedST.RetainedAmount = "2.31"
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-quotes", q, 409, nil)
	var n int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_tax_quotes`).Scan(&n); err != nil || n != 1 {
		t.Fatal("invalid quote partially persisted", n, err)
	}
}
func TestHTTPFiscalRetainedSTRejectsMissingAndTamperedSource(t *testing.T) {
	p := newProductionFixture(t, "55", "3", "America/Sao_Paulo")
	policy := retainedPolicy(t, p)
	for _, kind := range []string{"missing", "quantity", "checksum", "FCP", "null", "unknown", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			q := retainedQuoteInput(t, p, policy)
			switch kind {
			case "missing":
				q.Item.RetainedST = nil
			case "quantity":
				q.Item.RetainedST.Quantity = "3"
			case "checksum":
				q.Item.RetainedST.SourceKey = "invalid"
			case "FCP":
				q.Item.RetainedST.FCP.Amount = "0.91"
			}
			raw, _ := json.Marshal(q)
			if kind == "null" {
				var m map[string]any
				_ = json.Unmarshal(raw, &m)
				m["item"].(map[string]any)["retained_st"] = nil
				raw, _ = json.Marshal(m)
			}
			if kind == "unknown" {
				raw = bytes.Replace(raw, []byte(`"source_item":2`), []byte(`"source_item":2,"extra":1`), 1)
			}
			if kind == "duplicate" {
				raw = bytes.Replace(raw, []byte(`"source_item":2`), []byte(`"source_item":2,"SOURCE_ITEM":3`), 1)
			}
			code, _ := request(t, p.f.app, "POST", "/local/v1/fiscal/tax-quotes", string(raw), p.f.token)
			if code != 400 {
				t.Fatal("bad source accepted", code)
			}
		})
	}
	var n int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_tax_quotes`).Scan(&n); err != nil || n != 0 {
		t.Fatal("invalid source persisted", n, err)
	}
}

func TestHTTPFiscalRetainedSTNativeLoopback(t *testing.T) {
	p, _, quote, in := retainedFixture(t, "1")
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
			t.Error("listener did not stop")
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
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, err = io.ReadAll(io.LimitReader(res.Body, 2*fiscalcore.MaxXML))
		if err != nil || res.StatusCode != want {
			t.Fatalf("HTTP %s %s -> %d want %d", method, path, res.StatusCode, want)
		}
		steps++
		t.Logf("HTTP %s %s -> %d", method, path, res.StatusCode)
		return raw
	}
	q := quote.Input
	q.OperationID = "native-retained-quote"
	call("POST", "/local/v1/fiscal/tax-quotes", q, 201)
	in.TaxQuoteOperations = []string{q.OperationID}
	var price sale.Quote
	if err = json.Unmarshal(call("POST", "/local/v1/sales/quote", sale.QuoteInput{CashSessionID: in.CashSessionID, Items: in.Items, TaxQuoteOperations: in.TaxQuoteOperations}, 200), &price); err != nil {
		t.Fatal(err)
	}
	if price.TotalCents != 3198 || price.Items[0].Tax.AdditionalCents != 0 {
		t.Fatal("prior ST charged again")
	}
	call("POST", "/local/v1/sales", in, 201)
	if err = json.Unmarshal(call("POST", "/local/v1/fiscal/intents", fiscal.IntentInput{OperationID: "native-retained-intent", SaleID: in.SaleID, ProfileID: "issuer-one", ProfileRevision: 2}, 201), &p.intent); err != nil {
		t.Fatal(err)
	}
	p.input.Lines[0].TaxQuoteOperationID = q.OperationID
	p.input.Lines[0].Taxes = nil
	p.input.Lines[0].NCM = ""
	p.input.Lines[0].CFOP = ""
	p.input.Lines[0].CEST = ""
	p.input.TaxTotals = nil
	var prep fiscal.ProductionPreparation
	if err = json.Unmarshal(call("POST", p.path("/prepare"), map[string]any{"operation_id": "native-retained-review", "expected_revision": 0, "invoice": p.input}, 201), &prep); err != nil {
		t.Fatal(err)
	}
	call("POST", p.path("/approve"), map[string]any{"operation_id": "native-retained-approve", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256, "tax_reference": "DISPOSABLE-REVIEW-NOT-OFFICIAL"}, 204)
	emit := map[string]any{"operation_id": "native-retained-issue", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256}
	first := call("POST", p.path("/emit"), emit, 200)
	again := call("POST", p.path("/emit"), emit, 200)
	if !bytes.Equal(first, again) || p.issues.Load() != 1 {
		t.Fatal("duplicate or changed issue")
	}
	xml := call("GET", p.path("/xml"), nil, 200)
	if fiscalcore.ValidateXML("nfe_process", xml) != nil || !bytes.Contains(xml, []byte("<vNF>31.98</vNF>")) {
		t.Fatal("invalid or incorrectly priced XML")
	}
	call("GET", "/local/v1/fiscal/tax-quotes/"+q.OperationID, nil, 200)
	if steps != 10 {
		t.Fatal(steps)
	}
	t.Log("RETAINED ST NATIVE: 10 real TCP HTTP calls; cash3198 unchanged by historical ST230/FCP90; one signed mTLS post; disposable authority, no official homologation.")
}
