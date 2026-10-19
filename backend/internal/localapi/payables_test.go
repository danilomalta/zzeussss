package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/csv"
	"encoding/json"
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

const payablesPath = "/local/v1/finance/payables"
const payableBody = `{"operation_id":"create-invoice","payable_id":"invoice-one","supplier_id":"supplier","document_reference":"NFe-123","description":"Compra de farinha","issued_on":"2026-10-01","currency":"BRL","total_cents":31500,"installments":[{"id":"part-one","due_on":"2026-10-10","amount_cents":10000},{"id":"part-two","due_on":"2026-10-20","amount_cents":9600},{"id":"part-three","due_on":"2026-11-20","amount_cents":11900}]}`
const payableApprove = `{"operation_id":"approve-invoice","reason":"Documento e vencimentos conferidos"}`
const payableQueryString = "?from=2026-10-01&to=2026-11-30&as_of=2026-10-20"

func payableSetup(t *testing.T) (*httpContractFixture, string, string) {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.Finance}), 200)
	if s, b := request(t, f.app, "POST", "/local/v1/purchase-suppliers", `{"operation_id":"supplier-op","id":"supplier","name":"Padaria","status":"active"}`, f.token); s != 201 {
		t.Fatal(s, string(b))
	}
	policyPerson(t, f.db, f.owner, "payable-reviewer", "manager")
	reviewer := loginToken(t, f.app, "payable-reviewer")
	policyPerson(t, f.db, f.owner, "payable-accountant", "accountant")
	accountant := loginToken(t, f.app, "payable-accountant")
	return f, reviewer, accountant
}
func payableResult(t *testing.T, f *httpContractFixture, token, method, path, body string, want int) payables.Result {
	t.Helper()
	s, b := request(t, f.app, method, path, body, token)
	if s != want {
		t.Fatalf("payable %s %s %d want%d %s", method, path, s, want, b)
	}
	var r payables.Result
	if want == 200 || want == 201 {
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e)
		}
		domainSchemaFile(t, "supplier-payables.openapi.json", "PayableResult", b)
	}
	return r
}
func payableRead(t *testing.T, f *httpContractFixture, token string) payables.Payable {
	t.Helper()
	s, b := request(t, f.app, "GET", payablesPath+"/invoice-one", "", token)
	if s != 200 {
		t.Fatal(s, string(b))
	}
	var p payables.Payable
	if e := json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	domainSchemaFile(t, "supplier-payables.openapi.json", "Payable", b)
	return p
}
func payableReport(t *testing.T, f *httpContractFixture, token, q string, want int) payables.Report {
	t.Helper()
	s, b := request(t, f.app, "GET", payablesPath+"/report"+q, "", token)
	if s != want {
		t.Fatal(s, want, string(b))
	}
	var r payables.Report
	if want == 200 {
		if e := json.Unmarshal(b, &r); e != nil {
			t.Fatal(e)
		}
		domainSchemaFile(t, "supplier-payables.openapi.json", "PayablesReport", b)
	} else if bytes.Contains(b, []byte("summary")) {
		t.Fatal("partial report")
	}
	return r
}
func payableCounts(t *testing.T, f *httpContractFixture) []int {
	t.Helper()
	out := []int{}
	for _, table := range []string{"supplier_payables", "supplier_payable_installments", "supplier_payable_operations", "supplier_payable_events", "outbox", "sales", "stock_movements", "cash_movements", "cash_journal_entries", "cash_vault_entries"} {
		var n int
		if e := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		out = append(out, n)
	}
	return out
}
func TestHTTPPayablesScheduleReviewAndNoFinancialEffects(t *testing.T) {
	f, reviewer, accountant := payableSetup(t)
	before := payableCounts(t, f)
	first := payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	if first.Payable.State != "pending" || first.Payable.TotalCents != 31500 || first.Payable.PaymentExecutionAvailable || first.Payable.SettledCents != nil {
		t.Fatal(first)
	}
	pending := payableReport(t, f, accountant, payableQueryString, 200)
	if pending.Summary["pending_cents"] != 31500 || pending.Summary["approved_cents"] != 0 {
		t.Fatal(pending)
	}
	payableResult(t, f, f.token, "POST", payablesPath+"/invoice-one/approve", payableApprove, 403)
	accepted := payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	if accepted.Payable.State != "approved" || len(accepted.Payable.History) != 2 {
		t.Fatal(accepted)
	}
	report := payableReport(t, f, accountant, payableQueryString, 200)
	expected := map[string]int64{"installment_count": 3, "pending_cents": 0, "approved_cents": 31500, "rejected_cents": 0, "cancelled_cents": 0, "approved_overdue_cents": 10000, "approved_today_cents": 9600, "approved_future_cents": 11900}
	if !report.Complete || report.Scope != "local_supplier_obligations" || report.SettledCents != nil || report.PaymentExecutionAvailable || !reflect.DeepEqual(expected, report.Summary) {
		t.Fatal(report)
	}
	after := payableCounts(t, f)
	if !reflect.DeepEqual(before[4:], after[4:]) || !reflect.DeepEqual(after[:4], []int{1, 3, 2, 2}) {
		t.Fatal(before, after)
	}
	// Snapshot remains the original supplier name after catalog changes.
	if _, e := f.db.Exec(`UPDATE purchase_suppliers SET name='Novo nome',status='inactive' WHERE id='supplier'`); e != nil {
		t.Fatal(e)
	}
	if p := payableRead(t, f, accountant); p.SupplierName != "Padaria" {
		t.Fatal(p)
	}
	replay := payableResult(t, f, f.token, "POST", payablesPath, payableBody, 200)
	if !replay.Repeated || !reflect.DeepEqual(replay.Payable, first.Payable) {
		t.Fatal(replay)
	}
	original := payableResult(t, f, f.token, "GET", payablesPath+"/operations/create-invoice", "", 200)
	if original.Payable.State != "pending" {
		t.Fatal(original)
	}
	payableResult(t, f, reviewer, "GET", payablesPath+"/operations/create-invoice", "", 404)
}
func TestHTTPPayablesCancellationRejectionAndIdentityConflicts(t *testing.T) {
	f, reviewer, _ := payableSetup(t)
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	cancel := `{"operation_id":"cancel-invoice","reason":"Documento cancelado no fornecedor"}`
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/cancel", cancel, 403)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	payableResult(t, f, f.token, "POST", payablesPath+"/invoice-one/cancel", cancel, 403)
	p := payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/cancel", cancel, 200).Payable
	if p.State != "cancelled" || len(p.History) != 3 {
		t.Fatal(p)
	}
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/reject", `{"operation_id":"reject-too-late","reason":"Conferencia"}`, 409)
	payableResult(t, f, f.token, "POST", payablesPath, strings.NewReplacer("create-invoice", "duplicate-create", "invoice-one", "invoice-two", "NFe-123", "nfe-123").Replace(payableBody), 409)
	payableResult(t, f, f.token, "POST", payablesPath, strings.Replace(payableBody, "Compra de farinha", "Compra divergente", 1), 409)
	second := strings.NewReplacer("create-invoice", "create-second", "invoice-one", "invoice-second", "NFe-123", "NFe-124").Replace(payableBody)
	payableResult(t, f, f.token, "POST", payablesPath, second, 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-second/reject", `{"operation_id":"reject-second","reason":"Documento incorreto"}`, 200)
	third := strings.NewReplacer("create-invoice", "create-third", "invoice-one", "invoice-third", "NFe-123", "NFe-125").Replace(payableBody)
	payableResult(t, f, f.token, "POST", payablesPath, third, 201)
	payableResult(t, f, f.token, "POST", payablesPath+"/invoice-third/cancel", `{"operation_id":"cancel-third","reason":"Registro retirado"}`, 200)
	s, b := request(t, f.app, "GET", payablesPath+"?state=cancelled&limit=1&offset=1", "", f.token)
	if s != 200 {
		t.Fatal(s, string(b))
	}
	var page payables.Page
	json.Unmarshal(b, &page)
	domainSchemaFile(t, "supplier-payables.openapi.json", "PayablePage", b)
	if page.TotalCount != 2 || len(page.Items) != 1 || page.HasMore {
		t.Fatal(page)
	}
}
func TestHTTPPayablesStrictBodiesAndQueries(t *testing.T) {
	f, _, _ := payableSetup(t)
	before := payableCounts(t, f)
	for _, body := range []string{
		strings.Replace(payableBody, `"total_cents":31500`, `"total_cents":31500.0`, 1),
		strings.Replace(payableBody, `"total_cents":31500`, `"total_cents":31501`, 1),
		strings.Replace(payableBody, `"total_cents":31500`, `"total_cents":9007199254740992`, 1),
		strings.Replace(payableBody, `"id":"part-one"`, `"id":"part-one","id":"duplicate"`, 1),
		strings.Replace(payableBody, `"id":"part-one"`, `"id":"part-one","approved":true`, 1),
		strings.Replace(payableBody, `"amount_cents":10000`, `"amount_cents":null`, 1),
		strings.Replace(payableBody, `"id":"part-two"`, `"id":"part-one"`, 1),
		strings.Replace(payableBody, `"due_on":"2026-10-10"`, `"due_on":"2026-02-30"`, 1),
		strings.Replace(payableBody, `"due_on":"2026-10-10"`, `"due_on":"2026-09-30"`, 1),
		strings.Replace(payableBody, `"due_on":"2026-10-20"`, `"due_on":"2026-10-09"`, 1),
		strings.Replace(payableBody, `"currency":"BRL"`, `"currency":"USD"`, 1),
		strings.Replace(payableBody, `"operation_id":`, `"tenant_id":"foreign","operation_id":`, 1), payableBody + `{}`,
	} {
		payableResult(t, f, f.token, "POST", payablesPath, body, 400)
	}
	if !reflect.DeepEqual(before, payableCounts(t, f)) {
		t.Fatal("invalid body wrote")
	}
	for _, q := range []string{"?from=2026-10-01&to=2026-10-20", "?from=2026-10-20&to=2026-10-01&as_of=2026-10-20", payableQueryString + "&limit=1", payableQueryString + "&from=2026-10-01", "?from=2026-01-01&to=2027-01-02&as_of=2026-01-01"} {
		payableReport(t, f, f.token, q, 400)
	}
	for _, q := range []string{"?state=paid", "?limit=0", "?offset=-1", "?limit=1&limit=2", "?tenant_id=foreign"} {
		if s, b := request(t, f.app, "GET", payablesPath+q, "", f.token); s != 400 {
			t.Fatal(q, s, string(b))
		}
	}
}
func TestHTTPPayablesRolesLicenseAndHistoricalRead(t *testing.T) {
	f, reviewer, accountant := payableSetup(t)
	payableResult(t, f, accountant, "POST", payablesPath, payableBody, 403)
	policyPerson(t, f.db, f.owner, "payable-cashier", "cashier")
	cashier := loginToken(t, f.app, "payable-cashier")
	payableResult(t, f, cashier, "POST", payablesPath, payableBody, 403)
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	if s, _ := request(t, f.app, "GET", payablesPath+"/invoice-one", "", cashier); s != 403 {
		t.Fatal(s)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core}), 200)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 403)
	payableRead(t, f, accountant)
	payableReport(t, f, accountant, payableQueryString, 200)
	if _, e := f.db.Exec(`INSERT INTO access_rules VALUES (?,?,'member','payable-accountant','view_accounting','deny',1)`, f.owner.TenantID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	if s, _ := request(t, f.app, "GET", payablesPath+"/invoice-one", "", accountant); s != 403 {
		t.Fatal(s)
	}
}
func TestHTTPPayablesRollbackAndConcurrentDuplicate(t *testing.T) {
	f, reviewer, _ := payableSetup(t)
	before := payableCounts(t, f)
	if _, e := f.db.Exec(`CREATE TRIGGER fail_payable_event BEFORE INSERT ON supplier_payable_events BEGIN SELECT RAISE(ABORT,'injected event failure'); END`); e != nil {
		t.Fatal(e)
	}
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 500)
	if !reflect.DeepEqual(before, payableCounts(t, f)) {
		t.Fatal("partial create")
	}
	if _, e := f.db.Exec(`DROP TRIGGER fail_payable_event`); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, id := range []string{"one", "two"} {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := strings.NewReplacer("create-invoice", "race-create-"+id, "invoice-one", "race-invoice-"+id).Replace(payableBody)
			s, _ := request(t, f.app, "POST", payablesPath, body, f.token)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict := 0, 0
	for s := range statuses {
		if s == 201 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
	var id string
	f.db.QueryRow(`SELECT id FROM supplier_payables`).Scan(&id)
	statuses = make(chan int, 2)
	for _, action := range []string{"approve", "reject"} {
		action := action
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := request(t, f.app, "POST", payablesPath+"/"+id+"/"+action, strings.Replace(payableApprove, "approve-invoice", action+"-race", 1), reviewer)
			statuses <- s
		}()
	}
	wg.Wait()
	close(statuses)
	ok, conflict = 0, 0
	for s := range statuses {
		if s == 200 {
			ok++
		} else if s == 409 {
			conflict++
		} else {
			t.Fatal(s)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
}
func TestHTTPPayablesCSVExactDatesAndNoFormulaRewrite(t *testing.T) {
	f, reviewer, accountant := payableSetup(t)
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	one := payableReport(t, f, accountant, "?from=2026-10-20&to=2026-10-20&as_of=2026-10-20", 200)
	if len(one.Items) != 1 || one.Summary["approved_cents"] != 9600 {
		t.Fatal(one)
	}
	s, b := request(t, f.app, "GET", payablesPath+"/report.csv"+payableQueryString, "", accountant)
	if s != 200 || !bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal(s, string(b))
	}
	reader := csv.NewReader(bytes.NewReader(b[3:]))
	reader.Comma = ';'
	rows, e := reader.ReadAll()
	if e != nil || len(rows) != 12 {
		t.Fatal(len(rows), e)
	}
	if rows[9][16] != "10000" || rows[10][16] != "9600" || rows[11][16] != "11900" {
		t.Fatal(rows)
	}
	// This is a newly created unsafe supplier snapshot, not a mutation of immutable history.
	if _, e = f.db.Exec(`UPDATE purchase_suppliers SET name='=HYPERLINK(1)' WHERE id='supplier'`); e != nil {
		t.Fatal(e)
	}
	payableResult(t, f, f.token, "POST", payablesPath, strings.NewReplacer("create-invoice", "create-unsafe", "invoice-one", "invoice-unsafe", "NFe-123", "NFe-unsafe").Replace(payableBody), 201)
	r := payableReport(t, f, accountant, payableQueryString, 200)
	found := false
	for _, v := range r.Items {
		if v.SupplierName == "=HYPERLINK(1)" {
			found = true
		}
	}
	if !found {
		t.Fatal("canonical name changed")
	}
	if s, b = request(t, f.app, "GET", payablesPath+"/report.csv"+payableQueryString, "", accountant); s != 409 || bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal(s, string(b))
	}
}
func TestHTTPPayablesCorruptionRefusesEmptyPagesAndHistoricalReplay(t *testing.T) {
	f, _, _ := payableSetup(t)
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	// Test-only corruption after deliberately removing the immutable event trigger.
	if _, e := f.db.Exec(`DROP TRIGGER supplier_payable_events_no_update; UPDATE supplier_payable_events SET payload_json='{}'`); e != nil {
		t.Fatal(e)
	}
	if s, _ := request(t, f.app, "GET", payablesPath+"?offset=999", "", f.token); s != 409 {
		t.Fatal(s)
	}
	payableReport(t, f, f.token, payableQueryString, 409)
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 409)
}
func TestHTTPPayablesMaximumPrecisionAndAggregateOverflow(t *testing.T) {
	f, reviewer, _ := payableSetup(t)
	var in payables.CreateInput
	json.Unmarshal([]byte(payableBody), &in)
	in.TotalCents = payables.MaxSafe
	in.Installments = []payables.Installment{{ID: "part", DueOn: "2026-10-20", AmountCents: payables.MaxSafe}}
	b, _ := json.Marshal(in)
	payableResult(t, f, f.token, "POST", payablesPath, string(b), 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	if r := payableReport(t, f, f.token, payableQueryString, 200); r.Summary["approved_cents"] != payables.MaxSafe {
		t.Fatal(r)
	}
	in.OperationID = "create-over"
	in.PayableID = "invoice-over"
	in.DocumentReference = "NFe-over"
	in.TotalCents = 1
	in.Installments[0].AmountCents = 1
	b, _ = json.Marshal(in)
	payableResult(t, f, f.token, "POST", payablesPath, string(b), 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-over/approve", strings.Replace(payableApprove, "approve-invoice", "approve-over", 1), 200)
	payableReport(t, f, f.token, payableQueryString, 422)
}
func TestHTTPPayablesBackupRestartAndLostResponse(t *testing.T) {
	f, reviewer, _ := payableSetup(t)
	want := payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	current := payableRead(t, f, f.token)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "payables.tytbak")
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "restored.sqlite")
	if e := backup.Restore(ctx, archive, path, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	got, e := payables.Get(ctx, db, a, f.device, "invoice-one")
	if e != nil || !reflect.DeepEqual(got, current) {
		t.Fatal(got, e)
	}
	var n int
	var name, original string
	if e = f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &original); e != nil {
		t.Fatal(e)
	}
	if e = f.db.Close(); e != nil {
		t.Fatal(e)
	}
	f.db, e = localdb.Open(ctx, original)
	if e != nil {
		t.Fatal(e)
	}
	defer f.db.Close()
	verifier, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	f.app, e = NewWithVerifier(f.db, f.device, verifier)
	if e != nil {
		t.Fatal(e)
	}
	retry := payableResult(t, f, f.token, "POST", payablesPath, payableBody, 200)
	if !retry.Repeated || !reflect.DeepEqual(retry.Payable, want.Payable) {
		t.Fatal(retry)
	}
	for _, q := range []string{`UPDATE supplier_payables SET total_cents=1`, `DELETE FROM supplier_payables`, `UPDATE supplier_payable_installments SET amount_cents=1`, `DELETE FROM supplier_payable_operations`, `DELETE FROM supplier_payable_events`} {
		if _, e = f.db.Exec(q); e == nil {
			t.Fatal("immutable bypass", q)
		}
	}
	var violation any
	rows, e := f.db.Query(`PRAGMA foreign_key_check`)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation", violation)
	}
}
func TestHTTPPayablesInactiveMissingSupplierAndIdempotentReview(t *testing.T) {
	f, reviewer, _ := payableSetup(t)
	before := payableCounts(t, f)
	payableResult(t, f, f.token, "POST", payablesPath, strings.Replace(payableBody, `"supplier_id":"supplier"`, `"supplier_id":"missing"`, 1), 404)
	if _, e := f.db.Exec(`UPDATE purchase_suppliers SET status='inactive'`); e != nil {
		t.Fatal(e)
	}
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 409)
	if !reflect.DeepEqual(before, payableCounts(t, f)) {
		t.Fatal("failed supplier check wrote")
	}
	if _, e := f.db.Exec(`UPDATE purchase_suppliers SET status='active'`); e != nil {
		t.Fatal(e)
	}
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	first := payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	retry := payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
	if !retry.Repeated || !reflect.DeepEqual(first.Payable, retry.Payable) {
		t.Fatal(retry)
	}
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", strings.Replace(payableApprove, "Documento e vencimentos conferidos", "Motivo diferente", 1), 409)
}
