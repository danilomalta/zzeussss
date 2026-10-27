package localapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/purchases"
)

const overviewPath = "/local/v1/purchase-orders/order/receiving-overview"

func overviewFixture(t *testing.T) *httpContractFixture {
	f := agendaFixture(t)
	agendaCall(t, f, "POST", receivingAuthPath, receivingAuthBody(), 200)
	if _, e := f.db.Exec(`INSERT INTO stock_locations VALUES(?,?,'receiving-room','receiving','Conferencia')`, f.owner.TenantID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	return f
}
func overviewResult(t *testing.T, f *httpContractFixture, path string) purchases.ReceivingOverview {
	t.Helper()
	b := agendaCall(t, f, "GET", path, nil, 200)
	var out purchases.ReceivingOverview
	if e := json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestReceivingOverviewHTTPLinksCompletedDockPartialReceiptAndRejection(t *testing.T) {
	f := overviewFixture(t)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("=appointment", "order", agendaStart()), 200)
	for _, a := range []struct {
		name string
		rev  int64
	}{{"arrive", 1}, {"start", 2}, {"complete", 3}} {
		agendaCall(t, f, "POST", appointmentPath+"/=appointment/actions", dockagenda.ActionInput{OperationID: "overview-" + a.name, ExpectedRevision: a.rev, Action: a.name, Reason: "Local observation"}, 200)
	}
	receivedResponse(t, f, receivingBody(), 201)
	agendaCall(t, f, "POST", rejectionPath, rejectionBody(), 201)
	before := traceReadState(t, f)
	out := overviewResult(t, f, overviewPath+"?offset=1&limit=1")
	if out.EffectiveAcceptedMilli != 2000 || out.RemainingMilli != 3000 || out.PartialRejectedMilli != 500 || out.FullyRejectedMilliExact != "5000" || out.Agenda.Completed != 1 || out.TotalAppointments != 1 || len(out.Appointments) != 0 || out.SourceRecords != 3 || out.SupplierConfirmation != "not_proven_by_local_records" || out.PaymentEvaluation != "not_evaluated" || !slices.Contains(out.ReviewFlags, "completed_dock_with_receiving_pending_review") {
		t.Fatal(out)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("read changed business state")
	}
	b := agendaCall(t, f, "GET", overviewPath+".csv", nil, 200)
	rows, e := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if e != nil || len(rows) != 3 || rows[1][0] != "summary" || rows[2][15] != "'=appointment" {
		t.Fatal(rows, e)
	}
	b = agendaCall(t, f, "GET", overviewPath+".csv?offset=1", nil, 200)
	rows, e = csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if e != nil || len(rows) != 2 {
		t.Fatal(rows, e)
	}
}
func TestReceivingOverviewHTTPVoidPreservesOriginalAndReopensQuantityOnly(t *testing.T) {
	f := overviewFixture(t)
	r := receivedResponse(t, f, receivingBody(), 201)
	agendaCall(t, f, "POST", receiptVoidPath(r.Receipt.ID), receiptVoidBody(), 200)
	out := overviewResult(t, f, overviewPath)
	if out.EffectiveAcceptedMilli != 0 || out.RemainingMilli != 5000 || out.VoidedAcceptedMilliExact != "2000" || out.VoidedReceiptCount != 1 || out.ReceiptCount != 1 || out.SourceRecords != 2 || !slices.Contains(out.ReviewFlags, "receipt_void_recorded_not_refund") {
		t.Fatal(out)
	}
	materialCount(t, f, "purchase_receipts", 1)
}
func TestReceivingOverviewHTTPMissingAuthorizationCancelledOrderAndAgenda(t *testing.T) {
	f := agendaFixture(t)
	out := overviewResult(t, f, overviewPath)
	if out.Authorization != nil || !slices.Contains(out.ReviewFlags, "local_receiving_authorization_missing") {
		t.Fatal(out)
	}
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", agendaStart()), 200)
	agendaCall(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	out = overviewResult(t, f, overviewPath)
	if out.CommercialStatus != "cancelled" || out.Agenda.Reserved != 1 || !slices.Contains(out.ReviewFlags, "cancelled_order_with_active_agenda_review") {
		t.Fatal(out)
	}
	var status string
	if e := f.db.QueryRow(`SELECT status FROM dock_agenda_appointments WHERE id='a'`).Scan(&status); e != nil || status != "reserved" {
		t.Fatal(status, e)
	}
}
func TestReceivingOverviewHTTPRejectsBrokenEvidenceOutsideReturnedPage(t *testing.T) {
	for _, damage := range []string{`DELETE FROM purchase_receiving_events WHERE kind='received'`, `UPDATE stock_movements SET reason='tampered'`, `UPDATE purchase_delivery_rejections SET reason='tampered'`, `UPDATE dock_agenda_appointments SET supplier_id='wrong'`} {
		t.Run(damage, func(t *testing.T) {
			f := overviewFixture(t)
			receivedResponse(t, f, receivingBody(), 201)
			agendaCall(t, f, "POST", rejectionPath, rejectionBody(), 201)
			agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", agendaStart()), 200)
			if strings.Contains(damage, "supplier_id") {
				agendaCall(t, f, "POST", "/local/v1/purchase-suppliers", map[string]any{"operation_id": "other-supplier", "id": "wrong", "name": "Another supplier", "status": "active"}, 201)
			}
			if _, e := f.db.Exec(damage); e != nil {
				t.Fatal(e)
			}
			agendaCall(t, f, "GET", overviewPath+"?offset=100", nil, 409)
		})
	}
}
func TestReceivingOverviewHTTPStrictQueriesAuthAndNoLicenseRenewal(t *testing.T) {
	f := overviewFixture(t)
	for _, q := range []string{"?tenant_id=other", "?limit=1&limit=2", "?offset=-1", "?limit=0", "?offset=1000001"} {
		agendaCall(t, f, "GET", overviewPath+q, nil, 400)
	}
	agendaCall(t, f, "GET", overviewPath, map[string]any{"body": true}, 400)
	agendaCall(t, f, "GET", strings.Replace(overviewPath, "/order/", "/absent/", 1), nil, 404)
	code, _ := request(t, f.app, "GET", overviewPath, "", "")
	if code != 401 {
		t.Fatal(code)
	}
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
		if _, e := purchases.Overview(context.Background(), f.db, a, d, purchases.OverviewInput{OrderID: "order", Limit: 50}); e == nil {
			t.Fatal(kind)
		}
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core}), 200)
	overviewResult(t, f, overviewPath)
	if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
		t.Fatal(e)
	}
	code, _ = request(t, f.app, "GET", overviewPath, "", f.token)
	if code != 401 && code != 403 {
		t.Fatal(code)
	}
}
func TestReceivingOverviewHTTPSourceCapBeforePagination(t *testing.T) {
	f := overviewFixture(t)
	_, e := f.db.Exec(`WITH RECURSIVE seq(n) AS(SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n<1001) INSERT INTO dock_agenda_appointments SELECT ?,?,'old-'||n,'dock','order','supplier',1767225600+n*300,1767229200+n*300,'cancelled',2,?,'2026-01-01T00:00:00Z','2026-01-01T00:03:00Z',NULL,NULL,NULL FROM seq`, f.owner.TenantID, f.owner.StoreID, f.owner.OwnerID)
	if e != nil {
		t.Fatal(e)
	}
	b := agendaCall(t, f, "GET", overviewPath+"?offset=1000000", nil, 422)
	if !strings.Contains(string(b), "receiving_overview_source_limit") {
		t.Fatal(string(b))
	}
}

func TestReceivingOverviewRestoredBackupPreservesCrossModuleSnapshot(t *testing.T) {
	f := overviewFixture(t)
	receivedResponse(t, f, receivingBody(), 201)
	agendaCall(t, f, "POST", rejectionPath, rejectionBody(), 201)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", agendaStart()), 200)
	before := overviewResult(t, f, overviewPath)
	db := restoredPurchaseDB(t, f)
	defer db.Close()
	after, e := purchases.Overview(context.Background(), db, agendaActor(f), f.device, purchases.OverviewInput{OrderID: "order", Limit: 50})
	before.MeasuredAt = ""
	after.MeasuredAt = ""
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatal(before, after, e)
	}
}
func TestReceivingOverviewLegacyOrderIDWithInteriorSpace(t *testing.T) {
	f := overviewFixture(t)
	agendaOtherOrder(t, f, "order space")
	out, e := purchases.Overview(context.Background(), f.db, agendaActor(f), f.device, purchases.OverviewInput{OrderID: "order space", Limit: 50})
	if e != nil || out.OrderID != "order space" || out.TotalAppointments != 0 {
		t.Fatal(out, e)
	}
}
