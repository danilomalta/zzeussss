package localapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// TCP on both sides: client -> actual Fiber API -> signed mTLS request to a
// disposable authority. All persistence uses the production SQLite stores.
func TestHTTPFiscalProductionTaxNativeLoopback(t *testing.T) {
	p := newProductionFixture(t, "55", "3")
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
	call := func(method, path string, body any, token string, want int) []byte {
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
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err = io.ReadAll(io.LimitReader(response.Body, 2*fiscalcore.MaxXML))
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s -> %d want %d %s", method, path, response.StatusCode, want, raw)
		}
		if response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("fiscal result cached")
		}
		steps++
		t.Logf("HTTP %s %s -> %d", method, path, response.StatusCode)
		return raw
	}
	call("GET", "/local/v1/fiscal/tax-policies/policy-one", nil, "", 401)
	policy := productionTaxPolicyInput(p)
	call("POST", "/local/v1/fiscal/tax-policies", policy, p.f.token, 201)
	quote := taxQuoteFixture(policy)
	quote.Context.CRT = "3"
	quote.Context.Model = "55"
	zone, _ := time.LoadLocation(p.binding.Timezone)
	quote.Context.Date = time.Now().In(zone).Format("2006-01-02")
	line := p.input.Lines[0]
	quote.Item = fiscalcore.TaxItem{ProductID: line.ProductID, Quantity: line.Quantity, UnitPrice: line.UnitPrice, Discount: line.Discount, Freight: line.Freight, Insurance: line.Insurance, Other: line.Other}
	var calculated fiscal.TaxQuote
	if err = json.Unmarshal(call("POST", "/local/v1/fiscal/tax-quotes", quote, p.f.token, 201), &calculated); err != nil {
		t.Fatal(err)
	}
	p.input.Lines[0].TaxQuoteOperationID = quote.OperationID
	p.input.Lines[0].NCM = ""
	p.input.Lines[0].CFOP = ""
	p.input.Lines[0].Taxes = nil
	p.input.TaxTotals = nil
	var prep fiscal.ProductionPreparation
	if err = json.Unmarshal(call("POST", p.path("/prepare"), map[string]any{"operation_id": "native-tax-review", "expected_revision": 0, "invoice": p.input}, p.f.token, 201), &prep); err != nil {
		t.Fatal(err)
	}
	call("POST", p.path("/approve"), map[string]any{"operation_id": "native-tax-approval", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256, "tax_reference": "SYNTHETIC-REVIEW-NOT-OFFICIAL"}, p.f.token, 204)
	emit := map[string]any{"operation_id": "native-tax-issue", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256}
	var evidence fiscalcore.ProductionEvidence
	if err = json.Unmarshal(call("POST", p.path("/emit"), emit, p.f.token, 200), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.State != "authorized" || p.issues.Load() != 1 || fiscalcore.ValidateXML("nfe_process", evidence.ProcessedXML) != nil {
		t.Fatal("fixture fiscal transport failed")
	}
	if !bytes.Equal(call("GET", p.path("/xml"), nil, p.f.token, 200), evidence.ProcessedXML) {
		t.Fatal("download changed original XML")
	}
	var recovered fiscal.TaxQuote
	if err = json.Unmarshal(call("GET", "/local/v1/fiscal/tax-quotes/quote-one", nil, p.f.token, 200), &recovered); err != nil {
		t.Fatal(err)
	}
	if recovered.ResultSHA256 != calculated.ResultSHA256 || recovered.PolicySHA256 != calculated.PolicySHA256 {
		t.Fatal("receipt source changed")
	}
	call("POST", p.path("/emit"), emit, p.f.token, 200)
	call("GET", p.path(""), nil, p.f.token, 200)
	if p.issues.Load() != 1 {
		t.Fatal("repeat transmitted again")
	}
	t.Logf("TAX NATIVE HTTP: %d real client requests, SQLite policy/quote/review/approval, official XSD and one signed mTLS POST; authority and data disposable, not official homologation", steps)
}
