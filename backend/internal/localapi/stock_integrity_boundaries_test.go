package localapi

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v2"
	"reflect"
	"testing"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/retail"
	"titansystem-backend/internal/localdb/stocktaking"
)

func TestRetailTransferIgnoredWriteRollsBack(t *testing.T) {
	for _, stage := range []string{"source", "destination", "binding"} {
		t.Run(stage, func(t *testing.T) {
			f, s := storesSetup(t)
			in := transferInput("p13-transfer")
			in.ProductID = f.product
			in.FromLocationID = f.shelf
			table, condition := "stock_movements", "NEW.store_id='"+f.owner.StoreID+"'"
			if stage == "destination" {
				condition = "NEW.store_id='store-two'"
			}
			if stage == "binding" {
				table, condition = "retail_store_transfers", "1=1"
			}
			if _, e := f.db.Exec(`CREATE TRIGGER p13_ignore BEFORE INSERT ON ` + table + ` WHEN ` + condition + ` BEGIN SELECT RAISE(IGNORE); END`); e != nil {
				t.Fatal(e)
			}
			if e := s.Transfer(context.Background(), in); !errors.Is(e, retail.ErrConflict) {
				t.Errorf("missing transfer leg/binding accepted: %v", e)
			}
			var source, target, bindings, audits int64
			if e := f.db.QueryRow(`SELECT COALESCE(sum(quantity_milli),0) FROM stock_movements WHERE location_id=?`, f.shelf).Scan(&source); e != nil {
				t.Fatal(e)
			}
			if e := f.db.QueryRow(`SELECT COALESCE(sum(quantity_milli),0) FROM stock_movements WHERE store_id='store-two'`).Scan(&target); e != nil {
				t.Fatal(e)
			}
			if e := f.db.QueryRow(`SELECT count(*) FROM retail_store_transfers`).Scan(&bindings); e != nil {
				t.Fatal(e)
			}
			if e := f.db.QueryRow(`SELECT count(*) FROM retail_operations WHERE operation_id='p13-transfer'`).Scan(&audits); e != nil {
				t.Fatal(e)
			}
			if source != 5000 || target != 0 || bindings != 0 || audits != 0 {
				t.Errorf("partial transfer: %d %d %d %d", source, target, bindings, audits)
			}
			if _, e := f.db.Exec(`DROP TRIGGER p13_ignore`); e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 2; i++ {
				if e := s.Transfer(context.Background(), in); e != nil {
					t.Fatal(e)
				}
			}
			assertStockBalance(t, f, f.shelf, 4000)
			if e := f.db.QueryRow(`SELECT COALESCE(sum(quantity_milli),0) FROM stock_movements WHERE store_id='store-two'`).Scan(&target); e != nil || target != 1000 {
				t.Fatal("replay duplicated destination", target, e)
			}
		})
	}
}

func TestHTTPStockIgnoredWriteRollsBack(t *testing.T) {
	for _, table := range []string{"stock_operations", "stock_movements", "stock_operation_movements", "outbox"} {
		t.Run(table, func(t *testing.T) {
			f := stockHTTPSetup(t)
			in := stockEntry(f, "p13-entry", 1000)
			if _, e := f.db.Exec(`CREATE TRIGGER p13_ignore BEFORE INSERT ON ` + table + ` BEGIN SELECT RAISE(IGNORE); END`); e != nil {
				t.Fatal(e)
			}
			code, _ := request(t, f.app, "POST", "/local/v1/stock/operations", inventoryHTTPJSON(t, in), f.token)
			if code != 409 {
				t.Errorf("incomplete movement accepted: %d", code)
			}
			for _, q := range []string{`SELECT count(*) FROM stock_operations`, `SELECT count(*) FROM stock_movements`, `SELECT count(*) FROM stock_operation_movements`, `SELECT count(*) FROM outbox WHERE event_type='stock.operation'`} {
				var n int
				if e := f.db.QueryRow(q).Scan(&n); e != nil || n != 0 {
					t.Errorf("partial movement: %d %v", n, e)
				}
			}
			if _, e := f.db.Exec(`DROP TRIGGER p13_ignore`); e != nil {
				t.Fatal(e)
			}
			stockRequest(t, f, in, 201)
			stockRequest(t, f, in, 200)
		})
	}
}

// Test-only seam captures a real admitted session before revocation/context
// replacement. All business work is performed by real handlers and SQLite.
func TestHTTPInventoryRejectsSessionInvalidatedAfterAdmission(t *testing.T) {
	for _, mode := range []string{"revoked", "foreign-session"} {
		for _, action := range []string{"movement", "open", "count", "state"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				f := stockHTTPSetup(t)
				s := &Server{DB: f.db, Device: f.device, contracts: retailServiceFor(f.httpContractFixture).Contracts}
				path, route := "/local/v1/stock/operations", "/local/v1/stock/operations"
				body := inventoryHTTPJSON(t, stockEntry(f, "p13-entry", 1000))
				var handler fiber.Handler = s.recordStock
				if action != "movement" {
					path, route = "/local/v1/stock/inventories", "/local/v1/stock/inventories"
					body = inventoryHTTPJSON(t, stocktaking.OpenInput{OperationID: "p13-open", ID: "new-inv", LocationID: f.shelf, Title: "Synthetic count", Items: []stocktaking.Item{{ProductID: f.product, Unit: "unit"}}})
					handler = s.openReviewedInventory
					if action != "open" {
						inventoryHTTPOpen(t, f, f.token)
						count := stocktaking.CountInput{OperationID: "p13-count", ExpectedRevision: 1, Items: []stocktaking.CountItem{{ProductID: f.product, CountedMilli: 1000}}}
						path, route, body, handler = "/local/v1/stock/inventories/inv/counts", "/local/v1/stock/inventories/:id/counts", inventoryHTTPJSON(t, count), s.countReviewedInventory
						if action == "state" {
							code, _ := request(t, f.app, "POST", path, body, f.token)
							if code != 200 {
								t.Fatal("count fixture", code)
							}
							path, route, body, handler = "/local/v1/stock/inventories/inv/state", "/local/v1/stock/inventories/:id/state", inventoryHTTPJSON(t, stocktaking.StateInput{OperationID: "p13-submit", ExpectedRevision: 2, Action: "submit", Reason: "Count complete"}), s.stateReviewedInventory
						}
					}
				}
				snapshot := func() []int64 {
					t.Helper()
					out := []int64{}
					for _, q := range []string{`SELECT count(*) FROM stock_movements`, `SELECT count(*) FROM stock_operations`, `SELECT count(*) FROM reviewed_inventory_receipts`, `SELECT COALESCE(sum(revision),0) FROM reviewed_inventory_sessions`, `SELECT count(*) FROM outbox WHERE event_type IN ('stock.operation','inventory.reviewed')`} {
						var n int64
						if e := f.db.QueryRow(q).Scan(&n); e != nil {
							t.Fatal(e)
						}
						out = append(out, n)
					}
					return out
				}
				before := snapshot()
				captured, e := localauth.Resolve(context.Background(), f.db, f.token)
				if e != nil {
					t.Fatal(e)
				}
				if mode == "revoked" {
					if e := localauth.Logout(context.Background(), f.db, f.token); e != nil {
						t.Fatal(e)
					}
				} else {
					captured.Token = httpContractSetup(t).token
				}
				app := fiber.New(fiber.Config{ErrorHandler: apicontract.FrameworkError})
				app.Post(route, func(c *fiber.Ctx) error { c.Locals("session", captured); return handler(c) })
				if code, _ := request(t, app, "POST", path, body, f.token); code != 401 {
					t.Errorf("stale admitted stock command accepted: %d", code)
				}
				if !reflect.DeepEqual(before, snapshot()) {
					t.Error("stale command changed stock/inventory/outbox")
				}
			})
		}
	}
}
