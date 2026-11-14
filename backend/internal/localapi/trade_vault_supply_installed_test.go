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
	"sync"
	"sync/atomic"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/cashvault"
)

func TestTradeInstalledVaultSupplyRecoveryRestartRestore(t *testing.T) {
	runTradeFinancialInstalled(t, true, true, true)
}

func runStandaloneVaultSupply(t *testing.T, b *tradeHTTPFixture, p *pilotHTTP, run func(string, string, string, bool), setEndpoint func(string), stop func(), start func(string), dbPath, station, ownerRoot string) {
	t.Helper()
	owner := p.token
	receiver := p.login("cycle-reviewer")
	// Already proven installed flows are real HTTP preconditions, not clicked again.
	p.call("POST", "/local/v1/cash/open", map[string]any{"session_id": "supply-source-turn", "opening_cents": 10000}, 201, nil)
	p.call("POST", "/local/v1/cash/movements", map[string]any{"session_id": "supply-source-turn", "operation_id": "supply-bag-withdrawal", "kind": "withdrawal", "amount_cents": 6850, "reason": "Separate counted custody source"}, 200, nil)
	p.call("POST", "/local/v1/cash/close", map[string]any{"session_id": "supply-source-turn", "operation_id": "supply-source-close", "declared_cents": 3150}, 200, nil)
	p.call("POST", "/local/v1/cash/open", map[string]any{"session_id": "supply-target-turn", "opening_cents": 0}, 201, nil)
	p.call("POST", bagsPath, map[string]string{"operation_id": "supply-bag-create", "bag_id": "supply-bag", "withdrawal_operation_id": "supply-bag-withdrawal", "seal_id": "SUPPLY-SEALED"}, 201, nil)
	p.call("POST", bagsPath+"/supply-bag/dispatch", map[string]string{"operation_id": "supply-bag-dispatch"}, 200, nil)
	p.token = receiver
	p.call("POST", bagsPath+"/supply-bag/receive", map[string]any{"operation_id": "supply-bag-receive", "seal_id": "SUPPLY-SEALED", "counted_cents": 6800, "reason": "Original counted source with fifty cents variance"}, 200, nil)
	p.token = owner
	p.call("POST", vaultsPath, map[string]string{"operation_id": "supply-vault-create", "vault_id": "supply-vault", "name": "Supply reserve vault"}, 201, nil)
	p.token = receiver
	p.call("POST", vaultsPath+"/supply-vault/deposit", map[string]string{"operation_id": "supply-deposit", "bag_id": "supply-bag"}, 200, nil)
	p.token = owner
	var mu sync.Mutex
	var uiToken string
	var input struct {
		OperationID   string `json:"operation_id"`
		CashSessionID string `json:"cash_session_id"`
		AmountCents   int64  `json:"amount_cents"`
		Reason        string `json:"reason"`
	}
	var posts atomic.Int32
	var dropped atomic.Bool
	target, _ := url.Parse(p.service.url)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = p.client.Transport
	director := proxy.Director
	supplyPath := vaultsPath + "/supply-vault/supply"
	proxy.Director = func(r *http.Request) {
		if r.Method == "POST" && r.URL.Path == supplyPath {
			posts.Add(1)
			raw, _ := io.ReadAll(io.LimitReader(r.Body, 8193))
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(raw))
			mu.Lock()
			_ = json.Unmarshal(raw, &input)
			mu.Unlock()
		}
		director(r)
	}
	proxy.ModifyResponse = func(r *http.Response) error {
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
				uiToken = auth.Token
				mu.Unlock()
			}
		}
		if r.Request.Method == "POST" && r.Request.URL.Path == supplyPath && r.StatusCode == 200 && dropped.CompareAndSwap(false, true) {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			return io.ErrUnexpectedEOF
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(502)
		io.WriteString(w, `{"code":"discarded_response_for_acceptance"}`)
	}
	gate := httptest.NewServer(proxy)
	defer gate.Close()
	setEndpoint(gate.URL)
	run("payment-closing-supply-submit", b.f.owner.OwnerID, ownerRoot, false)
	mu.Lock()
	original := input
	p.token = uiToken
	mu.Unlock()
	if !dropped.Load() || posts.Load() != 1 || original.OperationID == "" || original.CashSessionID != "supply-target-turn" || original.AmountCents != 2000 || p.token == "" {
		t.Fatal("real installed supply response loss not exercised")
	}
	var receipt cashvault.Result
	p.call("GET", vaultsPath+"/operations/"+original.OperationID, nil, 200, &receipt)
	if receipt.Vaults[0].BalanceCents != 4800 || receipt.Entries[0].DeltaCents != -2000 || receipt.CashMovementID == nil || receipt.Entries[0].CashSessionID == nil || *receipt.Entries[0].CashSessionID != "supply-target-turn" {
		t.Fatal("supply did not conserve exact debit and credit")
	}
	stop()
	start(dbPath)
	run("payment-closing-supply-recover", b.f.owner.OwnerID, ownerRoot, true)
	if posts.Load() != 1 {
		t.Fatal("restart recovery resent supply")
	}
	mu.Lock()
	p.token = uiToken
	mu.Unlock()
	var replay cashvault.Result
	p.call("POST", supplyPath, original, 200, &replay)
	if !replay.Repeated || !reflect.DeepEqual(receipt.Entries, replay.Entries) || receipt.CashMovementID == nil || *receipt.CashMovementID != *replay.CashMovementID {
		t.Fatal("replay changed original movement")
	}
	// A missing/closed drawer and insufficient vault balance must not debit money.
	p.call("POST", supplyPath, map[string]any{"operation_id": "supply-overdraft", "cash_session_id": "supply-target-turn", "amount_cents": 4801, "reason": "Explicit rejection test"}, 409, nil)
	snapshot := func() map[string]any {
		v := map[string]any{}
		for _, path := range []string{vaultsPath + "/supply-vault", vaultsPath + "/supply-vault/statement?limit=50&offset=0", vaultsPath + "/operations/" + original.OperationID, "/local/v1/cash/current", bagsPath + "/supply-bag"} {
			var record any
			p.call("GET", path, nil, 200, &record)
			v[path] = record
		}
		return v
	}
	saved := snapshot()
	stop()
	restored := pilotCLIBackupRoundTrip(t, &stockHTTPFixture{httpContractFixture: b.f}, dbPath, station)
	start(restored)
	p.call("GET", "/local/v1/me", nil, 401, nil)
	p.call("POST", supplyPath, original, 401, nil)
	mu.Lock()
	uiToken = ""
	mu.Unlock()
	run("payment-closing-supply-restored", b.f.owner.OwnerID, ownerRoot, true)
	mu.Lock()
	p.token = uiToken
	mu.Unlock()
	if p.token == "" {
		t.Fatal("restored UI did not authenticate")
	}
	if !reflect.DeepEqual(saved, snapshot()) {
		t.Fatal("restore changed original supply, target drawer or custody")
	}
	stop()
	db, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, v := range []struct {
		sql  string
		want int64
	}{{"SELECT balance_cents FROM cash_vaults WHERE id='supply-vault'", 4800}, {"SELECT count(*) FROM cash_vault_entries WHERE kind='cash_supply'", 1}, {"SELECT sum(delta_cents) FROM cash_vault_entries WHERE kind='cash_supply'", -2000}, {"SELECT count(*) FROM cash_adjustments WHERE kind='supply'", 1}, {"SELECT sum(amount_cents) FROM cash_movements WHERE cash_session_id='supply-target-turn'", 2000}, {"SELECT sum(cash_delta_cents) FROM cash_journal_entries WHERE session_id='supply-target-turn' AND category='supply'", 2000}, {"SELECT difference_cents FROM cash_bags WHERE id='supply-bag'", -50}, {"SELECT sum(quantity_milli) FROM stock_movements", 2500}} {
		var n int64
		if err = db.QueryRow(v.sql).Scan(&n); err != nil || n != v.want {
			t.Fatal("restored supply exact totals or original source differ", err)
		}
	}
	t.Log("PASS: original6800 vault -> supply2000 -> vault4800 and drawer2000; committed response loss and explicit recovery without second UI POST; repeated movement ID, exact journals, native restart and encrypted CLI restore")
}
