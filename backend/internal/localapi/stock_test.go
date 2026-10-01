package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/stock"
)

type stockHTTPFixture struct {
	*httpContractFixture
	product string
	back    string
	shelf   string
}

func stockHTTPSetup(t *testing.T) *stockHTTPFixture {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory}), 200)
	create := func(path, body string) string {
		status, result := request(t, f.app, "POST", path, body, f.token)
		if status != 201 {
			t.Fatalf("preparacao: %d %s", status, result)
		}
		var response struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(result, &response); err != nil || response.ID == "" {
			t.Fatalf("ID: %v", err)
		}
		return response.ID
	}
	return &stockHTTPFixture{httpContractFixture: f,
		product: create("/local/v1/products", testProductBody),
		back:    create("/local/v1/locations", `{"kind":"backroom","name":"Deposito"}`),
		shelf:   create("/local/v1/locations", `{"kind":"shelf","name":"Gondola"}`)}
}

func stockJSON(t *testing.T, input stock.Input) string {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func stockRequest(t *testing.T, f *stockHTTPFixture, input stock.Input, want int) []byte {
	t.Helper()
	status, body := request(t, f.app, "POST", "/local/v1/stock/operations", stockJSON(t, input), f.token)
	if status != want {
		t.Fatalf("movimento: %d esperado %d corpo=%s", status, want, body)
	}
	return body
}

func assertStockRows(t *testing.T, f *stockHTTPFixture, operations, movements, events int) {
	t.Helper()
	for _, check := range []struct {
		sql  string
		want int
	}{
		{`SELECT COUNT(*) FROM stock_operations`, operations},
		{`SELECT COUNT(*) FROM stock_movements`, movements},
		{`SELECT COUNT(*) FROM outbox WHERE event_type='stock.operation'`, events},
	} {
		var count int
		if err := f.db.QueryRow(check.sql).Scan(&count); err != nil || count != check.want {
			t.Fatalf("contagem=%d esperado=%d erro=%v", count, check.want, err)
		}
	}
}

func assertStockBalance(t *testing.T, f *stockHTTPFixture, location string, want int64) {
	t.Helper()
	status, body := request(t, f.app, "GET", "/local/v1/stock/balance?product_id="+f.product+"&location_id="+location, "", f.token)
	var result struct {
		Quantity int64 `json:"quantity_milli"`
	}
	if status != 200 {
		t.Fatalf("saldo: %d %s", status, body)
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Quantity != want {
		t.Fatalf("saldo=%d esperado=%d erro=%v", result.Quantity, want, err)
	}
}

func stockEntry(f *stockHTTPFixture, id string, quantity int64) stock.Input {
	return stock.Input{OperationID: id, Kind: "entry", ProductID: f.product, ToLocationID: f.back, QuantityMilli: quantity, Reason: "Recebimento"}
}

func TestHTTPStockEntryTransferLossAndIdempotency(t *testing.T) {
	f := stockHTTPSetup(t)
	entry := stockEntry(f, "entry-one", 10000)
	transfer := stock.Input{OperationID: "transfer-one", Kind: "transfer", ProductID: f.product,
		FromLocationID: f.back, ToLocationID: f.shelf, QuantityMilli: 3000, Reason: "Reposicao"}
	loss := stock.Input{OperationID: "loss-one", Kind: "loss", ProductID: f.product,
		FromLocationID: f.shelf, QuantityMilli: 1000, Reason: "Quebra"}
	for _, input := range []stock.Input{entry, transfer, loss} {
		if body := stockRequest(t, f, input, 201); !bytes.Contains(body, []byte(`"repeated":false`)) {
			t.Fatalf("resposta: %s", body)
		}
	}
	if body := stockRequest(t, f, transfer, 200); !bytes.Contains(body, []byte(`"repeated":true`)) {
		t.Fatalf("repeticao: %s", body)
	}
	altered := transfer
	altered.QuantityMilli = 2000
	stockRequest(t, f, altered, 409)
	assertStockBalance(t, f, f.back, 7000)
	assertStockBalance(t, f, f.shelf, 2000)
	assertStockRows(t, f, 3, 4, 3)
	var payload string
	if err := f.db.QueryRow(`SELECT payload_json FROM outbox WHERE operation_id=?`, entry.OperationID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Tenant string `json:"tenant_id"`
		Actor  string `json:"actor_id"`
		Device string `json:"device_id"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil || event.Tenant != f.owner.TenantID || event.Actor != f.owner.OwnerID || event.Device != f.owner.DeviceID {
		t.Fatalf("contexto de evento incorreto: %v", err)
	}
}

func TestHTTPStockRequiresSessionRoleAndActiveDevice(t *testing.T) {
	f := stockHTTPSetup(t)
	input := stockEntry(f, "denied", 1000)
	status, _ := request(t, f.app, "POST", "/local/v1/stock/operations", stockJSON(t, input), "")
	if status != 401 {
		t.Fatalf("sem sessao: %d", status)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	stockRequest(t, f, input, 403)
	status, _ = request(t, f.app, "GET", "/local/v1/stock/balance?product_id="+f.product+"&location_id="+f.back, "", f.token)
	if status != 403 {
		t.Fatalf("caixa consultou saldo restrito: %d", status)
	}
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, f.owner.TenantID, f.owner.DeviceID); err != nil {
		t.Fatal(err)
	}
	stockRequest(t, f, input, 401)
	assertStockRows(t, f, 0, 0, 0)
}

func TestHTTPStockMissingModuleOrVerifierDoesNotWrite(t *testing.T) {
	f := stockHTTPSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Staff}), 200)
	input := stockEntry(f, "unpaid", 1000)
	stockRequest(t, f, input, 403)
	assertStockBalance(t, f, f.back, 0)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	f.app = app
	f.token = loginToken(t, app, f.owner.OwnerID)
	stockRequest(t, f, input, 503)
	assertStockBalance(t, f, f.back, 0)
	assertStockRows(t, f, 0, 0, 0)
}

func TestHTTPStockOutboxFailureRollsBackEntireOperation(t *testing.T) {
	f := stockHTTPSetup(t)
	observed := time.Now().Unix() - 60
	if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, observed, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`CREATE TRIGGER stock_outbox_failure BEFORE INSERT ON outbox WHEN NEW.event_type='stock.operation' BEGIN SELECT RAISE(ABORT,'test outbox failure'); END`); err != nil {
		t.Fatal(err)
	}
	stockRequest(t, f, stockEntry(f, "rollback", 1000), 500)
	assertStockRows(t, f, 0, 0, 0)
	assertStockBalance(t, f, f.back, 0)
	var after int64
	if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, f.owner.TenantID).Scan(&after); err != nil || after != observed {
		t.Fatalf("observacao nao sofreu rollback: %d %v", after, err)
	}
}

func TestHTTPStockRejectsInsufficientOrUnknownLocation(t *testing.T) {
	f := stockHTTPSetup(t)
	stockRequest(t, f, stock.Input{OperationID: "insufficient", Kind: "loss", ProductID: f.product,
		FromLocationID: f.back, QuantityMilli: 1, Reason: "Quebra"}, 409)
	input := stockEntry(f, "missing-location", 1000)
	input.ToLocationID = "missing"
	stockRequest(t, f, input, 400)
	assertStockRows(t, f, 0, 0, 0)
}

func TestHTTPStockRejectsForeignCompanyAndStoreReferences(t *testing.T) {
	f := stockHTTPSetup(t)
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tenants VALUES (?,?,?)`, []any{"foreign", "Outra", "now"}},
		{`INSERT INTO products(tenant_id,id,sku,name,price_cents) VALUES (?,?,?,?,?)`, []any{"foreign", "foreign-product", "F", "Outro", 1}},
		{`INSERT INTO stores VALUES (?,?,?)`, []any{f.owner.TenantID, "other-store", "Filial"}},
		{`INSERT INTO stock_locations VALUES (?,?,?,?,?)`, []any{f.owner.TenantID, "other-store", "foreign-location", "backroom", "Deposito filial"}},
	} {
		if _, err := f.db.Exec(stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	input := stockEntry(f, "foreign-product-op", 1000)
	input.ProductID = "foreign-product"
	stockRequest(t, f, input, 400)
	input = stockEntry(f, "foreign-store-op", 1000)
	input.ToLocationID = "foreign-location"
	stockRequest(t, f, input, 400)
	assertStockRows(t, f, 0, 0, 0)
}

func TestHTTPStockRejectsAmbiguousJSONAndInvalidQuantities(t *testing.T) {
	f := stockHTTPSetup(t)
	valid := stockJSON(t, stockEntry(f, "invalid", 1000))
	for _, body := range []string{
		`{}`, `null`, valid + ` {}`,
		`{"operation_id":"a","operation_id":"b"}`,
		`{"tenant_id":"foreign"}`, `{"actor_id":"owner"}`, `{"device_id":"foreign"}`,
		`{"quantity_milli":1.5}`, `{"quantity_milli":9223372036854775808}`,
	} {
		status, _ := request(t, f.app, "POST", "/local/v1/stock/operations", body, f.token)
		if status != 400 {
			t.Fatalf("entrada invalida: %d", status)
		}
	}
	for _, quantity := range []int64{0, -1} {
		stockRequest(t, f, stockEntry(f, "bad-q", quantity), 400)
	}
	input := stockEntry(f, "bad-kind", 1000)
	input.Kind = "unknown"
	stockRequest(t, f, input, 400)
	assertStockRows(t, f, 0, 0, 0)
}

func TestHTTPStockRejectsBalanceOverflow(t *testing.T) {
	f := stockHTTPSetup(t)
	stockRequest(t, f, stockEntry(f, "max", math.MaxInt64), 201)
	stockRequest(t, f, stockEntry(f, "overflow", 1), 409)
	assertStockBalance(t, f, f.back, math.MaxInt64)
	assertStockRows(t, f, 1, 1, 1)
}

func TestHTTPStockPersistsAndReplaysAfterDatabaseReopen(t *testing.T) {
	f := stockHTTPSetup(t)
	input := stockEntry(f, "persistent", 5000)
	stockRequest(t, f, input, 201)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil || path == "" {
		t.Fatalf("arquivo descartavel: %v", err)
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
	assertStockBalance(t, f, f.back, 5000)
	stockRequest(t, f, input, 200)
	assertStockRows(t, f, 1, 1, 1)
}

func TestHTTPStockExpiredContractPreservesAuthorizedBalanceRead(t *testing.T) {
	f := stockHTTPSetup(t)
	stockRequest(t, f, stockEntry(f, "before-expiry", 1000), 201)
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1,
		IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=? WHERE tenant_id=? AND revision=1`, payload,
		ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload)), f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	stockRequest(t, f, stockEntry(f, "after-expiry", 1000), 403)
	assertStockBalance(t, f, f.back, 1000)
	assertStockRows(t, f, 1, 1, 1)
}

func TestHTTPStockReplayStillRequiresCurrentEntitlement(t *testing.T) {
	f := stockHTTPSetup(t)
	input := stockEntry(f, "past-operation", 1000)
	stockRequest(t, f, input, 201)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Staff}), 200)
	stockRequest(t, f, input, 403)
	assertStockBalance(t, f, f.back, 1000)
	assertStockRows(t, f, 1, 1, 1)
}

func TestHTTPStockConcurrentIdenticalRequestsDoNotDuplicate(t *testing.T) {
	f := stockHTTPSetup(t)
	body := stockJSON(t, stockEntry(f, "concurrent", 1000))
	type response struct {
		status int
		err    error
	}
	results := make(chan response, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/local/v1/stock/operations", bytes.NewBufferString(body))
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
	created, repeated := 0, 0
	for i := 0; i < 2; i++ {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		switch result.status {
		case 201:
			created++
		case 200:
			repeated++
		default:
			t.Fatalf("concorrencia: %d", result.status)
		}
	}
	if created != 1 || repeated != 1 {
		t.Fatalf("criadas=%d repetidas=%d", created, repeated)
	}
	assertStockBalance(t, f, f.back, 1000)
	assertStockRows(t, f, 1, 1, 1)
}
