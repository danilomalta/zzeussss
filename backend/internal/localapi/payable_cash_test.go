package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/payables"
)

const pcPath = "/local/v1/finance/payable-cash"
const pcPay = `{"operation_id":"pay-one","payment_id":"payment-one","installment_id":"part-one","withdrawal_operation_id":"withdraw-one","recipient":"Responsavel do fornecedor","reference":"receipt-one"}`
const pcReturn = `{"operation_id":"return-one","supply_operation_id":"return-supply","reason":"Fornecedor devolveu integralmente em especie"}`

func pcPrepare(t *testing.T, f *httpContractFixture) (string, string) {
	t.Helper()
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 90, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.POS, modules.Finance}), 200)
	if s, b := request(t, f.app, "POST", "/local/v1/purchase-suppliers", `{"operation_id":"supplier-op","id":"supplier","name":"Padaria","status":"active"}`, f.token); s != 201 {
		t.Fatal(s, string(b))
	}
	policyPerson(t, f.db, f.owner, "payable-reviewer", "manager")
	reviewer := loginToken(t, f.app, "payable-reviewer")
	policyPerson(t, f.db, f.owner, "payable-accountant", "accountant")
	accountant := loginToken(t, f.app, "payable-accountant")
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	return reviewer, accountant
}
func pcSetup(t *testing.T) (*httpContractFixture, string, string) {
	t.Helper()
	f := httpContractSetup(t)
	reviewer, accountant := pcPrepare(t, f)
	cashRequest(t, f, "open", `{"session_id":"turn-one","opening_cents":50000}`, 201)
	return f, reviewer, accountant
}
func pcMove(t *testing.T, f *httpContractFixture, token, op, kind string, amount int64) {
	t.Helper()
	body := fmt.Sprintf(`{"session_id":"turn-one","operation_id":%q,"kind":%q,"amount_cents":%d,"reason":"Movimento de teste conferido"}`, op, kind, amount)
	if s, b := request(t, f.app, "POST", "/local/v1/cash/movements", body, token); s != 200 {
		t.Fatal(s, string(b))
	}
}
func pcResult(t *testing.T, f *httpContractFixture, token, method, path, body string, want int) payables.CashResult {
	t.Helper()
	s, b := request(t, f.app, method, path, body, token)
	if s != want {
		t.Fatalf("cash payable %s %s got%d want%d %s", method, path, s, want, b)
	}
	var r payables.CashResult
	if want == 200 || want == 201 {
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e)
		}
		domainSchemaFile(t, "payable-cash.openapi.json", "CashResult", b)
	}
	return r
}
func pcList(t *testing.T, f *httpContractFixture, token, q string, want int) payables.CashPage {
	t.Helper()
	s, b := request(t, f.app, "GET", payablesPath+"/invoice-one/cash-payments"+q, "", token)
	if s != want {
		t.Fatal(s, want, string(b))
	}
	var r payables.CashPage
	if want == 200 {
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e)
		}
		domainSchemaFile(t, "payable-cash.openapi.json", "CashPage", b)
	}
	return r
}
func pcCounts(t *testing.T, f *httpContractFixture) []int {
	t.Helper()
	n := payableCounts(t, f)
	for _, table := range []string{"payable_cash_payments", "payable_cash_returns", "payable_cash_operations", "payable_cash_events"} {
		var v int
		if e := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&v); e != nil {
			t.Fatal(e)
		}
		n = append(n, v)
	}
	return n
}
func TestHTTPPayableCashPartialExactBalanceAndNoDuplicateMovement(t *testing.T) {
	f, _, accountant := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	before := payableCounts(t, f)
	first := pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	if first.Payment.AmountCents != 6000 || first.Payment.Return != nil {
		t.Fatal(first)
	}
	if !reflect.DeepEqual(before, payableCounts(t, f)) {
		t.Fatal("allocation moved drawer again")
	}
	s := pcList(t, f, accountant, "", 200).Summary
	if s.AppliedCents != 6000 || s.OutstandingCents != 25500 || s.PaidCents != 6000 || s.ReturnedCents != 0 {
		t.Fatal(s)
	}
	current := payableRead(t, f, accountant)
	if current.Cash == nil || *current.Cash != s || current.SettledCents != nil || current.PaymentExecutionAvailable {
		t.Fatal(current)
	}
	r := payableReport(t, f, accountant, payableQueryString, 200)
	if r.Summary["approved_cents"] != 31500 || r.Summary["cash_applied_cents"] != 6000 || r.Summary["approved_outstanding_cents"] != 25500 || r.Items[0].OutstandingCents != 4000 {
		t.Fatal(r)
	}
	pcMove(t, f, f.token, "withdraw-two", "withdrawal", 4000)
	second := strings.NewReplacer("pay-one", "pay-two", "payment-one", "payment-two", "withdraw-one", "withdraw-two", "receipt-one", "receipt-two").Replace(pcPay)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", second, 201)
	pcMove(t, f, f.token, "withdraw-three", "withdrawal", 1)
	over := strings.NewReplacer("pay-one", "pay-three", "payment-one", "payment-three", "withdraw-one", "withdraw-three", "receipt-one", "receipt-three").Replace(pcPay)
	before = pcCounts(t, f)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", over, 409)
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("overpayment linked")
	}
	retry := pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 200)
	if !retry.Repeated || !reflect.DeepEqual(first.Payment, retry.Payment) {
		t.Fatal(retry)
	}
	page := pcList(t, f, accountant, "?limit=1&offset=1", 200)
	if page.TotalCount != 2 || len(page.Items) != 1 || page.HasMore || page.Summary.AppliedCents != 10000 {
		t.Fatal(page)
	}
}
func TestHTTPPayableCashReturnRequiresActualSupplyAndBlocksCancellation(t *testing.T) {
	f, reviewer, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	original := pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	cancel := `{"operation_id":"cancel-paid","reason":"Pedido cancelado"}`
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/cancel", cancel, 409)
	if _, e := f.db.Exec(`UPDATE supplier_payables SET state='cancelled' WHERE id='invoice-one'`); e == nil {
		t.Fatal("SQL bypassed outstanding cash")
	}
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 404)
	pcMove(t, f, f.token, "return-supply", "supply", 9999)
	before := pcCounts(t, f)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 409)
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("wrong supply allocated")
	}
	pcMove(t, f, f.token, "return-full", "supply", 10000)
	body := strings.Replace(pcReturn, "return-supply", "return-full", 1)
	before = payableCounts(t, f)
	ret := pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", body, 201)
	if ret.Payment.Return == nil || !reflect.DeepEqual(before, payableCounts(t, f)) {
		t.Fatal("return moved drawer again")
	}
	if got := pcList(t, f, f.token, "", 200); got.Summary.AppliedCents != 0 || got.Summary.PaidCents != 10000 || got.Summary.ReturnedCents != 10000 || got.Items[0].Return == nil {
		t.Fatal(got)
	}
	retry := pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", body, 200)
	if !retry.Repeated || !reflect.DeepEqual(ret.Payment, retry.Payment) {
		t.Fatal(retry)
	}
	receipt := pcResult(t, f, f.token, "GET", pcPath+"/operations/pay-one", "", 200)
	if !reflect.DeepEqual(original.Payment, receipt.Payment) {
		t.Fatal("original receipt rewritten")
	}
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/cancel", cancel, 200)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", body, 200)
}
func TestHTTPPayableCashExclusiveCustodyBothOrders(t *testing.T) {
	for _, bagFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(bagFirst), func(t *testing.T) {
			f, _, _ := pcSetup(t)
			pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
			sf := &stockHTTPFixture{httpContractFixture: f}
			if bagFirst {
				bagResult(t, sf, f.token, "POST", bagsPath, createBagBody, 201)
				pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 409)
			} else {
				pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
				bagResult(t, sf, f.token, "POST", bagsPath, createBagBody, 409)
			}
		})
	}
}
func TestHTTPPayableCashVaultSupplyCannotBecomeSupplierReturn(t *testing.T) {
	sf, receiver := countSetup(t)
	f := sf.httpContractFixture
	pcPrepare(t, f)
	pcMove(t, f, f.token, "withdraw-payable", "withdrawal", 50)
	body := strings.Replace(pcPay, "withdraw-one", "withdraw-payable", 1)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", body, 201)
	vaultResult(t, sf, receiver, "POST", vaultsPath+"/vault-one/supply", supplyVaultBody, 200)
	before := pcCounts(t, f)
	pcResult(t, f, receiver, "POST", pcPath+"/payment-one/return", strings.Replace(pcReturn, "return-supply", "vault-supply-one", 1), 409)
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("vault money duplicated as supplier return")
	}
}
func TestHTTPPayableCashActorPermissionsLicenseAndStrictInput(t *testing.T) {
	f, reviewer, accountant := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	pcResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 403)
	pcResult(t, f, accountant, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 403)
	before := pcCounts(t, f)
	for _, body := range []string{strings.Replace(pcPay, `"operation_id":`, `"tenant_id":"foreign","operation_id":`, 1), strings.Replace(pcPay, `"recipient":`, `"amount_cents":1,"recipient":`, 1), strings.Replace(pcPay, `"payment_id":"payment-one"`, `"payment_id":"payment-one","payment_id":"other"`, 1), strings.Replace(pcPay, `"reference":"receipt-one"`, `"reference":null`, 1)} {
		pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", body, 400)
	}
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("invalid body wrote")
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 91, []modules.ID{modules.Core, modules.Inventory, modules.POS}), 200)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 403)
	pcList(t, f, accountant, "", 200)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 92, []modules.ID{modules.Core, modules.Finance}), 200)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	pcResult(t, f, reviewer, "GET", pcPath+"/operations/pay-one", "", 404)
	for _, q := range []string{"?state=approved", "?limit=0", "?offset=-1", "?limit=1&limit=2"} {
		pcList(t, f, accountant, q, 400)
	}
}
func TestHTTPPayableCashConcurrentOverpaymentAndSourceReuse(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
	pcMove(t, f, f.token, "withdraw-two", "withdrawal", 6000)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, n := range []string{"one", "two"} {
		n := n
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.NewReplacer("pay-one", "pay-"+n, "payment-one", "payment-"+n, "withdraw-one", "withdraw-"+n, "receipt-one", "receipt-"+n).Replace(pcPay)
			s, _ := request(t, f.app, "POST", payablesPath+"/invoice-one/cash-payments", body, f.token)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	success, conflict := 0, 0
	for s := range statuses {
		if s == 201 {
			success++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	if s := pcList(t, f, f.token, "", 200).Summary; s.AppliedCents != 6000 {
		t.Fatal(s)
	}
	var source string
	f.db.QueryRow(`SELECT withdrawal_operation_id FROM payable_cash_payments`).Scan(&source)
	body := strings.NewReplacer("pay-one", "pay-duplicate", "payment-one", "payment-duplicate", "withdraw-one", source, "part-one", "part-two", "receipt-one", "receipt-duplicate").Replace(pcPay)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", body, 409)
}
func TestHTTPPayableCashRollbackBothLinks(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	before := pcCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER fail_pc_event BEFORE INSERT ON payable_cash_events BEGIN SELECT RAISE(ABORT,'injected failure'); END`); e != nil {
		t.Fatal(e)
	}
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 500)
	if !reflect.DeepEqual(before, pcCounts(t, f)) {
		t.Fatal("partial payment")
	}
	if _, e := f.db.Exec(`DROP TRIGGER fail_pc_event`); e != nil {
		t.Fatal(e)
	}
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	pcMove(t, f, f.token, "return-supply", "supply", 10000)
	before = pcCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER fail_pc_return BEFORE INSERT ON payable_cash_events WHEN NEW.event_type='payable.cash.return' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); e != nil {
		t.Fatal(e)
	}
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 500)
	if !reflect.DeepEqual(before, pcCounts(t, f)) || pcList(t, f, f.token, "", 200).Summary.AppliedCents != 10000 {
		t.Fatal("partial return")
	}
	if _, e := f.db.Exec(`DROP TRIGGER fail_pc_return`); e != nil {
		t.Fatal(e)
	}
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 201)
}
func TestHTTPPayableCashIntegrityRefusesPartialEmptyPagesAndCSV(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	if _, e := f.db.Exec(`DROP TRIGGER payable_cash_events_no_update; UPDATE payable_cash_events SET payload_json='{}'`); e != nil {
		t.Fatal(e)
	}
	pcList(t, f, f.token, "?offset=999", 409)
	payableReport(t, f, f.token, payableQueryString, 409)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 409)
	if s, b := request(t, f.app, "GET", payablesPath+"/report.csv"+payableQueryString, "", f.token); s != 409 || strings.Contains(string(b), "format_version") {
		t.Fatal(s, string(b))
	}
}
func TestHTTPPayableCashClosedTurnCoverageAndAccountingDoNotDoubleCash(t *testing.T) {
	f, _, accountant := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	pcMove(t, f, f.token, "return-supply", "supply", 10000)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 201)
	cashRequest(t, f, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":50000}`, 200)
	sf := &stockHTTPFixture{httpContractFixture: f}
	setAccountingClosedAt(t, sf, "turn-one", closedFirst)
	c := bagCoverage(t, sf, f.token, 200)
	if c.WithdrawnCents != 10000 || c.SupplierPaidCents != 10000 || c.SupplierReturnedCents != 10000 || c.UnassignedCents != 0 {
		t.Fatal(c)
	}
	r := accountingReport(t, sf, accountant, accountingQuery(closedFirst, periodEnd), 200)
	if r.Summary["withdrawal_cents"] != 10000 || r.Summary["supply_cents"] != 10000 || r.Summary["sum_expected_at_closures_cents"] != 50000 || r.Summary["journal_entry_count"] != 3 {
		t.Fatal(r)
	}
}
func TestHTTPPayableCashBackupRestartAndOriginalReceipts(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 10000)
	first := pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	pcMove(t, f, f.token, "return-supply", "supply", 10000)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", pcReturn, 201)
	want := pcList(t, f, f.token, "", 200)
	ctx := context.Background()
	key := make([]byte, 32)
	rand.Read(key)
	dir := t.TempDir()
	archive := filepath.Join(dir, "cash.tytbak")
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	restored := filepath.Join(dir, "restored.sqlite")
	if e := backup.Restore(ctx, archive, restored, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	got, e := payables.CashPayments(ctx, db, a, f.device, "invoice-one", 50, 0)
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, e)
	}
	var n int
	var name, path string
	if e = f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &path); e != nil {
		t.Fatal(e)
	}
	f.db.Close()
	f.db, e = localdb.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.db.Close()
	v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	f.app, e = NewWithVerifier(f.db, f.device, v)
	if e != nil {
		t.Fatal(e)
	}
	retry := pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 200)
	if !retry.Repeated || !reflect.DeepEqual(first.Payment, retry.Payment) {
		t.Fatal(retry)
	}
	for _, q := range []string{`DELETE FROM payable_cash_payments`, `UPDATE payable_cash_returns SET reason='altered'`, `DELETE FROM payable_cash_operations`, `DELETE FROM payable_cash_events`} {
		if _, e = f.db.Exec(q); e == nil {
			t.Fatal("immutable bypass", q)
		}
	}
}
