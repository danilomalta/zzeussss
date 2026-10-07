package routes

import (
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasswordRouteRequiresActiveSessionAndLimitsAttempts(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	for _, state := range []string{"anonymous", "revoked", "active", "limited"} {
		t.Run(state, func(t *testing.T) {
			mock := onlineMock(t)
			app := fiber.New()
			Registrar(app)
			requests := 1
			if state == "limited" {
				requests = 6
			}
			for i := 0; i < requests; i++ {
				want := 400
				if state == "anonymous" || state == "revoked" {
					want = 401
				} else if state == "limited" && i == 5 {
					want = 429
				}
				if state != "anonymous" && want != 429 {
					mock.ExpectQuery(`SELECT EXISTS`).WithArgs("operator", "company-a", "owner", "11111111-1111-4111-8111-111111111112").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(state != "revoked"))
				}
				req := httptest.NewRequest("POST", "/api/v1/auth/password", strings.NewReader(`{"current_password":"secret-submitted","new_password":"short"}`))
				req.Header.Set("Content-Type", "application/json")
				if state != "anonymous" {
					req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
				}
				resp, e := app.Test(req)
				if e != nil {
					t.Fatal(e)
				}
				resp.Body.Close()
				if resp.StatusCode != want {
					t.Fatalf("got %d want %d", resp.StatusCode, want)
				}
			}
			if e := mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
