package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	"titansystem-backend/internal/localdb/stock"
)

const cashPlanID = "cash-cart"
const finalizeCashPath = planPath + "/" + cashPlanID + "/finalize-cash"
const finalizeCashBody = `{"operation_id":"cash-cart","received_cents":4000}`

func cashPlanSetup(t *testing.T) (*stockHTTPFixture, sale.PlanInput) {
	t.Helper()
	f := salesSetup(t)
	in := sale.PlanInput{OperationID: "prepare-cash", CheckoutID: cashPlanID, CashSessionID: "turn-one", ExpectedTotalCents: 3198, Items: salesInput(f).Items, Parts: []sale.PlanPartInput{{PartID: "cash-one", Method: "cash", AmountCents: 1000}, {PartID: "cash-two", Method: "cash", AmountCents: 2198}}}
	v := planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	if !v.CanFinalize {
		t.Fatal("cash plan not ready")
	}
	return f, in
}

func cashFinalCounts(t *testing.T, f *stockHTTPFixture, sales int) {
	t.Helper()
	for _, c := range []struct {
		table string
		n     int
	}{{"sales", sales}, {"sale_items", sales}, {"sale_item_stock", sales}, {"sale_operations", sales}, {"sale_payments", 2 * sales}, {"cash_movements", sales}, {"checkout_plan_sales", sales}} {
		var n int
		if err := f.db.QueryRow("SELECT count(*) FROM " + c.table).Scan(&n); err != nil || n != c.n {
			t.Fatalf("%s=%d want %d err %v", c.table, n, c.n, err)
		}
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='sale.committed'`).Scan(&n); err != nil || n != sales {
		t.Fatalf("sale events %d %v", n, err)
	}
	assertStockBalance(t, f, f.shelf, 5000-int64(sales)*2000)
}

func TestHTTPCheckoutCashFinalizationBooksNetCashAndStablePaymentIDsOnce(t *testing.T) {
	f, in := cashPlanSetup(t)
	v := planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	if v.State != "completed" || v.CanFinalize || v.Repeated || v.Completion == nil || v.Completion.SaleID != cashPlanID || v.Completion.OperationID != cashPlanID || v.Completion.SaleStatus != "committed" || v.Completion.ReceivedCents != 4000 || v.Completion.ChangeCents != 802 {
		t.Fatal(v)
	}
	for _, p := range v.Parts {
		if p.State != "confirmed" || p.ProviderConfirmed {
			t.Fatal(p)
		}
	}
	r := readReceipt(t, f, cashPlanID)
	if r.OperationID != cashPlanID || r.TotalCents != 3198 || len(r.Payments) != 2 || r.FiscalAuthorized {
		t.Fatal(r)
	}
	for _, p := range in.Parts {
		found := false
		for _, saved := range r.Payments {
			if saved.PaymentID == p.PartID && saved.AmountCents == p.AmountCents && saved.Status == "confirmed" {
				found = true
			}
		}
		if !found {
			t.Fatal("stable payment ID lost", p)
		}
	}
	cashFinalCounts(t, f, 1)
	var net int64
	if err := f.db.QueryRow(`SELECT amount_cents FROM cash_movements`).Scan(&net); err != nil || net != 3198 {
		t.Fatalf("tender was booked as revenue: %d %v", net, err)
	}
	// A new finalization ID or changed tender cannot erase the original request.
	planRequest(t, f, "POST", finalizeCashPath, strings.Replace(finalizeCashBody, "4000", "5000", 1), 409)
	planRequest(t, f, "POST", finalizeCashPath, strings.Replace(finalizeCashBody, `"cash-cart"`, `"new-operation"`, 1), 409)
	planRequest(t, f, "POST", planPath+"/"+cashPlanID+"/cancel", `{"operation_id":"cancel-plan","reason":"Too late"}`, 409)
	got := planRequest(t, f, "POST", planPath, planJSON(t, in), 200)
	if got.State != "completed" || got.Completion == nil {
		t.Fatal("creation replay reopened completed plan")
	}
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":4198}`, 200)
	got = planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 200)
	if !got.Repeated || !reflect.DeepEqual(got.Completion, v.Completion) {
		t.Fatal("closed shift replay changed completion")
	}
	report := closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if report.Cash.ExpectedCents != 4198 || report.NetSalesCents != 3198 || report.Payments[0].Parts != 2 || report.Payments[0].ConfirmedCents != 3198 {
		t.Fatal(report)
	}
	cashFinalCounts(t, f, 1)
}

func TestHTTPCheckoutCashFinalizationLostResponseSurvivesRestart(t *testing.T) {
	f, _ := cashPlanSetup(t)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201) // response lost to caller
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
	got := planRequest(t, f, "GET", planPath+"/"+cashPlanID, "", 200)
	if got.Completion == nil || got.Completion.ChangeCents != 802 || got.Completion.SaleStatus != "committed" {
		t.Fatal(got)
	}
	if !planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 200).Repeated {
		t.Fatal("lost-response replay not recognized")
	}
	cashFinalCounts(t, f, 1)
}

func TestHTTPCheckoutCashFinalizationRollbackIncludesEverySaleEffect(t *testing.T) {
	for _, stage := range []string{"part", "outbox", "completion_abort", "completion_ignore"} {
		t.Run(stage, func(t *testing.T) {
			f, _ := cashPlanSetup(t)
			triggers := map[string]string{
				"part":              `CREATE TRIGGER injected BEFORE INSERT ON sale_payments WHEN NEW.id='cash-two' BEGIN SELECT RAISE(ABORT,'part failed'); END`,
				"outbox":            `CREATE TRIGGER injected BEFORE INSERT ON outbox WHEN NEW.event_type='sale.committed' BEGIN SELECT RAISE(ABORT,'event failed'); END`,
				"completion_abort":  `CREATE TRIGGER injected BEFORE INSERT ON checkout_plan_sales BEGIN SELECT RAISE(ABORT,'completion failed'); END`,
				"completion_ignore": `CREATE TRIGGER injected BEFORE INSERT ON checkout_plan_sales BEGIN SELECT RAISE(IGNORE); END`,
			}
			if _, err := f.db.Exec(triggers[stage]); err != nil {
				t.Fatal(err)
			}
			want := 500
			if stage == "completion_ignore" {
				want = 409
			}
			planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, want)
			cashFinalCounts(t, f, 0)
			v := planRequest(t, f, "GET", planPath+"/"+cashPlanID, "", 200)
			if v.State != "prepared" || v.Completion != nil || v.Parts[0].State != "not_requested" {
				t.Fatal("partial completion survived", v)
			}
			if _, err := f.db.Exec(`DROP TRIGGER injected`); err != nil {
				t.Fatal(err)
			}
			planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
			cashFinalCounts(t, f, 1)
		})
	}
}

func TestHTTPCheckoutCashFinalizationDoesNotApproveMixedPayments(t *testing.T) {
	f := checkoutSetup(t)
	in := checkoutInput(f)
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	planRequest(t, f, "POST", planPath+"/"+in.CheckoutID+"/finalize-cash", `{"operation_id":"mixed-final","received_cents":31500}`, 409)
	planCount(t, f, 1, 3, 1, 0, 1)
	v := planRequest(t, f, "GET", planPath+"/"+in.CheckoutID, "", 200)
	if v.CanFinalize || v.Completion != nil {
		t.Fatal(v)
	}
}

func TestHTTPCheckoutCashFinalizationRevalidatesPriceUnitStockShiftAndContract(t *testing.T) {
	for _, change := range []string{"price", "unit", "stock", "shift", "contract", "cancelled"} {
		t.Run(change, func(t *testing.T) {
			f, _ := cashPlanSetup(t)
			want := 409
			switch change {
			case "price":
				if _, err := f.db.Exec(`UPDATE products SET price_cents=1600`); err != nil {
					t.Fatal(err)
				}
			case "unit":
				if _, err := f.db.Exec(`UPDATE products SET unit='kg'`); err != nil {
					t.Fatal(err)
				}
			case "stock":
				stockRequest(t, f, stock.Input{OperationID: "another-stock-use", Kind: "transfer", ProductID: f.product, FromLocationID: f.shelf, ToLocationID: f.back, QuantityMilli: 4000, Reason: "Another operation"}, 201)
			case "shift":
				cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
			case "contract":
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
				want = 403
			case "cancelled":
				planRequest(t, f, "POST", planPath+"/"+cashPlanID+"/cancel", `{"operation_id":"cancel-plan","reason":"Desistiu"}`, 200)
			}
			planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, want)
			for _, table := range []string{"sales", "sale_payments", "cash_movements", "checkout_plan_sales"} {
				var n int
				if err := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
					t.Fatalf("partial %s=%d %v", table, n, err)
				}
			}
		})
	}
}

func TestHTTPCheckoutCashFinalizationRejectsAmbiguousTenderAndForeignActor(t *testing.T) {
	f, original := cashPlanSetup(t)
	for _, body := range []string{`null`, `{}`, finalizeCashBody + ` {}`, strings.Replace(finalizeCashBody, `"received_cents":4000`, `"received_cents":4000,"approved":true`, 1), strings.Replace(finalizeCashBody, `"received_cents":4000`, `"received_cents":null`, 1), strings.Replace(finalizeCashBody, "4000", "1.5", 1), strings.Replace(finalizeCashBody, "4000", "3197", 1), strings.Replace(finalizeCashBody, "4000", "9007199254740992", 1)} {
		planRequest(t, f, "POST", finalizeCashPath, body, 400)
	}
	original.OperationID = "occupied-prepare"
	original.CheckoutID = "another-cart"
	planRequest(t, f, "POST", planPath, planJSON(t, original), 201)
	planRequest(t, f, "POST", finalizeCashPath, strings.Replace(finalizeCashBody, "cash-cart", "occupied-prepare", 1), 409)
	if status, _ := request(t, f.app, "POST", finalizeCashPath, finalizeCashBody, ""); status != 401 {
		t.Fatal(status)
	}
	policyPerson(t, f.db, f.owner, "other-cashier", "cashier")
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "other-cashier"}
	if _, err := sale.FinalizeCashPlan(context.Background(), f.db, nil, actor, f.device, cashPlanID, sale.FinalizeCashPlanInput{OperationID: cashPlanID, ReceivedCents: 4000}); !errors.Is(err, sale.ErrNotFound) {
		t.Fatal("foreign actor", err)
	}
	cashFinalCounts(t, f, 0)
}

func TestHTTPCheckoutCashFinalizationConcurrentRequestsPersistOneSale(t *testing.T) {
	f, _ := cashPlanSetup(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", finalizeCashPath, finalizeCashBody, f.token)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	created, repeated := 0, 0
	for s := range statuses {
		switch s {
		case 201:
			created++
		case 200:
			repeated++
		default:
			t.Fatal(s)
		}
	}
	if created != 1 || repeated != 1 {
		t.Fatalf("created%d repeated%d", created, repeated)
	}
	cashFinalCounts(t, f, 1)
}

func TestHTTPCheckoutCashFinalizationCancellationDoesNotPermitResale(t *testing.T) {
	f, _ := cashPlanSetup(t)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	if status, b := request(t, f.app, "POST", "/local/v1/sales/"+cashPlanID+"/cancel", cancelBody, f.token); status != 200 {
		t.Fatalf("cash refund: %d %s", status, b)
	}
	v := planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 200)
	if !v.Repeated || v.State != "completed" || v.Completion.SaleStatus != "cancelled" || v.Parts[0].State != "reversed" {
		t.Fatal(v)
	}
	assertStockBalance(t, f, f.shelf, 5000)
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM sales`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":1000}`, 200)
	r := closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if r.NetSalesCents != 0 || r.CancelledSalesCents != 3198 || r.Cash.ExpectedCents != 1000 || r.Payments[0].ReversedCents != 3198 {
		t.Fatal(r)
	}
}

func TestHTTPCheckoutCashFinalizationBackupRestoresTenderAndStableParts(t *testing.T) {
	f, _ := cashPlanSetup(t)
	want := planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	dir := t.TempDir()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "completed.tytbak")
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
	got, err := sale.ReadPlan(ctx, db, actor, f.device, cashPlanID)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("restored checkout %+v %v", got, err)
	}
	f.db = db
	cashFinalCounts(t, f, 1)
}

func TestHTTPCheckoutCashFinalizationPreservesAllTwentyCashPartIDs(t *testing.T) {
	f := salesSetup(t)
	in := sale.PlanInput{OperationID: "prepare20", CheckoutID: cashPlanID, CashSessionID: "turn-one", ExpectedTotalCents: 3198, Items: salesInput(f).Items}
	for i := 0; i < 20; i++ {
		amount := int64(1)
		if i == 19 {
			amount = 3179
		}
		in.Parts = append(in.Parts, sale.PlanPartInput{PartID: string(rune('a' + i)), Method: "cash", AmountCents: amount})
	}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	r := readReceipt(t, f, cashPlanID)
	if len(r.Payments) != 20 {
		t.Fatal(len(r.Payments))
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM cash_movements`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	status, body := request(t, f.app, "GET", "/local/v1/sales/"+cashPlanID, "", f.token)
	if status != 200 {
		t.Fatal(status)
	}
	cashSaleSchema(t, "Receipt", body)
}

func TestHTTPCheckoutCashFinalizationCannotAdoptUnrelatedExistingSale(t *testing.T) {
	f, _ := cashPlanSetup(t)
	in := salesInput(f)
	in.SaleID = cashPlanID
	in.OperationID = "legacy-operation"
	salesRequest(t, f, in, 201)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 409)
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM checkout_plan_sales`).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if r := readReceipt(t, f, cashPlanID); r.OperationID != "legacy-operation" || len(r.Payments) != 1 {
		t.Fatal(r)
	}
	assertStockBalance(t, f, f.shelf, 3000)
}

func TestHTTPCheckoutCashFinalizationAllowsCheckoutIDAsItsOwnCreationAndSaleOperation(t *testing.T) {
	f := salesSetup(t)
	in := sale.PlanInput{OperationID: cashPlanID, CheckoutID: cashPlanID, CashSessionID: "turn-one", ExpectedTotalCents: 3198, Items: salesInput(f).Items, Parts: []sale.PlanPartInput{{PartID: "cash", Method: "cash", AmountCents: 3198}}}
	planRequest(t, f, "POST", planPath, planJSON(t, in), 201)
	planRequest(t, f, "POST", finalizeCashPath, finalizeCashBody, 201)
	if r := readReceipt(t, f, cashPlanID); r.OperationID != cashPlanID || r.Payments[0].PaymentID != "cash" {
		t.Fatal(r)
	}
}
