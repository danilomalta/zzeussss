package localapi

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/payables"
)

const readinessPay = payablesPath + "/invoice-one/cash-sources?installment_id=part-one"
const readinessBalances = payablesPath + "/invoice-one/installment-balances"

func readinessSources(t *testing.T, f *httpContractFixture, token, path string, want int) payables.CashCandidates {
	t.Helper()
	s, b := request(t, f.app, "GET", path, "", token)
	if s != want {
		t.Fatalf("sources %s: %d want%d %s", path, s, want, b)
	}
	var r payables.CashCandidates
	if want == 200 {
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e)
		}
		domainSchemaFile(t, "payable-readiness.openapi.json", "CashCandidates", b)
	}
	return r
}
func readinessParts(t *testing.T, f *httpContractFixture, token string) payables.InstallmentBalances {
	t.Helper()
	s, b := request(t, f.app, "GET", readinessBalances, "", token)
	if s != 200 {
		t.Fatal(s, string(b))
	}
	domainSchemaFile(t, "payable-readiness.openapi.json", "InstallmentBalances", b)
	var r payables.InstallmentBalances
	if e := json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	return r
}
func TestHTTPPayableReadinessBalancesAndUnreservedSources(t *testing.T) {
	f, _, accountant := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	pcMove(t, f, f.token, "withdraw-two", "withdrawal", 4000)
	pcMove(t, f, f.token, "withdraw-too-large", "withdrawal", 11000)
	before := pcCounts(t, f)
	r := readinessSources(t, f, f.token, readinessPay+"&limit=1", 200)
	if r.TotalCount != 2 || !r.HasMore || len(r.Items) != 1 || r.Items[0].OperationID != "withdraw-one" || r.AmountLimitCents != 10000 || r.ReservationCreated || r.WriteAuthorizationChecked {
		t.Fatal(r)
	}
	second := readinessSources(t, f, f.token, readinessPay+"&limit=1&offset=1", 200)
	if second.HasMore || second.Items[0].OperationID != "withdraw-two" {
		t.Fatal(second)
	}
	if v := readinessSources(t, f, f.token, readinessPay+"&offset=999", 200); v.TotalCount != 2 || len(v.Items) != 0 || v.HasMore {
		t.Fatal(v)
	}
	if v := readinessSources(t, f, accountant, readinessPay, 200); v.TotalCount != 0 {
		t.Fatal("another actor's sources leaked", v)
	}
	again := readinessSources(t, f, f.token, readinessPay, 200)
	if again.TotalCount != 2 || !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("read reserved or changed money")
	}
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	r = readinessSources(t, f, f.token, readinessPay, 200)
	if r.TotalCount != 1 || r.Items[0].OperationID != "withdraw-two" || r.AmountLimitCents != 4000 {
		t.Fatal(r)
	}
	p := readinessParts(t, f, accountant)
	if !p.Complete || len(p.Items) != 3 || p.Summary.AppliedCents != 6000 || p.Items[0].AppliedCents != 6000 || p.Items[0].OutstandingCents != 4000 || p.Items[1].OutstandingCents != 9600 {
		t.Fatal(p)
	}
	// The separate installment query must reconcile to the existing agenda.
	report := payableReport(t, f, accountant, payableQueryString, 200)
	for i, item := range p.Items {
		if item.OutstandingCents != report.Items[i].OutstandingCents {
			t.Fatal(p, report)
		}
	}
}
func TestHTTPPayableReadinessReturnSourcesAndHistoryRemainUsed(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "supply-before", "supply", 10000)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	if r := readinessSources(t, f, f.token, readinessPay, 200); r.BlockedReason != "installment_fully_applied" || r.TotalCount != 0 {
		t.Fatal(r)
	}
	pcMove(t, f, f.token, "return-wrong", "supply", 9999)
	pcMove(t, f, f.token, "return-supply", "supply", 10000)
	path := pcPath + "/payment-one/return-sources"
	if r := readinessSources(t, f, f.token, path, 200); r.TotalCount != 1 || r.Items[0].OperationID != "return-supply" || r.AmountRule != "exact_payment_amount" {
		t.Fatal(r)
	}
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 201)
	if r := readinessSources(t, f, f.token, path, 200); r.BlockedReason != "payment_already_returned" || r.TotalCount != 0 {
		t.Fatal(r)
	}
	if r := readinessSources(t, f, f.token, readinessPay, 200); r.TotalCount != 0 || r.AmountLimitCents != 10000 {
		t.Fatal("returned withdrawal reused", r)
	}
	p := readinessParts(t, f, f.token)
	if p.Items[0].PaidCents != 10000 || p.Items[0].ReturnedCents != 10000 || p.Items[0].AppliedCents != 0 || p.Items[0].OutstandingCents != 10000 {
		t.Fatal(p)
	}
	pcMove(t, f, f.token, "withdraw-new", "withdrawal", 10000)
	body := strings.NewReplacer("pay-one", "pay-new", "payment-one", "payment-new", "withdraw-one", "withdraw-new", "receipt-one", "receipt-new").Replace(pcPay)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", body, 201)
	if r := readinessSources(t, f, f.token, pcPath+"/payment-new/return-sources", 200); r.TotalCount != 0 {
		t.Fatal("used/old/wrong supply reused", r)
	}
}
func TestHTTPPayableReadinessCustodySourcesExcludedEvenCancelled(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	sf := &stockHTTPFixture{httpContractFixture: f}
	bagResult(t, sf, f.token, "POST", bagsPath, createBagBody, 201)
	bagResult(t, sf, f.token, "POST", bagsPath+"/bag-one/cancel", `{"operation_id":"cancel-bag","reason":"Conferido"}`, 200)
	if r := readinessSources(t, f, f.token, readinessPay, 200); r.TotalCount != 0 {
		t.Fatal(r)
	}
}
func TestHTTPPayableReadinessVaultSupplyExcluded(t *testing.T) {
	sf, receiver := countSetup(t)
	f := sf.httpContractFixture
	pcPrepare(t, f)
	pcMove(t, f, f.token, "withdraw-payable", "withdrawal", 50)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", strings.Replace(pcPay, "withdraw-one", "withdraw-payable", 1), 201)
	vaultResult(t, sf, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
	if r := readinessSources(t, f, receiver, pcPath+"/payment-one/return-sources", 200); r.TotalCount != 0 {
		t.Fatal("vault supply offered for supplier return", r)
	}
}
func TestHTTPPayableReadinessAuthorizationStrictQueriesAndReadOnlyLicense(t *testing.T) {
	f, _, accountant := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	before := pcCounts(t, f)
	for _, q := range []string{"", "?installment_id=", "?installment_id=part-one&installment_id=part-two", "?installment_id=part-one&limit=0", "?installment_id=part-one&offset=-1", "?installment_id=part-one&limit=1&limit=2", "?installment_id=part-one&actor_id=other", "?installment_id=part-one&limit=1e2", "?installment_id=part-one&offset=9999999999999999999999"} {
		readinessSources(t, f, f.token, payablesPath+"/invoice-one/cash-sources"+q, 400)
	}
	readinessSources(t, f, f.token, readinessPay+"&tenant_id=foreign", 400)
	readinessSources(t, f, f.token, strings.Replace(readinessPay, "part-one", "missing", 1), 404)
	readinessSources(t, f, f.token, strings.Replace(readinessPay, "invoice-one", "missing", 1), 404)
	readinessSources(t, f, f.token, pcPath+"/missing/return-sources?installment_id=x", 400)
	readinessSources(t, f, f.token, pcPath+"/missing/return-sources", 404)
	for _, path := range []string{readinessPay, readinessBalances, pcPath + "/missing/return-sources"} {
		if s, _ := request(t, f.app, "GET", path, `{}`, f.token); s != 400 {
			t.Fatal(path, s)
		}
		if s, _ := request(t, f.app, "GET", path, "", ""); s != 401 {
			t.Fatal(path, s)
		}
	}
	policyPerson(t, f.db, f.owner, "readiness-cashier", "cashier")
	cashier := loginToken(t, f.app, "readiness-cashier")
	readinessSources(t, f, cashier, readinessPay, 403)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 91, []modules.ID{modules.Core}), 200)
	if r := readinessSources(t, f, f.token, readinessPay, 200); r.TotalCount != 1 || r.WriteAuthorizationChecked {
		t.Fatal(r)
	}
	readinessParts(t, f, accountant)
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("read endpoints changed business data")
	}
	if _, e := f.db.Exec(`INSERT INTO access_rules VALUES (?,?,'member','payable-accountant','view_accounting','deny',1)`, f.owner.TenantID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	readinessSources(t, f, accountant, readinessPay, 403)
	// A device change cannot use the original actor scope to inspect its sources.
	d := f.device
	d.DeviceID = "foreign-device"
	if _, e := payables.PaymentSources(context.Background(), f.db, identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}, d, "invoice-one", "part-one", 50, 0); e == nil {
		t.Fatal("foreign device authorized")
	}
}
func TestHTTPPayableReadinessCorruptionBeyondPageIsRejected(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "a-good", "withdrawal", 10)
	pcMove(t, f, f.token, "z-broken", "withdrawal", 11000)
	if _, e := f.db.Exec(`DROP TRIGGER cash_journal_no_update; UPDATE cash_journal_entries SET cash_delta_cents=-1 WHERE operation_id='z-broken'`); e != nil {
		t.Fatal(e)
	}
	readinessSources(t, f, f.token, readinessPay+"&limit=1", 409)
	readinessSources(t, f, f.token, readinessPay+"&offset=999", 409)
}
func TestHTTPPayableReadinessSourcesAreNotReservationsAndWritesRevalidate(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	pcMove(t, f, f.token, "withdraw-two", "withdrawal", 6000)
	r := readinessSources(t, f, f.token, readinessPay, 200)
	if r.TotalCount != 2 {
		t.Fatal(r)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 1; i <= 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			suffix := []string{"one", "two"}[i-1]
			body := strings.NewReplacer("pay-one", "pay-"+suffix, "payment-one", "payment-"+suffix, "withdraw-one", "withdraw-"+suffix, "receipt-one", "receipt-"+suffix).Replace(pcPay)
			s, _ := request(t, f.app, "POST", payablesPath+"/invoice-one/cash-payments", body, f.token)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	got := map[int]int{}
	for s := range statuses {
		got[s]++
	}
	if got[201] != 1 || got[409] != 1 {
		t.Fatal(got)
	}
	if p := readinessParts(t, f, f.token); p.Summary.AppliedCents != 6000 || p.Items[0].OutstandingCents != 4000 {
		t.Fatal(p)
	}
	if r := readinessSources(t, f, f.token, readinessPay, 200); r.TotalCount != 0 {
		t.Fatal(r)
	}
}
func TestHTTPPayableReadinessReopenAndCancelledBalances(t *testing.T) {
	f, reviewer, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	original := readinessSources(t, f, f.token, readinessPay, 200)
	var n int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &path); e != nil {
		t.Fatal(e)
	}
	f.db.Close()
	db, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	f.db = db
	defer db.Close()
	verifier, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	f.app, e = NewWithVerifier(db, f.device, verifier)
	if e != nil {
		t.Fatal(e)
	}
	now := readinessSources(t, f, f.token, readinessPay, 200)
	if !reflect.DeepEqual(original.Items, now.Items) {
		t.Fatal("restart changed source rules")
	}
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/cancel", `{"operation_id":"cancel-doc","reason":"Compra desfeita"}`, 200)
	if r := readinessSources(t, f, f.token, readinessPay, 200); r.BlockedReason != "document_not_approved" || r.TotalCount != 0 {
		t.Fatal(r)
	}
	if r := readinessParts(t, f, f.token); r.Summary.OutstandingCents != 0 || r.Items[0].OutstandingCents != 0 {
		t.Fatal(r)
	}
}
func TestHTTPPayableReadinessSourceLimitAndCacheHeaders(t *testing.T) {
	f, _, _ := pcSetup(t)
	req := httptest.NewRequest("GET", readinessBalances, nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	response, e := f.app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal(response.StatusCode, response.Header)
	}
	// Inject only in disposable DB: the hard limit must be checked before pagination
	// or per-source decoding. The new query never silently scans a partial set.
	tx, e := f.db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i <= payables.MaxCashSources; i++ {
		_, e = tx.Exec(`INSERT INTO cash_movements(id,tenant_id,store_id,cash_session_id,amount_cents,reason,created_at) VALUES(?,?,?,?,?,?,?)`, fmt.Sprintf("movement-%05d", i), f.owner.TenantID, f.owner.StoreID, "turn-one", -1, "Limit fixture", "2026-10-01T00:00:00Z")
		if e != nil {
			tx.Rollback()
			t.Fatal(e)
		}
		_, e = tx.Exec(`INSERT INTO cash_adjustments(tenant_id,store_id,device_id,operation_id,session_id,movement_id,actor_identity_id,kind,amount_cents,reason,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, f.owner.TenantID, f.owner.StoreID, f.owner.DeviceID, fmt.Sprintf("limit-%05d", i), "turn-one", fmt.Sprintf("movement-%05d", i), f.owner.OwnerID, "withdrawal", 1, "Limit fixture", "2026-10-01T00:00:00Z")
		if e != nil {
			tx.Rollback()
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	readinessSources(t, f, f.token, readinessPay+"&offset=999999", 422)
}

func TestHTTPPayableReadinessExactMaximumAndHistoricalBudget(t *testing.T) {
	f, reviewer, _ := pcSetup(t)
	var input payables.CreateInput
	if e := json.Unmarshal([]byte(payableBody), &input); e != nil {
		t.Fatal(e)
	}
	input.OperationID = "create-large"
	input.PayableID = "large"
	input.DocumentReference = "NFe-large"
	input.TotalCents = payables.MaxSafe
	input.Installments = []payables.Installment{{ID: "large-part", DueOn: "2026-10-10", AmountCents: payables.MaxSafe - 1}, {ID: "cent", DueOn: "2026-10-20", AmountCents: 1}}
	body, _ := json.Marshal(input)
	payableResult(t, f, f.token, "POST", payablesPath, string(body), 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/large/approve", `{"operation_id":"approve-large","reason":"Conferido"}`, 200)
	pcMove(t, f, f.token, "fund-large", "supply", payables.MaxSafe-50000)
	pcMove(t, f, f.token, "withdraw-large", "withdrawal", payables.MaxSafe-1)
	query := payablesPath + "/large/cash-sources?installment_id=large-part"
	if r := readinessSources(t, f, f.token, query, 200); r.AmountLimitCents != payables.MaxSafe-1 || r.TotalCount != 1 || r.Items[0].AmountCents != payables.MaxSafe-1 {
		t.Fatal(r)
	}
	pay := `{"operation_id":"pay-large","payment_id":"payment-large","installment_id":"large-part","withdrawal_operation_id":"withdraw-large","recipient":"Fornecedor","reference":"receipt-large"}`
	pcResult(t, f, f.token, "POST", payablesPath+"/large/cash-payments", pay, 201)
	s, b := request(t, f.app, "GET", payablesPath+"/large/installment-balances", "", f.token)
	if s != 200 {
		t.Fatal(s, string(b))
	}
	domainSchemaFile(t, "payable-readiness.openapi.json", "InstallmentBalances", b)
	var r payables.InstallmentBalances
	if e := json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	if r.Summary.PaidCents != payables.MaxSafe-1 || r.Summary.OutstandingCents != 1 || r.Items[0].OutstandingCents != 0 || r.Items[1].OutstandingCents != 1 {
		t.Fatal(r)
	}
	pcMove(t, f, f.token, "withdraw-cent", "withdrawal", 1)
	pay = `{"operation_id":"pay-cent","payment_id":"payment-cent","installment_id":"cent","withdrawal_operation_id":"withdraw-cent","recipient":"Fornecedor","reference":"receipt-cent"}`
	pcResult(t, f, f.token, "POST", payablesPath+"/large/cash-payments", pay, 201)
	pcMove(t, f, f.token, "supply-large", "supply", payables.MaxSafe-1)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-large/return", `{"operation_id":"return-large","supply_operation_id":"supply-large","reason":"Dinheiro devolvido"}`, 201)
	if r := readinessSources(t, f, f.token, query, 200); r.BlockedReason != "historical_amount_limit" || r.AmountLimitCents != 0 || r.TotalCount != 0 {
		t.Fatal(r)
	}
}
