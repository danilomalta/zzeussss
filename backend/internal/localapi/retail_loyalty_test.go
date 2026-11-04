package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/retail"
)

func loyaltySetup(t *testing.T) (*stockHTTPFixture, retail.Service) {
	t.Helper()
	f := salesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
	retailEnable(t, f.httpContractFixture, 10000)
	s := retailServiceFor(f.httpContractFixture)
	cfg := retail.Config{Revision: 1, LoyaltyEnabled: true, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}, EarnCents: 1, EarnPoints: 1, RedeemPoints: 1, RedeemCents: 1, ValidDays: 30}
	if e := s.Configure(context.Background(), "loyalty-policy", cfg); e != nil {
		t.Fatal(e)
	}
	if e := s.SaveCustomer(context.Background(), "loyalty-consent", retail.Customer{ID: "customer-external", Revision: 1, Name: "Synthetic", ConsentReference: "explicit-loyalty-consent", LoyaltyConsent: true}); e != nil {
		t.Fatal(e)
	}
	salesRequest(t, f, salesInput(f), 201)
	if e := s.Award(context.Background(), "customer-external", retail.PointsInput{OperationID: "award", SaleID: "sale-one", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	return f, s
}
func TestRetailConcurrentLoyaltyRedemption(t *testing.T) {
	_, s := loyaltySetup(t)
	const n = 6
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- s.Redeem(context.Background(), "customer-external", retail.PointsInput{OperationID: fmt.Sprintf("redeem-%d", i), Points: 2000, ExpectedBalance: 3198, Confirmed: true})
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	winner := ""
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, retail.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
	out, e := s.Points(context.Background(), "customer-external")
	if e != nil || out.Balance != 1198 {
		t.Fatal(out, e)
	}
	if e = s.DB.QueryRow(`SELECT operation_id FROM retail_point_redemptions`).Scan(&winner); e != nil {
		t.Fatal(e)
	}
	if e = s.Redeem(context.Background(), "customer-external", retail.PointsInput{OperationID: winner, Points: 2000, ExpectedBalance: 3198, Confirmed: true}); e != nil {
		t.Fatal("replay", e)
	}
	out, e = s.Points(context.Background(), "customer-external")
	if e != nil || out.Balance != 1198 {
		t.Fatal("duplicate redemption", out, e)
	}
}
func TestRetailLoyaltyRollbackAndCancellation(t *testing.T) {
	f, s := loyaltySetup(t)
	if _, e := f.db.Exec(`CREATE TRIGGER synthetic_points_abort BEFORE INSERT ON retail_point_redemptions BEGIN SELECT RAISE(ABORT,'rollback'); END`); e != nil {
		t.Fatal(e)
	}
	if e := s.Redeem(context.Background(), "customer-external", retail.PointsInput{OperationID: "rollback-points", Points: 100, ExpectedBalance: 3198, Confirmed: true}); e == nil {
		t.Fatal("expected rollback")
	}
	out, e := s.Points(context.Background(), "customer-external")
	if e != nil || out.Balance != 3198 {
		t.Fatal("partial debit", out, e)
	}
	code, raw := request(t, f.app, "POST", "/local/v1/sales/sale-one/cancel", `{"operation_id":"cancel-for-points","reason":"Synthetic cancellation"}`, f.token)
	if code != 200 {
		t.Fatal("cancel", code, string(raw))
	}
	out, e = s.Points(context.Background(), "customer-external")
	if e != nil || out.Balance != 0 {
		t.Fatal("cancellation points", out, e)
	}
	if e = s.Award(context.Background(), "customer-external", retail.PointsInput{OperationID: "award-cancelled", SaleID: "sale-one", Confirmed: true}); !errors.Is(e, retail.ErrConflict) {
		t.Fatal(e)
	}
}
func TestRetailLoyaltyConsentAndUnknownCustomer(t *testing.T) {
	_, s := loyaltySetup(t)
	if _, e := s.Points(context.Background(), "other-customer"); !errors.Is(e, retail.ErrDisabled) {
		t.Fatal("foreign customer", e)
	}
	if _, e := s.Privacy(context.Background(), "anonymous-loyalty", "customer-external", 2, true); e != nil {
		t.Fatal(e)
	}
	if e := s.Redeem(context.Background(), "customer-external", retail.PointsInput{OperationID: "after-anonymize", Points: 10, ExpectedBalance: 3198, Confirmed: true}); !errors.Is(e, retail.ErrDisabled) {
		t.Fatal("consent revoked", e)
	}
}
func TestRetailLoyaltyPartialRefundReconcilesOriginalPolicy(t *testing.T) {
	f, s := loyaltySetup(t)
	cfg := retail.Config{Revision: 2, LoyaltyEnabled: true, RefundEnabled: true, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}, EarnCents: 100, EarnPoints: 1, RedeemPoints: 1, RedeemCents: 100, ValidDays: 30}
	if e := s.Configure(context.Background(), "new-policy", cfg); e != nil {
		t.Fatal(e)
	}
	var item string
	f.db.QueryRow(`SELECT id FROM sale_items WHERE sale_id='sale-one'`).Scan(&item)
	if e := s.RefundSale(context.Background(), "sale-one", retail.RefundInput{OperationID: "partial-point-refund", ItemID: item, QuantityMilli: 1000, ExpectedCents: 1599, SessionID: "turn-one", Reason: "Synthetic correction"}); e != nil {
		t.Fatal(e)
	}
	out, e := s.Points(context.Background(), "customer-external")
	if e != nil || out.Balance != 1599 {
		t.Fatal("original earning policy lost", out, e)
	}
	again, e := s.Points(context.Background(), "customer-external")
	if e != nil || again.Balance != 1599 || len(again.Entries) != 2 {
		t.Fatal("duplicated reversal", again, e)
	}
}

func TestRetailLoyaltyExpiryIsExplicit(t *testing.T) {
	_, s := loyaltySetup(t)
	s.Clock = func() time.Time { return time.Now().Add(31 * 24 * time.Hour) }
	out, e := s.Points(context.Background(), "customer-external")
	if e != nil || out.Balance != 0 || len(out.Entries) != 1 {
		t.Fatal(out, e)
	}
	if e = s.Redeem(context.Background(), "customer-external", retail.PointsInput{OperationID: "expired-redemption", Points: 1, ExpectedBalance: 0, Confirmed: true}); !errors.Is(e, retail.ErrConflict) {
		t.Fatal(e)
	}
}
func TestHTTPRetailLoyaltyRealAPI(t *testing.T) {
	f, _ := loyaltySetup(t)
	path := "/local/v1/retail/customers/customer-external/loyalty"
	for i := 0; i < 2; i++ {
		code, raw := request(t, f.app, "POST", path+"/redeem", `{"operation_id":"http-redemption","points":1000,"expected_balance":3198,"confirmed":true}`, f.token)
		if code != 204 {
			t.Fatal(code, string(raw))
		}
	}
	code, raw := request(t, f.app, "GET", path, "", f.token)
	if code != 200 {
		t.Fatal(code, string(raw))
	}
	var out retail.PointsAccount
	if e := json.Unmarshal(raw, &out); e != nil || out.Balance != 2198 || len(out.Benefits) != 1 || out.Benefits[0].BenefitCents != 1000 {
		t.Fatal(out, e)
	}
	code, _ = request(t, f.app, "POST", path+"/redeem", `{"operation_id":"http-stale","points":1000,"expected_balance":3198,"confirmed":true}`, f.token)
	if code != 409 {
		t.Fatal("stale balance", code)
	}
}
