package stocktaking

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/production"
)

type fixture struct {
	db         *sql.DB
	path       string
	a, manager identity.Scope
	d          identity.DeviceContext
	contracts  *entitlementstore.Store
	private    ed25519.PrivateKey
}

func setup(t *testing.T) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inventory.sqlite")
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{
		`INSERT INTO tenants VALUES('a','Market','now'),('b','Other','now')`,
		`INSERT INTO stores VALUES('a','s','Store'),('a','other','Other store'),('b','s','Foreign')`,
		`INSERT INTO devices VALUES('a','s','d','Station')`,
		`INSERT INTO identities VALUES('counter','Counter','now'),('manager','Manager','now'),('other','Other','now')`,
		`INSERT INTO memberships VALUES('a','counter','stock','active','now'),('a','manager','owner','active','now'),('a','other','stock','active','now')`,
		`INSERT INTO membership_stores VALUES('a','counter','s'),('a','other','s')`,
		`INSERT INTO stock_locations VALUES('a','s','shelf','shelf','Shelf'),('a','other','shelf','shelf','Other'),('b','s','shelf','shelf','Foreign')`,
		`INSERT INTO products(tenant_id,id,sku,name,price_cents,unit) VALUES('a','p','p','Product',200,'unit'),('a','q','q','Fraction',20,'kg'),('b','foreign','f','Foreign',1,'unit')`,
		`INSERT INTO stock_movements VALUES('opening-p','a','s','d','p','shelf',5000,'opening','now'),('opening-q','a','s','d','q','shelf',3500,'opening','now')`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by) VALUES('a','s','d',?,?,9999999999,'approved','manager')`, make([]byte, 32), make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test": public})
	if err != nil {
		t.Fatal(err)
	}
	store, err := entitlementstore.New(db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: db, path: path, a: identity.Scope{TenantID: "a", StoreID: "s", IdentityID: "counter"}, manager: identity.Scope{TenantID: "a", StoreID: "s", IdentityID: "manager"}, d: identity.DeviceContext{TenantID: "a", StoreID: "s", DeviceID: "d"}, contracts: store, private: private}
	f.license(t, 1, true)
	return f
}
func (f *fixture) license(t *testing.T, revision int64, inventory bool) {
	t.Helper()
	now := time.Now().Unix()
	selected := []modules.ID{modules.Core}
	if inventory {
		selected = append(selected, modules.Inventory)
	}
	b, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: "a", Revision: revision, IssuedAt: now - 60, NotBefore: now - 60, ExpiresAt: now + 3600, Modules: selected})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.contracts.Install(context.Background(), f.manager, f.d, entitlements.Envelope{KeyID: "test", Payload: b, Signature: ed25519.Sign(f.private, entitlements.SigningMessage("test", b))})
	if err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) open(t *testing.T, id string, items ...Item) Result {
	t.Helper()
	if len(items) == 0 {
		items = []Item{{ProductID: "p", Unit: "unit"}}
	}
	r, err := Open(context.Background(), f.db, f.contracts, f.a, f.d, OpenInput{OperationID: "open-" + id, ID: id, LocationID: "shelf", Title: "Physical count", Items: items})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func (f *fixture) count(t *testing.T, id string, revision int64, items ...CountItem) Result {
	t.Helper()
	if len(items) == 0 {
		items = []CountItem{{ProductID: "p", CountedMilli: 3000}}
	}
	r, err := Count(context.Background(), f.db, f.contracts, f.a, f.d, id, CountInput{OperationID: "count-" + id, ExpectedRevision: revision, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func (f *fixture) state(t *testing.T, a identity.Scope, id, op, action string, revision int64) Result {
	t.Helper()
	r, err := ChangeState(context.Background(), f.db, f.contracts, a, f.d, id, StateInput{OperationID: op, ExpectedRevision: revision, Action: action, Reason: "Reviewed in person"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func (f *fixture) balance(t *testing.T, product string) int64 {
	t.Helper()
	var n int64
	if err := f.db.QueryRow(`SELECT COALESCE(SUM(quantity_milli),0) FROM stock_movements WHERE tenant_id='a' AND store_id='s' AND product_id=?`, product).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestReviewedInventoryLifecycleAndOriginalReceipts(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.open(t, "i")
	f.count(t, "i", 1)
	if n := f.balance(t, "p"); n != 5000 {
		t.Fatal(n)
	}
	f.state(t, f.a, "i", "submit", "submit", 2)
	r := f.state(t, f.manager, "i", "approve", "approve", 3)
	if r.Status != "approved" || r.Revision != 4 || r.ChangedLines != 1 || f.balance(t, "p") != 3000 {
		t.Fatal(r)
	}
	replay, err := ChangeState(ctx, f.db, f.contracts, f.manager, f.d, "i", StateInput{OperationID: "approve", ExpectedRevision: 3, Action: "approve", Reason: "Reviewed in person"})
	if err != nil || !replay.Repeated {
		t.Fatal(replay, err)
	}
	old, err := Receipt(ctx, f.db, f.a, f.d, "count-i")
	if err != nil || old.Revision != 2 || old.Status != "open" {
		t.Fatal(old, err)
	}
	h, err := History(ctx, f.db, f.manager, f.d, "i", 2, 2)
	if err != nil || h.Total != 4 || len(h.Items) != 2 {
		t.Fatal(h, err)
	}
	detail, err := Get(ctx, f.db, f.manager, f.d, "i")
	if err != nil || detail.Lines[0].MovementID == nil || *detail.Lines[0].DifferenceMilli != -2000 {
		t.Fatal(detail, err)
	}
	var n int
	f.db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='inventory.reviewed'`).Scan(&n)
	if n != 4 {
		t.Fatal(n)
	}
}
func TestReviewedInventorySameOperationDifferentPayload(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	_, err := Open(context.Background(), f.db, f.contracts, f.a, f.d, OpenInput{OperationID: "open-i", ID: "i", LocationID: "shelf", Title: "Changed", Items: []Item{{"p", "unit"}}})
	if !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
func TestReviewedInventoryCanonicalItemOrderReplay(t *testing.T) {
	f := setup(t)
	in := OpenInput{OperationID: "open", ID: "i", LocationID: "shelf", Title: "Count", Items: []Item{{"q", "kg"}, {"p", "unit"}}}
	if _, err := Open(context.Background(), f.db, f.contracts, f.a, f.d, in); err != nil {
		t.Fatal(err)
	}
	in.Items[0], in.Items[1] = in.Items[1], in.Items[0]
	r, err := Open(context.Background(), f.db, f.contracts, f.a, f.d, in)
	if err != nil || !r.Repeated {
		t.Fatal(r, err)
	}
}
func TestReviewedInventoryZeroIsCountedButNullIsNot(t *testing.T) {
	f := setup(t)
	f.open(t, "i", Item{"p", "unit"}, Item{"q", "kg"})
	f.count(t, "i", 1, CountItem{"p", 0})
	_, err := ChangeState(context.Background(), f.db, f.contracts, f.a, f.d, "i", StateInput{"submit", 2, "submit", "All counted"})
	if !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	f.countWith(t, "i", "q-count", 2, CountItem{"q", 0})
	f.state(t, f.a, "i", "submit", "submit", 3)
	f.state(t, f.manager, "i", "approve", "approve", 4)
	if f.balance(t, "p") != 0 || f.balance(t, "q") != 0 {
		t.Fatal("zero not applied")
	}
}
func (f *fixture) countWith(t *testing.T, id, op string, revision int64, items ...CountItem) {
	t.Helper()
	if _, err := Count(context.Background(), f.db, f.contracts, f.a, f.d, id, CountInput{op, revision, items}); err != nil {
		t.Fatal(err)
	}
}
func TestReviewedInventoryCannotSelfApproveEvenAsOwner(t *testing.T) {
	f := setup(t)
	f.a = f.manager
	f.open(t, "i")
	f.count(t, "i", 1)
	f.state(t, f.a, "i", "submit", "submit", 2)
	_, err := ChangeState(context.Background(), f.db, f.contracts, f.manager, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
	if !errors.Is(err, identity.ErrDenied) {
		t.Fatal(err)
	}
}
func TestReviewedInventoryStockWorkerCannotApproveAnotherWorker(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	f.count(t, "i", 1)
	f.state(t, f.a, "i", "submit", "submit", 2)
	other := f.a
	other.IdentityID = "other"
	_, err := ChangeState(context.Background(), f.db, f.contracts, other, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
	if !errors.Is(err, identity.ErrDenied) {
		t.Fatal(err)
	}
}
func TestReviewedInventoryChangedStockRequiresRecountIncludingNetZero(t *testing.T) {
	for _, netZero := range []bool{false, true} {
		t.Run(map[bool]string{false: "movement", true: "net-zero"}[netZero], func(t *testing.T) {
			f := setup(t)
			f.open(t, "i")
			f.count(t, "i", 1)
			f.state(t, f.a, "i", "submit", "submit", 2)
			f.db.Exec(`INSERT INTO stock_movements VALUES('after','a','s','d','p','shelf',1000,'sale correction','now')`)
			if netZero {
				f.db.Exec(`INSERT INTO stock_movements VALUES('opposite','a','s','d','p','shelf',-1000,'sale','now')`)
			}
			_, err := ChangeState(context.Background(), f.db, f.contracts, f.manager, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
			if !errors.Is(err, ErrStale) {
				t.Fatal(err)
			}
			f.state(t, f.a, "i", "reopen", "reopen", 3)
			f.countWith(t, "i", "recount", 4, CountItem{"p", 3000})
			f.state(t, f.a, "i", "resubmit", "submit", 5)
			f.state(t, f.manager, "i", "approve", "approve", 6)
			if f.balance(t, "p") != 3000 {
				t.Fatal("recount not applied")
			}
		})
	}
}
func TestReviewedInventoryApprovalIsAtomicAcrossProducts(t *testing.T) {
	f := setup(t)
	f.open(t, "i", Item{"p", "unit"}, Item{"q", "kg"})
	f.count(t, "i", 1, CountItem{"p", 1000}, CountItem{"q", 2500})
	f.state(t, f.a, "i", "submit", "submit", 2)
	f.db.Exec(`INSERT INTO stock_movements VALUES('after-q','a','s','d','q','shelf',1,'new movement','now')`)
	_, err := ChangeState(context.Background(), f.db, f.contracts, f.manager, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
	if !errors.Is(err, ErrStale) || f.balance(t, "p") != 5000 {
		t.Fatal(err, "partial adjustment")
	}
	d, _ := Get(context.Background(), f.db, f.a, f.d, "i")
	if d.Session.Revision != 3 || d.Lines[0].MovementID != nil {
		t.Fatal(d)
	}
}
func TestReviewedInventoryOutboxFailureRollsBackStockAndReceipt(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	f.count(t, "i", 1)
	f.state(t, f.a, "i", "submit", "submit", 2)
	if _, err := f.db.Exec(`CREATE TRIGGER test_outbox_fail BEFORE INSERT ON outbox BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := ChangeState(context.Background(), f.db, f.contracts, f.manager, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
	if err == nil || f.balance(t, "p") != 5000 {
		t.Fatal(err)
	}
	_, err = Receipt(context.Background(), f.db, f.manager, f.d, "approve")
	if !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
func TestReviewedInventoryRevisionConflictAndConcurrentReview(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	f.count(t, "i", 1)
	f.state(t, f.a, "i", "submit", "submit", 2)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, op := range []string{"approve-a", "approve-b"} {
		wg.Add(1)
		go func(op string) {
			defer wg.Done()
			_, err := ChangeState(context.Background(), f.db, f.contracts, f.manager, f.d, "i", StateInput{op, 3, "approve", "Checked"})
			errs <- err
		}(op)
	}
	wg.Wait()
	close(errs)
	ok, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflicts != 1 || f.balance(t, "p") != 3000 {
		t.Fatal(ok, conflicts)
	}
}
func TestReviewedInventoryHistorySurvivesLicenseRemoval(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	f.license(t, 2, false)
	if _, err := Get(context.Background(), f.db, f.a, f.d, "i"); err != nil {
		t.Fatal(err)
	}
	r, err := Receipt(context.Background(), f.db, f.a, f.d, "open-i")
	if err != nil || r.Revision != 1 {
		t.Fatal(err)
	}
	_, err = Count(context.Background(), f.db, f.contracts, f.a, f.d, "i", CountInput{"new", 1, []CountItem{{"p", 0}}})
	if err == nil {
		t.Fatal("unlicensed write")
	}
	r, err = Open(context.Background(), f.db, f.contracts, f.a, f.d, OpenInput{OperationID: "open-i", ID: "i", LocationID: "shelf", Title: "Physical count", Items: []Item{{"p", "unit"}}})
	if err != nil || !r.Repeated {
		t.Fatal(r, err)
	}
}
func TestReviewedInventoryRevocationBlocksReadWriteAndReplay(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id='a' AND device_id='d'`)
	_, err := Get(context.Background(), f.db, f.a, f.d, "i")
	if !errors.Is(err, identity.ErrDenied) {
		t.Fatal(err)
	}
	_, err = Receipt(context.Background(), f.db, f.a, f.d, "open-i")
	if !errors.Is(err, identity.ErrDenied) {
		t.Fatal(err)
	}
}
func TestReviewedInventoryStoreAndTenantIsolation(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	for _, a := range []identity.Scope{{TenantID: "a", StoreID: "other", IdentityID: "manager"}, {TenantID: "b", StoreID: "s", IdentityID: "manager"}} {
		if _, err := Get(context.Background(), f.db, a, f.d, "i"); err == nil {
			t.Fatal("cross context")
		}
	}
	a := f.a
	a.IdentityID = "other"
	if _, err := Receipt(context.Background(), f.db, a, f.d, "open-i"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
func TestReviewedInventoryReopenAndTerminalStates(t *testing.T) {
	for _, action := range []string{"reject", "cancel"} {
		t.Run(action, func(t *testing.T) {
			f := setup(t)
			f.open(t, "i")
			f.count(t, "i", 1)
			f.state(t, f.a, "i", "submit", "submit", 2)
			f.state(t, f.manager, "i", "terminal", action, 3)
			_, err := ChangeState(context.Background(), f.db, f.contracts, f.a, f.d, "i", StateInput{"reopen", 4, "reopen", "Checked"})
			if !errors.Is(err, ErrConflict) || f.balance(t, "p") != 5000 {
				t.Fatal(err)
			}
		})
	}
}
func TestReviewedInventoryUnitChangeInvalidatesCountAndApproval(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	f.count(t, "i", 1)
	f.state(t, f.a, "i", "submit", "submit", 2)
	f.db.Exec(`UPDATE products SET unit='kg' WHERE tenant_id='a' AND id='p'`)
	_, err := ChangeState(context.Background(), f.db, f.contracts, f.manager, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
	if !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
func TestReviewedInventoryInvalidOpenAndCounts(t *testing.T) {
	f := setup(t)
	for _, tc := range []struct {
		name  string
		items []Item
	}{{"duplicate", []Item{{"p", "unit"}, {"p", "unit"}}}, {"foreign", []Item{{"foreign", "unit"}}}, {"unit", []Item{{"p", "kg"}}}, {"unknown", []Item{{"missing", "unit"}}}, {"empty", nil}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Open(context.Background(), f.db, f.contracts, f.a, f.d, OpenInput{"invalid-" + tc.name, tc.name, "shelf", "Count", tc.items})
			if err == nil {
				t.Fatal("accepted")
			}
		})
	}
	f.open(t, "i")
	for _, n := range []int64{-1, 1, MaxQuantity + 1} {
		_, err := Count(context.Background(), f.db, f.contracts, f.a, f.d, "i", CountInput{"invalid", 1, []CountItem{{"p", n}}})
		if !errors.Is(err, ErrInvalid) {
			t.Fatal(n, err)
		}
	}
}
func TestReviewedInventoryEmptyPagesStillHaveGlobalTotal(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	p, err := List(context.Background(), f.db, f.a, f.d, "open", 1, 100)
	if err != nil || p.Total != 1 || len(p.Items) != 0 {
		t.Fatal(p, err)
	}
	h, err := History(context.Background(), f.db, f.a, f.d, "i", 1, 100)
	if err != nil || h.Total != 1 || len(h.Items) != 0 {
		t.Fatal(h, err)
	}
}
func TestReviewedInventoryReopenDatabaseAndEncryptedRestore(t *testing.T) {
	f := setup(t)
	// Backups intentionally accept exactly one tenant. Other tests retain the
	// second tenant to exercise isolation; this fixture removes only that data.
	for _, q := range []string{`DELETE FROM products WHERE tenant_id='b'`, `DELETE FROM stock_locations WHERE tenant_id='b'`, `DELETE FROM stores WHERE tenant_id='b'`, `DELETE FROM tenants WHERE id='b'`} {
		if _, err := f.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	f.open(t, "i")
	f.count(t, "i", 1)
	key := make([]byte, 32)
	rand.Read(key)
	archive := filepath.Join(t.TempDir(), "inventory.tytbak")
	if err := backup.Create(context.Background(), f.db, f.d, key, archive); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	if err := backup.Restore(context.Background(), archive, restored, f.d, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err := Get(context.Background(), db, f.a, f.d, "i")
	if err != nil || r.Session.Revision != 2 || *r.Lines[0].CountedMilli != 3000 {
		t.Fatal(r, err)
	}
	if _, err = Receipt(context.Background(), db, f.a, f.d, "count-i"); err != nil {
		t.Fatal(err)
	}
}
func TestReviewedInventoryExportNeutralizesFormulaAndPreservesMissing(t *testing.T) {
	n := int64(-2000)
	var b bytes.Buffer
	if err := Export(&b, Detail{Session: Session{ID: "=SUM(1)", Status: "open", Revision: 1, LocationID: "@location"}, Lines: []Line{{ProductID: "+product", Unit: "unit", DifferenceMilli: &n}}}); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if err != nil || rows[1][0] != "'=SUM(1)" || rows[1][4] != "'+product" || rows[1][6] != "" || rows[1][8] != "-2000" {
		t.Fatal(rows, err)
	}
}

func TestReviewedInventoryCannotConsumeProductionReservations(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	now := time.Now().Unix()
	body, _ := json.Marshal(entitlements.Claims{Version: 1, TenantID: "a", Revision: 2, IssuedAt: now - 60, NotBefore: now - 60, ExpiresAt: now + 3600, Modules: []modules.ID{modules.Core, modules.Inventory, modules.Production}})
	if _, err := f.contracts.Install(ctx, f.manager, f.d, entitlements.Envelope{KeyID: "test", Payload: body, Signature: ed25519.Sign(f.private, entitlements.SigningMessage("test", body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := production.Publish(ctx, f.db, f.contracts, f.manager, f.d, production.PublishInput{OperationID: "publish", RecipeID: "recipe", VersionID: "version", Name: "Recipe", OutputProductID: "p", OutputUnit: "unit", YieldMilli: 1000, Ingredients: []production.Ingredient{{ProductID: "q", Unit: "kg", QuantityMilli: 1000}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := production.CreateOrder(ctx, f.db, f.contracts, f.manager, f.d, production.OrderInput{OperationID: "plan", OrderID: "order", VersionID: "version", LocationID: "shelf", ResponsibleID: "manager", PlannedBatches: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := production.ChangeOrderState(ctx, f.db, f.contracts, f.manager, f.d, production.OrderStateInput{OperationID: "approve-order", OrderID: "order", ExpectedRevision: 1, Status: "approved", Reason: "Checked"}); err != nil {
		t.Fatal(err)
	}
	if _, err := production.ReserveMaterials(ctx, f.db, f.contracts, f.manager, f.d, production.ReserveInput{OperationID: "reserve", ReservationID: "reservation", OrderID: "order", Reason: "For production"}); err != nil {
		t.Fatal(err)
	}
	f.open(t, "i", Item{"q", "kg"})
	f.count(t, "i", 1, CountItem{"q", 2000})
	f.state(t, f.a, "i", "submit", "submit", 2)
	_, err := ChangeState(ctx, f.db, f.contracts, f.manager, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
	if !errors.Is(err, ErrReserved) || f.balance(t, "q") != 3500 {
		t.Fatal(err, "reservation was consumed by count")
	}
}

func TestReviewedInventoryIgnoredWritesCannotProduceSuccess(t *testing.T) {
	for _, table := range []string{"stock_movements", "reviewed_inventory_sessions", "reviewed_inventory_events", "reviewed_inventory_receipts"} {
		t.Run(table, func(t *testing.T) {
			f := setup(t)
			f.open(t, "i")
			f.count(t, "i", 1)
			f.state(t, f.a, "i", "submit", "submit", 2)
			verb := "INSERT"
			if table == "reviewed_inventory_sessions" {
				verb = "UPDATE"
			}
			if _, err := f.db.Exec("CREATE TRIGGER ignore_write BEFORE " + verb + " ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); err != nil {
				t.Fatal(err)
			}
			_, err := ChangeState(context.Background(), f.db, f.contracts, f.manager, f.d, "i", StateInput{"approve", 3, "approve", "Checked"})
			if err == nil || f.balance(t, "p") != 5000 {
				t.Fatal(err, "partial effect")
			}
			detail, err := Get(context.Background(), f.db, f.a, f.d, "i")
			if err != nil || detail.Session.Revision != 3 || detail.Session.Status != "submitted" {
				t.Fatal(detail, err)
			}
		})
	}
}

func TestReviewedInventoryHistoryRetainsOriginalCountsAfterRecount(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	f.count(t, "i", 1)
	f.countWith(t, "i", "recount", 2, CountItem{"p", 4000})
	h, err := History(context.Background(), f.db, f.a, f.d, "i", 100, 0)
	if err != nil || h.Total != 3 {
		t.Fatal(h, err)
	}
	var original CountInput
	if err = json.Unmarshal(h.Items[1].Request, &original); err != nil || original.Items[0].CountedMilli != 3000 {
		t.Fatal(original, err)
	}
	var changed CountInput
	if err = json.Unmarshal(h.Items[2].Request, &changed); err != nil || changed.Items[0].CountedMilli != 4000 {
		t.Fatal(changed, err)
	}
}

func TestReviewedInventoryAuthorizedOtherStoreCannotSeeThisInventory(t *testing.T) {
	f := setup(t)
	f.open(t, "i")
	if _, err := f.db.Exec(`INSERT INTO devices VALUES('a','other','other-device','Other')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by) VALUES('a','other','other-device',?,?,9999999999,'approved','manager')`, make([]byte, 32), make([]byte, 32)); err != nil {
		t.Fatal(err)
	}
	a := f.manager
	a.StoreID = "other"
	d := identity.DeviceContext{TenantID: "a", StoreID: "other", DeviceID: "other-device"}
	if _, err := Get(context.Background(), f.db, a, d, "i"); !errors.Is(err, ErrNotFound) {
		t.Fatal("record leaked between authorized stores", err)
	}
	p, err := List(context.Background(), f.db, a, d, "", 50, 0)
	if err != nil || p.Total != 0 || len(p.Items) != 0 {
		t.Fatal(p, err)
	}
}
