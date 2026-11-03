package localapi

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/localdb/replenishment"
)

const dockPath = "/local/v1/logistics/docks"
const appointmentPath = "/local/v1/logistics/appointments"

func agendaFixture(t *testing.T) *httpContractFixture {
	t.Helper()
	f, _ := purchaseTraceFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.Logistics}), 200)
	agendaCall(t, f, "POST", dockPath, dockagenda.DockInput{OperationID: "dock-op", DockID: "dock", Name: "Receiving dock", Enabled: true}, 200)
	return f
}
func agendaCall(t *testing.T, f *httpContractFixture, method, path string, in any, want int) []byte {
	t.Helper()
	body := ""
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			t.Fatal(e)
		}
		body = string(b)
	}
	status, b := request(t, f.app, method, path, body, f.token)
	if status != want {
		t.Fatalf("%s %s: %d want%d %s", method, path, status, want, b)
	}
	return b
}
func agendaBookInput(id, order string, from time.Time) dockagenda.BookInput {
	return dockagenda.BookInput{OperationID: "book-" + id, ID: id, DockID: "dock", OrderID: order, StartsAt: from.UTC().Format(time.RFC3339), EndsAt: from.Add(time.Hour).UTC().Format(time.RFC3339), Reason: "Local window approved by receiver"}
}
func agendaStart() time.Time { return time.Now().UTC().Truncate(time.Minute).Add(24 * time.Hour) }
func agendaActor(f *httpContractFixture) identity.Scope {
	return identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
}
func agendaOtherOrder(t *testing.T, f *httpContractFixture, id string) {
	t.Helper()
	ctx := context.Background()
	a := agendaActor(f)
	status, b := request(t, f.app, "POST", "/local/v1/products", `{"sku":"`+id+`","name":"Second product","price_cents":100,"cost_cents":50}`, f.token)
	if status != 201 {
		t.Fatal(status, string(b))
	}
	var p struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	if _, e := replenishment.SetPolicy(ctx, f.db, a, f.device, replenishment.PolicyInput{OperationID: id + "-policy", ProductID: p.ID, MinimumMilli: 1000, TargetMilli: 5000}); e != nil {
		t.Fatal(e)
	}
	s, e := replenishment.Suggest(ctx, f.db, a, f.device, replenishment.SuggestInput{OperationID: id + "-suggest", ProductID: p.ID})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = replenishment.Review(ctx, f.db, a, f.device, replenishment.ReviewInput{OperationID: id + "-approve", SuggestionID: s.SuggestionID, Decision: "approved", Reason: "Restock"}); e != nil {
		t.Fatal(e)
	}
	agendaCall(t, f, "POST", "/local/v1/purchase-orders", purchases.Input{OperationID: id + "-purchase", OrderID: id, SupplierID: "supplier", SuggestionID: s.SuggestionID}, 201)
}
func TestDockAgendaHTTPLifecycleReceiptsHistoryAndNoStock(t *testing.T) {
	f := agendaFixture(t)
	in := agendaBookInput("a", "order", agendaStart())
	before := map[string]int{}
	for _, table := range []string{"stock_movements", "stock_operations", "purchase_receipts", "cash_movements", "production_orders"} {
		var n int
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil {
			t.Fatal(e)
		}
		before[table] = n
	}
	first := agendaCall(t, f, "POST", appointmentPath, in, 200)
	var r dockagenda.Result
	if e := json.Unmarshal(first, &r); e != nil || r.Appointment == nil || r.Revision != 1 || r.Appointment.SupplierConfirmed || r.Appointment.StockReceived {
		t.Fatal(r, e)
	}
	for _, step := range []struct {
		action   string
		revision int64
	}{{"arrive", 1}, {"start", 2}, {"complete", 3}} {
		b := agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "act-" + step.action, ExpectedRevision: step.revision, Action: step.action, Reason: "Checked by receiving staff"}, 200)
		if e := json.Unmarshal(b, &r); e != nil || r.Revision != step.revision+1 {
			t.Fatal(r, e)
		}
	}
	b := agendaCall(t, f, "GET", appointmentPath+"/a", nil, 200)
	var detail dockagenda.Detail
	if e := json.Unmarshal(b, &detail); e != nil || detail.Appointment.Status != "completed" || detail.WaitingSeconds == nil || detail.UnloadingSeconds == nil || detail.ArrivalDelaySeconds == nil || *detail.ArrivalDelaySeconds >= 0 {
		t.Fatal(detail, e)
	}
	original := agendaCall(t, f, "GET", "/local/v1/logistics/operations/book-a", nil, 200)
	if string(original) != string(first) {
		t.Fatal("receipt changed with later state", string(original), string(first))
	}
	retry := agendaCall(t, f, "POST", appointmentPath, in, 200)
	if e := json.Unmarshal(retry, &r); e != nil || !r.Repeated || r.Appointment.Status != "reserved" {
		t.Fatal(r, e)
	}
	b = agendaCall(t, f, "GET", appointmentPath+"/a/history", nil, 200)
	var history dockagenda.Page[dockagenda.Event]
	if e := json.Unmarshal(b, &history); e != nil || history.Total != 4 || history.Items[0].Result.Revision != 1 || history.Items[3].Result.Revision != 4 {
		t.Fatal(history, e)
	}
	for table, n := range before {
		var after int
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&after); e != nil || after != n {
			t.Fatal("agenda touched", table, after, n, e)
		}
	}
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "cancel-complete", ExpectedRevision: 4, Action: "cancel", Reason: "Invalid transition"}, 409)
}
func TestDockAgendaOverlapBoundariesRescheduleAndDisable(t *testing.T) {
	f := agendaFixture(t)
	agendaOtherOrder(t, f, "order2")
	start := agendaStart()
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", start), 200)
	overlap := agendaBookInput("b", "order2", start.Add(30*time.Minute))
	agendaCall(t, f, "POST", appointmentPath, overlap, 409)
	adjacent := agendaBookInput("b", "order2", start.Add(time.Hour))
	agendaCall(t, f, "POST", appointmentPath, adjacent, 200)
	agendaCall(t, f, "POST", appointmentPath+"/b/actions", dockagenda.ActionInput{OperationID: "reschedule", ExpectedRevision: 1, Action: "reschedule", Reason: "Try overlap", DockID: "dock", StartsAt: start.Format(time.RFC3339), EndsAt: start.Add(time.Hour).Format(time.RFC3339)}, 409)
	agendaCall(t, f, "POST", dockPath, dockagenda.DockInput{OperationID: "disable-busy", DockID: "dock", Name: "Dock", Enabled: false, ExpectedRevision: 1}, 409)
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "cancel-a", ExpectedRevision: 1, Action: "cancel", Reason: "Cancelled local reservation"}, 200)
	agendaCall(t, f, "POST", appointmentPath+"/b/actions", dockagenda.ActionInput{OperationID: "reschedule2", ExpectedRevision: 1, Action: "reschedule", Reason: "Free slot", DockID: "dock", StartsAt: start.Format(time.RFC3339), EndsAt: start.Add(time.Hour).Format(time.RFC3339)}, 200)
	agendaCall(t, f, "POST", appointmentPath+"/b/actions", dockagenda.ActionInput{OperationID: "cancel-b", ExpectedRevision: 2, Action: "cancel", Reason: "Cancelled"}, 200)
	agendaCall(t, f, "POST", dockPath, dockagenda.DockInput{OperationID: "disable", DockID: "dock", Name: "Dock", Enabled: false, ExpectedRevision: 1}, 200)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("new", "order", start), 409)
}
func TestDockAgendaLateUnloadingBlocksActualDockUse(t *testing.T) {
	f := agendaFixture(t)
	agendaOtherOrder(t, f, "order2")
	start := time.Now().UTC().Truncate(time.Minute).Add(-3 * time.Hour)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", start), 200)
	agendaCall(t, f, "POST", appointmentPath, agendaBookInput("b", "order2", start.Add(time.Hour)), 200)
	for _, id := range []string{"a", "b"} {
		agendaCall(t, f, "POST", appointmentPath+"/"+id+"/actions", dockagenda.ActionInput{OperationID: "arrive-" + id, ExpectedRevision: 1, Action: "arrive", Reason: "Arrived"}, 200)
	}
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "start-a", ExpectedRevision: 2, Action: "start", Reason: "Dock occupied"}, 200)
	agendaCall(t, f, "POST", appointmentPath+"/b/actions", dockagenda.ActionInput{OperationID: "start-b", ExpectedRevision: 2, Action: "start", Reason: "Blocked until release"}, 409)
	b := agendaCall(t, f, "GET", "/local/v1/logistics/occupancy", nil, 200)
	var occupancy dockagenda.Page[dockagenda.Occupancy]
	if e := json.Unmarshal(b, &occupancy); e != nil || len(occupancy.Items) != 1 || occupancy.Items[0].Unloading == nil || occupancy.Items[0].Unloading.ID != "a" || occupancy.Items[0].WaitingCount != 1 {
		t.Fatal(occupancy, e)
	}
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "end-a", ExpectedRevision: 3, Action: "complete", Reason: "Dock released"}, 200)
	agendaCall(t, f, "POST", appointmentPath+"/b/actions", dockagenda.ActionInput{OperationID: "start-b", ExpectedRevision: 2, Action: "start", Reason: "Blocked until release"}, 200)
}
func TestDockAgendaCancelledOrderAndNoShow(t *testing.T) {
	f := agendaFixture(t)
	in := agendaBookInput("a", "order", agendaStart())
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "early-no-show", ExpectedRevision: 1, Action: "no_show", Reason: "Too early"}, 409)
	agendaCall(t, f, "POST", purchaseCancelPath, purchaseCancelBody(), 200)
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "arrive", ExpectedRevision: 1, Action: "arrive", Reason: "Cancelled order"}, 409)
	b := agendaCall(t, f, "GET", appointmentPath+"/a", nil, 200)
	var detail dockagenda.Detail
	if e := json.Unmarshal(b, &detail); e != nil || !detail.OrderCancelled {
		t.Fatal(detail, e)
	}
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "cancel", ExpectedRevision: 1, Action: "cancel", Reason: "Free local slot after order cancellation"}, 200)
	f2 := agendaFixture(t)
	past := agendaBookInput("past", "order", time.Now().UTC().Truncate(time.Minute).Add(-3*time.Hour))
	agendaCall(t, f2, "POST", appointmentPath, past, 200)
	agendaCall(t, f2, "POST", appointmentPath+"/past/actions", dockagenda.ActionInput{OperationID: "missed", ExpectedRevision: 1, Action: "no_show", Reason: "Not arrived"}, 200)
}
func TestDockAgendaStrictInputsAndScopedReads(t *testing.T) {
	f := agendaFixture(t)
	in := agendaBookInput("a", "order", agendaStart())
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	raw, _ := json.Marshal(in)
	for _, body := range []string{`{}`, `null`, strings.Replace(string(raw), `"dock_id":"dock"`, `"dock_id":"dock","dock_id":"other"`, 1), strings.Replace(string(raw), `"starts_at":`, `"tenant_id":"foreign","starts_at":`, 1), strings.Replace(string(raw), `"reason":"Local window approved by receiver"`, `"reason":null`, 1)} {
		status, b := request(t, f.app, "POST", appointmentPath, body, f.token)
		if status != 400 {
			t.Fatal(body, status, string(b))
		}
	}
	for _, q := range []string{"?from=bad&to=bad", "?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z", "?limit=0", "?limit=1&limit=2", "?tenant_id=other"} {
		status, b := request(t, f.app, "GET", appointmentPath+q, "", f.token)
		if status != 400 {
			t.Fatal(q, status, string(b))
		}
	}
	agendaCall(t, f, "GET", appointmentPath+"/missing", nil, 404)
	status, _ := request(t, f.app, "GET", dockPath, "", "")
	if status != 401 {
		t.Fatal(status)
	}
	a := agendaActor(f)
	a.StoreID = "other"
	if _, e := dockagenda.Get(context.Background(), f.db, a, f.device, "a"); !errors.Is(e, identity.ErrDenied) {
		t.Fatal(e)
	}
}
func TestDockAgendaFailedWriteRollsBackAllEffects(t *testing.T) {
	for _, table := range []string{"dock_agenda_appointments", "dock_agenda_receipts", "dock_agenda_events", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := agendaFixture(t)
			var before int
			if e := f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&before); e != nil {
				t.Fatal(e)
			}
			if _, e := f.db.Exec("CREATE TRIGGER agenda_fail BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", agendaStart()), 409)
			for _, q := range []string{`SELECT count(*) FROM dock_agenda_appointments`, `SELECT count(*) FROM dock_agenda_receipts WHERE operation_id='book-a'`, `SELECT count(*) FROM dock_agenda_events WHERE entity_kind='appointment'`} {
				var n int
				if e := f.db.QueryRow(q).Scan(&n); e != nil || n != 0 {
					t.Fatal("partial booking", q, n, e)
				}
			}
			var after int
			if e := f.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&after); e != nil || after != before {
				t.Fatal("partial outbox", after, before, e)
			}
		})
	}
}
func TestDockAgendaReplayIntegrityAndContractHistoricalReceipt(t *testing.T) {
	f := agendaFixture(t)
	in := agendaBookInput("a", "order", agendaStart())
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	changed := in
	changed.Reason = "Different"
	agendaCall(t, f, "POST", appointmentPath, changed, 409)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core}), 200)
	agendaCall(t, f, "POST", appointmentPath, in, 403)
	agendaCall(t, f, "GET", "/local/v1/logistics/operations/book-a", nil, 200)
	agendaCall(t, f, "GET", appointmentPath+"/a/history", nil, 200)
	if _, e := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); e != nil {
		t.Fatal(e)
	}
	status, _ := request(t, f.app, "GET", "/local/v1/logistics/operations/book-a", "", f.token)
	if status != 401 && status != 403 {
		t.Fatal(status)
	}
}
func TestDockAgendaCSVAndEncryptedBackupRestore(t *testing.T) {
	f := agendaFixture(t)
	start := agendaStart()
	in := agendaBookInput("=formula", "order", start)
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	from := start.Add(-time.Hour).Format(time.RFC3339)
	to := start.Add(3 * time.Hour).Format(time.RFC3339)
	b := agendaCall(t, f, "GET", appointmentPath+".csv?from="+from+"&to="+to, nil, 200)
	if !strings.Contains(string(b), "'=formula") {
		t.Fatal(string(b))
	}
	db := restoredPurchaseDB(t, f)
	out, e := dockagenda.Get(context.Background(), db, agendaActor(f), f.device, in.ID)
	if e != nil || out.Appointment.Status != "reserved" {
		t.Fatal(out, e)
	}
	r, e := dockagenda.Receipt(context.Background(), db, agendaActor(f), f.device, in.OperationID)
	if e != nil || r.EntityID != in.ID {
		t.Fatal(r, e)
	}
	history, e := dockagenda.History(context.Background(), db, agendaActor(f), f.device, "appointment", in.ID, 50, 0)
	if e != nil || history.Total != 1 {
		t.Fatal(history, e)
	}
}
func TestDockAgendaConcurrentConnectionsOnlyOneSlotWins(t *testing.T) {
	f := agendaFixture(t)
	agendaOtherOrder(t, f, "order2")
	var seq int
	var name, path string
	if e := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); e != nil {
		t.Fatal(e)
	}
	other, e := localdb.Open(context.Background(), path)
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	a := agendaActor(f)
	ctx := context.Background()
	start := agendaStart()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	// Build a verifier from the real disposable fixture public key below.
	licenses := []*entitlementstore.Store{agendaLicense(t, f, f.db), agendaLicense(t, f, other)}
	for index, db := range []*sql.DB{f.db, other} {
		wg.Add(1)
		go func(index int, db *sql.DB) {
			defer wg.Done()
			order := "order"
			if index == 1 {
				order = "order2"
			}
			_, err := dockagenda.Book(ctx, db, licenses[index], a, f.device, agendaBookInput(fmt.Sprint(index), order, start))
			results <- err
		}(index, db)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("double allocation", success)
	}
	var n int
	if e = f.db.QueryRow(`SELECT count(*) FROM dock_agenda_appointments`).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
}

func agendaLicense(t *testing.T, f *httpContractFixture, db *sql.DB) *entitlementstore.Store {
	t.Helper()
	v, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	s, e := entitlementstore.New(db, v, nil)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestDockAgendaWorkerPermissionsAndActorBoundReceipt(t *testing.T) {
	f := agendaFixture(t)
	for _, q := range []string{`INSERT INTO identities VALUES('dock-worker','Worker','now')`, `INSERT INTO memberships VALUES(?,'dock-worker','stock','active','now')`, `INSERT INTO membership_stores VALUES(?,'dock-worker',?)`} {
		var e error
		if strings.Contains(q, "membership_stores") {
			_, e = f.db.Exec(q, f.owner.TenantID, f.owner.StoreID)
		} else if strings.Contains(q, "memberships VALUES") {
			_, e = f.db.Exec(q, f.owner.TenantID)
		} else {
			_, e = f.db.Exec(q)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if e := localauth.SetPassword(context.Background(), f.db, agendaActor(f), f.device, "dock-worker", "senha-forte-de-teste-1"); e != nil {
		t.Fatal(e)
	}
	token := loginToken(t, f.app, "dock-worker")
	status, b := request(t, f.app, "POST", dockPath, `{"operation_id":"worker-dock","dock_id":"new","name":"New dock","enabled":true,"expected_revision":0}`, token)
	if status != 403 {
		t.Fatal(status, string(b))
	}
	in := agendaBookInput("a", "order", agendaStart())
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	status, b = request(t, f.app, "GET", "/local/v1/logistics/operations/book-a", "", token)
	if status != 404 {
		t.Fatal("other actor receipt", status, string(b))
	}
	status, b = request(t, f.app, "GET", appointmentPath+"/a", "", token)
	if status != 200 {
		t.Fatal(status, string(b))
	}
}
func TestDockAgendaAuthorizedOtherStoreCannotSeeLocalRecords(t *testing.T) {
	f := agendaFixture(t)
	in := agendaBookInput("a", "order", agendaStart())
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	if _, e := f.db.Exec(`INSERT INTO stores VALUES(?,'other-store','Other')`, f.owner.TenantID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`INSERT INTO devices VALUES(?,'other-store','other-device','Other')`, f.owner.TenantID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by) VALUES(?,'other-store','other-device',?,?,9999999999,'approved',?)`, f.owner.TenantID, make([]byte, 32), make([]byte, 32), f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	a := agendaActor(f)
	a.StoreID = "other-store"
	d := identity.DeviceContext{TenantID: a.TenantID, StoreID: a.StoreID, DeviceID: "other-device"}
	if _, e := dockagenda.Get(context.Background(), f.db, a, d, "a"); !errors.Is(e, dockagenda.ErrNotFound) {
		t.Fatal(e)
	}
	page, e := dockagenda.Docks(context.Background(), f.db, a, d, 50, 0)
	if e != nil || page.Total != 0 {
		t.Fatal(page, e)
	}
	if _, e = dockagenda.Receipt(context.Background(), f.db, a, d, "book-a"); !errors.Is(e, dockagenda.ErrNotFound) {
		t.Fatal(e)
	}
}
func TestDockAgendaIgnoredUpdatesDoNotCreateFalseReceipt(t *testing.T) {
	for _, table := range []string{"dock_agenda_resources", "dock_agenda_appointments"} {
		t.Run(table, func(t *testing.T) {
			f := agendaFixture(t)
			agendaCall(t, f, "POST", appointmentPath, agendaBookInput("a", "order", agendaStart()), 200)
			if _, e := f.db.Exec("CREATE TRIGGER ignore_agenda_update BEFORE UPDATE ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			if table == "dock_agenda_resources" {
				agendaCall(t, f, "POST", dockPath, dockagenda.DockInput{OperationID: "ignored", DockID: "dock", Name: "Changed name", Enabled: true, ExpectedRevision: 1}, 409)
			} else {
				agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "ignored", ExpectedRevision: 1, Action: "arrive", Reason: "No false success"}, 409)
			}
			var n int
			if e := f.db.QueryRow(`SELECT count(*) FROM dock_agenda_receipts WHERE operation_id='ignored'`).Scan(&n); e != nil || n != 0 {
				t.Fatal(n, e)
			}
		})
	}
}

func TestDockAgendaOneOrderAcrossDocksAndRevisionConflicts(t *testing.T) {
	f := agendaFixture(t)
	start := agendaStart()
	in := agendaBookInput("a", "order", start)
	agendaCall(t, f, "POST", appointmentPath, in, 200)
	agendaCall(t, f, "POST", dockPath, dockagenda.DockInput{OperationID: "dock2-op", DockID: "dock2", Name: "Second dock", Enabled: true}, 200)
	duplicate := agendaBookInput("b", "order", start.Add(2*time.Hour))
	duplicate.DockID = "dock2"
	agendaCall(t, f, "POST", appointmentPath, duplicate, 409)
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "stale", ExpectedRevision: 2, Action: "arrive", Reason: "Wrong revision"}, 409)
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "wrong-transition", ExpectedRevision: 1, Action: "complete", Reason: "Not unloading"}, 409)
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "arrive", ExpectedRevision: 1, Action: "arrive", Reason: "Arrived"}, 200)
	agendaCall(t, f, "POST", appointmentPath+"/a/actions", dockagenda.ActionInput{OperationID: "reschedule-arrived", ExpectedRevision: 2, Action: "reschedule", Reason: "Cannot silently move waiting vehicle", DockID: "dock2", StartsAt: start.Format(time.RFC3339), EndsAt: start.Add(time.Hour).Format(time.RFC3339)}, 409)
}
