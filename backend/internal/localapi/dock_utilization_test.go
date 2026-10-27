package localapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/identity"
)

const utilizationPath = insightsPath + "/utilization"

func utilizationQuery() string {
	return "?dock_id=dock&from=2026-01-02T10:00:00Z&to=2026-01-02T12:00:00Z"
}

func TestDockUtilizationHTTPDisabledHistoryAndIntegrityFailure(t *testing.T) {
	f := agendaFixture(t)
	agendaCall(t, f, "POST", dockPath, dockagenda.DockInput{OperationID: "disable-util", DockID: "dock", Name: "Dock", Enabled: false, ExpectedRevision: 1}, 200)
	b := agendaCall(t, f, "GET", utilizationPath+utilizationQuery(), nil, 200)
	var out dockagenda.UtilizationReport
	if e := json.Unmarshal(b, &out); e != nil || out.Dock.Enabled || out.PeriodSeconds.Numerator != "7200" || out.UnloadingFraction.Numerator != "0" {
		t.Fatal(out, e)
	}
	// A real corruption response is 409, not an invented zero or an over-100% rate.
	_, e := f.db.Exec(`INSERT INTO dock_agenda_appointments VALUES(?,?,'broken','dock','order','supplier',1767348000,1767351600,'completed',4,?,'2026-01-02T10:00:00Z','2026-01-02T11:00:00Z','2026-01-02T10:00:00Z','2026-01-02T10:00:00Z','2026-01-02T09:00:00Z')`, f.owner.TenantID, f.owner.StoreID, f.owner.OwnerID)
	if e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", utilizationPath+utilizationQuery(), nil, 409)
}

func TestDockUtilizationScopeAndRestrictedRole(t *testing.T) {
	f := agendaFixture(t)
	a := agendaActor(f)
	a.StoreID = "other"
	if _, e := dockagenda.Utilization(context.Background(), f.db, a, f.device, dockagenda.UtilizationInput{DockID: "dock", From: "2026-01-02T10:00:00Z", To: "2026-01-02T12:00:00Z", Limit: 50}); !errors.Is(e, identity.ErrDenied) {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	code, _ := request(t, f.app, "GET", utilizationPath+utilizationQuery(), "", f.token)
	if code != 401 && code != 403 {
		t.Fatal(code)
	}
	if _, e := dockagenda.Utilization(context.Background(), f.db, agendaActor(f), f.device, dockagenda.UtilizationInput{DockID: "dock", From: "2026-01-02T10:00:00Z", To: "2026-01-02T12:00:00Z", Limit: 50}); !errors.Is(e, identity.ErrDenied) {
		t.Fatal(e)
	}
}
func TestDockUtilizationHTTPReadOnlyPaginationAndCSV(t *testing.T) {
	f := agendaFixture(t)
	past := time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("=a", "order", past), 200)
	for _, a := range []struct {
		name     string
		revision int64
	}{{"arrive", 1}, {"start", 2}, {"complete", 3}} {
		agendaCall(t, f, "POST", appointmentPath+"/=a/actions", dockagenda.ActionInput{OperationID: "util-" + a.name, ExpectedRevision: a.revision, Action: a.name, Reason: "Observed"}, 200)
	}
	// Disposable fixture models a past actual discharge outside its planned slot.
	if _, e := f.db.Exec(`UPDATE dock_agenda_appointments SET arrived_at='2026-01-02T09:00:00Z',unloading_at='2026-01-02T09:30:00Z',completed_at='2026-01-02T10:30:00.5Z'`); e != nil {
		t.Fatal(e)
	}
	var before int
	f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&before)
	b := agendaCall(t, f, "GET", utilizationPath+utilizationQuery(), nil, 200)
	var out dockagenda.UtilizationReport
	if e := json.Unmarshal(b, &out); e != nil || out.TotalSegments != 1 || out.PlannedAppointments != 0 || out.UnloadingSeconds != (dockagenda.ExactSeconds{Numerator: "3601", Denominator: "2"}) {
		t.Fatal(out, e)
	}
	b = agendaCall(t, f, "GET", utilizationPath+utilizationQuery()+"&offset=1", nil, 200)
	if e := json.Unmarshal(b, &out); e != nil || len(out.Items) != 0 || out.TotalSegments != 1 || out.UnloadingSeconds.Numerator != "3601" {
		t.Fatal(out, e)
	}
	b = agendaCall(t, f, "GET", utilizationPath+".csv"+utilizationQuery(), nil, 200)
	rows, e := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if e != nil || len(rows) != 4 || rows[1][0] != "summary" || rows[2][0] != "day_utc" || rows[3][2] != "'=a" {
		t.Fatal(rows, e)
	}
	b = agendaCall(t, f, "GET", utilizationPath+".csv"+utilizationQuery()+"&offset=1", nil, 200)
	rows, e = csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if e != nil || len(rows) != 3 {
		t.Fatal(rows, e)
	}
	var after int
	f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&after)
	if after != before {
		t.Fatal("read mutated outbox")
	}
}
func TestDockUtilizationHTTPStrictInputAndCurrentAuthorization(t *testing.T) {
	f := agendaFixture(t)
	for _, q := range []string{"", utilizationQuery() + "&dock_id=dock", utilizationQuery() + "&tenant_id=foreign", strings.Replace(utilizationQuery(), "2026-01-02T12:00:00Z", "2200-01-01T00:00:00Z", 1), utilizationQuery() + "&limit=0", strings.Replace(utilizationQuery(), "dock_id=dock", "dock_id=", 1)} {
		agendaCall(t, f, "GET", utilizationPath+q, nil, 400)
	}
	agendaCall(t, f, "GET", utilizationPath+strings.Replace(utilizationQuery(), "dock_id=dock", "dock_id=foreign", 1), nil, 404)
	code, _ := request(t, f.app, "GET", utilizationPath+utilizationQuery(), "", "")
	if code != 401 {
		t.Fatal(code)
	}
	agendaCall(t, f, "GET", utilizationPath+utilizationQuery(), map[string]string{"body": "not allowed"}, 400)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core}), 200)
	agendaCall(t, f, "GET", utilizationPath+utilizationQuery(), nil, 200)
	if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); e != nil {
		t.Fatal(e)
	}
	code, _ = request(t, f.app, "GET", utilizationPath+utilizationQuery(), "", f.token)
	if code != 401 && code != 403 {
		t.Fatal(code)
	}
}
func TestDockUtilizationHTTPSourceLimitAndNoPartialNumbers(t *testing.T) {
	f := agendaFixture(t)
	_, e := f.db.Exec(`WITH RECURSIVE seq(n) AS(SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n<5001) INSERT INTO dock_agenda_appointments SELECT ?,?,'old-'||n,'dock','order','supplier',1767225600+n*300,1767229200+n*300,'cancelled',2,?,'2026-01-01T00:00:00Z','2026-01-01T00:03:00Z',NULL,NULL,NULL FROM seq`, f.owner.TenantID, f.owner.StoreID, f.owner.OwnerID)
	if e != nil {
		t.Fatal(e)
	}
	b := agendaCall(t, f, "GET", utilizationPath+utilizationQuery(), nil, 422)
	if !strings.Contains(string(b), "agenda_report_source_limit") {
		t.Fatal(string(b))
	}
}
