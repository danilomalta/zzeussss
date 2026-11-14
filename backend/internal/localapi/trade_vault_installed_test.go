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
	"titansystem-backend/internal/localdb/cashvault"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/payables"
)

func runTradeVaultCustody(t *testing.T, b *tradeHTTPFixture, p *pilotHTTP, run func(string, string, string, bool), setEndpoint func(string), stop func(), start func(string), dbPath, station, ownerRoot, reviewerRoot string) string {
	t.Helper()
	var mu sync.Mutex
	var vaultID, uiToken string
	var input cashvault.Input
	var posts atomic.Int32
	var dropped atomic.Bool
	target, _ := url.Parse(p.service.url)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = p.client.Transport
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		if r.Method == "POST" && (r.URL.Path == vaultsPath || strings.HasSuffix(r.URL.Path, "/deposit")) {
			raw, _ := io.ReadAll(io.LimitReader(r.Body, 8193))
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(raw))
			mu.Lock()
			if r.URL.Path == vaultsPath {
				var create cashvault.CreateInput
				_ = json.Unmarshal(raw, &create)
				vaultID = create.VaultID
			} else {
				posts.Add(1)
				_ = json.Unmarshal(raw, &input)
			}
			mu.Unlock()
		}
		director(r)
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		if r.Request.URL.Path == "/local/v1/login" {
			t.Logf("Disposable UI login HTTP %d", r.StatusCode)
		}
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
		if r.Request.Method == "POST" && strings.HasSuffix(r.Request.URL.Path, "/deposit") && r.StatusCode == 200 && dropped.CompareAndSwap(false, true) {
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
	run("payment-closing-vault-create", b.f.owner.OwnerID, ownerRoot, false)
	mu.Lock()
	id := vaultID
	mu.Unlock()
	if id == "" {
		t.Fatal("installed vault creation did not persist an ID")
	}
	var empty cashvault.Vault
	p.call("GET", vaultsPath+"/"+id, nil, 200, &empty)
	if empty.BalanceCents != 0 {
		t.Fatal("vault did not start empty")
	}
	run("payment-closing-vault-deposit", "cycle-reviewer", reviewerRoot, false)
	mu.Lock()
	original := input
	receiverToken := uiToken
	mu.Unlock()
	if !dropped.Load() || posts.Load() != 1 || original.OperationID == "" || receiverToken == "" {
		t.Fatal("deposit committed response loss not exercised")
	}
	p.token = receiverToken
	var before cashvault.Result
	p.call("GET", vaultsPath+"/operations/"+original.OperationID, nil, 200, &before)
	if len(before.Entries) != 1 || before.Entries[0].DeltaCents != 6800 || before.Vaults[0].BalanceCents != 6800 {
		t.Fatal("deposit used investigation amount instead of original received count")
	}
	stop()
	start(dbPath)
	run("payment-closing-vault-recover", "cycle-reviewer", reviewerRoot, true)
	if posts.Load() != 1 {
		t.Fatal("deposit recovery automatically resent")
	}
	var after cashvault.Result
	p.call("GET", vaultsPath+"/operations/"+original.OperationID, nil, 200, &after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("restart changed original deposit receipt")
	}
	var replay cashvault.Result
	p.call("POST", vaultsPath+"/"+id+"/deposit", map[string]string{"operation_id": original.OperationID, "bag_id": original.BagID}, 200, &replay)
	if !replay.Repeated || !reflect.DeepEqual(before.Entries, replay.Entries) {
		t.Fatal("explicit replay duplicated deposit")
	}
	p.call("POST", vaultsPath+"/"+id+"/deposit", map[string]string{"operation_id": "second-deposit", "bag_id": original.BagID}, 409, nil)
	snapshot := func() map[string]any {
		v := map[string]any{}
		for _, path := range []string{vaultsPath + "/" + id, vaultsPath + "/" + id + "/statement?limit=50&offset=0", bagsPath + "/" + original.BagID, bagsPath + "/" + original.BagID + "/review"} {
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
	mu.Lock()
	uiToken = ""
	mu.Unlock()
	run("payment-closing-vault-restored", b.f.owner.OwnerID, ownerRoot, true)
	mu.Lock()
	p.token = uiToken
	mu.Unlock()
	if p.token == "" {
		t.Fatal("restored UI did not establish its own authenticated session")
	}
	if !reflect.DeepEqual(saved, snapshot()) {
		t.Fatal("restore changed vault, statement, bag or investigation")
	}
	p.call("GET", vaultsPath+"/operations/"+original.OperationID, nil, 404, nil)
	stop()
	db, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := identity.Scope{TenantID: b.f.owner.TenantID, StoreID: b.f.owner.StoreID, IdentityID: "cycle-reviewer"}
	receipt, err := cashvault.Receipt(context.Background(), db, actor, b.f.device, original.OperationID)
	if err != nil || !reflect.DeepEqual(receipt, before) {
		t.Fatal("restoration lost original receiver deposit receipt", err)
	}
	for _, check := range []struct {
		sql  string
		want int64
	}{{"SELECT count(*) FROM cash_vault_deposits", 1}, {"SELECT sum(delta_cents) FROM cash_vault_entries", 6800}, {"SELECT sum(balance_cents) FROM cash_vaults", 6800}, {"SELECT sum(amount_cents) FROM cash_movements", -10000}, {"SELECT sum(delta_cents) FROM cash_bag_reviews", 50}} {
		var n int64
		if err = db.QueryRow(check.sql).Scan(&n); err != nil || n != check.want {
			t.Fatal("restored custody/vault exact totals differ", err)
		}
	}
	start(restored)
	t.Log("PASS: installed vault deposit credits original6800, not recounted6850; one deposit, explicit recovery/replay, original variance retained, restart and CLI restore preserve balance/history and revoke old sessions")
	return restored
}

// Prior installed proofs cover the commercial preconditions. This unit uses real
// API preconditions and spends its installed acceptance on the vault operations.
func runStandaloneVaultCustody(t *testing.T, b *tradeHTTPFixture, p *pilotHTTP, run func(string, string, string, bool), setEndpoint func(string), stop func(), start func(string), dbPath, station, ownerRoot, reviewerRoot string) {
	t.Helper()
	owner := p.token
	receiver := p.login("cycle-reviewer")
	p.token = receiver
	p.call("POST", payablesPath+"/authenticated-payable/approve", map[string]string{"operation_id": "vault-precondition-approval", "reason": "Independent approved source"}, 200, nil)
	p.token = owner
	p.call("POST", "/local/v1/cash/open", map[string]any{"session_id": "vault-turn", "opening_cents": 10000}, 201, nil)
	for _, v := range []struct {
		id     string
		amount int64
	}{{"vault-paid-source", 3150}, {"vault-bag-source", 6850}} {
		p.call("POST", "/local/v1/cash/movements", map[string]any{"session_id": "vault-turn", "operation_id": v.id, "kind": "withdrawal", "amount_cents": v.amount, "reason": "Explicit separate source"}, 200, nil)
	}
	p.call("POST", payablesPath+"/authenticated-payable/cash-payments", payables.CashInput{OperationID: "vault-payment", PaymentID: "vault-payment", InstallmentID: "accepted-part", WithdrawalOperationID: "vault-paid-source", Recipient: "Synthetic representative", Reference: "VAULT-PAYMENT"}, 201, nil)
	p.call("POST", bagsPath, cashcustody.CreateInput{OperationID: "vault-prepare", BagID: "vault-bag", WithdrawalOperationID: "vault-bag-source", SealID: "AUTH-CUSTODY-001"}, 201, nil)
	p.call("POST", bagsPath+"/vault-bag/dispatch", map[string]string{"operation_id": "vault-dispatch"}, 200, nil)
	p.token = receiver
	p.call("POST", bagsPath+"/vault-bag/receive", cashcustody.ActionInput{OperationID: "vault-receive", SealID: "AUTH-CUSTODY-001", CountedCents: 6800, Reason: "Original blind receipt"}, 200, nil)
	p.token = owner
	for i, action := range []string{"investigate", "recount", "resolve"} {
		amount := int64(6800)
		if i > 0 {
			amount = 6850
		}
		p.call("POST", bagsPath+"/vault-bag/review", cashcustody.ReviewInput{OperationID: "vault-" + action, ExpectedRevision: int64(i), Action: action, CountedCents: amount, Reason: "Custody evidence does not create money"}, 200, nil)
	}
	restored := runTradeVaultCustody(t, b, p, run, setEndpoint, stop, start, dbPath, station, ownerRoot, reviewerRoot)
	stop()
	db, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, v := range []struct {
		sql  string
		want int64
	}{{"SELECT count(*) FROM payable_cash_payments", 1}, {"SELECT sum(quantity_milli) FROM stock_movements", 2500}, {"SELECT difference_cents FROM cash_bags WHERE id='vault-bag'", -50}} {
		var n int64
		if err = db.QueryRow(v.sql).Scan(&n); err != nil || n != v.want {
			t.Fatal("vault changed commercial origin", err)
		}
	}
}
