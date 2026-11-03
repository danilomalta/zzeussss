package localapi

// Optional screen-driven acceptance fixture. It starts with an empty business
// workflow. Provisioning only creates identities, a licensed catalog and five
// units of initial stock. Every sale/order/receipt/payment must come from UI.
import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/register"
	"titansystem-backend/internal/localdb/stock"
)

var pilotScreenResume = flag.Bool("pilot-screen-resume", false, "Resume only the private preserved disposable screen fixture")

var pilotScreenDir = flag.String("pilot-screen-dir", "", "Explicit private directory to run the real UI walkthrough; empty skips")

func TestPilotCashScreenFlow(t *testing.T) {
	if *pilotScreenDir == "" {
		t.Skip("explicit disposable screen walkthrough only")
	}
	dir := *pilotScreenDir
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("private 0700 directory required")
	}
	f := stockHTTPSetup(t)
	var n int
	var name, dbPath string
	if *pilotScreenResume {
		// Resume only our explicit private test fixture after a browser timeout.
		raw, err := os.ReadFile(filepath.Join(dir, "review.json"))
		if err != nil {
			t.Fatal(err)
		}
		var info struct {
			TestDataOnly bool   `json:"test_data_only"`
			IdentityID   string `json:"identity_id"`
			ProductID    string `json:"product_id"`
			LocationID   string `json:"location_id"`
		}
		if json.Unmarshal(raw, &info) != nil || !info.TestDataOnly || info.IdentityID == "" {
			t.Fatal("discardable fixture descriptor required")
		}
		f.db.Close()
		dbPath = filepath.Join(t.TempDir(), "resumed-screen.sqlite")
		snapshotPath := filepath.Join(dir, "preserved-screen.sqlite")
		snapshotInfo, err := os.Lstat(snapshotPath)
		if err != nil || !snapshotInfo.Mode().IsRegular() || snapshotInfo.Mode().Perm() != 0600 {
			t.Fatal("regular private disposable snapshot required")
		}
		in, err := os.Open(snapshotPath)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()
		out, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.Copy(out, in); err != nil {
			t.Fatal(err)
		}
		if err = out.Close(); err != nil {
			t.Fatal(err)
		}
		f.db, err = localdb.Open(context.Background(), dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if err = f.db.QueryRow(`SELECT tenant_id,store_id,id FROM devices`).Scan(&f.owner.TenantID, &f.owner.StoreID, &f.owner.DeviceID); err != nil {
			t.Fatal(err)
		}
		f.owner.OwnerID = info.IdentityID
		f.product = info.ProductID
		f.shelf = info.LocationID
		f.device.TenantID = f.owner.TenantID
		f.device.StoreID = f.owner.StoreID
		f.device.DeviceID = f.owner.DeviceID
		verifier, private := testIssuer(t)
		f.private = private
		f.app, err = NewWithVerifier(f.db, f.device, verifier)
		if err != nil {
			t.Fatal(err)
		}
		f.token = loginToken(t, f.app, f.owner.OwnerID)
		// New ephemeral test issuer, same modules; business data is never reconstructed.
		f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 101, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Orders, modules.Logistics, modules.Finance}), 200)
		t.Log("Resumed original screen-created records from consistent disposable snapshot; only test license renewed")
	} else {
		f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 100, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Orders, modules.Logistics, modules.Finance}), 200)
		policyPerson(t, f.db, f.owner, "pilot-reviewer", "manager")
		stockRequest(t, f, stock.Input{OperationID: "screen-initial-stock", Kind: "entry", ProductID: f.product, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Saldo inicial descartavel"}, 201)
		if e = f.db.QueryRow(`PRAGMA database_list`).Scan(&n, &name, &dbPath); e != nil {
			t.Fatal(e)
		}
	}
	if e = f.db.Close(); e != nil {
		t.Fatal(e)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	defer tr.CloseIdleConnections()
	p := &pilotHTTP{t: t, service: pilotStart(t, dbPath, f), client: &http.Client{Transport: tr, Timeout: 15 * time.Second}, token: f.token}
	defer func() { p.service.stop(t) }()
	publish := func(phase string) {
		t.Helper()
		raw, e := json.Marshal(map[string]any{"test_data_only": true, "url": p.service.url, "identity_id": f.owner.OwnerID, "reviewer_id": "pilot-reviewer", "password": "senha-forte-de-teste-1", "product_id": f.product, "location_id": f.shelf, "phase": phase})
		if e != nil {
			t.Fatal(e)
		}
		tmp := filepath.Join(dir, "review-next.json")
		file, e := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = file.Write(raw); e != nil {
			file.Close()
			t.Fatal(e)
		}
		if e = file.Close(); e != nil {
			t.Fatal(e)
		}
		if e = os.Rename(tmp, filepath.Join(dir, "review.json")); e != nil {
			t.Fatal(e)
		}
	}
	wait := func(marker string) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Minute)
		for time.Now().Before(deadline) {
			if _, e := os.Stat(filepath.Join(dir, marker)); e == nil {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatal("screen walkthrough did not complete: " + marker)
	}
	publish("empty_workflow")
	t.Log("Real API ready: waiting for screen workflow completion and DONE marker; resume mode:", *pilotScreenResume)
	wait("DONE")
	db, e := localdb.Open(context.Background(), dbPath)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, c := range []struct {
		sql  string
		want int64
	}{
		{`SELECT COUNT(*) FROM sales`, 1}, {`SELECT SUM(total_cents) FROM sales WHERE status='committed'`, 3198}, {`SELECT COUNT(*) FROM sale_payments WHERE method='cash' AND status='confirmed'`, 1},
		{`SELECT COUNT(*) FROM cash_movements`, 2}, {`SELECT SUM(amount_cents) FROM cash_movements`, 1148}, {`SELECT SUM(quantity_milli) FROM stock_movements`, 5000},
		{`SELECT COUNT(*) FROM purchase_orders`, 1}, {`SELECT SUM(quantity_milli) FROM purchase_order_items`, 2000}, {`SELECT COUNT(*) FROM purchase_receipts`, 1},
		{`SELECT COUNT(*) FROM supplier_payables WHERE state='approved' AND total_cents=2050`, 1}, {`SELECT COUNT(*) FROM payable_cash_payments WHERE amount_cents=2050`, 1},
	} {
		var got int64
		if e = db.QueryRow(c.sql).Scan(&got); e != nil || got != c.want {
			t.Fatalf("screen outcome %s: got%d want%d err%v", c.sql, got, c.want, e)
		}
	}
	ids := map[string]string{}
	for _, table := range []string{"sales", "cash_sessions", "purchase_orders", "purchase_deliveries", "supplier_payables"} {
		var id string
		if e = db.QueryRow("SELECT id FROM " + table).Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids[table] = id
	}
	paths := []string{"/local/v1/sales/" + ids["sales"], "/local/v1/cash/sessions/" + ids["cash_sessions"] + "/report", "/local/v1/purchase-orders/" + ids["purchase_orders"] + "/trace", "/local/v1/purchase-orders/" + ids["purchase_orders"] + "/supply-cycle?limit=50&offset=0", "/local/v1/purchase-deliveries/" + ids["purchase_deliveries"], "/local/v1/finance/payables/" + ids["supplier_payables"], "/local/v1/finance/receiving-origins/" + ids["supplier_payables"]}
	p.token = p.login(f.owner.OwnerID)
	snapshot := func() map[string]any {
		out := map[string]any{}
		for _, path := range paths {
			var v any
			p.call("GET", path, nil, 200, &v)
			out[path] = v
		}
		return out
	}
	var closed register.ClosedReport
	p.call("GET", paths[1], nil, 200, &closed)
	if closed.Cash.ExpectedCents != 2148 || closed.Cash.DifferenceCents != 0 {
		t.Fatal("closure not reconciled", closed.Cash)
	}
	var payable payables.Payable
	p.call("GET", paths[5], nil, 200, &payable)
	if payable.Cash == nil || payable.Cash.AppliedCents != 2050 || payable.Cash.OutstandingCents != 0 {
		t.Fatal("UI payable not fully paid")
	}
	want := snapshot()
	p.service.stop(t)
	p.service = pilotStart(t, dbPath, f)
	if !reflect.DeepEqual(want, snapshot()) {
		t.Fatal("UI workflow lost across restart")
	}
	key := make([]byte, 32)
	if _, e = rand.Read(key); e != nil {
		t.Fatal(e)
	}
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "ui-flow.tytbak")
	restore := filepath.Join(tmp, "restored.sqlite")
	ctx := context.Background()
	if e = backup.Create(ctx, db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	if e = backup.Verify(ctx, archive, f.device, key); e != nil {
		t.Fatal(e)
	}
	if e = backup.Restore(ctx, archive, restore, f.device, key); e != nil {
		t.Fatal(e)
	}
	p.service.stop(t)
	p.service = pilotStart(t, restore, f)
	p.call("GET", "/local/v1/me", nil, 401, nil)
	p.token = p.login(f.owner.OwnerID)
	if !reflect.DeepEqual(want, snapshot()) {
		t.Fatal("UI workflow lost after backup restore")
	}
	publish("restored_workflow")
	t.Log("Screen-originated workflow persisted, restarted and restored; waiting for final screen reread and FINISH marker")
	wait("FINISH")
	t.Log("PASS: original UI-originated sale/order/receipt/payment/closure preserved across restart and encrypted backup restoration; no workflow POST from acceptance driver")
}
