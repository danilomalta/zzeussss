package localapi

import (
	"bytes"
	"encoding/json"
	"testing"
)

func cashSaleSchema(t *testing.T, name string, body []byte) {
	t.Helper()
	domainSchemaFile(t, "cash-sales.openapi.json", name, body)
}

func TestCashSalesOpenAPILifecycleBlindClosingAndCancellation(t *testing.T) {
	f := salesSetup(t)
	read := func(path, schema string) []byte {
		status, body := request(t, f.app, "GET", path, "", f.token)
		if status != 200 {
			t.Fatalf("contract read failed %s", path)
		}
		cashSaleSchema(t, schema, body)
		return body
	}
	current := read("/local/v1/cash/current", "Current")
	if bytes.Contains(current, []byte("expected_cents")) || bytes.Contains(current, []byte("opening_cents")) {
		t.Fatal("blind turn leaked values")
	}
	cashSaleSchema(t, "OpenInput", []byte(cashOpenBody))
	cashSaleSchema(t, "OpenResult", cashRequest(t, f.httpContractFixture, "open", cashOpenBody, 200))
	for _, movement := range []string{supplyBody, withdrawalBody} {
		cashSaleSchema(t, "MovementInput", []byte(movement))
		cashSaleSchema(t, "MovementResult", cashRequest(t, f.httpContractFixture, "movements", movement, 200))
		cashSaleSchema(t, "MovementResult", cashRequest(t, f.httpContractFixture, "movements", movement, 200))
	}
	in := salesInput(f)
	cashSaleSchema(t, "SaleInput", []byte(salesJSON(t, in)))
	cashSaleSchema(t, "SaleResult", salesRequest(t, f, in, 201))
	cashSaleSchema(t, "SaleResult", salesRequest(t, f, in, 200))
	receipt := read("/local/v1/sales/sale-one", "Receipt")
	if !bytes.Contains(receipt, []byte(`"fiscal_authorized":false`)) {
		t.Fatal("operational receipt changed fiscal state")
	}
	read("/local/v1/sales?limit=1&offset=0", "History")
	read("/local/v1/sales?limit=1&offset=100", "History")
	cashSaleSchema(t, "CancelInput", []byte(cancelBody))
	for i := 0; i < 2; i++ {
		status, body := request(t, f.app, "POST", "/local/v1/sales/sale-one/cancel", cancelBody, f.token)
		if status != 200 {
			t.Fatal("cancel contract failed")
		}
		cashSaleSchema(t, "CancelResult", body)
	}
	receipt = read("/local/v1/sales/sale-one", "Receipt")
	if !bytes.Contains(receipt, []byte(`"cancellation":`)) || !bytes.Contains(receipt, []byte(`"status":"cancelled"`)) {
		t.Fatal("cancellation absent from durable receipt")
	}
	cancellationCounts(t, f, 1, 1)
	cashSaleSchema(t, "CloseInput", []byte(cashCloseBody))
	for i := 0; i < 2; i++ {
		closed := cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
		cashSaleSchema(t, "CloseResult", closed)
		var amounts struct {
			Expected   int64 `json:"expected_cents"`
			Difference int64 `json:"difference_cents"`
		}
		if json.Unmarshal(closed, &amounts) != nil || amounts.Expected != 1200 || amounts.Difference != 0 {
			t.Fatal("cash contract conservation failed")
		}
	}
	current = read("/local/v1/cash/current", "Current")
	if !bytes.Contains(current, []byte(`"session":null`)) {
		t.Fatal("closed turn remained active")
	}
	cashSaleSchema(t, "MovementResult", cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200))
	salesRequest(t, f, in, 409)
	for _, check := range []struct {
		query string
		want  int
	}{
		{"SELECT count(*) FROM cash_adjustments", 2},
		{"SELECT count(*) FROM cash_movements", 4},
		{"SELECT count(*) FROM outbox WHERE event_type='cash.movement'", 2},
	} {
		var n int
		if f.db.QueryRow(check.query).Scan(&n) != nil || n != check.want {
			t.Fatal("cash effects duplicated")
		}
	}
}
