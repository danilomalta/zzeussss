package localapi

import (
	"context"
	"errors"
	"sync"
	"testing"

	"titansystem-backend/internal/localdb/retail"
)

// SQLite RAISE(IGNORE) returns no SQL error: each required write must still
// exist before the operation receipt can commit. All data here is disposable.
func TestRetailLoyaltyIgnoredWritesRollback(t *testing.T) {
	for _, table := range []string{"retail_points", "retail_point_redemptions"} {
		t.Run(table, func(t *testing.T) {
			f, s := loyaltySetup(t)
			ctx := context.Background()
			if _, e := f.db.Exec("CREATE TRIGGER synthetic_ignore BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			in := retail.PointsInput{OperationID: "ignored-redeem", Points: 100, ExpectedBalance: 3198, Confirmed: true}
			if e := s.Redeem(ctx, "customer-external", in); !errors.Is(e, retail.ErrConflict) {
				t.Fatalf("required write ignored: %v", e)
			}
			out, e := s.Points(ctx, "customer-external")
			if e != nil || out.Balance != 3198 || len(out.Benefits) != 0 || len(out.Entries) != 1 {
				t.Fatalf("partial redemption: %+v %v", out, e)
			}
			var n int
			if e = f.db.QueryRow(`SELECT count(*) FROM retail_operations WHERE operation_id=?`, in.OperationID).Scan(&n); e != nil || n != 0 {
				t.Fatal("receipt committed", n, e)
			}
			if _, e = f.db.Exec(`DROP TRIGGER synthetic_ignore`); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				if e = s.Redeem(ctx, "customer-external", in); e != nil {
					t.Fatal(e)
				}
			}
			out, e = s.Points(ctx, "customer-external")
			if e != nil || out.Balance != 3098 || len(out.Benefits) != 1 || len(out.Entries) != 2 {
				t.Fatalf("retry duplicated: %+v %v", out, e)
			}
		})
	}
}

func TestRetailLoyaltyIgnoredAwardRollback(t *testing.T) {
	for _, table := range []string{"retail_point_awards", "retail_points"} {
		t.Run(table, func(t *testing.T) {
			f, s := loyaltySetup(t)
			inSale := salesInput(f)
			inSale.SaleID = "sale-two"
			inSale.OperationID = "sale-two-operation"
			salesRequest(t, f, inSale, 201)
			if _, e := f.db.Exec("CREATE TRIGGER synthetic_ignore BEFORE INSERT ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			in := retail.PointsInput{OperationID: "ignored-award", SaleID: "sale-two", Confirmed: true}
			if e := s.Award(context.Background(), "customer-external", in); !errors.Is(e, retail.ErrConflict) {
				t.Fatalf("required award ignored: %v", e)
			}
			for _, q := range []string{`SELECT count(*) FROM retail_point_awards WHERE sale_id='sale-two'`, `SELECT count(*) FROM retail_points WHERE sale_id='sale-two'`, `SELECT count(*) FROM retail_operations WHERE operation_id='ignored-award'`} {
				var n int
				if e := f.db.QueryRow(q).Scan(&n); e != nil || n != 0 {
					t.Fatal("partial award", n, e)
				}
			}
			if _, e := f.db.Exec(`DROP TRIGGER synthetic_ignore`); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				if e := s.Award(context.Background(), "customer-external", in); e != nil {
					t.Fatal(e)
				}
			}
			out, e := s.Points(context.Background(), "customer-external")
			if e != nil || out.Balance != 6396 || len(out.Entries) != 2 {
				t.Fatalf("retry duplicated award: %+v %v", out, e)
			}
		})
	}
}

func TestRetailLoyaltyIgnoredPolicyConsentAndAnonymization(t *testing.T) {
	for _, action := range []string{"policy", "consent", "anonymize"} {
		t.Run(action, func(t *testing.T) {
			f, s := loyaltySetup(t)
			ctx := context.Background()
			table, event := "retail_customers", "UPDATE"
			if action == "policy" {
				table, event = "retail_config", "INSERT"
			}
			if action == "consent" {
				event = "INSERT"
			}
			if _, e := f.db.Exec("CREATE TRIGGER synthetic_ignore BEFORE " + event + " ON " + table + " BEGIN SELECT RAISE(IGNORE); END"); e != nil {
				t.Fatal(e)
			}
			var e error
			switch action {
			case "policy":
				e = s.Configure(ctx, "ignored-policy", retail.Config{Revision: 2, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}})
			case "consent":
				e = s.SaveCustomer(ctx, "ignored-consent", retail.Customer{ID: "customer-external", Revision: 2, Name: "Synthetic", ConsentReference: "explicit-revoke", LoyaltyConsent: false})
			case "anonymize":
				_, e = s.Privacy(ctx, "ignored-anonymize", "customer-external", 2, true)
			}
			if !errors.Is(e, retail.ErrConflict) {
				t.Fatalf("required %s write ignored: %v", action, e)
			}
			var n int
			if e = f.db.QueryRow(`SELECT count(*) FROM retail_operations WHERE operation_id=?`, "ignored-"+action).Scan(&n); e != nil || n != 0 {
				t.Fatal("audit without mutation", n, e)
			}
			out, e := s.Points(ctx, "customer-external")
			if e != nil || out.Balance != 3198 {
				t.Fatal("failed command changed policy/customer", e)
			}
			if _, e = f.db.Exec(`DROP TRIGGER synthetic_ignore`); e != nil {
				t.Fatal(e)
			}
			switch action {
			case "policy":
				e = s.Configure(ctx, "ignored-policy", retail.Config{Revision: 2, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}})
			case "consent":
				e = s.SaveCustomer(ctx, "ignored-consent", retail.Customer{ID: "customer-external", Revision: 2, Name: "Synthetic", ConsentReference: "explicit-revoke", LoyaltyConsent: false})
			case "anonymize":
				_, e = s.Privacy(ctx, "ignored-anonymize", "customer-external", 2, true)
			}
			if e != nil {
				t.Fatal("retry", e)
			}
			if _, e = s.Points(ctx, "customer-external"); !errors.Is(e, retail.ErrDisabled) {
				t.Fatal("revoke ignored", e)
			}
			if e = f.db.QueryRow(`SELECT count(*) FROM retail_points`).Scan(&n); e != nil || n != 1 {
				t.Fatal("ledger removed", n, e)
			}
		})
	}
}

func TestRetailLoyaltyIgnoredReversalBlocksRedemption(t *testing.T) {
	f, s := loyaltySetup(t)
	ctx := context.Background()
	cfg := retail.Config{Revision: 2, LoyaltyEnabled: true, RefundEnabled: true, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}, EarnCents: 100, EarnPoints: 1, RedeemPoints: 1, RedeemCents: 1, ValidDays: 30}
	if e := s.Configure(ctx, "reversal-policy", cfg); e != nil {
		t.Fatal(e)
	}
	var item string
	if e := f.db.QueryRow(`SELECT id FROM sale_items WHERE sale_id='sale-one'`).Scan(&item); e != nil {
		t.Fatal(e)
	}
	if e := s.RefundSale(ctx, "sale-one", retail.RefundInput{OperationID: "partial-refund", ItemID: item, QuantityMilli: 1000, ExpectedCents: 1599, SessionID: "turn-one", Reason: "Synthetic"}); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`CREATE TRIGGER synthetic_ignore BEFORE INSERT ON retail_points WHEN NEW.kind='reverse' BEGIN SELECT RAISE(IGNORE); END`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Points(ctx, "customer-external"); !errors.Is(e, retail.ErrConflict) {
		t.Fatal("unreconciled balance", e)
	}
	in := retail.PointsInput{OperationID: "after-refund", Points: 2000, ExpectedBalance: 3198, Confirmed: true}
	if e := s.Redeem(ctx, "customer-external", in); !errors.Is(e, retail.ErrConflict) {
		t.Fatal("unreconciled redemption", e)
	}
	var n int
	if e := f.db.QueryRow(`SELECT count(*) FROM retail_point_redemptions`).Scan(&n); e != nil || n != 0 {
		t.Fatal("benefit committed", n, e)
	}
	if _, e := f.db.Exec(`DROP TRIGGER synthetic_ignore`); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		out, e := s.Points(ctx, "customer-external")
		if e != nil || out.Balance != 1599 || len(out.Entries) != 2 {
			t.Fatal("original policy or reversal lost", out, e)
		}
	}
}

func TestRetailLoyaltyConcurrentIdenticalRedemption(t *testing.T) {
	_, s := loyaltySetup(t)
	ctx := context.Background()
	in := retail.PointsInput{OperationID: "same-redemption", Points: 3198, ExpectedBalance: 3198, Confirmed: true}
	const n = 6
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Redeem(ctx, "customer-external", in) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal("identical replay", e)
		}
	}
	out, e := s.Points(ctx, "customer-external")
	if e != nil || out.Balance != 0 || len(out.Entries) != 2 || len(out.Benefits) != 1 {
		t.Fatal("duplicate entitlement or debit", out, e)
	}
	var count int
	if e = s.DB.QueryRow(`SELECT count(*) FROM retail_operations WHERE operation_id=?`, in.OperationID).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate receipt", count, e)
	}
}
