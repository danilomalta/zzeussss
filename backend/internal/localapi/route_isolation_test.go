package localapi

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

// Enumerate actual Fiber routes. A new protected route automatically joins the
// authentication/isolation checks instead of depending on a hand-maintained list.
func protectedRoutes(t *testing.T, app *fiber.App) []fiber.Route {
	t.Helper()
	var routes []fiber.Route
	for _, r := range app.GetRoutes(true) {
		if !strings.HasPrefix(r.Path, "/local/v1/") || r.Method == "HEAD" || r.Path == "/local/v1/health" || r.Path == "/local/v1/login" || r.Path == "/local/v1/account/recover" {
			continue
		}
		routes = append(routes, r)
	}
	if len(routes) < 30 {
		t.Fatalf("unexpected protected route inventory: %d", len(routes))
	}
	return routes
}
func concretePath(pattern string) string {
	parts := strings.Split(pattern, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "nonexistent"
		}
	}
	return strings.Join(parts, "/")
}
func isolationCounts(t *testing.T, db *sql.DB) []int {
	t.Helper()
	var out []int
	for _, table := range []string{"products", "stock_movements", "cash_sessions", "sales", "outbox", "memberships", "staff_registrations", "access_policy_events", "account_access_operations", "incoming_events"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}
func TestEveryProtectedLocalRouteRejectsForeignOrRevokedSession(t *testing.T) {
	for _, scenario := range []string{"anonymous", "foreign_tenant", "foreign_store_device", "foreign_device", "revoked_user", "revoked_session", "revoked_device", "removed_store_link"} {
		t.Run(scenario, func(t *testing.T) {
			db, app, o := fixture(t)
			token := ""
			device := identity.DeviceContext{TenantID: o.TenantID, StoreID: o.StoreID, DeviceID: o.DeviceID}
			if scenario == "foreign_tenant" || scenario == "foreign_store_device" || scenario == "foreign_device" {
				tenant := o.TenantID
				store := "other-store"
				deviceID := "other-device"
				user := o.OwnerID
				if scenario == "foreign_tenant" {
					tenant = "foreign"
					user = "foreign-owner"
					for _, q := range []string{`INSERT INTO tenants VALUES ('foreign','Foreign','now')`, `INSERT INTO identities VALUES ('foreign-owner','Foreign','now')`, `INSERT INTO memberships VALUES ('foreign','foreign-owner','owner','active','now')`} {
						if _, err := db.Exec(q); err != nil {
							t.Fatal(err)
						}
					}
					hash, err := localauth.HashPassword("foreign-test-password")
					if err != nil {
						t.Fatal(err)
					}
					if _, err := db.Exec(`INSERT INTO local_passwords VALUES ('foreign','foreign-owner',?,'now')`, hash); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "foreign_device" {
					store = o.StoreID
				}
				if scenario != "foreign_device" {
					if _, err := db.Exec(`INSERT INTO stores VALUES (?,?,'Other')`, tenant, store); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := db.Exec(`INSERT INTO devices VALUES (?,?,?,'Other')`, tenant, store, deviceID); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`INSERT INTO device_pairings VALUES (?,?,?,zeroblob(32),zeroblob(32),9999999999,'approved',?,1,?,1,NULL,NULL)`, tenant, store, deviceID, user, user); err != nil {
					t.Fatal(err)
				}
				password := "senha-forte-de-teste-1"
				if scenario == "foreign_tenant" {
					password = "foreign-test-password"
				}
				session, err := localauth.Login(context.Background(), db, identity.DeviceContext{TenantID: tenant, StoreID: store, DeviceID: deviceID}, user, password)
				if err != nil {
					t.Fatal(err)
				}
				token = session.Token
			} else if scenario != "anonymous" {
				user := o.OwnerID
				if scenario == "removed_store_link" {
					policyPerson(t, db, o, "employee", "employee")
					user = "employee"
				}
				session, err := localauth.Login(context.Background(), db, device, user, "senha-forte-de-teste-1")
				if err != nil {
					t.Fatal(err)
				}
				token = session.Token
				switch scenario {
				case "revoked_session":
					_, err = db.Exec(`UPDATE local_sessions SET revoked_unix=1 WHERE tenant_id=? AND identity_id=?`, o.TenantID, user)
				case "revoked_user":
					_, err = db.Exec(`UPDATE memberships SET status='revoked' WHERE tenant_id=? AND identity_id=?`, o.TenantID, user)
				case "revoked_device":
					_, err = db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, o.TenantID, o.DeviceID)
				case "removed_store_link":
					_, err = db.Exec(`DELETE FROM membership_stores WHERE tenant_id=? AND identity_id=?`, o.TenantID, user)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			before := isolationCounts(t, db)
			routes := protectedRoutes(t, app)
			for _, r := range routes {
				body := ""
				if r.Method != "GET" {
					body = "{}"
				}
				status, response := request(t, app, r.Method, concretePath(r.Path), body, token)
				if status != 401 {
					t.Fatalf("%s %s accepted %s: %d", r.Method, r.Path, scenario, status)
				}
				if token != "" && bytes.Contains(response, []byte(token)) {
					t.Fatal("session exposed in failure response")
				}
			}
			after := isolationCounts(t, db)
			for i, n := range before {
				if after[i] != n {
					t.Fatal("denied route changed database")
				}
			}
			t.Logf("%d protected routes checked", len(routes))
		})
	}
}
