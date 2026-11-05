package localapi

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/localdb/localauth"
)

// A test-only admission seam captures a genuine session before logout/context
// replacement; real handlers and real SQLite must recheck it in the transaction.
// No authentication seam or additional route is installed in the product.
func TestHTTPPayablesRejectSessionInvalidatedAfterAdmission(t *testing.T) {
	for _, mode := range []string{"revoked", "foreign-session"} {
		for _, action := range []string{"create", "approve", "reject", "cancel", "pay", "return"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				f, reviewer, _ := pcSetup(t)
				s := &Server{DB: f.db, Device: f.device, contracts: retailServiceFor(f).Contracts}
				path, route, body, token := payablesPath, payablesPath, payableBody, f.token
				var handler fiber.Handler = s.createPayable
				switch action {
				case "create":
					body = strings.NewReplacer("create-invoice", "p12-create", "invoice-one", "p12-invoice", "NFe-123", "P12-document").Replace(payableBody)
				case "approve", "reject", "cancel":
					id := "invoice-one"
					if action != "cancel" {
						id = "p12-pending"
						pending := strings.NewReplacer("create-invoice", "p12-pending-create", "invoice-one", id, "NFe-123", "P12-pending").Replace(payableBody)
						payableResult(t, f, f.token, "POST", payablesPath, pending, 201)
					}
					path, route, body, token = payablesPath+"/"+id+"/"+action, payablesPath+"/:id/"+action, `{"operation_id":"p12-change","reason":"Explicit review"}`, reviewer
					handler = func(c *fiber.Ctx) error { return s.changePayable(c, action) }
				case "pay", "return":
					pcMove(t, f, f.token, "withdraw-one", "withdrawal", 6000)
					path, route, body = payablesPath+"/invoice-one/cash-payments", payablesPath+"/:id/cash-payments", pcPay
					handler = s.recordPayableCash
					if action == "return" {
						pcResult(t, f, f.token, "POST", path, body, 201)
						pcMove(t, f, f.token, "return-supply", "supply", 6000)
						path, route, body, handler = pcPath+"/payment-one/return", pcPath+"/:id/return", pcReturn, s.returnPayableCash
					}
				}
				captured, err := localauth.Resolve(context.Background(), f.db, token)
				if err != nil {
					t.Fatal("session fixture failed")
				}
				before := pcCounts(t, f)
				states := func() string {
					t.Helper()
					var v string
					if err := f.db.QueryRow(`SELECT group_concat(id||':'||state||':'||last_operation_id,'|') FROM (SELECT id,state,last_operation_id FROM supplier_payables ORDER BY id)`).Scan(&v); err != nil {
						t.Fatal(err)
					}
					return v
				}
				priorStates := states()
				if mode == "revoked" {
					if err := localauth.Logout(context.Background(), f.db, token); err != nil {
						t.Fatal(err)
					}
				} else {
					foreign := httpContractSetup(t)
					captured.Token = foreign.token
				}
				app := fiber.New(fiber.Config{ErrorHandler: apicontract.FrameworkError})
				app.Post(route, func(c *fiber.Ctx) error { c.Locals("session", captured); return handler(c) })
				if code, _ := request(t, app, "POST", path, body, token); code != 401 {
					t.Errorf("stale admitted command accepted: %d", code)
				}
				if !reflect.DeepEqual(before, pcCounts(t, f)) || priorStates != states() {
					t.Error("stale session changed financial state or journals")
				}
			})
		}
	}
}

func TestHTTPReceivingPayableRejectSessionInvalidatedAfterAdmission(t *testing.T) {
	for _, mode := range []string{"revoked", "foreign-session"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := cycleFixture(t)
			cycleReceived(t, f)
			captured, err := localauth.Resolve(context.Background(), f.db, f.token)
			if err != nil {
				t.Fatal(err)
			}
			before := cycleCounts(t, f)
			if mode == "revoked" {
				if err := localauth.Logout(context.Background(), f.db, f.token); err != nil {
					t.Fatal(err)
				}
			} else {
				captured.Token = httpContractSetup(t).token
			}
			s := &Server{DB: f.db, Device: f.device, contracts: retailServiceFor(f).Contracts}
			app := fiber.New(fiber.Config{ErrorHandler: apicontract.FrameworkError})
			app.Post("/local/v1/purchase-deliveries/:id/payable", func(c *fiber.Ctx) error { c.Locals("session", captured); return s.cycleFinance(c) })
			raw, err := json.Marshal(cycleFinanceInput())
			if err != nil {
				t.Fatal(err)
			}
			if code, _ := request(t, app, "POST", cycleDelivery+"/payable", string(raw), f.token); code != 401 {
				t.Errorf("stale receiving obligation accepted: %d", code)
			}
			if !reflect.DeepEqual(before, cycleCounts(t, f)) {
				t.Fatal("stale session changed receiving, stock, obligation or cash")
			}
		})
	}
}
