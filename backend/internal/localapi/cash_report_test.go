package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/register"
)

const closedReportPath = "/local/v1/cash/sessions/turn-one/report"

func closedReport(t *testing.T, f *httpContractFixture, path string, want int) register.ClosedReport {
	t.Helper()
	status, body := request(t, f.app, "GET", path, "", f.token)
	if status != want {
		t.Fatalf("%s: got %d want %d: %s", path, status, want, body)
	}
	var v register.ClosedReport
	if want == 200 {
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatal(err)
		}
		domainSchemaFile(t, "cash-closed-report.openapi.json", "ClosedReport", body)
		if !v.Complete || v.BankSettlementAvailable || v.FinancialLedgerAvailable {
			t.Fatal(v)
		}
	}
	return v
}

func TestHTTPClosedReportUsesOfficialClosureAndExcludesUnsubmittedPlans(t *testing.T) {
	f := checkoutSetup(t)
	// The315 plan remains an unsubmitted allocation and must not appear as sales.
	planRequest(t, f, "POST", planPath, planJSON(t, checkoutInput(f)), 201)
	planCount(t, f, 1, 3, 1, 0, 1)
	before := isolationCounts(t, f.db)
	status, body := request(t, f.app, "GET", closedReportPath, "", f.token)
	if status != 409 || bytes.Contains(body, []byte("expected_cents")) {
		t.Fatalf("open drawer exposed: %d %s", status, body)
	}
	cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
	cashRequest(t, f.httpContractFixture, "movements", withdrawalBody, 200)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	afterClose := isolationCounts(t, f.db)
	v := closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if v.SessionID != "turn-one" || v.OperationID != "close-one" || v.Cash.OpeningCents != 1000 || v.Cash.MovementNetCents != 200 || v.Cash.ExpectedCents != 1200 || v.Cash.DeclaredCents != 1200 || v.Cash.DifferenceCents != 0 || v.Cash.SupplyCents != 500 || v.Cash.WithdrawalCents != 300 || v.SaleCount != 0 || v.GrossSalesCents != 0 || len(v.Payments) != 4 {
		t.Fatal(v)
	}
	if !reflect.DeepEqual(afterClose, isolationCounts(t, f.db)) {
		t.Fatal("report wrote business data")
	}
	if reflect.DeepEqual(before, afterClose) {
		t.Fatal("fixture did not execute movements and closure")
	}
	plan := planRequest(t, f, "GET", planPath+"/stable-cart", "", 200)
	if plan.State != "prepared" || len(plan.Parts) != 3 {
		t.Fatal("report changed checkout plan")
	}
}

func TestHTTPClosedReportHistoricalSessionCannotBorrowNewSession(t *testing.T) {
	f := salesSetup(t)
	salesRequest(t, f, salesInput(f), 201)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":4200}`, 200)
	want := closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if want.SaleCount != 1 || want.GrossSalesCents != 3198 || want.NetSalesCents != 3198 || want.CancelledSalesCents != 0 || want.Payments[0].ConfirmedCents != 3198 || want.Payments[0].Parts != 1 || want.Cash.ExpectedCents != 4198 || want.Cash.DifferenceCents != 2 {
		t.Fatal(want)
	}
	cashRequest(t, f.httpContractFixture, "open", `{"session_id":"second-turn","opening_cents":9900}`, 201)
	in := salesInput(f)
	in.SaleID = "sale-two"
	in.OperationID = "sale-op-two"
	in.CashSessionID = "second-turn"
	salesRequest(t, f, in, 201)
	closedReport(t, f.httpContractFixture, "/local/v1/cash/sessions/second-turn/report", 409)
	got := closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("historical report borrowed live data: %+v", got)
	}
	// Actual database/HTTP server restart preserves the historical response.
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
	got = closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("restart changed historical report")
	}
}

func TestHTTPClosedReportCancellationPreservesGrossAndReversals(t *testing.T) {
	f := cancellationSetup(t)
	cancelRequest(t, f, cancelBody, 200)
	cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":1000}`, 200)
	v := closedReport(t, f.httpContractFixture, closedReportPath, 200)
	if v.GrossSalesCents != 3198 || v.CancelledSalesCents != 3198 || v.NetSalesCents != 0 || v.Payments[0].ReversedCents != 3198 || v.Payments[0].ConfirmedCents != 0 || v.Cash.ExpectedCents != 1000 || v.Cash.MovementNetCents != 0 {
		t.Fatal(v)
	}
}

func TestHTTPClosedReportRejectsScopeQueryAndRevocationWithoutPOSLicense(t *testing.T) {
	f := salesSetup(t)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	if status, _ := request(t, f.app, "GET", closedReportPath, "", ""); status != 401 {
		t.Fatal(status)
	}
	closedReport(t, f.httpContractFixture, closedReportPath+"?session_id=second", 400)
	closedReport(t, f.httpContractFixture, "/local/v1/cash/sessions/missing/report", 404)
	policyPerson(t, f.db, f.owner, "other-cashier", "cashier")
	other := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: "other-cashier"}
	if _, err := register.ReadClosedReport(context.Background(), f.db, other, f.device, "turn-one"); !errors.Is(err, register.ErrReportNotFound) {
		t.Fatalf("other cashier read report: %v", err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
	closedReport(t, f.httpContractFixture, closedReportPath, 200)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := request(t, app, "GET", closedReportPath, "", f.token); status != 200 {
		t.Fatal("historical read required current verifier", status)
	}
	if _, err = f.db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	closedReport(t, f.httpContractFixture, closedReportPath, 401)
}

func TestHTTPClosedReportRefusesInconsistentHistoryInsteadOfInventingTotals(t *testing.T) {
	for _, damage := range []string{"missing_payment", "wrong_payment_sum", "missing_closure", "changed_expected", "changed_movements", "mismatched_state", "wrong_adjustment"} {
		t.Run(damage, func(t *testing.T) {
			f := salesSetup(t)
			salesRequest(t, f, salesInput(f), 201)
			cashRequest(t, f.httpContractFixture, "movements", supplyBody, 200)
			cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
			queries := map[string]string{
				"missing_payment":   `DELETE FROM sale_payments`,
				"wrong_payment_sum": `UPDATE sale_payments SET amount_cents=1`,
				"missing_closure":   `DELETE FROM cash_closures`,
				"changed_expected":  `UPDATE cash_closures SET expected_cents=1`,
				"changed_movements": `UPDATE cash_movements SET amount_cents=1`,
				"mismatched_state":  `UPDATE sale_payments SET status='reversed'`,
				"wrong_adjustment":  `UPDATE cash_adjustments SET amount_cents=1`,
			}
			if _, err := f.db.Exec(queries[damage]); err != nil {
				t.Fatal(err)
			}
			closedReport(t, f.httpContractFixture, closedReportPath, 409)
		})
	}
}

func TestHTTPClosedReportPreservesSafeIntegerBoundaryAndRejectsUnsafeAmounts(t *testing.T) {
	f := cashSetup(t)
	cashRequest(t, f, "open", `{"session_id":"turn-one","opening_cents":9007199254740991}`, 201)
	cashRequest(t, f, "close", `{"session_id":"turn-one","operation_id":"close-one","declared_cents":9007199254740991}`, 200)
	v := closedReport(t, f, closedReportPath, 200)
	if v.Cash.ExpectedCents != 9007199254740991 || v.Cash.DifferenceCents != 0 {
		t.Fatal(v)
	}
	// A pair of large legacy movements cancels exactly; SQL SUM/int64 order must
	// not overflow or introduce a float on a read-only report.
	for i, delta := range []int64{math.MaxInt64, math.MaxInt64, -math.MaxInt64, -math.MaxInt64} {
		if _, err := f.db.Exec(`INSERT INTO cash_movements VALUES(?,?,?,?,?,?,?)`, string(rune('a'+i)), f.owner.TenantID, f.owner.StoreID, "turn-one", delta, "test historical arithmetic", "now"); err != nil {
			t.Fatal(err)
		}
	}
	closedReport(t, f, closedReportPath, 200)
	if _, err := f.db.Exec(`UPDATE cash_closures SET declared_cents=9007199254740992`); err != nil {
		t.Fatal(err)
	}
	closedReport(t, f, closedReportPath, 422)
}

func TestHTTPClosedReportConcurrentReadsNeverMutateCash(t *testing.T) {
	f := salesSetup(t)
	salesRequest(t, f, salesInput(f), 201)
	cashRequest(t, f.httpContractFixture, "close", cashCloseBody, 200)
	before := isolationCounts(t, f.db)
	var wg sync.WaitGroup
	statuses := make(chan int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "GET", closedReportPath, "", f.token)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatal(status)
		}
	}
	if !reflect.DeepEqual(before, isolationCounts(t, f.db)) {
		t.Fatal("report wrote data")
	}
}
