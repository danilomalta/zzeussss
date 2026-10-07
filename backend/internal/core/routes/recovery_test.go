package routes

import (
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryRouteRequiresAuthenticationForIssuanceAndBoundsPublicAttempts(t *testing.T) {
	t.Setenv("JWT_SECRET", "isolated-test-secret")
	m := onlineMock(t)
	app := fiber.New()
	Registrar(app)
	req := httptest.NewRequest("POST", "/api/v1/auth/recovery/key", strings.NewReader(`{"current_password":"fixture-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatal("anonymous issuance accepted")
	}
	for i := 0; i < 6; i++ {
		req = httptest.NewRequest("POST", "/api/v1/auth/recovery/reset", strings.NewReader(`{"recovery_key":"invalid-key","new_password":"valid-fixture-password"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, e = app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		want := 401
		if i == 5 {
			want = 429
		}
		if resp.StatusCode != want {
			t.Fatalf("got %d want %d", resp.StatusCode, want)
		}
	}
	req = httptest.NewRequest("POST", "/api/v1/auth/recovery/reset", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://untrusted.example")
	req.Header.Set("Content-Type", "application/json")
	resp, e = app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatal("cross-origin recovery accepted")
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
