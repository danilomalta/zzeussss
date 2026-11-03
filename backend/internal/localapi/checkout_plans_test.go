package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/sale"
)

const planPath = "/local/v1/checkout/plans"

func checkoutInput(f *stockHTTPFixture) sale.PlanInput {
	return sale.PlanInput{OperationID: "prepare-cart", CheckoutID: "stable-cart", CashSessionID: "turn-one", ExpectedTotalCents: 31500,
		Items: []sale.Item{{ProductID: f.product, LocationID: f.shelf, QuantityMilli: 2000}},
		Parts: []sale.PlanPartInput{{PartID: "cash-1", Method: "cash", AmountCents: 10000}, {PartID: "debit-1", Method: "debit", AmountCents: 9600}, {PartID: "credit-1", Method: "credit", AmountCents: 11900}}}
}

func checkoutSetup(t *testing.T) *stockHTTPFixture {
	t.Helper()
	f := salesSetup(t)
	if _, err := f.db.Exec(`UPDATE products SET price_cents=15750 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	return f
}

func planJSON(t *testing.T, in any) string {
	t.Helper()
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func planRequest(t *testing.T, f *stockHTTPFixture, method, path, body string, want int) sale.Plan {
	t.Helper()
	status, b := request(t, f.app, method, path, body, f.token)
	if status != want {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, status, want, b)
	}
	var v sale.Plan
	if want == 200 || want == 201 {
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "checkout-plans.openapi.json", "Plan", b)
	}
	return v
}

func planCount(t *testing.T, f *stockHTTPFixture, plans, parts, operations, cancellations, events int) {
	t.Helper()
	for _, c := range []struct {
		q    string
		want int
	}{
		{`SELECT count(*) FROM checkout_plans`, plans}, {`SELECT count(*) FROM checkout_plan_parts`, parts},
		{`SELECT count(*) FROM checkout_plan_operations`, operations}, {`SELECT count(*) FROM checkout_plan_cancellations`, cancellations},
		{`SELECT count(*) FROM outbox WHERE event_type LIKE 'checkout.plan.%'`, events},
	} {
		var n int
		if err := f.db.QueryRow(c.q).Scan(&n); err != nil || n != c.want {
			t.Fatalf("%s: %d want %d err %v", c.q, n, c.want, err)
		}
	}
	salesCounts(t, f, 0)
	assertStockBalance(t, f, f.shelf, 5000)
}

func TestHTTPCheckoutPlan315PersistsWithoutChargingAndRecoversAfterRestart(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	body := planJSON(t, in)
	original := planRequest(t, f, "POST", planPath, body, 201)
	if original.CheckoutID != in.CheckoutID || original.TotalCents != 31500 || original.CanFinalize || original.State != "prepared" || original.Repeated || len(original.Parts) != 3 {
		t.Fatal(original)
	}
	for i, p := range original.Parts {
		if p.PlanPartInput != in.Parts[i] || p.State != "not_requested" || p.ProviderConfirmed {
			t.Fatal(p)
		}
	}
	// Discard the creation response: a new connection/server must recover exact IDs.
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f.db = db
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	got := planRequest(t, f, "GET", planPath+"/"+in.CheckoutID, "", 200)
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("restart changed snapshot: %+v", got)
	}
	// Recovery is history, not a fresh quote: closing, stock and prices may change.
	if _, err = db.Exec(`UPDATE products SET price_cents=1 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	got = planRequest(t, f, "POST", planPath, body, 200)
	got.Repeated = false
	if !reflect.DeepEqual(got, original) {
		t.Fatal("replay repriced or changed snapshot")
	}
	planCount(t, f, 1, 3, 1, 0, 1)
}

func TestHTTPCheckoutPlanRejectsMutationStalePricesAndForgedApproval(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	body := planJSON(t, in)
	for _, bad := range []string{
		`null`, `{}`, body + ` {}`, strings.Replace(body, `"parts":`, `"tenant_id":"foreign","parts":`, 1),
		strings.Replace(body, `"part_id":"cash-1"`, `"part_id":"cash-1","part_id":"another"`, 1),
		strings.Replace(body, `"amount_cents":10000`, `"amount_cents":10000,"provider_confirmed":true`, 1),
		strings.Replace(body, `"amount_cents":10000`, `"amount_cents":10000,"state":"approved"`, 1),
		strings.Replace(body, `"amount_cents":10000`, `"amount_cents":null`, 1),
		strings.Replace(body, `"amount_cents":10000`, `"amount_cents":1.5`, 1),
		strings.Replace(body, `"amount_cents":10000`, `"amount_cents":0`, 1),
		strings.Replace(body, `"part_id":"debit-1"`, `"part_id":"cash-1"`, 1),
		strings.Replace(body, `"method":"debit"`, `"method":"unknown"`, 1),
		strings.Replace(body, `"amount_cents":10000`, `"amount_cents":10001`, 1),
	} {
		planRequest(t, f, "POST", planPath, bad, 400)
	}
	planCount(t, f, 0, 0, 0, 0, 0)
	// Correctly allocated but stale total must never silently change the allocation.
	if _, err := f.db.Exec(`UPDATE products SET price_cents=16000 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	planRequest(t, f, "POST", planPath, body, 409)
	planCount(t, f, 0, 0, 0, 0, 0)
	if _, err := f.db.Exec(`UPDATE products SET price_cents=15750 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	planRequest(t, f, "POST", planPath, body, 201)
	in.Parts[0].PartID = "different-id"
	planRequest(t, f, "POST", planPath, planJSON(t, in), 409)
	in = checkoutInput(f)
	in.OperationID = "another-operation"
	planRequest(t, f, "POST", planPath, planJSON(t, in), 409)
	in = checkoutInput(f)
	in.CheckoutID = "another-cart"
	planRequest(t, f, "POST", planPath, planJSON(t, in), 409)
	planCount(t, f, 1, 3, 1, 0, 1)
}

func TestHTTPCheckoutPlanCancelPreservesHistoryAndReplay(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	body := planJSON(t, in)
	original := planRequest(t, f, "POST", planPath, body, 201)
	url := planPath + "/" + in.CheckoutID + "/cancel"
	cancel := sale.CancelPlanInput{OperationID: "cancel-cart", Reason: "Cliente desistiu"}
	got := planRequest(t, f, "POST", url, planJSON(t, cancel), 200)
	if got.State != "cancelled" || got.Cancellation == nil || got.Cancellation.OperationID != cancel.OperationID || !reflect.DeepEqual(got.Parts, original.Parts) {
		t.Fatal(got)
	}
	got = planRequest(t, f, "POST", url, planJSON(t, cancel), 200)
	if !got.Repeated {
		t.Fatal("cancel not replayed")
	}
	cancel.Reason = "Reescrever passado"
	planRequest(t, f, "POST", url, planJSON(t, cancel), 409)
	cancel.OperationID = "second-cancel"
	planRequest(t, f, "POST", url, planJSON(t, cancel), 409)
	got = planRequest(t, f, "POST", planPath, body, 200)
	if got.State != "cancelled" || !got.Repeated {
		t.Fatal("creation replay reopened checkout")
	}
	planCount(t, f, 1, 3, 2, 1, 2)
}

func TestHTTPCheckoutPlanRollbackIncludesPartsOperationsAndOutbox(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	if _, err := f.db.Exec(`CREATE TRIGGER fail_plan_outbox BEFORE INSERT ON outbox WHEN NEW.event_type LIKE 'checkout.plan.%' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 500)
	planCount(t, f, 0, 0, 0, 0, 0)
	if _, err := f.db.Exec(`DROP TRIGGER fail_plan_outbox`); err != nil {
		t.Fatal(err)
	}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	if _, err := f.db.Exec(`CREATE TRIGGER fail_plan_cancel BEFORE INSERT ON outbox WHEN NEW.event_type='checkout.plan.cancelled' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	planRequest(t, f, "POST", planPath+"/"+in.CheckoutID+"/cancel", `{"operation_id":"cancel-cart","reason":"Desistiu"}`, 500)
	if got := planRequest(t, f, "GET", planPath+"/"+in.CheckoutID, "", 200); got.State != "prepared" {
		t.Fatal(got)
	}
	planCount(t, f, 1, 3, 1, 0, 1)
}

func TestHTTPCheckoutPlanConcurrentIdenticalRequestsPersistOnce(t *testing.T) {
	f := checkoutSetup(t)
	body := planJSON(t, checkoutInput(f))
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", planPath, body, f.token)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	var created, replayed int
	for status := range statuses {
		switch status {
		case 201:
			created++
		case 200:
			replayed++
		default:
			t.Fatalf("concurrent status %d", status)
		}
	}
	if created != 1 || replayed != 1 {
		t.Fatalf("created %d replayed %d", created, replayed)
	}
	planCount(t, f, 1, 3, 1, 0, 1)
}

func TestHTTPCheckoutPlanRequiresPOSAndProtectsExistingRecordOwner(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	body := planJSON(t, in)
	if status, _ := request(t, f.app, "POST", planPath, body, ""); status != 401 {
		t.Fatal(status)
	}
	planRequest(t, f, "POST", planPath, body, 201)
	policyPerson(t, f.db, f.owner, "other-cashier", "cashier")
	other := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "other-cashier"}
	if _, err := sale.ReadPlan(context.Background(), f.db, other, f.device, in.CheckoutID); !errors.Is(err, sale.ErrNotFound) {
		t.Fatalf("foreign actor read: %v", err)
	}
	if _, err := sale.CancelPlan(context.Background(), f.db, other, f.device, in.CheckoutID, sale.CancelPlanInput{OperationID: "steal", Reason: "Other"}); !errors.Is(err, sale.ErrNotFound) {
		t.Fatalf("foreign actor cancelled: %v", err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	in.OperationID = "unlicensed-op"
	in.CheckoutID = "unlicensed-cart"
	planRequest(t, f, "POST", planPath, planJSON(t, in), 403)
	// Historical read and abandoning an unsubmitted plan remain available.
	planRequest(t, f, "GET", planPath+"/stable-cart", "", 200)
	planRequest(t, f, "POST", planPath+"/stable-cart/cancel", `{"operation_id":"cancel-cart","reason":"Desistiu"}`, 200)
	planCount(t, f, 1, 3, 2, 1, 2)
}

func TestHTTPCheckoutPlanBackupRestorePreservesPartsCancellationAndAudit(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	want := planRequest(t, f, "POST", planPath+"/"+in.CheckoutID+"/cancel", `{"operation_id":"cancel-cart","reason":"Desistiu"}`, 200)
	dir := t.TempDir()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "checkout.tytbak")
	ctx := context.Background()
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err := backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "restored.sqlite")
	if err := backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	got, err := sale.ReadPlan(ctx, db, actor, f.device, in.CheckoutID)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("restored checkout: %+v err %v", got, err)
	}
	f.db = db
	planCount(t, f, 1, 3, 2, 1, 2)
}

func TestHTTPCheckoutPlanUsesExactWeightedRoundingAndCurrentFreeStock(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	if _, err := f.db.Exec(`UPDATE products SET unit='kg',price_cents=299 WHERE tenant_id=? AND id=?`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	in.Items[0].QuantityMilli = 1500
	in.ExpectedTotalCents = 449
	in.Parts = []sale.PlanPartInput{{PartID: "cash", Method: "cash", AmountCents: 1}, {PartID: "credit", Method: "credit", AmountCents: 448}}
	v := planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	if v.TotalCents != 449 || v.Items[0].Unit != "kg" || v.Items[0].TotalCents != 449 || v.Items[0].QuantityMilli != 1500 {
		t.Fatal(v)
	}
	in.CheckoutID = "second-cart"
	in.OperationID = "second-prepare"
	in.Items = []sale.Item{{ProductID: f.product, LocationID: f.shelf, QuantityMilli: 3000}, {ProductID: f.product, LocationID: f.shelf, QuantityMilli: 3000}}
	in.ExpectedTotalCents = 1794
	in.Parts = []sale.PlanPartInput{{PartID: "cash", Method: "cash", AmountCents: 1794}}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 409)
	if _, err := f.db.Exec(`INSERT INTO catalog_product_states(tenant_id,product_id,status,revision) VALUES(?,?,'inactive',1)`, f.owner.TenantID, f.product); err != nil {
		t.Fatal(err)
	}
	in.Items = in.Items[:1]
	in.Items[0].QuantityMilli = 1500
	in.ExpectedTotalCents = 449
	in.Parts[0].AmountCents = 449
	planRequest(t, f, "POST", planPath, planJSON(t, in), 409)
	planRequest(t, f, "GET", planPath+"/stable-cart", "", 200)
	planCount(t, f, 1, 2, 1, 0, 1)
}

func TestHTTPCheckoutPlanHonorsRealProductionReservation(t *testing.T) {
	f, reservation := materialsFixture(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory, modules.Production, modules.POS}), 200)
	requireMaterials(t, f, "POST", materialsPath, reservation, 201)
	if _, err := f.db.Exec(`UPDATE stock_locations SET kind='shelf' WHERE tenant_id=? AND store_id=? AND id='production-room'`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE products SET price_cents=1000 WHERE tenant_id=? AND id='flour'`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "open", `{"session_id":"reserved-shift","opening_cents":0}`, 201)
	in := sale.PlanInput{OperationID: "reserved-plan", CheckoutID: "reserved-cart", CashSessionID: "reserved-shift", ExpectedTotalCents: 500001, Items: []sale.Item{{ProductID: "flour", LocationID: "production-room", QuantityMilli: 500001}}, Parts: []sale.PlanPartInput{{PartID: "cash", Method: "cash", AmountCents: 500001}}}
	if status, _ := request(t, f.app, "POST", planPath, planJSON(t, in), f.token); status != 409 {
		t.Fatalf("reserved material planned: %d", status)
	}
	materialCount(t, f, "checkout_plans", 0)
	in.Items[0].QuantityMilli = 500000
	in.ExpectedTotalCents = 500000
	in.Parts[0].AmountCents = 500000
	status, body := request(t, f.app, "POST", planPath, planJSON(t, in), f.token)
	if status != 201 {
		t.Fatalf("free balance refused: %d %s", status, body)
	}
	var v sale.Plan
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	if v.Items[0].AvailableMilli != 500000 {
		t.Fatal("physical balance exposed as free")
	}
	materialCount(t, f, "sales", 0)
	materialCount(t, f, "cash_movements", 0)
}

func TestHTTPCheckoutPlanNoVerifierAndRoleDoNotPermitCreation(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	body := planJSON(t, in)
	planRequest(t, f, "POST", planPath+"?tenant_id=other", body, 400)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := request(t, app, "POST", planPath, body, f.token); status != 503 {
		t.Fatal(status)
	}
	if _, err = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE memberships SET role='stock' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	planRequest(t, f, "POST", planPath, body, 403)
	planCount(t, f, 0, 0, 0, 0, 0)
}
