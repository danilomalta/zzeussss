package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
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
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/receivables"
	"titansystem-backend/internal/localdb/register"
)

const arPath = "/local/v1/finance/receivables"
const arBody = `{"operation_id":"bill-create","id":"bill","customer_reference":"customer-external","document_reference":"commercial-001","description":"Synthetic external service","issued_on":"2026-11-01","total_cents":1000,"installments":[{"id":"one","due_on":"2026-11-05","amount_cents":600},{"id":"two","due_on":"2026-11-10","amount_cents":400}]}`

func arSetup(t *testing.T) *httpContractFixture {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 90, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
	cashRequest(t, f, "open", `{"session_id":"ar-turn","opening_cents":5000}`, 201)
	return f
}
func arResult(t *testing.T, f *httpContractFixture, path, body string, want int) receivables.Result {
	t.Helper()
	code, raw := request(t, f.app, "POST", path, body, f.token)
	if code != want {
		t.Fatalf("receivable %d want %d: %s", code, want, raw)
	}
	var r receivables.Result
	if want < 300 && json.Unmarshal(raw, &r) != nil {
		t.Fatal("result")
	}
	return r
}
func arReceive(op, part string, amount, expected int64) string {
	return fmt.Sprintf(`{"operation_id":%q,"installment_id":%q,"session_id":"ar-turn","amount_cents":%d,"expected_received_cents":%d}`, op, part, amount, expected)
}
func TestHTTPReceivablesCashFlowRestartRestoreNoDuplicate(t *testing.T) {
	f := arSetup(t)
	created := arResult(t, f, arPath, arBody, 201)
	if len(created.Document.History) != 1 || created.Document.OutstandingCents != 1000 {
		t.Fatal(created)
	}
	first := arResult(t, f, arPath+"/bill/receive-cash", arReceive("receipt-1", "one", 500, 0), 200)
	repeat := arResult(t, f, arPath+"/bill/receive-cash", arReceive("receipt-1", "one", 500, 0), 200)
	if !repeat.Repeated || !reflect.DeepEqual(first.Document, repeat.Document) {
		t.Fatal("lost response duplicated receipt")
	}
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("receipt-2", "one", 100, 500), 200)
	final := arResult(t, f, arPath+"/bill/receive-cash", arReceive("receipt-3", "two", 400, 0), 200)
	if final.Document.State != "settled" || final.Document.ReceivedCents != 1000 || final.Document.OutstandingCents != 0 || len(final.Document.History) != 4 {
		t.Fatal(final)
	}
	var amount, count int64
	if e := f.db.QueryRow(`SELECT COALESCE(sum(amount_cents),0),count(*) FROM cash_adjustments WHERE tenant_id=? AND kind='supply'`, f.owner.TenantID).Scan(&amount, &count); e != nil || amount != 1000 || count != 3 {
		t.Fatal("cash duplicated", amount, count, e)
	}
	code, raw := request(t, f.app, "GET", arPath+"?limit=1&offset=0", "", f.token)
	if code != 200 || !bytes.Contains(raw, []byte(`"total":1`)) {
		t.Fatal(code, string(raw))
	}
	arResult(t, f, arPath+"/bill/cancel", `{"operation_id":"cancel-paid","reason":"No implicit refund"}`, 409)
	closingRaw := cashRequest(t, f, "close", `{"session_id":"ar-turn","operation_id":"ar-close","declared_cents":6000}`, 200)
	var closing register.Result
	if json.Unmarshal(closingRaw, &closing) != nil {
		t.Fatal("closing response")
	}
	if closing.ExpectedCents != 6000 || closing.DifferenceCents != 0 {
		t.Fatal("cash closure mismatch", closing)
	}
	replayAfterClose := arResult(t, f, arPath+"/bill/receive-cash", arReceive("receipt-1", "one", 500, 0), 200)
	if !replayAfterClose.Repeated {
		t.Fatal("closed drawer replay not recovered")
	}
	key := make([]byte, 32)
	rand.Read(key)
	archive := filepath.Join(t.TempDir(), "ar.tytbak")
	ctx := context.Background()
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Restore(ctx, archive, restored, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	got, e := receivables.Read(ctx, db, a, f.device, "bill")
	if e != nil || !reflect.DeepEqual(final.Document, got) {
		t.Fatal("restore", e)
	}
	// Reopen the original database and restart the HTTP application.
	var seq int
	var dbName, dbPath string
	if e = f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &dbName, &dbPath); e != nil {
		t.Fatal(e)
	}
	if e = f.db.Close(); e != nil {
		t.Fatal(e)
	}
	f.db, e = localdb.Open(ctx, dbPath)
	if e != nil {
		t.Fatal(e)
	}
	defer f.db.Close()
	// Replays keep their original IDs.
	f.app, e = New(f.db, f.device)
	if e != nil {
		t.Fatal(e)
	}
	code, raw = request(t, f.app, "GET", arPath+"/bill", "", f.token)
	if code != 200 {
		t.Fatal("restart", code, string(raw))
	}
	for _, query := range []string{`DELETE FROM receivable_cash_receipts`, `UPDATE receivable_operations SET action='x'`, `DELETE FROM receivable_documents`} {
		if _, e = f.db.Exec(query); e == nil {
			t.Fatal("immutable evidence changed")
		}
	}
}
func TestHTTPReceivablesConcurrentRevisionRollbackAndIsolation(t *testing.T) {
	f := arSetup(t)
	arResult(t, f, arPath, arBody, 201)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", arPath+"/bill/receive-cash", arReceive(fmt.Sprintf("race-%d", i), "one", 400, 0), f.token)
			statuses <- code
		}(i)
	}
	wg.Wait()
	close(statuses)
	success, conflict := 0, 0
	for s := range statuses {
		if s == 200 {
			success++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal("unexpected concurrency response", s)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("overpay", "one", 201, 400), 409)
	_, e := f.db.Exec(`CREATE TRIGGER reject_ar_event BEFORE INSERT ON receivable_events BEGIN SELECT RAISE(ABORT,'synthetic final audit failure'); END;`)
	if e != nil {
		t.Fatal(e)
	}
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("rollback", "two", 100, 0), 500)
	var count int
	if e = f.db.QueryRow(`SELECT count(*) FROM cash_adjustments WHERE tenant_id=? AND kind='supply'`, f.owner.TenantID).Scan(&count); e != nil || count != 1 {
		t.Fatal("partial cash commit", count, e)
	}
	foreign := arSetup(t)
	code, _ := request(t, f.app, "GET", arPath+"/bill", "", foreign.token)
	if code != 401 {
		t.Fatal("foreign session", code)
	}
	a := identity.Scope{TenantID: foreign.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	if _, e = receivables.Read(context.Background(), f.db, a, f.device, "bill"); e == nil {
		t.Fatal("cross-tenant read")
	}
	policyPerson(t, f.db, f.owner, "ar-cashier", "cashier")
	cashier := loginToken(t, f.app, "ar-cashier")
	code, _ = request(t, f.app, "POST", arPath, arBody, cashier)
	if code != 403 {
		t.Fatal("cashier mutated finance", code)
	}
}
func TestHTTPReceivablesStrictInputAndCancelUnreceived(t *testing.T) {
	f := arSetup(t)
	for _, body := range []string{`{"operation_id":"a"}`, arBody[:len(arBody)-1] + `,"tenant_id":"forged"}`, bytesToString(bytes.ReplaceAll([]byte(arBody), []byte(`"amount_cents":600`), []byte(`"amount_cents":601`)))} {
		arResult(t, f, arPath, body, 400)
	}
	arResult(t, f, arPath, arBody, 201)
	arResult(t, f, arPath, arBody, 200)
	r := arResult(t, f, arPath+"/bill/cancel", `{"operation_id":"void","reason":"External document voided"}`, 200)
	if r.Document.State != "cancelled" || r.Document.OutstandingCents != 0 || r.Document.History[1].Reason == "" {
		t.Fatal(r)
	}
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("after-void", "one", 1, 0), 409)
}
func bytesToString(b []byte) string { return string(b) }

func TestHTTPReceivablesCashCannotAlsoBeSupplierRefund(t *testing.T) {
	f, _, _ := pcSetup(t)
	pcMove(t, f, f.token, "withdraw-one", "withdrawal", 500)
	pcResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cash-payments", pcPay, 201)
	arResult(t, f, arPath, arBody, 201)
	body := strings.Replace(arReceive("customer-cash", "one", 500, 0), "ar-turn", "turn-one", 1)
	arResult(t, f, arPath+"/bill/receive-cash", body, 200)
	var supply string
	if e := f.db.QueryRow(`SELECT movement_operation_id FROM receivable_cash_receipts WHERE operation_id='customer-cash'`).Scan(&supply); e != nil {
		t.Fatal(e)
	}
	attempt := fmt.Sprintf(`{"operation_id":"supplier-refund","supply_operation_id":%q,"reason":"Must not classify customer money as supplier return"}`, supply)
	pcResult(t, f, f.token, "POST", pcPath+"/payment-one/return", attempt, 409)
}

func TestHTTPReceivablesIgnoredWriteCannotConfirmCash(t *testing.T) {
	for _, table := range []string{"receivable_cash_receipts", "receivable_operations", "receivable_events"} {
		t.Run(table, func(t *testing.T) {
			f := arSetup(t)
			arResult(t, f, arPath, arBody, 201)
			if _, e := f.db.Exec("CREATE TRIGGER ignore_ar_write BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END;"); e != nil {
				t.Fatal(e)
			}
			arResult(t, f, arPath+"/bill/receive-cash", arReceive("ignored", "one", 100, 0), 409)
			var n int
			if e := f.db.QueryRow(`SELECT count(*) FROM cash_adjustments WHERE tenant_id=?`, f.owner.TenantID).Scan(&n); e != nil || n != 0 {
				t.Fatal("unclassified cash committed", n, e)
			}
			code, _ := request(t, f.app, "GET", arPath+"/operations/ignored", "", f.token)
			if code != 404 {
				t.Fatal("false operation receipt", code)
			}
		})
	}
}
func TestHTTPReceivablesAccountantReadManagerWriteAndNoContract(t *testing.T) {
	f := arSetup(t)
	arResult(t, f, arPath, arBody, 201)
	policyPerson(t, f.db, f.owner, "ar-accountant", "accountant")
	accountant := loginToken(t, f.app, "ar-accountant")
	code, _ := request(t, f.app, "GET", arPath+"/bill", "", accountant)
	if code != 200 {
		t.Fatal("accountant cannot read", code)
	}
	code, _ = request(t, f.app, "POST", arPath+"/bill/receive-cash", arReceive("accountant-cash", "one", 100, 0), accountant)
	if code != 403 {
		t.Fatal("accountant wrote cash", code)
	}
	policyPerson(t, f.db, f.owner, "ar-manager", "manager")
	manager := loginToken(t, f.app, "ar-manager")
	body := strings.ReplaceAll(strings.ReplaceAll(arBody, "bill-create", "manager-create"), `"bill"`, `"manager-bill"`)
	body = strings.ReplaceAll(body, "commercial-001", "manager-001")
	code, _ = request(t, f.app, "POST", arPath, body, manager)
	if code != 201 {
		t.Fatal("authorized manager cannot create", code)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 91, []modules.ID{modules.Core}), 200)
	code, _ = request(t, f.app, "POST", arPath+"/bill/receive-cash", arReceive("unlicensed", "one", 100, 0), f.token)
	if code != 403 {
		t.Fatal("missing contract status", code)
	}
	code, _ = request(t, f.app, "GET", arPath+"/bill", "", f.token)
	if code != 200 {
		t.Fatal("history lost after contract change", code)
	}
}

func TestHTTPReceivablesCashSafeIntegerLimitRollsBack(t *testing.T) {
	f := arSetup(t)
	var input receivables.CreateInput
	if json.Unmarshal([]byte(arBody), &input) != nil {
		t.Fatal("fixture")
	}
	input.TotalCents = receivables.MaxSafe
	input.Installments = []receivables.Installment{{ID: "one", DueOn: "2026-11-05", AmountCents: receivables.MaxSafe}}
	// The input contract never accepts server-calculated received_cents.
	body := fmt.Sprintf(`{"operation_id":"limit-create","id":"limit-bill","customer_reference":"limit-customer","document_reference":"limit-doc","description":"Synthetic boundary","issued_on":"2026-11-01","total_cents":%d,"installments":[{"id":"one","due_on":"2026-11-05","amount_cents":%d}]}`, input.TotalCents, input.TotalCents)
	arResult(t, f, arPath, body, 201)
	arResult(t, f, arPath+"/limit-bill/receive-cash", arReceive("too-large-cash", "one", receivables.MaxSafe, 0), 409)
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM cash_adjustments`).Scan(&n); e != nil || n != 0 {
		t.Fatal("out-of-range cash committed", n, e)
	}
}

func TestHTTPReceivablesNoIssuerRuntimeIsUnavailableNotInvalidInput(t *testing.T) {
	f := arSetup(t)
	var e error
	f.app, e = New(f.db, f.device)
	if e != nil {
		t.Fatal(e)
	}
	for _, target := range []struct{ path, body string }{{arPath, arBody}, {arPath + "/bill/receive-cash", arReceive("unavailable", "one", 100, 0)}, {arPath + "/bill/cancel", `{"operation_id":"unavailable-cancel","reason":"Synthetic cancellation"}`}} {
		code, _ := request(t, f.app, "POST", target.path, target.body, f.token)
		if code != 503 {
			t.Fatal("missing runtime status", code)
		}
	}
}

func TestHTTPReceivablesRevokedSessionCannotCommit(t *testing.T) {
	f := arSetup(t)
	arResult(t, f, arPath, arBody, 201)
	if e := localauth.Logout(context.Background(), f.db, f.token); e != nil {
		t.Fatal(e)
	}
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	var input receivables.ReceiptInput
	if json.Unmarshal([]byte(arReceive("stale-session", "one", 100, 0)), &input) != nil {
		t.Fatal("fixture")
	}
	// Direct call models a session revoked after the outer HTTP guard has run.
	licenseVerifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	license, err := entitlementstore.New(f.db, licenseVerifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = receivables.ReceiveCash(context.Background(), f.db, license, actor, f.device, f.token, "bill", input); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("stale session passed", err)
	}
	code, _ := request(t, f.app, "POST", arPath+"/bill/receive-cash", arReceive("stale-session", "one", 100, 0), f.token)
	if code != 401 {
		t.Fatal("revoked token status", code)
	}
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM cash_adjustments`).Scan(&n); e != nil || n != 0 {
		t.Fatal("revoked token changed cash", n, e)
	}
}
