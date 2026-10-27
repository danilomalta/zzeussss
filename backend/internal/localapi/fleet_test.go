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
	"titansystem-backend/internal/localdb/fleet"
	"titansystem-backend/internal/localdb/identity"
)

const fp = "/local/v1/fleet"

func fleetFixture(t *testing.T) *httpContractFixture {
	t.Helper()
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.Logistics, modules.Accounting}), 200)
	return f
}
func fleetBody(id string, rev int64, action string, data any) fleet.Input {
	b, _ := json.Marshal(data)
	return fleet.Input{OperationID: fmt.Sprintf("%s-%s-%d", id, action, rev), ID: id, ExpectedRevision: rev, Action: action, Reason: "Conferência humana", Data: b}
}
func fleetCall(t *testing.T, f *httpContractFixture, resource string, in fleet.Input, want int) fleet.Result {
	t.Helper()
	path := fp + "/" + resource
	if in.ExpectedRevision > 0 {
		path += "/" + in.ID + "/actions"
	}
	b := agendaCall(t, f, "POST", path, in, want)
	var out fleet.Result
	if want == 200 && json.Unmarshal(b, &out) != nil {
		t.Fatal(string(b))
	}
	return out
}
func fleetAsset(t *testing.T, f *httpContractFixture, id string) fleet.Result {
	t.Helper()
	return fleetCall(t, f, "assets", fleetBody(id, 0, "create", fleet.Asset{Reference: id, Name: "Caminhão", Category: "truck", MeterBasis: "km", MeterMilli: 100000}), 200)
}
func fleetRequest(t *testing.T, f *httpContractFixture, id string) fleet.Result {
	t.Helper()
	return fleetCall(t, f, "requests", fleetBody(id, 0, "create", fleet.OpenRequest{AssetID: "truck", Issue: "Freio", Priority: "emergency", Location: "Pátio", Blocks: true}), 200)
}
func TestFleetLifecycleReceiptsNoticesAndNoCommercialEffects(t *testing.T) {
	f := fleetFixture(t)
	before := caseCommercialState(t, f)
	first := fleetAsset(t, f, "truck")
	fleetRequest(t, f, "repair")
	var av fleet.Availability
	b := agendaCall(t, f, "GET", fp+"/assets/truck/availability", nil, 200)
	if json.Unmarshal(b, &av) != nil || av.AvailableForDispatch || av.BlockingRequests != 1 {
		t.Fatal(av)
	}
	fleetCall(t, f, "trips", fleetBody("trip", 0, "create", fleet.TripOpen{AssetID: "truck", DriverID: f.owner.OwnerID, Reference: "route"}), 409)
	fleetCall(t, f, "requests", fleetBody("repair", 1, "triage", fleet.RequestAction{}), 200)
	fleetCall(t, f, "quotes", fleetBody("quote", 0, "create", fleet.Quote{AssetID: "truck", RequestID: "repair", Note: "Serviço confirmado", LaborCents: 2500}), 200)
	for i, action := range []string{"approve", "start", "wait", "start", "inspect", "release"} {
		data := fleet.RequestAction{}
		if action == "approve" {
			data.QuoteID = "quote"
		}
		out := fleetCall(t, f, "requests", fleetBody("repair", int64(i+2), action, data), 200)
		if out.Record.Revision != int64(i+3) {
			t.Fatal(out)
		}
	}
	b = agendaCall(t, f, "GET", fp+"/assets/truck/availability", nil, 200)
	if json.Unmarshal(b, &av) != nil || !av.AvailableForDispatch {
		t.Fatal(av)
	}
	fleetCall(t, f, "trips", fleetBody("trip", 0, "create", fleet.TripOpen{AssetID: "truck", DriverID: f.owner.OwnerID, Reference: "route"}), 200)
	fleetCall(t, f, "trips", fleetBody("trip", 1, "depart", fleet.TripAction{Loaded: true}), 200)
	end := int64(125000)
	fleetCall(t, f, "trips", fleetBody("trip", 2, "return", fleet.TripAction{EndMeterMilli: &end}), 200)
	var feed fleet.NoticePage
	b = agendaCall(t, f, "GET", fp+"/notices", nil, 200)
	if json.Unmarshal(b, &feed) != nil || len(feed.Items) != 8 || feed.PushDelivered {
		t.Fatal(feed)
	}
	notice := feed.Items[0]
	ack := fleetBody(fmt.Sprint(notice.ID), 0, "ack", struct{}{})
	ack.OperationID = "ack-notice"
	b = agendaCall(t, f, "POST", fmt.Sprintf("%s/notices/%d/ack", fp, notice.ID), ack, 200)
	var r fleet.Result
	if json.Unmarshal(b, &r) != nil || r.Ack == nil {
		t.Fatal(string(b))
	}
	agendaCall(t, f, "POST", fmt.Sprintf("%s/notices/%d/ack", fp, notice.ID), ack, 200)
	agendaCall(t, f, "GET", fp+"/operations/ack-notice", nil, 200)
	b = agendaCall(t, f, "GET", fp+"/requests/repair/history?limit=2&offset=6", nil, 200)
	var h fleet.History
	if json.Unmarshal(b, &h) != nil || h.Total != 8 || len(h.Items) != 2 {
		t.Fatal(h)
	}
	repeat := fleetAsset(t, f, "truck")
	if !repeat.Repeated || !reflect.DeepEqual(repeat.Record, first.Record) {
		t.Fatal(repeat)
	}
	changed := fleetBody("truck", 0, "create", fleet.Asset{Reference: "truck", Name: "changed", Category: "truck", MeterBasis: "km"})
	fleetCall(t, f, "assets", changed, 409)
	if !reflect.DeepEqual(before, caseCommercialState(t, f)) {
		t.Fatal("fleet changed stock, money or purchases")
	}
}
func TestFleetPermissionsWorkerAssignmentAndManagerRelease(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	worker := caseWorker(t, f, "mechanic")
	other := caseWorker(t, f, "other")
	ownerToken := f.token
	f.token = worker
	fleetRequest(t, f, "repair")
	fleetCall(t, f, "requests", fleetBody("repair", 1, "triage", fleet.RequestAction{}), 200)
	fleetCall(t, f, "assets", fleetBody("blocked", 0, "create", fleet.Asset{}), 403)
	agendaCall(t, f, "GET", fp+"/expenses", nil, 403)
	fleetCall(t, f, "requests", fleetBody("repair", 2, "approve", fleet.RequestAction{QuoteID: "quote"}), 403)
	f.token = ownerToken
	fleetCall(t, f, "quotes", fleetBody("quote", 0, "create", fleet.Quote{AssetID: "truck", RequestID: "repair", Note: "Diagnóstico"}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 2, "approve", fleet.RequestAction{QuoteID: "quote"}), 200)
	f.token = other
	fleetCall(t, f, "requests", fleetBody("repair", 3, "start", fleet.RequestAction{}), 403)
	agendaCall(t, f, "GET", fp+"/operations/quote-create-0", nil, 404)
	f.token = worker
	fleetCall(t, f, "requests", fleetBody("repair", 3, "start", fleet.RequestAction{}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 4, "inspect", fleet.RequestAction{}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 5, "release", fleet.RequestAction{}), 403)
	f.token = ownerToken
	fleetCall(t, f, "requests", fleetBody("repair", 5, "release", fleet.RequestAction{}), 200)
}
func TestFleetCancelHoldAndMultipleFailuresBlockDispatch(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	fleetRequest(t, f, "repair")
	fleetCall(t, f, "requests", fleetBody("repair", 1, "triage", fleet.RequestAction{}), 200)
	fleetCall(t, f, "quotes", fleetBody("quote", 0, "create", fleet.Quote{AssetID: "truck", RequestID: "repair", Note: "Diagnóstico"}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 2, "approve", fleet.RequestAction{QuoteID: "quote"}), 200)
	fleetCall(t, f, "requests", fleetBody("repair", 3, "cancel", fleet.RequestAction{}), 200)
	fleetCall(t, f, "assets", fleetBody("truck", 1, "release", struct{}{}), 409)
	fleetCall(t, f, "requests", fleetBody("repair", 4, "clear_hold", fleet.RequestAction{}), 200)
	fleetRequest(t, f, "second")
	fleetCall(t, f, "assets", fleetBody("truck", 1, "release", struct{}{}), 409)
	fleetCall(t, f, "requests", fleetBody("second", 1, "cancel", fleet.RequestAction{}), 200)
	fleetCall(t, f, "assets", fleetBody("truck", 1, "release", struct{}{}), 200)
}
func TestFleetMoneyExactProfileAndSeparateEstimatedLoss(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	for _, v := range []struct {
		resource, id, category string
		value                  int64
	}{{"expenses", "fuel", "fuel", fleet.MaxValue}, {"expenses", "parts", "parts", fleet.MaxValue}, {"income", "freight", "freight", 10000}, {"impacts", "loss", "downtime", 8000}} {
		fleetCall(t, f, v.resource, fleetBody(v.id, 0, "create", fleet.Amount{AssetID: "truck", Category: v.category, AmountCents: v.value, Reference: "Documento interno", IncurredAt: "2026-06-15T00:00:00Z"}), 200)
	}
	fleetCall(t, f, "profiles", fleetBody("profile", 0, "create", fleet.Profile{AssetID: "truck", AcquisitionCents: 120000, ResidualCents: 0, UsefulMonths: 12, InServiceOn: "2026-01-01", Note: "Política gerencial"}), 200)
	b := agendaCall(t, f, "GET", fp+"/assets/truck/costs?from=2026-06-01T00:00:00Z&to=2026-07-01T00:00:00Z", nil, 200)
	var r fleet.CostReport
	if json.Unmarshal(b, &r) != nil || r.ExpenseCentsExact != "18014398509481982" || r.PeriodDepreciationCentsExact != "10000" || r.PartialContributionCentsExact != "-18014398509481982" || r.EstimatedImpactCentsExact != "8000" || r.NetProfit || r.CashConfirmed {
		t.Fatal(string(b))
	}
	fleetCall(t, f, "expenses", fleetBody("parts", 1, "void", struct{}{}), 200)
	b = agendaCall(t, f, "GET", fp+"/assets/truck/costs.csv?from=2026-06-01T00:00:00Z&to=2026-07-01T00:00:00Z", nil, 200)
	if !strings.Contains(string(b), "9007199254740991") {
		t.Fatal(string(b))
	}
	fleetCall(t, f, "expenses", fleetBody("invalid", 0, "create", fleet.Amount{AssetID: "truck", Category: "fuel", AmountCents: fleet.MaxValue + 1, Reference: "Documento", IncurredAt: "2026-06-15T00:00:00Z"}), 400)
}
func TestFleetPreventiveMeterRevisionAndValidation(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	meter := int64(90000)
	fleetCall(t, f, "plans", fleetBody("plan", 0, "create", fleet.Plan{AssetID: "truck", Label: "Revisão", DueMeterMilli: &meter}), 200)
	b := agendaCall(t, f, "GET", fp+"/assets/truck/preventive", nil, 200)
	var p fleet.PreventivePage
	if json.Unmarshal(b, &p) != nil || p.Total != 1 || p.AutomaticRequest {
		t.Fatal(p)
	}
	a := fleet.Asset{Reference: "truck", Name: "Truck", Category: "truck", MeterBasis: "km", MeterMilli: 90000}
	fleetCall(t, f, "assets", fleetBody("truck", 1, "update", a), 409)
	fleetCall(t, f, "assets", fleetBody("truck", 1, "correct_meter", a), 200)
	fleetCall(t, f, "assets", fleetBody("truck", 1, "block", struct{}{}), 409)
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=1&limit=2", "?state=", "?tenant_id=other", "?offset=-1"} {
		t.Run(query, func(t *testing.T) { agendaCall(t, f, "GET", fp+"/assets"+query, nil, 400) })
	}
	for _, body := range []string{`{}`, `{"operation_id":"x","id":"x","expected_revision":0,"action":"create","reason":"x","data":null}`, `{"operation_id":"x","id":"x","expected_revision":0,"action":"create","reason":"x","data":{"name":"a","name":"b"}}`, `{"operation_id":"x","id":"x","expected_revision":0,"action":"create","reason":"x","data":{},"tenant_id":"fake"}`} {
		status, _ := request(t, f.app, "POST", fp+"/assets", body, f.token)
		if status != 400 {
			t.Fatal(status, body)
		}
	}
}
func TestFleetConcurrentSameOperationAndRevision(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	body := fleetBody("repair", 0, "create", fleet.OpenRequest{AssetID: "truck", Issue: "Falha", Priority: "urgent", Location: "Pátio"})
	raw, _ := json.Marshal(body)
	codes := make(chan int, 6)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", fp+"/requests", string(raw), f.token)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatal(code)
		}
	}
	codes = make(chan int, 2)
	for i, action := range []string{"triage", "cancel"} {
		wg.Add(1)
		go func(i int, action string) {
			defer wg.Done()
			in := fleetBody("repair", 1, action, fleet.RequestAction{})
			in.OperationID = fmt.Sprint("competing-", i)
			raw, _ := json.Marshal(in)
			code, _ := request(t, f.app, "POST", fp+"/requests/repair/actions", string(raw), f.token)
			codes <- code
		}(i, action)
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
func TestFleetRollbackWhenStorageIgnoresAnyCriticalWrite(t *testing.T) {
	for _, table := range []string{"fleet_records", "fleet_operations", "fleet_events", "fleet_notices", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := fleetFixture(t)
			fleetAsset(t, f, "truck")
			if _, e := f.db.Exec("CREATE TRIGGER ignore_fleet BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			in := fleetBody("repair", 0, "create", fleet.OpenRequest{AssetID: "truck", Issue: "Falha", Priority: "normal", Location: "Pátio"})
			fleetCall(t, f, "requests", in, 409)
			for _, q := range []string{`SELECT count(*) FROM fleet_records WHERE id='repair'`, `SELECT count(*) FROM fleet_operations WHERE operation_id='repair-create-0'`, `SELECT count(*) FROM fleet_notices`} {
				var count int
				if e := f.db.QueryRow(q).Scan(&count); e != nil || count != 0 {
					t.Fatal(count, e)
				}
			}
		})
	}
}
func TestFleetBackupRestoreAndTamperDetection(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	fleetRequest(t, f, "repair")
	restored := restoredPurchaseDB(t, f)
	defer restored.Close()
	r, e := fleet.Get(context.Background(), restored, agendaActor(f), f.device, "request", "repair")
	if e != nil || r.State != "open" {
		t.Fatal(r, e)
	}
	if _, e = f.db.Exec(`UPDATE fleet_records SET state='released' WHERE kind='request'`); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", fp+"/requests/repair", nil, 409)
}
func TestFleetContractsRevocationAndActorBoundReceipt(t *testing.T) {
	f := httpContractSetup(t)
	in := fleetBody("truck", 0, "create", fleet.Asset{Reference: "truck", Name: "Truck", Category: "truck", MeterBasis: "km"})
	fleetCall(t, f, "assets", in, 403)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.Logistics, modules.Accounting}), 200)
	fleetCall(t, f, "assets", in, 200)
	token := caseWorker(t, f, "worker")
	status, _ := request(t, f.app, "GET", fp+"/operations/truck-create-0", "", token)
	if status != 404 {
		t.Fatal(status)
	}
	if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE device_id=?`, f.device.DeviceID); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", fp+"/assets", nil, 401)
}
func TestFleetWrongTenantStoreAndDeviceAreDenied(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	a := agendaActor(f)
	a.StoreID = "other"
	if _, e := fleet.Get(context.Background(), f.db, a, f.device, "asset", "truck"); e == nil {
		t.Fatal("foreign store")
	}
	a = agendaActor(f)
	a.TenantID = "other"
	if _, e := fleet.Get(context.Background(), f.db, a, f.device, "asset", "truck"); e == nil {
		t.Fatal("foreign tenant")
	}
	a = agendaActor(f)
	device := identity.DeviceContext{TenantID: a.TenantID, StoreID: a.StoreID, DeviceID: "other"}
	if _, e := fleet.Get(context.Background(), f.db, a, device, "asset", "truck"); e == nil {
		t.Fatal("foreign device")
	}
}
func TestFleetReportWindowIsExplicitAndBounded(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	for _, q := range []string{"", "?from=2026-01-01T00:00:00Z&to=2026-01-01T00:00:00Z", "?from=2020-01-01T00:00:00Z&to=2026-01-01T00:00:00Z", "?from=2026-01-01T00:00:00.1Z&to=2026-02-01T00:00:00Z"} {
		agendaCall(t, f, "GET", fp+"/assets/truck/costs"+q, nil, 400)
	}
}

func TestFleetTripRechecksNewFailureAndUsesLastReturnedMeter(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	trip := fleet.TripOpen{AssetID: "truck", DriverID: f.owner.OwnerID, Reference: "Carga conferida"}
	fleetCall(t, f, "trips", fleetBody("t1", 0, "create", trip), 200)
	fleetCall(t, f, "trips", fleetBody("t2", 0, "create", trip), 409)
	fleetRequest(t, f, "repair")
	fleetCall(t, f, "trips", fleetBody("t1", 1, "depart", fleet.TripAction{Loaded: true}), 409)
	fleetCall(t, f, "requests", fleetBody("repair", 1, "cancel", fleet.RequestAction{}), 200)
	fleetCall(t, f, "trips", fleetBody("t1", 1, "depart", fleet.TripAction{Loaded: true}), 200)
	end := int64(120000)
	fleetCall(t, f, "trips", fleetBody("t1", 2, "return", fleet.TripAction{EndMeterMilli: &end}), 200)
	fleetCall(t, f, "trips", fleetBody("t2", 0, "create", trip), 200)
	r := fleetCall(t, f, "trips", fleetBody("t2", 1, "depart", fleet.TripAction{Loaded: true}), 200)
	var v fleet.Trip
	if json.Unmarshal(r.Record.Data, &v) != nil || v.StartMeterMilli != end {
		t.Fatal(v)
	}
	fleetCall(t, f, "trips", fleetBody("t2", 2, "return", fleet.TripAction{EndMeterMilli: &end}), 200)
	due := int64(115000)
	fleetCall(t, f, "plans", fleetBody("plan", 0, "create", fleet.Plan{AssetID: "truck", Label: "Revisão", DueMeterMilli: &due}), 200)
	b := agendaCall(t, f, "GET", fp+"/assets/truck/preventive", nil, 200)
	var p fleet.PreventivePage
	if json.Unmarshal(b, &p) != nil || p.Total != 1 {
		t.Fatal(string(b))
	}
}
func TestFleetProviderProfileIdentityAndScopedUniqueness(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	other := fleet.Asset{Reference: "TRUCK", Name: "Duplicado", Category: "car", MeterBasis: "none"}
	fleetCall(t, f, "assets", fleetBody("other", 0, "create", other), 409)
	provider := fleet.Provider{Name: "Oficina", Contact: "Telefone", Services: "Mecânica"}
	fleetCall(t, f, "providers", fleetBody("provider", 0, "create", provider), 200)
	fleetCall(t, f, "providers", fleetBody("provider", 1, "disable", struct{}{}), 200)
	fleetRequest(t, f, "repair")
	fleetCall(t, f, "requests", fleetBody("repair", 1, "triage", fleet.RequestAction{}), 200)
	q := fleet.Quote{AssetID: "truck", RequestID: "repair", ProviderID: "provider", Note: "Orçamento"}
	fleetCall(t, f, "quotes", fleetBody("quote", 0, "create", q), 409)
	fleetCall(t, f, "providers", fleetBody("provider", 2, "enable", struct{}{}), 200)
	fleetCall(t, f, "quotes", fleetBody("quote", 0, "create", q), 200)
	p := fleet.Profile{AssetID: "truck", AcquisitionCents: 100, UsefulMonths: 12, InServiceOn: "2026-01-01", Note: "Linear"}
	fleetCall(t, f, "profiles", fleetBody("p1", 0, "create", p), 200)
	fleetCall(t, f, "profiles", fleetBody("p2", 0, "create", p), 409)
}
func TestFleetSourceLimitsFailWithoutTruncation(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	_, e := f.db.Exec(`WITH RECURSIVE n(x) AS(VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<5001) INSERT INTO fleet_records(tenant_id,store_id,kind,id,asset_id,state,revision,data_json,created_by,created_at,updated_at) SELECT ?,?,'expense','seed-'||x,'truck','recorded',1,'{}',?,'now','now' FROM n`, f.owner.TenantID, f.owner.StoreID, f.owner.OwnerID)
	if e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", fp+"/assets/truck/costs?from=2026-01-01T00:00:00Z&to=2026-02-01T00:00:00Z", nil, 422)
}
func TestFleetCorruptRequestCannotFreeAssetAndAcknowledgmentIsOnce(t *testing.T) {
	f := fleetFixture(t)
	fleetAsset(t, f, "truck")
	fleetRequest(t, f, "repair")
	if _, e := f.db.Exec(`UPDATE fleet_records SET state='cancelled' WHERE kind='request'`); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "GET", fp+"/assets/truck/availability", nil, 409)
	fleetCall(t, f, "trips", fleetBody("trip", 0, "create", fleet.TripOpen{AssetID: "truck", DriverID: f.owner.OwnerID, Reference: "Rota"}), 409)
	b := agendaCall(t, f, "GET", fp+"/notices", nil, 200)
	var p fleet.NoticePage
	if json.Unmarshal(b, &p) != nil || len(p.Items) != 1 {
		t.Fatal(string(b))
	}
	ack := fleetBody(fmt.Sprint(p.Items[0].ID), 0, "ack", struct{}{})
	path := fmt.Sprintf("%s/notices/%d/ack", fp, p.Items[0].ID)
	agendaCall(t, f, "POST", path, ack, 200)
	ack.OperationID = "another-ack"
	agendaCall(t, f, "POST", path, ack, 409)
	agendaCall(t, f, "GET", fp+"/operations/another-ack", nil, 404)
}
