package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/localdb/fleet"
	"titansystem-backend/internal/localdb/identity"
)

const wp = fp + "/workshop"

func workshopCall(t *testing.T, f *httpContractFixture, resource string, in fleet.Input, want int) fleet.WorkshopResult {
	t.Helper()
	in.OperationID = "workshop-" + in.OperationID
	path := wp + "/" + resource
	if in.ExpectedRevision > 0 {
		path += "/" + in.ID + "/actions"
	}
	b := agendaCall(t, f, "POST", path, in, want)
	var r fleet.WorkshopResult
	if want == 200 && json.Unmarshal(b, &r) != nil {
		t.Fatal(string(b))
	}
	return r
}
func workshopTemplate() fleet.CheckTemplate {
	return fleet.CheckTemplate{Name: "Freios e segurança", Items: []fleet.CheckItem{{ID: "brake", Label: "Freio conferido", Required: true}, {ID: "wash", Label: "Limpeza", Required: false}}, Note: "Liberação depende do gerente"}
}
func workshopReady(t *testing.T, required bool) *httpContractFixture {
	t.Helper()
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	fleetRequest(t, f, "repair")
	fleetCall(t, f, "requests", fleetBody("repair", 1, "triage", fleet.RequestAction{}), 200)
	fleetCall(t, f, "quotes", fleetBody("quote", 0, "create", fleet.Quote{AssetID: "truck", RequestID: "repair", Note: "Serviço"}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 2, "approve", fleet.RequestAction{QuoteID: "quote"}), 200)
	if required {
		workshopCall(t, f, "templates", fleetBody("template", 0, "create", workshopTemplate()), 200)
		workshopCall(t, f, "requirements", fleetBody("repair", 0, "create", fleet.BindingInput{RequestID: "repair", ExpectedRequestRevision: 3, TemplateID: "template"}), 200)
	}
	fleetCall(t, f, "requests", fleetBody("repair", 3, "start", fleet.RequestAction{}), 200)
	return f
}
func workshopAnswers(rev int64, pass bool) fleet.InspectionInput {
	outcome := "pass"
	if !pass {
		outcome = "fail"
	}
	return fleet.InspectionInput{RequestID: "repair", ExpectedRequestRevision: rev, ExpectedBindingRevision: 1, Answers: []fleet.CheckAnswer{{ItemID: "brake", Outcome: outcome, Note: "Conferido no local"}, {ItemID: "wash", Outcome: "not_applicable", Note: "Não exigido neste serviço"}}, Note: "Registro técnico de conferência"}
}
func workshopLabor() fleet.LaborInput {
	return fleet.LaborInput{RequestID: "repair", ExpectedRequestRevision: 4, StartedAt: "2026-01-01T10:00:00Z", EndedAt: "2026-01-01T11:00:00Z", Description: "Conferência e reparo declarados"}
}
func TestWorkshopRequiredChecklistBlocksThenAllowsRelease(t *testing.T) {
	f := workshopReady(t, true)
	before := caseCommercialState(t, f)
	fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 409)
	first := workshopCall(t, f, "inspections", fleetBody("failed", 0, "create", workshopAnswers(4, false)), 200)
	if first.State != "failed" {
		t.Fatal(first)
	}
	fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 409)
	workshopCall(t, f, "inspections", fleetBody("pass", 0, "create", workshopAnswers(4, true)), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 200)
	workshopCall(t, f, "inspections", fleetBody("regression", 0, "create", workshopAnswers(5, false)), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 5, "release", fleet.RequestAction{}), 409)
	workshopCall(t, f, "inspections", fleetBody("correction", 0, "create", workshopAnswers(5, true)), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 5, "release", fleet.RequestAction{}), 200)
	var report fleet.WorkshopSummary
	b := agendaCall(t, f, "GET", fp+"/requests/repair/workshop", nil, 200)
	if json.Unmarshal(b, &report) != nil || !report.ChecklistRequired || !report.InspectionSatisfied || report.LatestInspectionID != "correction" || report.PayrollPosted {
		t.Fatal(string(b))
	}
	repeat := workshopCall(t, f, "inspections", fleetBody("failed", 0, "create", workshopAnswers(4, false)), 200)
	if !repeat.Repeated || repeat.State != "failed" {
		t.Fatal(repeat)
	}
	if !reflect.DeepEqual(before, caseCommercialState(t, f)) {
		t.Fatal("workshop changed stock/cash/purchases")
	}
}
func TestWorkshopLaborReviewIdempotenceAndNoOverlap(t *testing.T) {
	f := workshopReady(t, false)
	in := fleetBody("labor1", 0, "create", workshopLabor())
	first := workshopCall(t, f, "labor", in, 200)
	var labor fleet.Labor
	if json.Unmarshal(first.Data, &labor) != nil || labor.Seconds != 3600 || labor.WorkerID != f.owner.OwnerID {
		t.Fatal(labor)
	}
	workshopCall(t, f, "labor", fleetBody("labor2", 0, "create", workshopLabor()), 409)
	confirmed := workshopCall(t, f, "labor", fleetBody("labor1", 1, "confirm", struct{}{}), 200)
	if confirmed.State != "confirmed" {
		t.Fatal(confirmed)
	}
	repeat := workshopCall(t, f, "labor", in, 200)
	if !repeat.Repeated || repeat.State != "declared" {
		t.Fatal(repeat)
	}
	agendaCall(t, f, "GET", wp+"/operations/workshop-labor1-create-0", nil, 200)
	workshopCall(t, f, "labor", fleetBody("labor1", 2, "void", struct{}{}), 200)
	workshopCall(t, f, "labor", fleetBody("labor2", 0, "create", workshopLabor()), 200)
	workshopCall(t, f, "labor", fleetBody("labor2", 1, "reject", struct{}{}), 200)
	b := agendaCall(t, f, "GET", fp+"/requests/repair/workshop", nil, 200)
	var r fleet.WorkshopSummary
	if json.Unmarshal(b, &r) != nil || r.VoidedSecondsExact != "3600" || r.RejectedSecondsExact != "3600" || r.ConfirmedSecondsExact != "0" || r.DeclaredSecondsExact != "0" {
		t.Fatal(string(b))
	}
	agendaCall(t, f, "GET", fp+"/requests/repair/workshop.csv", nil, 200)
	var h fleet.WorkshopHistory
	b = agendaCall(t, f, "GET", wp+"/labor/labor1/history?limit=2&offset=1", nil, 200)
	if json.Unmarshal(b, &h) != nil || h.Total != 3 || len(h.Items) != 2 {
		t.Fatal(string(b))
	}
}
func TestWorkshopPermissionsAssignmentAndManagerReview(t *testing.T) {
	f := workshopReady(t, true)
	owner := f.token
	worker := caseWorker(t, f, "technician")
	other := caseWorker(t, f, "other-tech")
	fleetCall(t, f, "requests", fleetBody("repair", 4, "assign", fleet.RequestAction{AssignedTo: "technician"}), 200)
	f.token = worker
	workshopCall(t, f, "templates", fleetBody("other", 0, "create", workshopTemplate()), 403)
	v := workshopAnswers(5, true)
	workshopCall(t, f, "inspections", fleetBody("tech-pass", 0, "create", v), 200)
	labor := workshopLabor()
	labor.ExpectedRequestRevision = 5
	workshopCall(t, f, "labor", fleetBody("work", 0, "create", labor), 200)
	workshopCall(t, f, "labor", fleetBody("work", 1, "confirm", struct{}{}), 403)
	f.token = other
	workshopCall(t, f, "inspections", fleetBody("unauthorized", 0, "create", v), 403)
	agendaCall(t, f, "GET", wp+"/operations/workshop-work-create-0", nil, 404)
	f.token = owner
	workshopCall(t, f, "labor", fleetBody("work", 1, "confirm", struct{}{}), 200)
	f.token = worker
	fleetCall(t, f, "requests", fleetBody("repair", 5, "inspect", fleet.RequestAction{}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 6, "release", fleet.RequestAction{}), 403)
	f.token = owner
	fleetCall(t, f, "requests", fleetBody("repair", 6, "release", fleet.RequestAction{}), 200)
}
func TestWorkshopReassignmentAndNewServiceEpochInvalidateInspection(t *testing.T) {
	f := workshopReady(t, true)
	workshopCall(t, f, "inspections", fleetBody("pass", 0, "create", workshopAnswers(4, true)), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 4, "assign", fleet.RequestAction{AssignedTo: f.owner.OwnerID}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 5, "inspect", fleet.RequestAction{}), 409)
	workshopCall(t, f, "inspections", fleetBody("pass-new", 0, "create", workshopAnswers(5, true)), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 5, "wait", fleet.RequestAction{}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 6, "start", fleet.RequestAction{}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 7, "inspect", fleet.RequestAction{}), 409)
	workshopCall(t, f, "inspections", fleetBody("pass-after-resume", 0, "create", workshopAnswers(7, true)), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 7, "inspect", fleet.RequestAction{}), 200)
}
func TestWorkshopRequirementCannotChangeOnceServiceStarted(t *testing.T) {
	f := workshopReady(t, true)
	workshopCall(t, f, "requirements", fleetBody("repair", 1, "replace", fleet.BindingInput{RequestID: "repair", ExpectedRequestRevision: 4, TemplateID: "template"}), 409)
	workshopCall(t, f, "templates", fleetBody("template", 1, "disable", struct{}{}), 200)
	workshopCall(t, f, "inspections", fleetBody("pass", 0, "create", workshopAnswers(4, true)), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 5, "release", fleet.RequestAction{}), 200)
}
func TestWorkshopStrictNestedPayloadsRequiredAnswersAndTimeBounds(t *testing.T) {
	f := workshopReady(t, true)
	for _, raw := range []string{`{"name":"t","items":[{"id":"a","label":"A","required":true,"required":false}],"note":""}`, `{"name":"t","items":[{"id":"a","label":"A"}],"note":""}`, `{"name":"t","items":[{"id":"a","label":"A","required":null}],"note":""}`, `{"name":"t","items":[],"note":""}`} {
		in := fleetBody("invalid", 0, "create", struct{}{})
		in.Data = json.RawMessage(raw)
		workshopCall(t, f, "templates", in, 400)
	}
	for _, mutate := range []func(*fleet.InspectionInput){func(v *fleet.InspectionInput) { v.Answers = v.Answers[:1] }, func(v *fleet.InspectionInput) { v.Answers[1].ItemID = "brake" }, func(v *fleet.InspectionInput) { v.Answers[0].Outcome = "fail"; v.Answers[0].Note = "" }, func(v *fleet.InspectionInput) { v.Answers[0].ItemID = "unknown" }} {
		v := workshopAnswers(4, true)
		mutate(&v)
		workshopCall(t, f, "inspections", fleetBody("invalid", 0, "create", v), 400)
	}
	v := workshopAnswers(4, true)
	v.Answers[0].Outcome = "not_applicable"
	r := workshopCall(t, f, "inspections", fleetBody("required-na", 0, "create", v), 200)
	if r.State != "failed" {
		t.Fatal(r)
	}
	for _, end := range []string{"2026-01-01T10:00:00Z", "2026-01-01T09:00:00Z", "2026-01-02T10:00:01Z", "2030-01-01T11:00:00Z", "2026-01-01T11:00:00.1Z"} {
		v := workshopLabor()
		v.EndedAt = end
		workshopCall(t, f, "labor", fleetBody("invalid-labor", 0, "create", v), 400)
	}
	for _, q := range []string{"?limit=0", "?limit=1&limit=2", "?request_id=", "?tenant_id=fake"} {
		agendaCall(t, f, "GET", wp+"/labor"+q, nil, 400)
	}
}
func TestWorkshopConcurrentReplayAndReviewRevision(t *testing.T) {
	f := workshopReady(t, true)
	in := fleetBody("inspect", 0, "create", workshopAnswers(4, true))
	raw, _ := json.Marshal(in)
	var wg sync.WaitGroup
	ch := make(chan int, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", wp+"/inspections", string(raw), f.token)
			ch <- status
		}()
	}
	wg.Wait()
	close(ch)
	for status := range ch {
		if status != 200 {
			t.Fatal(status)
		}
	}
	workshopCall(t, f, "labor", fleetBody("work", 0, "create", workshopLabor()), 200)
	ch = make(chan int, 2)
	for _, action := range []string{"confirm", "reject"} {
		wg.Add(1)
		go func(action string) {
			defer wg.Done()
			in := fleetBody("work", 1, action, struct{}{})
			raw, _ := json.Marshal(in)
			status, _ := request(t, f.app, "POST", wp+"/labor/work/actions", string(raw), f.token)
			ch <- status
		}(action)
	}
	wg.Wait()
	close(ch)
	counts := map[int]int{}
	for status := range ch {
		counts[status]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
func TestWorkshopAtomicRollbackIgnoredWrites(t *testing.T) {
	for _, table := range []string{"fleet_workshop_records", "fleet_workshop_operations", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := workshopReady(t, true)
			if _, e := f.db.Exec("CREATE TRIGGER ignored_workshop BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			workshopCall(t, f, "inspections", fleetBody("inspect", 0, "create", workshopAnswers(4, true)), 409)
			var n int
			if e := f.db.QueryRow(`SELECT count(*) FROM fleet_workshop_records WHERE kind='inspection'`).Scan(&n); e != nil || n != 0 {
				t.Fatal(n, e)
			}
			agendaCall(t, f, "GET", wp+"/operations/workshop-inspect-create-0", nil, 404)
		})
	}
}
func TestWorkshopTamperBlocksReleaseAndBackupRestoresProof(t *testing.T) {
	f := workshopReady(t, true)
	workshopCall(t, f, "inspections", fleetBody("pass", 0, "create", workshopAnswers(4, true)), 200)
	restored := restoredPurchaseDB(t, f)
	defer restored.Close()
	r, e := fleet.WorkshopGet(context.Background(), restored, agendaActor(f), f.device, "inspection", "pass")
	if e != nil || r.State != "passed" {
		t.Fatal(r, e)
	}
	if _, e = f.db.Exec(`UPDATE fleet_workshop_records SET state='failed' WHERE kind='inspection'`); e != nil {
		t.Fatal(e)
	}
	fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 409)
	agendaCall(t, f, "GET", wp+"/inspections/pass", nil, 409)
}
func TestWorkshopScopeReceiptMismatchAndBoundedSource(t *testing.T) {
	f := workshopReady(t, true)
	in := fleetBody("pass", 0, "create", workshopAnswers(4, true))
	workshopCall(t, f, "inspections", in, 200)
	in.Reason = "Alterado"
	workshopCall(t, f, "inspections", in, 409)
	a := agendaActor(f)
	a.TenantID = "foreign"
	if _, e := fleet.WorkshopGet(context.Background(), f.db, a, f.device, "inspection", "pass"); e == nil {
		t.Fatal("tenant leak")
	}
	a = agendaActor(f)
	if _, e := fleet.WorkshopGet(context.Background(), f.db, a, identity.DeviceContext{TenantID: a.TenantID, StoreID: a.StoreID, DeviceID: "foreign"}, "inspection", "pass"); e == nil {
		t.Fatal("device leak")
	}
	_, e := f.db.Exec(`WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1001) INSERT INTO fleet_workshop_records(tenant_id,store_id,kind,id,request_id,revision,state,body_json) SELECT ?,?,'labor','seed-'||x,'repair',1,'declared','{}' FROM n`, f.owner.TenantID, f.owner.StoreID)
	if e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", fp+"/requests/repair/workshop", nil, 422)
}
func TestWorkshopLegacyNoChecklistStillWorks(t *testing.T) {
	f := workshopReady(t, false)
	fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 5, "release", fleet.RequestAction{}), 200)
	b := agendaCall(t, f, "GET", wp+"/templates", nil, 200)
	if !strings.Contains(string(b), `"total":0`) {
		t.Fatal(string(b))
	}
}

func TestWorkshopDeletedEvidenceCannotSilentlyUnlock(t *testing.T) {
	for _, kind := range []string{"binding", "inspection"} {
		t.Run(kind, func(t *testing.T) {
			f := workshopReady(t, true)
			workshopCall(t, f, "inspections", fleetBody("passed", 0, "create", workshopAnswers(4, true)), 200)
			if _, e := f.db.Exec(`DELETE FROM fleet_workshop_records WHERE kind=?`, kind); e != nil {
				t.Fatal(e)
			}
			fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 409)
			agendaCall(t, f, "GET", fp+"/requests/repair/workshop", nil, 409)
		})
	}
}
func TestWorkshopOperationIDsCannotCrossFleetNamespace(t *testing.T) {
	f := workshopReady(t, false)
	in := fleetBody("template", 0, "create", workshopTemplate())
	in.OperationID = "truck-create-0"
	raw, _ := json.Marshal(in)
	status, _ := request(t, f.app, "POST", wp+"/templates", string(raw), f.token)
	if status != 409 {
		t.Fatal(status)
	}
	workshopCall(t, f, "templates", fleetBody("template", 0, "create", workshopTemplate()), 200)
	body := fleetBody("other-asset", 0, "create", fleet.Asset{Reference: "other", Name: "Outro", Category: "car", MeterBasis: "none"})
	body.OperationID = "workshop-template-create-0"
	fleetCall(t, f, "assets", body, 409)
}
