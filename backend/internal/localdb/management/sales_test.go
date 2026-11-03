package management

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stock"
)

type salesFixture struct {
	db *sql.DB
	a  identity.Scope
	d  identity.DeviceContext
}

func setupSales(t *testing.T) *salesFixture {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "reports.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{
		`INSERT INTO tenants VALUES('t','Company','now'),('foreign','Other company','now')`,
		`INSERT INTO stores VALUES('t','s','Store'),('t','other','Second'),('foreign','s','Foreign')`,
		`INSERT INTO devices VALUES('t','s','d','Device'),('t','other','d2','Second'),('foreign','s','df','Foreign')`,
		`INSERT INTO identities VALUES('owner','Owner','now'),('worker','Worker','now')`,
		`INSERT INTO memberships VALUES('t','owner','owner','active','now'),('t','worker','stock','active','now')`,
		`INSERT INTO membership_stores VALUES('t','worker','s')`,
		`INSERT INTO users VALUES('t','owner','Owner'),('foreign','owner','Foreign')`,
		`INSERT INTO products(tenant_id,id,sku,name,price_cents,unit) VALUES('t','a','A','=danger',100,'unit'),('t','b','B','B',100,'unit'),('t','c','C','C',100,'unit'),('t','d','D','Old',100,'unit'),('foreign','a','A','Foreign',100,'unit')`,
		`INSERT INTO cash_sessions VALUES('t','s','d','cash','owner','now',NULL,0),('t','other','d2','cash2','owner','now',NULL,0),('foreign','s','df','cashf','owner','now',NULL,0)`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	_, err = db.Exec(`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by) VALUES('t','s','d',?,?,9999999999,'approved','owner')`, make([]byte, 32), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return &salesFixture{db, identity.Scope{IdentityID: "owner", TenantID: "t", StoreID: "s"}, identity.DeviceContext{TenantID: "t", StoreID: "s", DeviceID: "d"}}
}
func (f *salesFixture) add(t *testing.T, id, tenant, store, device, cash, product, stamp, status string, q, c int64) {
	t.Helper()
	_, err := f.db.Exec(`INSERT INTO sales(tenant_id,store_id,device_id,id,cash_session_id,status,total_cents,committed_at) VALUES(?,?,?,?,?,?,?,?)`, tenant, store, device, id, cash, status, c, stamp)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.db.Exec(`INSERT INTO sale_items VALUES(?,?,?,?,?,?,?)`, tenant, id, id, product, q, 100, c)
	if err != nil {
		t.Fatal(err)
	}
}
func window(a, b string) Window {
	from, _ := time.Parse(time.RFC3339Nano, a)
	to, _ := time.Parse(time.RFC3339Nano, b)
	return Window{from, to}
}
func input() SalesInput {
	return SalesInput{Current: window("2026-10-02T00:00:00Z", "2026-10-03T00:00:00Z"), Previous: window("2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z"), Limit: 100}
}
func TestManagementSalesABCComparisonScopeAndPageTotals(t *testing.T) {
	f := setupSales(t)
	for _, v := range []struct {
		id, p, stamp string
		c            int64
	}{{"a", "a", "2026-10-02T10:00:00Z", 8000}, {"b", "b", "2026-10-02T11:00:00Z", 1500}, {"c", "c", "2026-10-02T12:00:00Z", 500}, {"p", "a", "2026-10-01T12:00:00Z", 4000}, {"old", "d", "2026-10-01T13:00:00Z", 10}} {
		f.add(t, v.id, "t", "s", "d", "cash", v.p, v.stamp, "committed", 1000, v.c)
	}
	f.add(t, "cancel", "t", "s", "d", "cash", "a", "2026-10-02T12:00:00Z", "cancelled", 1000, 9999)
	f.add(t, "other", "t", "other", "d2", "cash2", "a", "2026-10-02T12:00:00Z", "committed", 1000, 9999)
	f.add(t, "foreign", "foreign", "s", "df", "cashf", "a", "2026-10-02T12:00:00Z", "committed", 1000, 9999)
	in := input()
	out, err := Sales(context.Background(), f.db, f.a, f.d, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 4 || out.RevenueCents != 10000 || out.PreviousRevenueCents != 4010 || out.Items[0].ABCCategory != "A" || out.Items[1].ABCCategory != "B" || out.Items[2].ABCCategory != "C" || out.Items[3].ABCCategory != "no_current_revenue" || out.Items[0].RevenueChangeBasisPoints == nil || *out.Items[0].RevenueChangeBasisPoints != 10000 || out.Items[1].ChangeBasis != "new_activity" || out.Items[1].RevenueChangeBasisPoints != nil || out.PaymentConfirmed || out.Profit {
		t.Fatalf("%+v", out)
	}
	in.Limit = 1
	in.Offset = 1
	p, err := Sales(context.Background(), f.db, f.a, f.d, in)
	if err != nil || len(p.Items) != 1 || p.Items[0].ProductID != "b" || p.RevenueCents != 10000 {
		t.Fatal(p, err)
	}
	var b bytes.Buffer
	if err = ExportSales(&b, out); err != nil || !strings.Contains(b.String(), "'=danger") {
		t.Fatal(b.String(), err)
	}
	in.Previous = window("2026-09-01T00:00:00Z", "2026-10-02T00:00:00Z")
	p, err = Sales(context.Background(), f.db, f.a, f.d, in)
	if err != nil || p.EqualDuration || p.Items[0].RevenueChangeBasisPoints != nil || p.Items[0].ChangeBasis != "different_duration_totals_only" {
		t.Fatal(p, err)
	}
}
func TestManagementSalesNanoBoundaryTimezoneAndOverlap(t *testing.T) {
	f := setupSales(t)
	f.add(t, "begin", "t", "s", "d", "cash", "a", "2026-10-01T21:00:00-03:00", "committed", 1000, 100)
	f.add(t, "last", "t", "s", "d", "cash", "a", "2026-10-02T23:59:59.999999999Z", "committed", 1000, 200)
	f.add(t, "excluded", "t", "s", "d", "cash", "a", "2026-10-03T00:00:00Z", "committed", 1000, 400)
	in := input()
	in.Previous = in.Current
	out, err := Sales(context.Background(), f.db, f.a, f.d, in)
	if err != nil || out.RevenueCents != 300 || out.PreviousRevenueCents != 300 || *out.Items[0].RevenueChangeBasisPoints != 0 {
		t.Fatal(out, err)
	}
}
func TestManagementSalesPermissionsDeviceBoundsAndOverflow(t *testing.T) {
	f := setupSales(t)
	ctx := context.Background()
	in := input()
	worker := f.a
	worker.IdentityID = "worker"
	if _, err := Sales(ctx, f.db, worker, f.d, in); !errors.Is(err, identity.ErrDenied) {
		t.Fatal(err)
	}
	d := f.d
	d.StoreID = "other"
	if _, err := Sales(ctx, f.db, f.a, d, in); !errors.Is(err, identity.ErrDenied) {
		t.Fatal(err)
	}
	for _, bad := range []SalesInput{{}, {Current: in.Current, Previous: in.Previous, Limit: 101}, {Current: in.Current, Previous: in.Previous, Limit: 1, Offset: -1}, {Current: Window{in.Current.To, in.Current.From}, Previous: in.Previous, Limit: 1}} {
		if _, err := Sales(ctx, f.db, f.a, f.d, bad); !errors.Is(err, ErrInvalid) {
			t.Fatal(bad, err)
		}
	}
	for i := 0; i < 2; i++ {
		f.add(t, fmt.Sprint(i), "t", "s", "d", "cash", "a", "2026-10-02T10:00:00Z", "committed", MaxExact, MaxExact)
	}
	if _, err := Sales(ctx, f.db, f.a, f.d, in); !errors.Is(err, ErrIntegrity) {
		t.Fatal("unsafe aggregate", err)
	}
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked'`); err != nil {
		t.Fatal(err)
	}
	if _, err := Sales(ctx, f.db, f.a, f.d, in); !errors.Is(err, identity.ErrDenied) {
		t.Fatal(err)
	}
}
func TestManagementExactChangeAndWindowLimits(t *testing.T) {
	for _, tc := range []struct{ current, previous, want int64 }{{100, 100, 0}, {0, 100, -10000}, {150, 100, 5000}, {1, 3, -6666}} {
		n, basis := change(tc.current, tc.previous)
		if n == nil || *n != tc.want || basis != "comparable_totals" {
			t.Fatal(tc, n, basis)
		}
	}
	if n, b := change(MaxExact, 1); n != nil || b != "change_out_of_range" {
		t.Fatal(n, b)
	}
	if n, b := change(0, 0); n != nil || b != "no_baseline" {
		t.Fatal(n, b)
	}
	for _, s := range []string{"=cmd", " +cmd", "\t-cmd", "@SUM(A1)"} {
		if safeCell(s) != "'"+s {
			t.Fatal(s)
		}
	}
	if safeCell("ordinary") != "ordinary" {
		t.Fatal("ordinary escaped")
	}
	if _, err := exactInt(new(big.Int).Lsh(big.NewInt(1), 100)); err == nil {
		t.Fatal("overflow")
	}
}

func TestManagementSalesHardSourceLimitAndInvalidStoredDate(t *testing.T) {
	f := setupSales(t)
	f.add(t, "bulk", "t", "s", "d", "cash", "a", "2026-10-02T12:00:00Z", "committed", 1000, 100)
	if _, err := f.db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n<100000) INSERT INTO sale_items SELECT 't','bulk','row-'||n,'a',1000,100,100 FROM seq`); err != nil {
		t.Fatal(err)
	}
	if _, err := Sales(context.Background(), f.db, f.a, f.d, input()); !errors.Is(err, ErrLimit) {
		t.Fatal("report silently truncated", err)
	}
}

func TestManagementSalesMalformedTimestampCannotBecomeEmptySuccess(t *testing.T) {
	f := setupSales(t)
	f.add(t, "bad-date", "t", "s", "d", "cash", "a", "not-a-date", "committed", 1000, 100)
	if _, err := Sales(context.Background(), f.db, f.a, f.d, input()); !errors.Is(err, ErrIntegrity) {
		t.Fatal(err)
	}
}

func TestManagementRestockObservedDemandPolicyAndPhysicalScope(t *testing.T) {
	f := setupSales(t)
	for _, q := range []string{
		`INSERT INTO stock_locations VALUES('t','s','shelf','shelf','Shelf'),('t','s','back','backroom','Back'),('t','other','shelf','shelf','Other')`,
		`INSERT INTO stock_movements VALUES('one','t','s','d','a','shelf',1000,'opening','now'),('two','t','s','d','a','back',1000,'opening','now'),('other','t','other','d2','a','shelf',90000,'opening','now')`,
		`INSERT INTO restock_policies VALUES('t','s','a',1000,5000,1,'owner','now')`,
	} {
		if _, err := f.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	f.add(t, "one", "t", "s", "d", "cash", "a", "2026-10-01T10:00:00Z", "committed", 6000, 600)
	f.add(t, "other", "t", "other", "d2", "cash2", "a", "2026-10-01T10:00:00Z", "committed", 900000, 90000)
	in := stock.RestockPreviewInput{ProductID: "a", From: "2026-10-01T00:00:00Z", To: "2026-10-03T00:00:00Z", LeadDays: 2, CoverageDays: 3, SafetyMilli: 1000}
	out, err := stock.PreviewRestock(context.Background(), f.db, f.a, f.d, in)
	if err != nil || out.SoldMilli != 6000 || out.DemandNumerator != "3000" || out.DemandDenominator != "1" || out.PhysicalMilli != 2000 || out.SuggestedMinimumMilli != 7000 || out.SuggestedTargetMilli != 16000 || out.SuggestedOrderMilli != 14000 || out.OrderCreated {
		t.Fatal(out, err)
	}
	if _, err := f.db.Exec(`UPDATE restock_policies SET minimum_milli=20000,target_milli=30000`); err != nil {
		t.Fatal(err)
	}
	out, err = stock.PreviewRestock(context.Background(), f.db, f.a, f.d, in)
	if err != nil || out.SuggestedOrderMilli != 28000 || out.SuggestedMinimumMilli != 20000 {
		t.Fatal(out, err)
	}
}
