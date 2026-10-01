package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
)

func cashSetup(t *testing.T) *httpContractFixture {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1,
		[]modules.ID{modules.Core, modules.Inventory, modules.POS}), 200)
	return f
}

func cashRequest(t *testing.T, f *httpContractFixture, path, body string, want int) []byte {
	t.Helper()
	status, result := request(t, f.app, "POST", "/local/v1/cash/"+path, body, f.token)
	if status != want {
		t.Fatalf("%s: recebido=%d esperado=%d corpo=%s", path, status, want, result)
	}
	return result
}

func cashCounts(t *testing.T, f *httpContractFixture, turns, closures, events int) {
	t.Helper()
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM cash_sessions`, turns},
		{`SELECT COUNT(*) FROM cash_session_operators`, turns},
		{`SELECT COUNT(*) FROM cash_closures`, closures},
		{`SELECT COUNT(*) FROM outbox WHERE event_type IN ('cash.open','cash.close')`, events},
	} {
		var got int
		if err := f.db.QueryRow(check.query).Scan(&got); err != nil || got != check.want {
			t.Fatalf("contagem=%d esperado=%d erro=%v consulta=%s", got, check.want, err, check.query)
		}
	}
}

const cashOpenBody = `{"session_id":"turn-one","opening_cents":1000}`
const cashCloseBody = `{"session_id":"turn-one","operation_id":"close-one","declared_cents":1200}`

func TestHTTPCashBlindClosingAndIdempotency(t *testing.T) {
	f := cashSetup(t)
	status, body := request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"session":null`)) {
		t.Fatalf("sem turno: %d %s", status, body)
	}
	cashRequest(t, f, "open", cashOpenBody, 201)
	body = cashRequest(t, f, "open", cashOpenBody, 200)
	if !bytes.Contains(body, []byte(`"repeated":true`)) || bytes.Contains(body, []byte("expected_cents")) {
		t.Fatalf("abertura nao cega: %s", body)
	}
	cashRequest(t, f, "open", `{"session_id":"turn-two","opening_cents":0}`, 409)
	cashRequest(t, f, "open", `{"session_id":"turn-one","opening_cents":999}`, 409)
	if _, err := f.db.Exec(`INSERT INTO cash_movements(id,tenant_id,store_id,cash_session_id,amount_cents,reason,created_at)
		VALUES ('test-movement',?,?,'turn-one',245,'test','now')`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	status, body = request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte("turn-one")) || bytes.Contains(body, []byte("expected_cents")) || bytes.Contains(body, []byte("1245")) {
		t.Fatalf("consulta revelou saldo: %d %s", status, body)
	}
	body = cashRequest(t, f, "close", cashCloseBody, 200)
	if !bytes.Contains(body, []byte(`"expected_cents":1245`)) || !bytes.Contains(body, []byte(`"difference_cents":-45`)) {
		t.Fatalf("conferencia: %s", body)
	}
	body = cashRequest(t, f, "close", cashCloseBody, 200)
	if !bytes.Contains(body, []byte(`"repeated":true`)) {
		t.Fatalf("repeticao: %s", body)
	}
	cashRequest(t, f, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":1245}`, 409)
	cashRequest(t, f, "open", cashOpenBody, 409)
	cashCounts(t, f, 1, 1, 2)
	status, body = request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"session":null`)) {
		t.Fatalf("turno fechado ainda aberto: %d %s", status, body)
	}
}

func TestHTTPCashRequiresPOSAndConfiguredVerifier(t *testing.T) {
	f := httpContractSetup(t)
	cashRequest(t, f, "open", cashOpenBody, 403)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1,
		[]modules.ID{modules.Core, modules.Inventory}), 200)
	cashRequest(t, f, "open", cashOpenBody, 403)
	cashRequest(t, f, "close", cashCloseBody, 403)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	f.app, f.token = app, loginToken(t, app, f.owner.OwnerID)
	cashRequest(t, f, "open", cashOpenBody, 503)
	cashRequest(t, f, "close", cashCloseBody, 503)
	status, _ := request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 200 {
		t.Fatalf("consulta sem emissor: %d", status)
	}
	cashCounts(t, f, 0, 0, 0)
}

func TestHTTPCashSessionRoleAndRevocation(t *testing.T) {
	f := cashSetup(t)
	status, _ := request(t, f.app, "POST", "/local/v1/cash/open", cashOpenBody, "")
	if status != 401 {
		t.Fatalf("sem sessao: %d", status)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='stock' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "open", cashOpenBody, 403)
	status, _ = request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 403 {
		t.Fatalf("estoquista consultou caixa: %d", status)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "open", cashOpenBody, 201)
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, f.owner.TenantID, f.owner.DeviceID); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "close", cashCloseBody, 401)
	cashCounts(t, f, 1, 0, 1)
}

func TestHTTPCashRejectsAmbiguousAndInexactInput(t *testing.T) {
	f := cashSetup(t)
	for _, body := range []string{
		`{}`, `null`, cashOpenBody + ` {}`,
		`{"session_id":"a","session_id":"b","opening_cents":0}`,
		`{"session_id":"a","opening_cents":null}`,
		`{"session_id":"a","opening_cents":1.5}`,
		`{"session_id":"a","opening_cents":9223372036854775808}`,
		`{"session_id":"a","opening_cents":-1}`,
		`{"session_id":"a","opening_cents":0,"tenant_id":"foreign"}`,
		`{"session_id":"a","opening_cents":0,"identity_id":"someone"}`,
		`{"session_id":"a","opening_cents":0,"device_id":"someone"}`,
	} {
		cashRequest(t, f, "open", body, 400)
	}
	for _, body := range []string{
		`{"session_id":"a","operation_id":"x"}`,
		`{"session_id":"a","operation_id":"x","declared_cents":null}`,
		`{"session_id":"a","operation_id":"x","declared_cents":-1}`,
		`{"session_id":"a","operation_id":"x","declared_cents":1.2}`,
		`{"session_id":"a","operation_id":"x","declared_cents":0,"store_id":"foreign"}`,
	} {
		cashRequest(t, f, "close", body, 400)
	}
	cashCounts(t, f, 0, 0, 0)
}

func TestHTTPCashOutboxFailureRollsBackOpeningAndClosing(t *testing.T) {
	f := cashSetup(t)
	observed := time.Now().Unix() - 30
	setObservation := func() {
		if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=? WHERE tenant_id=?`, observed, f.owner.TenantID); err != nil {
			t.Fatal(err)
		}
	}
	assertObservation := func() {
		var got int64
		if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, f.owner.TenantID).Scan(&got); err != nil || got != observed {
			t.Fatalf("observacao nao sofreu rollback: %d %v", got, err)
		}
	}
	setObservation()
	// Failure affects only this disposable test database.
	if _, err := f.db.Exec(`CREATE TRIGGER cash_open_failure BEFORE INSERT ON outbox WHEN NEW.event_type='cash.open' BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "open", cashOpenBody, 500)
	cashCounts(t, f, 0, 0, 0)
	assertObservation()
	if _, err := f.db.Exec(`DROP TRIGGER cash_open_failure`); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "open", cashOpenBody, 201)
	setObservation()
	if _, err := f.db.Exec(`CREATE TRIGGER cash_close_failure BEFORE INSERT ON outbox WHEN NEW.event_type='cash.close' BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "close", cashCloseBody, 500)
	cashCounts(t, f, 1, 0, 1)
	assertObservation()
	status, body := request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte("turn-one")) {
		t.Fatalf("fechamento parcial: %d %s", status, body)
	}
}

func TestHTTPCashPersistsAfterReopen(t *testing.T) {
	f := cashSetup(t)
	cashRequest(t, f, "open", cashOpenBody, 201)
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
	cashRequest(t, f, "open", cashOpenBody, 200)
	cashRequest(t, f, "close", cashCloseBody, 200)
	cashCounts(t, f, 1, 1, 2)
}

func TestHTTPCashConcurrentOpeningDoesNotDuplicate(t *testing.T) {
	f := cashSetup(t)
	type response struct {
		status int
		err    error
	}
	results := make(chan response, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req := httptest.NewRequest("POST", "/local/v1/cash/open", bytes.NewBufferString(cashOpenBody))
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
		res := <-results
		if res.err != nil {
			t.Fatal(res.err)
		}
		switch res.status {
		case 201:
			created++
		case 200:
			replayed++
		default:
			t.Fatalf("concorrencia: %d", res.status)
		}
	}
	if created != 1 || replayed != 1 {
		t.Fatalf("criadas=%d repetidas=%d", created, replayed)
	}
	cashCounts(t, f, 1, 0, 1)
}

func TestHTTPCashExpiredContractKeepsBlindReadButBlocksMutations(t *testing.T) {
	f := cashSetup(t)
	cashRequest(t, f, "open", cashOpenBody, 201)
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1,
		IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60,
		Modules: []modules.ID{modules.Core, modules.Inventory, modules.POS}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=? WHERE tenant_id=? AND revision=1`, payload,
		ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload)), f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "close", cashCloseBody, 403)
	cashRequest(t, f, "open", cashOpenBody, 403)
	status, body := request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte("turn-one")) || bytes.Contains(body, []byte("expected_cents")) {
		t.Fatalf("consulta: %d %s", status, body)
	}
	cashCounts(t, f, 1, 0, 1)
}

func TestHTTPCashClosureOperationCannotBeReusedForAnotherTurn(t *testing.T) {
	f := cashSetup(t)
	cashRequest(t, f, "close", cashCloseBody, 409)
	cashRequest(t, f, "open", cashOpenBody, 201)
	cashRequest(t, f, "close", cashCloseBody, 200)
	cashRequest(t, f, "open", `{"session_id":"turn-two","opening_cents":0}`, 201)
	cashRequest(t, f, "close", `{"session_id":"turn-two","operation_id":"close-one","declared_cents":0}`, 409)
	cashCounts(t, f, 2, 1, 3)
}
