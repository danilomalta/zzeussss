package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/cashcustody"
	"titansystem-backend/internal/localdb/payables"
)

func runTradeClosingCustody(t *testing.T, b *tradeHTTPFixture, p *pilotHTTP, run func(string, string, string, bool), setEndpoint func(string), stop func(), start func(string), dbPath, station, ownerRoot, reviewerRoot string) {
	t.Helper()
	// Existing real HTTP preconditions: independent approval and paid supplier.
	ownerToken := p.token
	reviewer := p.login("cycle-reviewer")
	p.token = reviewer
	p.call("POST", payablesPath+"/authenticated-payable/approve", map[string]string{"operation_id": "closing-approval", "reason": "Independent approved precondition"}, 200, nil)
	p.token = ownerToken
	p.call("POST", "/local/v1/cash/open", map[string]any{"session_id": "closing-turn", "opening_cents": 10000}, 201, nil)
	p.call("POST", "/local/v1/cash/movements", map[string]any{"session_id": "closing-turn", "operation_id": "supplier-paid-withdrawal", "kind": "withdrawal", "amount_cents": 3150, "reason": "Explicit supplier payment precondition"}, 200, nil)
	p.call("POST", payablesPath+"/authenticated-payable/cash-payments", payables.CashInput{OperationID: "closing-payment", PaymentID: "closing-payment", InstallmentID: "accepted-part", WithdrawalOperationID: "supplier-paid-withdrawal", Recipient: "Synthetic representative", Reference: "CLOSING-PAYMENT"}, 201, nil)
	p.call("POST", bagsPath, map[string]string{"operation_id": "reject-paid-source", "bag_id": "must-not-exist", "withdrawal_operation_id": "supplier-paid-withdrawal", "seal_id": "invalid-destination"}, 409, nil)
	var posts atomic.Int32
	var dropped atomic.Bool
	var mu sync.Mutex
	var in cashcustody.ActionInput
	var bagID string
	var latestUIToken string
	target, _ := url.Parse(p.service.url)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = p.client.Transport
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		if r.Method == "POST" && strings.HasPrefix(r.URL.Path, bagsPath+"/") && strings.HasSuffix(r.URL.Path, "/receive") {
			posts.Add(1)
			raw, _ := io.ReadAll(io.LimitReader(r.Body, 8193))
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(raw))
			mu.Lock()
			_ = json.Unmarshal(raw, &in)
			bagID = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, bagsPath+"/"), "/receive")
			mu.Unlock()
		}
		director(r)
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		// Observe only this disposable UI's genuine login response, in RAM. No extra
		// authentication attempt, credential injection, log or file is involved.
		if r.Request.URL.Path == "/local/v1/login" && r.StatusCode == 200 {
			raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			r.Body.Close()
			if err != nil {
				return err
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			var auth struct {
				Token string `json:"token"`
			}
			if json.Unmarshal(raw, &auth) == nil {
				mu.Lock()
				latestUIToken = auth.Token
				mu.Unlock()
			}
		}

		if r.Request.Method == "POST" && strings.HasSuffix(r.Request.URL.Path, "/receive") && r.StatusCode == 200 && dropped.CompareAndSwap(false, true) {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		io.WriteString(w, `{"code":"discarded_response_for_acceptance"}`)
	}
	gate := httptest.NewServer(proxy)
	defer gate.Close()
	setEndpoint(gate.URL)
	run("payment-closing-close", b.f.owner.OwnerID, ownerRoot, false)
	var page struct {
		Items []cashcustody.Bag `json:"items"`
	}
	p.call("GET", bagsPath+"?limit=50&offset=0", nil, 200, &page)
	if len(page.Items) != 1 || page.Items[0].DeclaredCents == nil || *page.Items[0].DeclaredCents != 6850 {
		t.Fatal("remaining cash did not become one distinct bag")
	}
	id := page.Items[0].ID
	// The receiver cannot see the declared amount before independent count.
	p.token = reviewer
	var blind cashcustody.Bag
	p.call("GET", bagsPath+"/"+id, nil, 200, &blind)
	if blind.DeclaredCents != nil || blind.CountedCents != nil {
		t.Fatal("blind receiver saw declared or counted amount")
	}
	p.token = ownerToken
	p.call("POST", bagsPath+"/"+id+"/receive", cashcustody.ActionInput{OperationID: "wrong-actor", SealID: "AUTH-CUSTODY-001", CountedCents: 6800, Reason: "Cannot review own bag"}, 403, nil)
	run("payment-closing-receive", "cycle-reviewer", reviewerRoot, false)
	if !dropped.Load() || posts.Load() != 1 {
		t.Fatal("custody response loss not exercised once")
	}
	mu.Lock()
	original, originalID := in, bagID
	mu.Unlock()
	if originalID != id || original.OperationID == "" || original.Reason == "" {
		t.Fatal("review origin missing")
	}
	p.token = reviewer
	var before cashcustody.Result
	p.call("GET", bagsPath+"/operations/"+original.OperationID, nil, 200, &before)
	if before.Bag.CountedCents == nil || *before.Bag.CountedCents != 6800 || before.Bag.DifferenceCents == nil || *before.Bag.DifferenceCents != -50 || before.Bag.History[2].Reason != original.Reason || before.Bag.History[2].ActorID != "cycle-reviewer" {
		t.Fatal("count or authorized explanation was not persisted")
	}
	stop()
	start(dbPath)
	run("payment-closing-recover", "cycle-reviewer", reviewerRoot, true)
	if posts.Load() != 1 {
		t.Fatal("custody recovery resent command")
	}
	var recovered cashcustody.Result
	p.call("GET", bagsPath+"/operations/"+original.OperationID, nil, 200, &recovered)
	if !reflect.DeepEqual(before, recovered) {
		t.Fatal("restart changed original custody receipt")
	}
	var replay cashcustody.Result
	p.call("POST", bagsPath+"/"+id+"/receive", original, 200, &replay)
	if !replay.Repeated || !reflect.DeepEqual(before.Bag, replay.Bag) {
		t.Fatal("custody replay changed receipt")
	}
	p.token = ownerToken
	var coverage cashcustody.Coverage
	p.call("GET", "/local/v1/cash/sessions/closing-turn/custody", nil, 200, &coverage)
	if !coverage.Complete || coverage.WithdrawnCents != 10000 || coverage.SupplierPaidCents != 3150 || coverage.ReceivedDeclaredCents != 6850 || coverage.ReceivedCountedCents != 6800 || coverage.DifferenceCents != -50 || coverage.UnassignedCents != 0 {
		t.Fatal("custody double counted supplier payment or hid variance")
	}
	snapshot := func() map[string]any {
		m := map[string]any{}
		for _, path := range []string{cycleOrder + "/supply-cycle", payablesPath + "/authenticated-payable", bagsPath + "/" + id, "/local/v1/cash/sessions/closing-turn/custody", "/local/v1/cash/sessions/closing-turn/report"} {
			var v any
			p.call("GET", path, nil, 200, &v)
			m[path] = v
		}
		return m
	}
	saved := snapshot()
	stop()
	restored := pilotCLIBackupRoundTrip(t, &stockHTTPFixture{httpContractFixture: b.f}, dbPath, station)
	start(restored)
	p.call("GET", "/local/v1/me", nil, 401, nil)

	mu.Lock()
	latestUIToken = ""
	mu.Unlock()
	run("payment-closing-restored", b.f.owner.OwnerID, ownerRoot, true)
	mu.Lock()
	p.token = latestUIToken
	latestUIToken = ""
	mu.Unlock()
	if p.token == "" {
		t.Fatal("restored UI did not authenticate a fresh session")
	}

	var investigation cashcustody.Review
	p.call("GET", bagsPath+"/"+id+"/review", nil, 200, &investigation)
	if investigation.State != "resolved" || investigation.Revision != 3 || investigation.OriginalCountedCents != 6800 || investigation.CountedCents != 6850 || investigation.DifferenceCents != 0 || investigation.History[1].DeltaCents != 50 {
		t.Fatal("installed investigation lost original or compensation")
	}
	if !reflect.DeepEqual(saved, snapshot()) {
		t.Fatal("restore changed closure, variance, explanation or commercial origin")
	}
	stop()
	db, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, c := range []struct {
		sql  string
		want int64
	}{
		{"SELECT count(*) FROM cash_closures WHERE declared_cents=0 AND expected_cents=0", 1},
		{"SELECT count(*) FROM cash_movements", 2},
		{"SELECT sum(amount_cents) FROM cash_movements", -10000},
		{"SELECT count(*) FROM payable_cash_payments", 1},
		{"SELECT count(*) FROM cash_bags WHERE state='received' AND declared_cents=6850 AND counted_cents=6800 AND difference_cents=-50 AND sender_id<>receiver_id AND operator_id<>receiver_id", 1},
		{"SELECT count(*) FROM cash_bag_operations", 3},
		{"SELECT count(*) FROM cash_bag_reviews", 3},
		{"SELECT count(*) FROM cash_bag_review_events", 3},
		{"SELECT sum(delta_cents) FROM cash_bag_reviews", 50},
		{"SELECT count(*) FROM cash_bag_operations WHERE action='receive' AND reason<>'' AND actor_id='cycle-reviewer'", 1},
		{"SELECT sum(quantity_milli) FROM stock_movements", 2500},
	} {
		var n int64
		if err = db.QueryRow(c.sql).Scan(&n); err != nil || n != c.want {
			t.Fatal("restored exact ledger or custody mismatch", err)
		}
	}
	t.Log("PASS: opening10000-paid3150=remaining6850; separate custody withdrawal6850; drawer0/count0; independent bag count6800/variance-50 with immutable explanation; no automatic resend; native restart and encrypted CLI restore preserve all originals and no extra cash/stock")
}
