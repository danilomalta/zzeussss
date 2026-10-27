package localapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/dockagenda"
)

const insightsPath = "/local/v1/logistics/insights"

func insightQuery(start time.Time) string {
	return "?from=" + start.Add(-time.Hour).UTC().Format(time.RFC3339) + "&to=" + start.Add(5*time.Hour).UTC().Format(time.RFC3339)
}
func TestDockInsightHTTPGapsOccupancyAndReadOnly(t *testing.T) {
	f := agendaFixture(t)
	start := agendaStart()
	in := agendaBookInput("a", "order", start)
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	var before int
	if e := f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&before); e != nil {
		t.Fatal(e)
	}
	b := agendaCall(t, f, "GET", insightsPath+"/availability"+insightQuery(start)+"&dock_id=dock&duration_minutes=60", nil, 200)
	var out dockagenda.FreeWindows
	if e := json.Unmarshal(b, &out); e != nil || len(out.Windows) != 2 || out.ConfirmedAvailable || out.OccupiedWindows != 1 {
		t.Fatal(out, e)
	}
	for _, action := range []struct {
		name     string
		revision int64
	}{{"arrive", 1}, {"start", 2}} {
		agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "i-" + action.name, ExpectedRevision: action.revision, Action: action.name, Reason: "Checked"}, 200)
	}
	future := start.Add(48 * time.Hour)
	b = agendaCall(t, f, "GET", insightsPath+"/availability"+insightQuery(future)+"&dock_id=dock&duration_minutes=60", nil, 200)
	if e := json.Unmarshal(b, &out); e != nil || out.ActiveUnloading == nil || out.ConfirmedAvailable || out.OccupiedWindows != 0 {
		t.Fatal(out, e)
	}
	var count int
	if e := f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&count); e != nil || count != before+2 {
		t.Fatal("report wrote outbox", count, before, e)
	}
}
func TestDockInsightHTTPPerformanceSamplesAndCSV(t *testing.T) {
	f := agendaFixture(t)
	start := agendaStart()
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", start), 200)
	for _, a := range []struct {
		name     string
		revision int64
	}{{"arrive", 1}, {"start", 2}, {"complete", 3}} {
		agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "metric-" + a.name, ExpectedRevision: a.revision, Action: a.name, Reason: "Local observation"}, 200)
	}
	path := insightsPath + "/suppliers/performance"
	b := agendaCall(t, f, "GET", path+insightQuery(start), nil, 200)
	var out dockagenda.Performance
	if e := json.Unmarshal(b, &out); e != nil || out.TotalSuppliers != 1 || out.All.Completed != 1 || out.Items[0].SupplierID != "supplier" || out.All.WaitingSamples != 1 || out.All.UnloadingSamples != 1 || out.All.AverageWaitingSeconds == nil {
		t.Fatal(out, e)
	}
	b = agendaCall(t, f, "GET", path+".csv"+insightQuery(start), nil, 200)
	if !strings.Contains(string(b), "average_wait_seconds_exact") {
		t.Fatal(string(b))
	}
	b = agendaCall(t, f, "GET", path+insightQuery(start)+"&offset=1&limit=1", nil, 200)
	if e := json.Unmarshal(b, &out); e != nil || len(out.Items) != 0 || out.All.Completed != 1 {
		t.Fatal(out, e)
	}
}
func TestDockInsightPendingAdvisoryNeverMarksNoShowOrCancels(t *testing.T) {
	f := agendaFixture(t)
	agendaOtherOrder(t, f, "order2")
	past := time.Now().UTC().Truncate(time.Minute).Add(-4 * time.Hour)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("=a", "order", past), 200)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("b", "order2", past.Add(time.Hour)), 200)
	agendaCall(t, f, "POST", appointmentPath+"/b/actions", dockagenda.ActionInput{OperationID: "waiting", ExpectedRevision: 1, Action: "arrive", Reason: "Waiting vehicle"}, 200)
	b := agendaCall(t, f, "GET", insightsPath+"/pending", nil, 200)
	var out dockagenda.PendingPage
	if e := json.Unmarshal(b, &out); e != nil || out.Total != 2 || out.Items[0].Reason != "arrived_waiting" || out.Items[1].Reason != "planned_end_passed_review_required" {
		t.Fatal(out, e)
	}
	agendaCall(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	b = agendaCall(t, f, "GET", insightsPath+"/pending?limit=1", nil, 200)
	if e := json.Unmarshal(b, &out); e != nil || out.Total != 2 || len(out.Items) != 1 || !out.Items[0].OrderCancelled || out.Items[0].Appointment.Status != "reserved" {
		t.Fatal(out, e)
	}
	b = agendaCall(t, f, "GET", insightsPath+"/pending.csv", nil, 200)
	if !strings.Contains(string(b), "'=a") {
		t.Fatal(string(b))
	}
}
func TestDockInsightQueriesPermissionsAndNoContractRenewal(t *testing.T) {
	f := agendaFixture(t)
	start := agendaStart()
	for _, q := range []string{"", "?dock_id=dock&from=bad&to=bad&duration_minutes=60", insightQuery(start) + "&dock_id=dock&duration_minutes=0", insightQuery(start) + "&dock_id=dock&duration_minutes=60&duration_minutes=30", insightQuery(start) + "&dock_id=dock&duration_minutes=60&tenant_id=other"} {
		status, b := request(t, f.app, "GET", insightsPath+"/availability"+q, "", f.token)
		if status != 400 {
			t.Fatal(q, status, string(b))
		}
	}
	status, _ := request(t, f.app, "GET", insightsPath+"/pending", "", "")
	if status != 401 {
		t.Fatal(status)
	}
	agendaCall(t, f, "GET", insightsPath+"/pending?from=2026-01-01T00:00:00Z", nil, 400)
	agendaCall(t, f, "GET", insightsPath+"/suppliers/performance"+insightQuery(start)+"&supplier_id=", nil, 400)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core}), 200)
	agendaCall(t, f, "GET", insightsPath+"/pending", nil, 200)
	agendaCall(t, f, "GET", insightsPath+"/availability"+insightQuery(start)+"&dock_id=dock&duration_minutes=60", nil, 200)
	agendaCall(t, f, "GET", insightsPath+"/suppliers/performance"+insightQuery(start), nil, 200)
	if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); e != nil {
		t.Fatal(e)
	}
	status, _ = request(t, f.app, "GET", insightsPath+"/pending", "", f.token)
	if status != 401 && status != 403 {
		t.Fatal(status)
	}
}
func TestDockInsightSourceLimitNeverReturnsPartialTotals(t *testing.T) {
	f := agendaFixture(t)
	_, e := f.db.Exec(`WITH RECURSIVE seq(n) AS(SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n<5001) INSERT INTO dock_agenda_appointments SELECT ?,?,'closed-'||n,'dock','order','supplier',1767225600+n*300,1767229200+n*300,'completed',4,?,'2026-01-01T00:00:00Z','2026-01-01T00:03:00Z','2026-01-01T00:00:00Z','2026-01-01T00:01:00Z','2026-01-01T00:03:00Z' FROM seq`, f.owner.TenantID, f.owner.StoreID, f.owner.OwnerID)
	if e != nil {
		t.Fatal(e)
	}
	status, b := request(t, f.app, "GET", insightsPath+"/suppliers/performance?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z", "", f.token)
	if status != 422 || !strings.Contains(string(b), "agenda_report_source_limit") {
		t.Fatal(status, string(b))
	}
}
