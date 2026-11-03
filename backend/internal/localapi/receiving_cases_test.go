package localapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
)

const casesPath = "/local/v1/purchase-receiving-cases"
const caseOpenPath = "/local/v1/purchase-orders/order/receiving-cases"

func caseCommercialState(t *testing.T, f *httpContractFixture) []int64 {
	t.Helper()
	out := []int64{}
	for _, table := range []string{"stock_movements", "stock_operations", "cash_movements", "purchase_receipts", "purchase_receipt_voids", "purchase_delivery_rejections", "purchase_order_cancellations", "dock_agenda_appointments"} {
		var n int64
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		out = append(out, n)
	}
	var balance int64
	if e := f.db.QueryRow(`SELECT COALESCE(sum(quantity_milli),0) FROM stock_movements`).Scan(&balance); e != nil {
		t.Fatal(e)
	}
	return append(out, balance)
}

func caseOpenBody() purchases.CaseOpenInput {
	return purchases.CaseOpenInput{OperationID: "case-open", CaseID: "case", Kind: "shortage", RequestedOutcome: "replacement", Reference: "delivery-1", DueAt: "2026-01-01T00:00:00Z", Reason: "Confirmar quantidade e acordo com fornecedor"}
}
func caseResponse(t *testing.T, f *httpContractFixture, path string, in any, want int) purchases.CaseResult {
	t.Helper()
	b := agendaCall(t, f, "POST", path, in, want)
	var out purchases.CaseResult
	if want == 200 && json.Unmarshal(b, &out) != nil {
		t.Fatal(string(b))
	}
	return out
}
func caseActionBody(action string, rev int64) purchases.CaseActionInput {
	return purchases.CaseActionInput{OperationID: fmt.Sprintf("case-%s-%d", action, rev), ExpectedRevision: rev, Action: action, Reason: "Conferido pelo responsável"}
}
func caseGet(t *testing.T, f *httpContractFixture) purchases.CaseDetail {
	t.Helper()
	b := agendaCall(t, f, "GET", casesPath+"/case", nil, 200)
	var out purchases.CaseDetail
	if e := json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestReceivingCaseLifecycleImmutableSnapshotReceiptAndNoCommercialEffects(t *testing.T) {
	f := overviewFixture(t)
	receivedResponse(t, f, receivingBody(), 201)
	before := caseCommercialState(t, f)
	first := caseResponse(t, f, caseOpenPath, caseOpenBody(), 200)
	if first.Case.Revision != 1 || first.Case.OpeningEvidence.AcceptedMilli != 2000 || first.Case.OpeningEvidence.RemainingMilli != 3000 || first.Case.Status != "open" {
		t.Fatal(first)
	}
	if !caseGet(t, f).Overdue {
		t.Fatal("past deadline not visible")
	}
	for i, action := range []string{"start", "note", "assign", "set_due", "resolve"} {
		in := caseActionBody(action, int64(i+1))
		if action == "assign" {
			in.AssignedTo = f.owner.OwnerID
		}
		if action == "set_due" {
			in.DueAt = "2026-01-02T00:00:00Z"
		}
		out := caseResponse(t, f, casesPath+"/case/actions", in, 200)
		if out.Case.Revision != int64(i+2) {
			t.Fatal(out)
		}
	}
	if out := caseGet(t, f); out.Case.Status != "resolved" || out.Overdue {
		t.Fatal(out)
	}
	repeat := caseResponse(t, f, caseOpenPath, caseOpenBody(), 200)
	if !repeat.Repeated || repeat.Case.Revision != 1 {
		t.Fatal(repeat)
	}
	b := agendaCall(t, f, "GET", "/local/v1/purchase-receiving-case-operations/case-open", nil, 200)
	var receipt purchases.CaseResult
	if json.Unmarshal(b, &receipt) != nil || !reflect.DeepEqual(receipt, first) {
		t.Fatal(receipt)
	}
	changed := caseOpenBody()
	changed.Reason = "Changed payload"
	caseResponse(t, f, caseOpenPath, changed, 409)
	b = agendaCall(t, f, "GET", casesPath+"/case/history?limit=2&offset=4", nil, 200)
	var history purchases.CaseHistory
	if json.Unmarshal(b, &history) != nil || history.Total != 6 || len(history.Items) != 2 || history.Items[0].Result.Case.Revision != 5 {
		t.Fatal(history)
	}
	if !reflect.DeepEqual(before, caseCommercialState(t, f)) {
		t.Fatal("case changed stock/financial/purchase state")
	}
	caseResponse(t, f, casesPath+"/case/actions", caseActionBody("note", 6), 409)
	caseResponse(t, f, casesPath+"/case/actions", caseActionBody("reopen", 6), 200)
	caseResponse(t, f, casesPath+"/case/actions", caseActionBody("cancel", 7), 200)
	var page purchases.CaseList
	b = agendaCall(t, f, "GET", casesPath+"?status=cancelled&order_id=order", nil, 200)
	if json.Unmarshal(b, &page) != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatal(page)
	}
}
func caseWorker(t *testing.T, f *httpContractFixture, id string) string {
	t.Helper()
	for _, q := range []string{`INSERT INTO identities VALUES(?,?,'now')`, `INSERT INTO memberships VALUES(?,?,'stock','active','now')`, `INSERT INTO membership_stores VALUES(?,?,?)`} {
		var args []any
		switch {
		case strings.Contains(q, "identities"):
			args = []any{id, id}
		case strings.Contains(q, "membership_stores"):
			args = []any{f.owner.TenantID, id, f.owner.StoreID}
		default:
			args = []any{f.owner.TenantID, id}
		}
		if _, e := f.db.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	if e := localauth.SetPassword(context.Background(), f.db, agendaActor(f), f.device, id, "senha-forte-de-teste-1"); e != nil {
		t.Fatal(e)
	}
	return loginToken(t, f.app, id)
}
func TestReceivingCaseEmployeeCanWorkButOnlyManagerClosesAndReassigns(t *testing.T) {
	f := overviewFixture(t)
	token := caseWorker(t, f, "case-worker")
	other := caseWorker(t, f, "case-other")
	in := caseOpenBody()
	in.AssignedTo = "case-worker"
	caseResponse(t, f, caseOpenPath, in, 200)
	body, _ := json.Marshal(caseActionBody("start", 1))
	status, b := request(t, f.app, "POST", casesPath+"/case/actions", string(body), token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
	body, _ = json.Marshal(caseActionBody("resolve", 2))
	status, _ = request(t, f.app, "POST", casesPath+"/case/actions", string(body), token)
	if status != 403 {
		t.Fatal(status)
	}
	body, _ = json.Marshal(caseActionBody("note", 2))
	status, _ = request(t, f.app, "POST", casesPath+"/case/actions", string(body), other)
	if status != 403 {
		t.Fatal(status)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/purchase-receiving-case-operations/case-open", "", token)
	if status != 404 {
		t.Fatal("receipt leaked", status)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id='case-other'`); e != nil {
		t.Fatal(e)
	}
	assign := caseActionBody("assign", 2)
	assign.AssignedTo = "case-other"
	caseResponse(t, f, casesPath+"/case/actions", assign, 403)
	caseResponse(t, f, casesPath+"/case/actions", caseActionBody("resolve", 2), 200)
}
func TestReceivingCaseEveryWriteIgnoredRollsBack(t *testing.T) {
	for _, table := range []string{"purchase_receiving_cases", "purchase_receiving_case_operations", "purchase_receiving_case_events", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := overviewFixture(t)
			if _, e := f.db.Exec("CREATE TRIGGER ignore_case BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			caseResponse(t, f, caseOpenPath, caseOpenBody(), 409)
			for _, name := range []string{"purchase_receiving_cases", "purchase_receiving_case_operations", "purchase_receiving_case_events"} {
				materialCount(t, f, name, 0)
			}
		})
	}
}
func TestReceivingCaseIgnoredUpdateAndStaleRevisionPreserveOriginal(t *testing.T) {
	f := overviewFixture(t)
	caseResponse(t, f, caseOpenPath, caseOpenBody(), 200)
	if _, e := f.db.Exec(`CREATE TRIGGER ignore_case_update BEFORE UPDATE ON purchase_receiving_cases BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	caseResponse(t, f, casesPath+"/case/actions", caseActionBody("resolve", 1), 409)
	if caseGet(t, f).Case.Revision != 1 {
		t.Fatal("ignored update accepted")
	}
	materialCount(t, f, "purchase_receiving_case_operations", 1)
}
func TestReceivingCaseConcurrentReplayAndConflictingRevision(t *testing.T) {
	f := overviewFixture(t)
	raw, _ := json.Marshal(caseOpenBody())
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, replayed := 0, 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, b := request(t, f.app, "POST", caseOpenPath, string(raw), f.token)
			var v purchases.CaseResult
			_ = json.Unmarshal(b, &v)
			mu.Lock()
			defer mu.Unlock()
			if code == 200 {
				ok++
				if v.Repeated {
					replayed++
				}
			}
		}()
	}
	wg.Wait()
	if ok != 6 || replayed != 5 {
		t.Fatal(ok, replayed)
	}
	materialCount(t, f, "purchase_receiving_case_events", 1)
	ok = 0
	conflict := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := caseActionBody("note", 1)
			in.OperationID = fmt.Sprintf("concurrent-note-%d", i)
			raw, _ := json.Marshal(in)
			code, _ := request(t, f.app, "POST", casesPath+"/case/actions", string(raw), f.token)
			mu.Lock()
			defer mu.Unlock()
			if code == 200 {
				ok++
			}
			if code == 409 {
				conflict++
			}
		}(i)
	}
	wg.Wait()
	if ok != 1 || conflict != 1 {
		t.Fatal(ok, conflict)
	}
}
func TestReceivingCaseBackupRestoreAuditTamperAndDuePreserved(t *testing.T) {
	f := overviewFixture(t)
	caseResponse(t, f, caseOpenPath, caseOpenBody(), 200)
	before := caseGet(t, f)
	db := restoredPurchaseDB(t, f)
	defer db.Close()
	after, e := purchases.GetReceivingCase(context.Background(), db, agendaActor(f), f.device, "case")
	before.MeasuredAt = ""
	after.MeasuredAt = ""
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatal(before, after, e)
	}
	if _, e = f.db.Exec(`UPDATE purchase_receiving_cases SET last_reason='tampered'`); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", casesPath+"/case", nil, 409)
}
func TestReceivingCaseStrictInputScopeAndReadAfterContractRemoval(t *testing.T) {
	f := overviewFixture(t)
	in := caseOpenBody()
	raw, _ := json.Marshal(in)
	for _, body := range []string{"{}", "null", strings.Replace(string(raw), `"case_id":"case"`, `"case_id":"case","case_id":"other"`, 1), strings.Replace(string(raw), `"kind":"shortage"`, `"kind":"refund_paid"`, 1)} {
		status, _ := request(t, f.app, "POST", caseOpenPath, body, f.token)
		if status != 400 {
			t.Fatal(status, body)
		}
	}
	caseResponse(t, f, caseOpenPath, in, 200)
	for _, q := range []string{"?tenant_id=other", "?status=", "?limit=1&limit=2", "?offset=-1"} {
		agendaCall(t, f, "GET", casesPath+q, nil, 400)
	}
	agendaCall(t, f, "GET", casesPath+"/case", map[string]any{"body": true}, 400)
	for _, kind := range []string{"tenant", "store", "user", "device"} {
		a := agendaActor(f)
		d := f.device
		switch kind {
		case "tenant":
			a.TenantID = "foreign"
		case "store":
			a.StoreID = "foreign"
		case "user":
			a.IdentityID = "foreign"
		case "device":
			d.DeviceID = "foreign"
		}
		if _, e := purchases.GetReceivingCase(context.Background(), f.db, a, d, "case"); e == nil {
			t.Fatal(kind)
		}
	}
	code, _ := request(t, f.app, "GET", casesPath+"/case", "", "")
	if code != 401 {
		t.Fatal(code)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core}), 200)
	caseGet(t, f)
	caseResponse(t, f, casesPath+"/case/actions", caseActionBody("resolve", 1), 403)
}

func TestReceivingCaseNewReceiptDoesNotAutoResolveOrRewriteOpeningSnapshot(t *testing.T) {
	f := overviewFixture(t)
	caseResponse(t, f, caseOpenPath, caseOpenBody(), 200)
	in := receivingBody()
	in["delivered_milli"] = 5000
	in["accepted_milli"] = 5000
	receivedResponse(t, f, in, 201)
	out := caseGet(t, f)
	if out.Case.Status != "open" || out.Case.OpeningEvidence.RemainingMilli != 5000 {
		t.Fatal(out)
	}
	current := overviewResult(t, f, overviewPath)
	if current.RemainingMilli != 0 {
		t.Fatal(current)
	}
}
func TestReceivingCaseOperationAndMissingHistoryEvidenceFailClosed(t *testing.T) {
	for _, damage := range []string{`UPDATE purchase_receiving_case_operations SET request_hash='bad'`, `DELETE FROM purchase_receiving_case_events`} {
		t.Run(damage, func(t *testing.T) {
			f := overviewFixture(t)
			caseResponse(t, f, caseOpenPath, caseOpenBody(), 200)
			if _, e := f.db.Exec(damage); e != nil {
				t.Fatal(e)
			}
			agendaCall(t, f, "GET", casesPath+"/case", nil, 409)
		})
	}
}
