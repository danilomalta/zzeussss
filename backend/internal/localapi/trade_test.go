package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/supplycycle"
)

type tradeHTTPFixture struct {
	f   *httpContractFixture
	key ed25519.PrivateKey
	p   *pilotHTTP
}

func newTradeHTTP(t *testing.T, buyer bool) *tradeHTTPFixture {
	t.Helper()
	var f *httpContractFixture
	if buyer {
		f, _ = cycleFixture(t)
	} else {
		f = httpContractSetup(t)
		f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 100, []modules.ID{modules.Core, modules.Inventory, modules.Orders}), 200)
	}
	public, key, e := ed25519.GenerateKey(rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.db.Exec(`UPDATE device_pairings SET public_key=? WHERE tenant_id=? AND store_id=? AND device_id=?`, []byte(public), f.device.TenantID, f.device.StoreID, f.device.DeviceID); e != nil {
		t.Fatal(e)
	}
	v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	app, e := NewWithTradeSignerAndGate(f.db, f.device, v, nil, key)
	if e != nil {
		t.Fatal(e)
	}
	f.app = app
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(l) }()
	t.Cleanup(func() {
		_ = app.ShutdownWithTimeout(3 * time.Second)
		if e := <-done; e != nil {
			t.Error(e)
		}
	})
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	t.Cleanup(tr.CloseIdleConnections)
	return &tradeHTTPFixture{f, key, &pilotHTTP{t: t, service: &pilotService{url: "http://" + l.Addr().String()}, client: &http.Client{Transport: tr, Timeout: 10 * time.Second}, token: f.token}}
}
func tradePartyOf(f *httpContractFixture) supplycycle.TradeParty {
	return supplycycle.TradeParty{TenantID: f.device.TenantID, StoreID: f.device.StoreID, DeviceID: f.device.DeviceID}
}
func tradeTrustInput(id, role, supplier string, remote *httpContractFixture, public []byte) supplycycle.TradePeerInput {
	return supplycycle.TradePeerInput{OperationID: "trust-" + id, ID: id, Remote: tradePartyOf(remote), PublicKey: public, FingerprintSHA256: supplycycle.TradeFingerprint(public), Role: role, SupplierID: supplier, VerificationReference: "Fingerprint conferido por canal independente pelo dono"}
}
func tradeBind(t *testing.T, b, s *tradeHTTPFixture) {
	t.Helper()
	b.p.call("POST", "/local/v1/company-trade/peers", tradeTrustInput("vendor", "supplier", "supplier", s.f, s.key.Public().(ed25519.PublicKey)), 200, nil)
	s.p.call("POST", "/local/v1/company-trade/peers", tradeTrustInput("customer", "customer", "", b.f, b.key.Public().(ed25519.PublicKey)), 200, nil)
}
func tradeOffer(t *testing.T, b, s *tradeHTTPFixture) supplycycle.TradeResult {
	t.Helper()
	tradeBind(t, b, s)
	var offer supplycycle.TradeResult
	b.p.call("POST", cycleOrder+"/company-trade/export", supplycycle.TradeExportInput{OperationID: "export-order", PeerID: "vendor", Reason: "Pedido aprovado pelo comercio"}, 200, &offer)
	s.p.call("POST", "/local/v1/company-trade/orders/import", supplycycle.TradeImportInput{OperationID: "import-order", PeerID: "customer", Envelope: *offer.Envelope}, 200, nil)
	return offer
}
func tradeDecide(t *testing.T, b, s *tradeHTTPFixture) supplycycle.TradeResult {
	t.Helper()
	offer := tradeOffer(t, b, s)
	var decision supplycycle.TradeResult
	s.p.call("POST", "/local/v1/company-trade/orders/"+offer.Message.ID+"/decision", supplycycle.TradeDecisionInput{OperationID: "supplier-decide", Decision: "confirmed", AcceptedMilli: 3000, PromisedAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339), Reason: "Fornecedor confirmou atendimento parcial"}, 200, &decision)
	return decision
}
func tradeImportDecision(d supplycycle.TradeResult) supplycycle.TradeImportInput {
	return supplycycle.TradeImportInput{OperationID: "apply-decision", PeerID: "vendor", Envelope: *d.Envelope}
}
func TestTradeAuthenticatedDecisionEnablesOnlyCommittedDelivery(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, b, s)
	var out supplycycle.TradeResult
	raw := b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 200, &out)
	domainSchemaFile(t, "company-trade.openapi.json", "TradeResult", raw)
	if out.Cycle.Decision.EvidenceMode != "authenticated_company" || !out.Cycle.Decision.ExternalVerified || out.Cycle.Decision.AcceptedMilli != 3000 {
		t.Fatal("missing authenticated acceptance")
	}
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 200, &out)
	if !out.Repeated {
		t.Fatal("not idempotent")
	}
	b.p.call("POST", cycleOrder+"/supplier-decisions", cycleDecisionInput(), 409, nil)
	b.p.call("POST", cycleOrder+"/deliveries", cyclePlanInput("overcommit", 4000), 409, nil)
	b.p.call("POST", cycleOrder+"/deliveries", cyclePlanInput("trade-trip", 3000), 200, nil)
	var cycle supplycycle.Cycle
	b.p.call("GET", cycleOrder+"/supply-cycle", nil, 200, &cycle)
	if cycle.Decision == nil || !cycle.Decision.ExternalVerified || cycle.AllocatedMilli != 3000 {
		t.Fatal("authenticated cycle not readable")
	}
	// Closing the relationship preserves the historic, signed decision.
	b.p.call("POST", "/local/v1/company-trade/peers/vendor/revoke", supplycycle.TradeRevokeInput{OperationID: "revoke", ExpectedRevision: 1, Reason: "Fim do relacionamento"}, 200, nil)
	b.p.call("GET", cycleOrder+"/supply-cycle", nil, 200, &cycle)
	b.p.call("GET", "/local/v1/company-trade/operations/apply-decision", nil, 200, &out)
	if !out.Cycle.Decision.ExternalVerified {
		t.Fatal("history rewritten by revocation")
	}
}
func TestTradeTrustRequiresOwnerAndMatchingFingerprint(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	in := tradeTrustInput("vendor", "supplier", "supplier", s.f, s.key.Public().(ed25519.PublicKey))
	original := b.p.token
	policyPerson(t, b.f.db, b.f.owner, "trade-manager", "manager")
	b.p.token = b.p.login("trade-manager")
	b.p.call("POST", "/local/v1/company-trade/peers", in, 403, nil)
	b.p.token = original
	in.FingerprintSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	b.p.call("POST", "/local/v1/company-trade/peers", in, 400, nil)
	in = tradeTrustInput("vendor", "supplier", "supplier", s.f, s.key.Public().(ed25519.PublicKey))
	in.Remote = tradePartyOf(b.f)
	b.p.call("POST", "/local/v1/company-trade/peers", in, 400, nil)
	var n int
	if e := b.f.db.QueryRow(`SELECT count(*) FROM company_trade_peers`).Scan(&n); e != nil || n != 0 {
		t.Fatal(n, e)
	}
}
func resignTrade(t *testing.T, in supplycycle.TradeEnvelope, k ed25519.PrivateKey, edit func(*supplycycle.TradeMessage)) supplycycle.TradeEnvelope {
	t.Helper()
	var m supplycycle.TradeMessage
	if e := json.Unmarshal(in.Payload, &m); e != nil {
		t.Fatal(e)
	}
	edit(&m)
	raw, _ := json.Marshal(m)
	return supplycycle.TradeEnvelope{Payload: raw, Signature: ed25519.Sign(k, append([]byte("titan-company-order-v1\x00"), raw...))}
}
func TestTradeRejectsTamperingReplayWrongContextAndStaleMessages(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, b, s)
	in := tradeImportDecision(decision)
	cases := []struct {
		name   string
		status int
		edit   func(*supplycycle.TradeMessage)
	}{
		{"target_tenant", 403, func(m *supplycycle.TradeMessage) { m.Target.TenantID = "other-company" }},
		{"target_store", 403, func(m *supplycycle.TradeMessage) { m.Target.StoreID = "other-store" }},
		{"target_device", 403, func(m *supplycycle.TradeMessage) { m.Target.DeviceID = "other-device" }},
		{"source_device", 403, func(m *supplycycle.TradeMessage) { m.Source.DeviceID = "other-device" }},
		{"offer_hash", 409, func(m *supplycycle.TradeMessage) {
			m.OfferSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		}},
		{"quantity", 409, func(m *supplycycle.TradeMessage) { m.QuantityMilli = 6000 }},
		{"foreign_product", 409, func(m *supplycycle.TradeMessage) { m.ProductID = "another-product" }},
		{"expiry", 403, func(m *supplycycle.TradeMessage) {
			m.CreatedAt = time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
			m.ExpiresAt = time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
		}},
		{"future", 403, func(m *supplycycle.TradeMessage) { m.CreatedAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339) }},
		{"too_much", 400, func(m *supplycycle.TradeMessage) { m.AcceptedMilli = 6000 }},
		{"wrong_order", 409, func(m *supplycycle.TradeMessage) { m.OrderID = "another-order" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bad := in
			bad.OperationID = "bad-" + c.name
			bad.Envelope = resignTrade(t, *decision.Envelope, s.key, c.edit)
			b.p.call("POST", cycleOrder+"/company-trade/decision", bad, c.status, nil)
		})
	}
	broken := in
	broken.Envelope = supplycycle.TradeEnvelope{Payload: append([]byte(nil), in.Envelope.Payload...), Signature: append([]byte(nil), in.Envelope.Signature...)}
	broken.Envelope.Signature[0] ^= 1
	b.p.call("POST", cycleOrder+"/company-trade/decision", broken, 403, nil)
	// A supplier's bearer token cannot act as the buyer, even with a valid signature.
	original := b.p.token
	b.p.token = s.p.token
	b.p.call("POST", cycleOrder+"/company-trade/decision", in, 401, nil)
	b.p.token = original
	var n int
	if e := b.f.db.QueryRow(`SELECT count(*) FROM purchase_supplier_decisions`).Scan(&n); e != nil || n != 0 {
		t.Fatal("partial write on rejection", n, e)
	}
	b.p.call("POST", cycleOrder+"/company-trade/decision", in, 200, nil)
	changed := in
	changed.Envelope = resignTrade(t, *decision.Envelope, s.key, func(m *supplycycle.TradeMessage) { m.Reason = "another agreement" })
	b.p.call("POST", cycleOrder+"/company-trade/decision", changed, 409, nil)
	changed = in
	changed.OperationID = "duplicate-under-new-operation"
	b.p.call("POST", cycleOrder+"/company-trade/decision", changed, 409, nil)
}
func TestTradeRevocationBlocksNewDecisionAndReceiptsRemainRecoverable(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, b, s)
	b.p.call("POST", "/local/v1/company-trade/peers/vendor/revoke", supplycycle.TradeRevokeInput{OperationID: "revoke", ExpectedRevision: 1, Reason: "Acesso encerrado"}, 200, nil)
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 403, nil)
	b.p.call("GET", "/local/v1/company-trade/operations/export-order", nil, 200, nil)
	s.p.call("POST", "/local/v1/company-trade/peers/customer/revoke", supplycycle.TradeRevokeInput{OperationID: "revoke", ExpectedRevision: 1, Reason: "Acesso encerrado"}, 200, nil)
	s.p.call("GET", "/local/v1/company-trade/operations/supplier-decide", nil, 200, nil)
	for _, q := range []string{`DELETE FROM company_trade_peers`, `UPDATE company_trade_peers SET status='active'`, `DELETE FROM company_trade_messages`, `UPDATE company_trade_messages SET envelope_json='{}'`, `DELETE FROM company_trade_operations`} {
		if _, e := b.f.db.Exec(q); e == nil {
			t.Fatal("mutable trade history", q)
		}
	}
}
func TestTradeApplyRollsBackAllEffectsIfAuditCannotPersist(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, b, s)
	if _, e := b.f.db.Exec(`CREATE TRIGGER fail_trade_receipt BEFORE INSERT ON company_trade_operations BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 409, nil)
	for _, table := range []string{"purchase_supplier_decisions", "company_trade_applied", "purchase_cycle_operations"} {
		var n int
		if e := b.f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal("non-atomic", table, n, e)
		}
	}
	var n int
	if e := b.f.db.QueryRow(`SELECT count(*) FROM company_trade_messages WHERE kind='decision'`).Scan(&n); e != nil || n != 0 {
		t.Fatal("orphan decision", n, e)
	}
	if _, e := b.f.db.Exec(`DROP TRIGGER fail_trade_receipt`); e != nil {
		t.Fatal(e)
	}
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 200, nil)
}
func TestTradeConcurrentDecisionImportsHaveExactlyOneEffect(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, b, s)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(tradeImportDecision(decision))
			status, _ := request(t, b.f.app, "POST", cycleOrder+"/company-trade/decision", string(body), b.f.token)
			if status != 200 {
				t.Errorf("concurrent import: %d", status)
			}
		}()
	}
	wg.Wait()
	for _, table := range []string{"purchase_supplier_decisions", "company_trade_applied"} {
		var n int
		if e := b.f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != 1 {
			t.Fatal(table, n, e)
		}
	}
}
func TestTradeDecisionDeclineDoesNotAuthorizeDelivery(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	offer := tradeOffer(t, b, s)
	var out supplycycle.TradeResult
	s.p.call("POST", "/local/v1/company-trade/orders/"+offer.Message.ID+"/decision", supplycycle.TradeDecisionInput{OperationID: "decline", Decision: "declined", Reason: "Sem capacidade para este atendimento"}, 200, &out)
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(out), 200, &out)
	b.p.call("POST", cycleOrder+"/deliveries", cyclePlanInput("trade-trip", 1000), 409, nil)
	if out.Cycle.Decision.Decision != "declined" || out.Cycle.Decision.AcceptedMilli != 0 {
		t.Fatal("decline was changed")
	}
}
func TestTradeConstructorRejectsForeignSigningKeyAndReadReceiptsAreActorScoped(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	if _, e := NewWithTradeSignerAndGate(b.f.db, b.f.device, nil, nil, s.key); e == nil {
		t.Fatal("foreign signer accepted")
	}
	tradeOffer(t, b, s)
	var before supplycycle.TradeResult
	b.p.call("GET", "/local/v1/company-trade/operations/export-order", nil, 200, &before)
	policyPerson(t, b.f.db, b.f.owner, "trade-reader", "manager")
	original := b.p.token
	b.p.token = b.p.login("trade-reader")
	b.p.call("GET", "/local/v1/company-trade/operations/export-order", nil, 404, nil)
	b.p.token = original
	var after supplycycle.TradeResult
	b.p.call("GET", "/local/v1/company-trade/operations/export-order", nil, 200, &after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("receipt changed")
	}
	b.p.call("POST", "/local/v1/logout", map[string]any{}, 204, nil)
	b.p.call("POST", cycleOrder+"/company-trade/export", supplycycle.TradeExportInput{OperationID: "stale-session", PeerID: "vendor", Reason: "old session"}, 401, nil)
}

func TestTradeMessageAndPeerDiscoveryPaginationPermissionsAndStrictInputs(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, b, s)
	var page supplycycle.TradeMessagePage
	s.p.call("GET", "/local/v1/company-trade/messages?direction=received&kind=order&limit=1", nil, 200, &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Message.OrderID != "order" || page.HasMore {
		t.Fatal("inbound order unavailable")
	}
	s.p.call("GET", "/local/v1/company-trade/messages/"+page.Items[0].Message.ID, nil, 200, nil)
	s.p.call("GET", "/local/v1/company-trade/messages?limit=1&offset=1", nil, 200, &page)
	if page.Total != 2 || len(page.Items) != 1 || page.HasMore {
		t.Fatal("incorrect pagination")
	}
	s.p.call("GET", "/local/v1/company-trade/messages?offset=99", nil, 200, &page)
	if page.Total != 2 || len(page.Items) != 0 || page.HasMore {
		t.Fatal("out of range pagination")
	}
	var peers supplycycle.TradePeerPage
	b.p.call("GET", "/local/v1/company-trade/peers?limit=1", nil, 200, &peers)
	if peers.Total != 1 || len(peers.Items) != 1 || peers.Items[0].ID != "vendor" {
		t.Fatal("peer discovery")
	}
	for _, q := range []string{"?kind=unknown", "?direction=other", "?limit=101", "?offset=-1", "?limit=1&limit=2", "?tenant_id=another"} {
		b.p.call("GET", "/local/v1/company-trade/messages"+q, nil, 400, nil)
	}
	original := s.p.token
	s.p.token = b.p.token
	s.p.call("GET", "/local/v1/company-trade/messages", nil, 401, nil)
	s.p.token = original
	b.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 200, nil)
	b.p.call("POST", cycleOrder+"/company-trade/decision", json.RawMessage(`{"operation_id":"bad","peer_id":"vendor","envelope":{"payload":"AA==","payload":"AA==","signature":"AA=="}}`), 400, nil)
	b.p.call("POST", "/local/v1/company-trade/peers", json.RawMessage(`{"operation_id":"bad","id":"bad","remote":{"tenant_id":"x","store_id":"y","device_id":"z","device_id":"z"},"public_key":"AA==","fingerprint_sha256":"x","role":"customer","supplier_id":"","verification_reference":"test"}`), 400, nil)
}

func restoredTradeHTTP(t *testing.T, source *tradeHTTPFixture, path string) *tradeHTTPFixture {
	t.Helper()
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = db.Close() })
	f := *source.f
	f.db = db
	v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	app, e := NewWithTradeSignerAndGate(db, f.device, v, nil, source.key)
	if e != nil {
		t.Fatal(e)
	}
	f.app = app
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(l) }()
	t.Cleanup(func() {
		_ = app.ShutdownWithTimeout(3 * time.Second)
		if e := <-done; e != nil {
			t.Error(e)
		}
	})
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	t.Cleanup(tr.CloseIdleConnections)
	p := &pilotHTTP{t: t, service: &pilotService{url: "http://" + l.Addr().String()}, client: &http.Client{Transport: tr, Timeout: 10 * time.Second}, token: f.token}
	p.call("GET", "/local/v1/me", nil, 401, nil)
	p.token = p.login(f.owner.OwnerID)
	f.token = p.token
	return &tradeHTTPFixture{&f, source.key, p}
}
func TestTradeOldBackupCannotSilentlyResurrectTrustOrReplayOwnerReview(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, b, s)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "older.tytbak")
	target := filepath.Join(dir, "restored.sqlite")
	if e := backup.Create(ctx, b.f.db, b.f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	b.p.call("POST", "/local/v1/company-trade/peers/vendor/revoke", supplycycle.TradeRevokeInput{OperationID: "revoke-after-backup", ExpectedRevision: 1, Reason: "Revogacao posterior ao snapshot"}, 200, nil)
	if e := backup.Restore(ctx, archive, target, b.f.device, key); e != nil {
		t.Fatal(e)
	}
	restored := restoredTradeHTTP(t, b, target)
	var state supplycycle.TradeRecovery
	restored.p.call("GET", "/local/v1/company-trade/recovery", nil, 200, &state)
	if !state.Required || state.Generation == "" {
		t.Fatal("restored trust was automatically active")
	}
	restored.p.call("GET", "/local/v1/company-trade/operations/export-order", nil, 200, nil)
	restored.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 503, nil)
	review := supplycycle.TradeRecoveryInput{OperationID: "owner-review", Generation: state.Generation, VerificationReference: "Dono revisou todas as digitais com contrapartes por canal independente"}
	policyPerson(t, restored.f.db, restored.f.owner, "restored-manager", "manager")
	original := restored.p.token
	restored.p.token = restored.p.login("restored-manager")
	restored.p.call("POST", "/local/v1/company-trade/recovery/review", review, 403, nil)
	restored.p.token = original
	wrong := review
	wrong.Generation = "old-generation"
	restored.p.call("POST", "/local/v1/company-trade/recovery/review", wrong, 409, nil)
	restored.p.call("POST", "/local/v1/company-trade/recovery/review", review, 200, nil)
	restored.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 200, nil)
	second := filepath.Join(dir, "reviewed.tytbak")
	third := filepath.Join(dir, "restored-again.sqlite")
	if e := backup.Create(ctx, restored.f.db, b.f.device, key, second); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(ctx, second, third, b.f.device, key); e != nil {
		t.Fatal(e)
	}
	again := restoredTradeHTTP(t, restored, third)
	var next supplycycle.TradeRecovery
	again.p.call("GET", "/local/v1/company-trade/recovery", nil, 200, &next)
	if !next.Required || next.Generation == state.Generation {
		t.Fatal("restoration reused reviewed generation")
	}
	again.p.call("POST", "/local/v1/company-trade/recovery/review", review, 409, nil)
	again.p.call("GET", "/local/v1/company-trade/operations/owner-review", nil, 200, nil)
	again.p.call("GET", cycleOrder+"/supply-cycle", nil, 200, nil)
	again.p.call("POST", cycleOrder+"/company-trade/decision", tradeImportDecision(decision), 503, nil)
	var peers supplycycle.TradePeerPage
	b.p.call("GET", "/local/v1/company-trade/peers", nil, 200, &peers)
	if peers.Items[0].Status != "revoked" {
		t.Fatal("restore changed original source")
	}
}
func TestTradeRestoreRefusesToPublishIfRecoveryGateCannotPersist(t *testing.T) {
	b, s := newTradeHTTP(t, true), newTradeHTTP(t, false)
	tradeOffer(t, b, s)
	if _, e := b.f.db.Exec(`CREATE TRIGGER ignore_trade_recovery BEFORE INSERT ON company_trade_recovery BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "ignored.tytbak")
	target := filepath.Join(dir, "not-published.sqlite")
	if e := backup.Create(context.Background(), b.f.db, b.f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e := backup.Restore(context.Background(), archive, target, b.f.device, key); e == nil {
		t.Fatal("published unguarded trust")
	}
	if _, e := os.Stat(target); !os.IsNotExist(e) {
		t.Fatal("unsafe destination published", e)
	}
}
