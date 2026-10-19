package localapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/cashaccounting"
)

const accountingPath = "/local/v1/accounting/cash"
const closedFirst = "2030-01-01T00:00:00.000000001Z"
const closedSecond = "2030-01-01T00:00:00.000000002Z"
const periodEnd = "2030-01-01T00:00:00.000000003Z"

func accountingQuery(from, to string) string {
	return "?" + url.Values{"from": {from}, "to": {to}}.Encode()
}
func setAccountingClosedAt(t *testing.T, f *stockHTTPFixture, id, at string) {
	t.Helper()
	tx, err := f.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, q := range []string{`UPDATE cash_sessions SET closed_at=? WHERE tenant_id=? AND id=?`, `UPDATE cash_closures SET closed_at=? WHERE tenant_id=? AND session_id=?`} {
		if _, err := tx.Exec(q, at, f.owner.TenantID, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
func accountingSetup(t *testing.T) (*stockHTTPFixture, string, string) {
	t.Helper()
	f := salesSetup(t)
	salesRequest(t, f, salesInput(f), 201)
	cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", withdrawalBody, 200)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":4400}`, 200)
	setAccountingClosedAt(t, f, "turn-one", closedFirst)
	policyPerson(t, f.db, f.owner, "period-cashier", "cashier")
	cashier := loginToken(t, f.app, "period-cashier")
	if s, b := request(t, f.app, "POST", "/local/v1/cash/open", `{"session_id":"turn-two","opening_cents":2000}`, cashier); s != 201 {
		t.Fatal(s, string(b))
	}
	cashRequest(t, f.httpContractFixture, "movements", `{"session_id":"turn-two","operation_id":"supply-two","kind":"supply","amount_cents":100,"reason":"Troco do segundo turno"}`, 200)
	if s, b := request(t, f.app, "POST", "/local/v1/cash/close", `{"session_id":"turn-two","operation_id":"close-two","declared_cents":2095}`, cashier); s != 200 {
		t.Fatal(s, string(b))
	}
	setAccountingClosedAt(t, f, "turn-two", closedSecond)
	if s, b := request(t, f.app, "POST", "/local/v1/cash/open", `{"session_id":"turn-three","opening_cents":999}`, cashier); s != 201 {
		t.Fatal(s, string(b))
	}
	policyPerson(t, f.db, f.owner, "period-accountant", "accountant")
	accountant := loginToken(t, f.app, "period-accountant")
	return f, accountant, cashier
}
func accountingReport(t *testing.T, f *stockHTTPFixture, token, query string, want int) cashaccounting.Report {
	t.Helper()
	s, b := request(t, f.app, "GET", accountingPath+"/report"+query, "", token)
	if s != want {
		t.Fatalf("accounting %d want%d %s", s, want, b)
	}
	var r cashaccounting.Report
	if want == 200 {
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-accounting.openapi.json", "AccountingReport", b)
	} else if bytes.Contains(b, []byte("summary")) {
		t.Fatal("partial report", string(b))
	}
	return r
}
func accountingCounts(t *testing.T, f *stockHTTPFixture) []int {
	t.Helper()
	counts := []int{}
	for _, table := range []string{"cash_sessions", "cash_closures", "sales", "sale_payments", "stock_movements", "cash_movements", "cash_journal_entries", "outbox", "checkout_payment_proofs", "cash_vaults"} {
		var n int
		if err := f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		counts = append(counts, n)
	}
	return counts
}
func TestHTTPAccountingPeriodCrossOperatorTotalsExcludeCurrentDrawer(t *testing.T) {
	f, accountant, cashier := accountingSetup(t)
	before := accountingCounts(t, f)
	q := accountingQuery(closedFirst, periodEnd)
	r := accountingReport(t, f, accountant, q, 200)
	expected := map[string]int64{"session_count": 2, "sale_count": 1, "gross_sales_cents": 3198, "cancelled_sales_cents": 0, "net_sales_cents": 3198, "sum_openings_cents": 3000, "sum_expected_at_closures_cents": 6498, "sum_declared_at_closures_cents": 6495, "sum_closure_differences_cents": -3, "supply_cents": 600, "withdrawal_cents": 300, "journal_entry_count": 6, "journal_debit_cents": 7098, "journal_credit_cents": 7098, "unclassified_count": 0}
	if !r.Complete || r.Scope != "local_closed_cash_sessions" || r.BankSettlementAvailable || r.StatutoryLedgerAvailable || !reflect.DeepEqual(expected, r.Summary) || len(r.Sessions) != 2 || len(r.Entries) != 6 {
		t.Fatal(r)
	}
	if r.Sessions[0].OperatorID != f.owner.OwnerID || r.Sessions[1].OperatorID != "period-cashier" || r.Payments[0].ConfirmedCents != 3198 {
		t.Fatal(r)
	}
	for _, e := range r.Entries {
		if e.SessionID == "turn-three" {
			t.Fatal("open drawer leaked", e)
		}
		if e.SessionID == "turn-two" && e.Entry.Category == "supply" && (e.Entry.ActorID == nil || *e.Entry.ActorID != f.owner.OwnerID || e.OperatorID != "period-cashier") {
			t.Fatal("source actor rewritten", e)
		}
	}
	accountingReport(t, f, cashier, q, 403)
	if s, _ := request(t, f.app, "GET", "/local/v1/cash/sessions/turn-two/report", "", f.token); s != 404 {
		t.Fatal("legacy operator ownership expanded", s)
	}
	if s, _ := request(t, f.app, "GET", "/local/v1/cash/sessions/turn-one/report", "", accountant); s != 403 {
		t.Fatal("accountant was granted Sell", s)
	}
	if !reflect.DeepEqual(before, accountingCounts(t, f)) {
		t.Fatal("read changed financial data")
	}
}
func TestHTTPAccountingPeriodExactWindowTimezoneAndPagination(t *testing.T) {
	f, accountant, _ := accountingSetup(t)
	first := accountingReport(t, f, accountant, accountingQuery(closedFirst, closedSecond), 200)
	if len(first.Sessions) != 1 || first.Sessions[0].Report.SessionID != "turn-one" {
		t.Fatal(first)
	}
	second := accountingReport(t, f, accountant, accountingQuery(closedSecond, periodEnd), 200)
	if len(second.Sessions) != 1 || second.Sessions[0].Report.SessionID != "turn-two" {
		t.Fatal(second)
	}
	zone := accountingReport(t, f, accountant, accountingQuery("2029-12-31T21:00:00.000000001-03:00", periodEnd), 200)
	if zone.Window.From != closedFirst || zone.Summary["session_count"] != 2 {
		t.Fatal(zone)
	}
	q := accountingQuery(closedFirst, periodEnd)
	s, b := request(t, f.app, "GET", accountingPath+"/sessions"+q+"&limit=1&offset=1", "", accountant)
	if s != 200 {
		t.Fatal(s, string(b))
	}
	domainSchemaFile(t, "cash-accounting.openapi.json", "SessionPage", b)
	var p cashaccounting.SessionPage
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.TotalCount != 2 || p.HasMore || len(p.Items) != 1 || p.Items[0].Report.SessionID != "turn-two" || p.Summary["session_count"] != 2 {
		t.Fatal(p)
	}
	s, b = request(t, f.app, "GET", accountingPath+"/journal"+q+"&limit=1&offset=4", "", accountant)
	if s != 200 {
		t.Fatal(s, string(b))
	}
	domainSchemaFile(t, "cash-accounting.openapi.json", "PostingPage", b)
	var j cashaccounting.PostingPage
	if err := json.Unmarshal(b, &j); err != nil {
		t.Fatal(err)
	}
	if j.TotalCount != 6 || !j.HasMore || len(j.Items) != 1 || j.Items[0].SessionID != "turn-two" || j.Summary["journal_entry_count"] != 6 {
		t.Fatal(j)
	}
	empty := accountingReport(t, f, accountant, accountingQuery(periodEnd, "2030-01-02T00:00:00Z"), 200)
	if !empty.Complete || len(empty.Sessions) != 0 || len(empty.Entries) != 0 || empty.Summary["sum_expected_at_closures_cents"] != 0 {
		t.Fatal(empty)
	}
}
func accountingCSV(t *testing.T, f *stockHTTPFixture, token, query string, want int) []map[string]string {
	t.Helper()
	req := httptest.NewRequest("GET", accountingPath+"/report.csv"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := f.app.Test(req, 10000)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("CSV %d want%d %s", resp.StatusCode, want, body)
	}
	if want != 200 {
		if bytes.HasPrefix(body, []byte{0xef, 0xbb, 0xbf}) || bytes.Contains(body, []byte("format_version")) || resp.Header.Get("Content-Disposition") != "" {
			t.Fatal("partial CSV", string(body))
		}
		return nil
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Content-Disposition") != `attachment; filename="titan-caixas-encerrados.csv"` || resp.Header.Get("Content-Type") != "text/csv; charset=utf-8" || !bytes.HasPrefix(body, []byte{0xef, 0xbb, 0xbf}) || !bytes.Contains(body, []byte("\r\n")) {
		t.Fatal(resp.Header, string(body))
	}
	reader := csv.NewReader(bytes.NewReader(body[3:]))
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	items := []map[string]string{}
	for _, row := range rows[1:] {
		item := map[string]string{}
		for i, key := range rows[0] {
			item[key] = row[i]
		}
		items = append(items, item)
	}
	return items
}
func TestHTTPAccountingCSVMatchesSummarySourcesAndPayments(t *testing.T) {
	f, accountant, _ := accountingSetup(t)
	q := accountingQuery(closedFirst, periodEnd)
	r := accountingReport(t, f, accountant, q, 200)
	rows := accountingCSV(t, f, accountant, q, 200)
	metrics, journals, sessions, payments := 0, 0, 0, 0
	for _, row := range rows {
		switch row["row_type"] {
		case "period_metric":
			metrics++
			if row["metric"] == "sum_expected_at_closures_cents" && row["metric_value"] != "6498" {
				t.Fatal(row)
			}
		case "session_metric":
			sessions++
		case "period_payment":
			payments++
			if row["method"] == "cash" && row["recorded_confirmed_cents"] != "3198" {
				t.Fatal(row)
			}
		case "journal":
			journals++
			if row["session_id"] == "turn-three" || row["source_id"] == "" || row["debit_account"] == "" || row["credit_account"] == "" {
				t.Fatal(row)
			}
		}
	}
	if metrics != len(r.Summary) || journals != len(r.Entries) || sessions != 20 || payments != 4 {
		t.Fatal(metrics, journals, sessions, payments)
	}
}
func TestHTTPAccountingInvalidInputsAndHistoricPermissionRechecks(t *testing.T) {
	f, accountant, _ := accountingSetup(t)
	q := accountingQuery(closedFirst, periodEnd)
	for _, bad := range []string{"", "?from=x&to=y", accountingQuery(periodEnd, closedFirst), accountingQuery(closedFirst, closedFirst), accountingQuery("2030-01-01T00:00:00Z", "2030-02-02T00:00:00Z"), q + "&from=" + url.QueryEscape(closedFirst), q + "&operator_id=other", q + "&limit=1"} {
		accountingReport(t, f, accountant, bad, 400)
	}
	for _, bad := range []string{"&limit=0", "&limit=101", "&offset=-1", "&offset=1000000001", "&limit=1&limit=2"} {
		if s, _ := request(t, f.app, "GET", accountingPath+"/journal"+q+bad, "", accountant); s != 400 {
			t.Fatal(bad, s)
		}
	}
	if s, _ := request(t, f.app, "GET", accountingPath+"/report"+q, "{}", accountant); s != 400 {
		t.Fatal(s)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core}), 200)
	accountingReport(t, f, accountant, q, 200)
	if _, err := f.db.Exec(`INSERT INTO access_rules VALUES(?,?,'member','period-accountant','view_accounting','deny',1)`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	accountingReport(t, f, accountant, q, 403)
	if _, err := f.db.Exec(`DELETE FROM access_rules WHERE target_id='period-accountant'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE identity_id='period-accountant'`); err != nil {
		t.Fatal(err)
	}
	accountingReport(t, f, accountant, q, 401)
}
