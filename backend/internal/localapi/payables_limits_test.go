package localapi

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"titansystem-backend/internal/localdb/payables"
)

func TestHTTPPayablesTransitionFailureRollsBackStateAndReceipt(t *testing.T) {
	f, reviewer, _ := payableSetup(t)
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	before := payableCounts(t, f)
	p := payableRead(t, f, f.token)
	if _, e := f.db.Exec(`CREATE TRIGGER fail_payable_review BEFORE INSERT ON supplier_payable_events WHEN NEW.event_type='supplier.payable.approve' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); e != nil {
		t.Fatal(e)
	}
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 500)
	if !reflect.DeepEqual(before, payableCounts(t, f)) || !reflect.DeepEqual(p, payableRead(t, f, f.token)) {
		t.Fatal("partial review")
	}
	if _, e := f.db.Exec(`DROP TRIGGER fail_payable_review`); e != nil {
		t.Fatal(e)
	}
	payableResult(t, f, reviewer, "POST", payablesPath+"/invoice-one/approve", payableApprove, 200)
}

func TestHTTPPayablesReportTenThousandInstallmentsAndExplicitLimit(t *testing.T) {
	f, _, _ := payableSetup(t)
	var in payables.CreateInput
	if e := json.Unmarshal([]byte(payableBody), &in); e != nil {
		t.Fatal(e)
	}
	create := func(n, parts int) {
		in.OperationID = fmt.Sprintf("bulk-create-%d", n)
		in.PayableID = fmt.Sprintf("bulk-payable-%d", n)
		in.DocumentReference = fmt.Sprintf("NF-bulk-%d", n)
		in.TotalCents = int64(parts)
		in.Installments = []payables.Installment{}
		for i := 0; i < parts; i++ {
			in.Installments = append(in.Installments, payables.Installment{ID: fmt.Sprintf("part-%03d", i), DueOn: "2026-10-20", AmountCents: 1})
		}
		b, e := json.Marshal(in)
		if e != nil {
			t.Fatal(e)
		}
		payableResult(t, f, f.token, "POST", payablesPath, string(b), 201)
	}
	for n := 0; n < 83; n++ {
		create(n, 120)
	}
	create(83, 40)
	r := payableReport(t, f, f.token, payableQueryString, 200)
	if len(r.Items) != 10000 || r.Summary["pending_cents"] != 10000 {
		t.Fatal(len(r.Items), r.Summary)
	}
	if s, b := request(t, f.app, "GET", payablesPath+"/report.csv"+payableQueryString, "", f.token); s != 200 || strings.Count(string(b), "\r\n") != 10014 {
		t.Fatal(s, len(b))
	}
	create(84, 1)
	payableReport(t, f, f.token, payableQueryString, 422)
	if s, b := request(t, f.app, "GET", payablesPath+"/report.csv"+payableQueryString, "", f.token); s != 422 || strings.Contains(string(b), "format_version") {
		t.Fatal(s, string(b))
	}
}
func TestHTTPPayablesDocumentGuardDoesNotReturnPartialPage(t *testing.T) {
	f, _, _ := payableSetup(t)
	payableResult(t, f, f.token, "POST", payablesPath, payableBody, 201)
	// Guard-only fixture: duplicate header metadata references the existing FK-valid
	// operation. It is deliberately not evidence of 1001 valid obligation histories.
	tx, e := f.db.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for n := 1; n <= 1000; n++ {
		id := fmt.Sprintf("guard-%04d", n)
		if _, e = tx.Exec(`INSERT INTO supplier_payables SELECT tenant_id,store_id,device_id,?,supplier_id,supplier_name,?,description,issued_on,currency,total_cents,creator_id,state,created_at,updated_at,last_operation_id FROM supplier_payables WHERE id='invoice-one'`, id, id); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	if s, b := request(t, f.app, "GET", payablesPath+"?offset=999999", "", f.token); s != 422 || strings.Contains(string(b), "items") {
		t.Fatal(s, string(b))
	}
	payableReport(t, f, f.token, payableQueryString, 422)
}
