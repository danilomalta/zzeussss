package localapi

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"strings"
	"testing"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/localdb/localauth"
)

// Exercise the real handlers with a session genuinely resolved BEFORE revocation
// or credential/context replacement. This deterministic seam models the interval
// after requireSession and before the business transaction; no production route
// or authentication bypass is installed. All business effects use real SQLite.
func TestHTTPCheckoutRejectsSessionInvalidatedAfterAdmission(t *testing.T) {
	for _, mode := range []string{"revoked", "foreign-session"} {
		for _, action := range []string{"prepare", "finalize", "cancel-plan", "legacy-sale"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				f, in := cashPlanSetup(t)
				captured, err := localauth.Resolve(context.Background(), f.db, f.token)
				if err != nil {
					t.Fatal("session fixture failed")
				}
				if mode == "revoked" {
					if err := localauth.Logout(context.Background(), f.db, f.token); err != nil {
						t.Fatal(err)
					}
				} else {
					other, _ := cashPlanSetup(t)
					captured.Token = other.token
				}
				s := &Server{DB: f.db, Device: f.device, contracts: journalLicense(t, f, f.db)}
				app := fiber.New(fiber.Config{ErrorHandler: apicontract.FrameworkError})
				path := finalizeCashPath
				body := finalizeCashBody
				handler := s.finalizeCashCheckoutPlan
				switch action {
				case "prepare":
					in.CheckoutID = "new-after-admission"
					in.OperationID = "new-after-admission"
					path = planPath
					body = planJSON(t, in)
					handler = s.createCheckoutPlan
				case "cancel-plan":
					path = planPath + "/" + cashPlanID + "/cancel"
					body = `{"operation_id":"cancel-after-admission","reason":"explicit operator action"}`
					handler = s.cancelCheckoutPlan
				case "legacy-sale":
					path = "/local/v1/sales"
					body = salesJSON(t, salesInput(f))
					handler = s.completeSale
				}
				route := path
				if action == "finalize" {
					route = planPath + "/:id/finalize-cash"
				}
				if action == "cancel-plan" {
					route = planPath + "/:id/cancel"
				}
				app.Post(route, func(c *fiber.Ctx) error { c.Locals("session", captured); return handler(c) })
				code, _ := request(t, app, "POST", path, body, f.token)
				if code != 401 {
					t.Errorf("stale admitted request returned %d, want 401", code)
				}
				// Inspect through a NEW legitimate login after denying the old
				// admitted request; never reuse the revoked token for assertions.
				if mode == "revoked" {
					f.token = loginToken(t, f.app, f.owner.OwnerID)
				}
				cashFinalCounts(t, f, 0)
				var plans, cancelled int
				if err := f.db.QueryRow(`SELECT count(*) FROM checkout_plans`).Scan(&plans); err != nil || plans != 1 {
					t.Error("stale session created a plan")
				}
				if err := f.db.QueryRow(`SELECT count(*) FROM checkout_plan_cancellations`).Scan(&cancelled); err != nil || cancelled != 0 {
					t.Error("stale session cancelled a plan")
				}
			})
		}
	}
}

func TestHTTPCheckoutIgnoredJournalOrOutboxRollsBack(t *testing.T) {
	for _, stage := range []string{"sale-event", "plan-operation", "plan-event"} {
		t.Run(stage, func(t *testing.T) {
			f, in := cashPlanSetup(t)
			path := finalizeCashPath
			body := finalizeCashBody
			trigger := `CREATE TRIGGER p10_ignore BEFORE INSERT ON outbox WHEN NEW.event_type='sale.committed' BEGIN SELECT RAISE(IGNORE); END`
			if stage != "sale-event" {
				in.CheckoutID = "p10-plan"
				in.OperationID = "p10-plan"
				path = planPath
				body = planJSON(t, in)
				trigger = `CREATE TRIGGER p10_ignore BEFORE INSERT ON checkout_plan_operations WHEN NEW.operation_id='p10-plan' BEGIN SELECT RAISE(IGNORE); END`
				if stage == "plan-event" {
					trigger = `CREATE TRIGGER p10_ignore BEFORE INSERT ON outbox WHEN NEW.aggregate_id='p10-plan' AND NEW.event_type='checkout.plan.created' BEGIN SELECT RAISE(IGNORE); END`
				}
			}
			if _, err := f.db.Exec(trigger); err != nil {
				t.Fatal("fault fixture failed")
			}
			code, _ := request(t, f.app, "POST", path, body, f.token)
			if code != 409 {
				t.Errorf("missing durable journal/event accepted: %d", code)
			}
			cashFinalCounts(t, f, 0)
			var n int
			if err := f.db.QueryRow(`SELECT count(*) FROM checkout_plans WHERE checkout_id='p10-plan'`).Scan(&n); err != nil || n != 0 {
				t.Error("partial plan committed")
			}
			if _, err := f.db.Exec(`DROP TRIGGER p10_ignore`); err != nil {
				t.Fatal(err)
			}
			code, _ = request(t, f.app, "POST", path, body, f.token)
			if code != 201 {
				t.Fatal("operation could not resume", code)
			}
			if stage == "sale-event" {
				cashFinalCounts(t, f, 1)
			} else if !strings.Contains(body, "p10-plan") {
				t.Fatal("invalid plan fixture")
			}
		})
	}
}
