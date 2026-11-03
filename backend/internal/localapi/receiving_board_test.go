package localapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/purchases"
)

const boardPath = "/local/v1/management/purchases/receiving-board"

func boardQuery() string {
	return "?from=" + time.Now().UTC().Add(-24*time.Hour).Format("2006-01-02T15:04:05Z") + "&to=" + time.Now().UTC().Add(24*time.Hour).Format("2006-01-02T15:04:05Z")
}
func boardResult(t *testing.T, f *httpContractFixture, path string) purchases.ReceivingBoard {
	t.Helper()
	b := agendaCall(t, f, "GET", path, nil, 200)
	var out purchases.ReceivingBoard
	if e := json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestReceivingBoardHTTPStatesPaginationAndCSV(t *testing.T) {
	f := overviewFixture(t)
	agendaOtherOrder(t, f, "=other")
	receivedResponse(t, f, receivingBody(), 201)
	before := traceReadState(t, f)
	out := boardResult(t, f, boardPath+boardQuery()+"&limit=1&offset=1")
	if out.CohortOrders != 2 || out.MatchedOrders != 2 || len(out.Items) != 1 || len(out.Groups) != 2 || out.SourceEvidence != 1 {
		t.Fatal(out)
	}
	empty := boardResult(t, f, boardPath+boardQuery()+"&offset=100")
	if len(empty.Items) != 0 || !reflect.DeepEqual(empty.Groups, out.Groups) {
		t.Fatal(empty)
	}
	b := agendaCall(t, f, "GET", boardPath+".csv"+boardQuery()+"&state=all", nil, 200)
	rows, e := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if e != nil || len(rows) != 6 || rows[1][0] != "summary" || !strings.Contains(string(b), "'=other") {
		t.Fatal(rows, e)
	}
	if !reflect.DeepEqual(before, traceReadState(t, f)) {
		t.Fatal("read mutated state")
	}
	agendaCall(t, f, "POST", strings.Replace(purchaseCancelPath, "/order/", "/=other/", 1), purchaseCancelBody(), 200)
	out = boardResult(t, f, boardPath+boardQuery()+"&state=cancelled")
	if out.MatchedOrders != 1 || out.Groups[0].PendingMilliExact != "0" || out.Groups[0].RemainingMilliExact != "5000" {
		t.Fatal(out)
	}
	out = boardResult(t, f, boardPath+boardQuery())
	if out.MatchedOrders != 1 || out.Items[0].OrderID != "order" || out.Groups[0].PendingMilliExact != "3000" {
		t.Fatal(out)
	}
	in := receivingBody()
	in["operation_id"] = "finish"
	in["delivery_reference"] = "finish"
	in["accepted_milli"] = 3000
	in["delivered_milli"] = 3000
	receivedResponse(t, f, in, 201)
	out = boardResult(t, f, boardPath+boardQuery()+"&state=received")
	if out.MatchedOrders != 1 || out.Groups[0].ReceivedOrders != 1 {
		t.Fatal(out)
	}
}
func TestReceivingBoardHTTPPeriodFractionsTimezoneAndStableChronologicalOrder(t *testing.T) {
	f := overviewFixture(t)
	agendaOtherOrder(t, f, "second")
	agendaOtherOrder(t, f, "excluded")
	for id, stamp := range map[string]string{"order": "2026-01-02T10:00:00.5Z", "second": "2026-01-02T10:00:00Z", "excluded": "2026-01-02T11:00:00Z"} {
		if _, e := f.db.Exec(`UPDATE purchase_orders SET created_at=? WHERE id=?`, stamp, id); e != nil {
			t.Fatal(e)
		}
	}
	out := boardResult(t, f, boardPath+"?from=2026-01-02T07:00:00-03:00&to=2026-01-02T08:00:00-03:00&state=all")
	if out.CohortOrders != 2 || out.Items[0].OrderID != "second" || out.Items[1].OrderID != "order" || out.From != "2026-01-02T10:00:00Z" {
		t.Fatal(out)
	}
	product := out.Items[0].ProductID
	filtered := boardResult(t, f, boardPath+"?from=2026-01-02T10:00:00Z&to=2026-01-02T11:00:00Z&product_id="+product+"&supplier_id=supplier")
	if filtered.MatchedOrders != 1 || filtered.Items[0].OrderID != "second" {
		t.Fatal(filtered)
	}
}
func TestReceivingBoardHTTPBrokenEvidenceBeforeStateAndPageFilter(t *testing.T) {
	f := overviewFixture(t)
	receivedResponse(t, f, receivingBody(), 201)
	if _, e := f.db.Exec(`UPDATE stock_movements SET reason='tampered'`); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", boardPath+boardQuery()+"&state=cancelled&offset=100", nil, 409)
}
func TestReceivingBoardHTTPStrictQueriesScopeAndLicenseRead(t *testing.T) {
	f := overviewFixture(t)
	for _, q := range []string{"", boardQuery() + "&state=paid", boardQuery() + "&supplier_id=", boardQuery() + "&tenant_id=foreign", boardQuery() + "&limit=1&limit=2", boardQuery() + "&offset=-1"} {
		agendaCall(t, f, "GET", boardPath+q, nil, 400)
	}
	agendaCall(t, f, "GET", boardPath+boardQuery(), map[string]bool{"body": true}, 400)
	code, _ := request(t, f.app, "GET", boardPath+boardQuery(), "", "")
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
		if _, e := purchases.Board(context.Background(), f.db, a, d, purchases.BoardInput{From: "2026-01-01T00:00:00Z", To: "2026-12-31T00:00:00Z", State: "all", Limit: 50}); e == nil {
			t.Fatal(kind)
		}
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core}), 200)
	boardResult(t, f, boardPath+boardQuery())
	if _, e := f.db.Exec(`UPDATE memberships SET role='cashier'`); e != nil {
		t.Fatal(e)
	}
	code, _ = request(t, f.app, "GET", boardPath+boardQuery(), "", f.token)
	if code != 401 && code != 403 {
		t.Fatal(code)
	}
}
func TestReceivingBoardHTTPOrderLimitBeforeValidation(t *testing.T) {
	f := overviewFixture(t)
	for i := 0; i < 200; i++ {
		agendaOtherOrder(t, f, fmt.Sprintf("cap-%03d", i))
	}
	b := agendaCall(t, f, "GET", boardPath+boardQuery()+"&limit=1&state=cancelled", nil, 422)
	if !strings.Contains(string(b), "receiving_board_source_limit") {
		t.Fatal(string(b))
	}
}
func TestReceivingBoardHTTPCombinedEvidenceLimit(t *testing.T) {
	f := overviewFixture(t)
	agendaOtherOrder(t, f, "other")
	agendaOtherOrder(t, f, "third")
	for _, order := range []string{"order", "other", "third"} {
		_, e := f.db.Exec(`WITH RECURSIVE seq(n) AS(SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n<700) INSERT INTO dock_agenda_appointments SELECT ?,?,?||n,'dock',?,'supplier',1767225600+n*300,1767229200+n*300,'cancelled',2,?,'2026-01-01T00:00:00Z','2026-01-01T00:03:00Z',NULL,NULL,NULL FROM seq`, f.owner.TenantID, f.owner.StoreID, order, order, f.owner.OwnerID)
		if e != nil {
			t.Fatal(e)
		}
	}
	agendaCall(t, f, "GET", boardPath+boardQuery()+"&offset=1000000", nil, 422)
}

func TestReceivingBoardBackupSnapshotAndAgendaCorruption(t *testing.T) {
	f := overviewFixture(t)
	receivedResponse(t, f, receivingBody(), 201)
	agendaOtherOrder(t, f, "other")
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", agendaStart()), 200)
	before := boardResult(t, f, boardPath+boardQuery()+"&state=all")
	db := restoredPurchaseDB(t, f)
	defer db.Close()
	after, e := purchases.Board(context.Background(), db, agendaActor(f), f.device, purchases.BoardInput{From: before.From, To: before.To, State: "all", Limit: 50})
	before.MeasuredAt = ""
	after.MeasuredAt = ""
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatal(before, after, e)
	}
	if _, e := f.db.Exec(`UPDATE dock_agenda_appointments SET status='arrived'`); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", boardPath+boardQuery(), nil, 409)
}
func TestReceivingBoardAuthorizedOtherStoreSeesNoLocalOrders(t *testing.T) {
	f := overviewFixture(t)
	if _, e := f.db.Exec(`INSERT INTO stores VALUES(?,'board-other','Other')`, f.owner.TenantID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`INSERT INTO devices VALUES(?,'board-other','board-device','Other')`, f.owner.TenantID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by) VALUES(?,'board-other','board-device',?,?,9999999999,'approved',?)`, f.owner.TenantID, make([]byte, 32), make([]byte, 32), f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	a := agendaActor(f)
	a.StoreID = "board-other"
	d := f.device
	d.StoreID = "board-other"
	d.DeviceID = "board-device"
	out, e := purchases.Board(context.Background(), f.db, a, d, purchases.BoardInput{From: "2026-01-01T00:00:00Z", To: "2026-12-31T00:00:00Z", State: "all", Limit: 50})
	if e != nil || out.CohortOrders != 0 || out.MatchedOrders != 0 || len(out.Items) != 0 || len(out.Groups) != 0 {
		t.Fatal(out, e)
	}
}
