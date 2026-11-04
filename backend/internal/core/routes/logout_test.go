package routes

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSessionRevocationRoutesRejectOriginBeforeDatabase(t *testing.T) {
	t.Setenv("JWT_SECRET", jwtFixtureKey)
	t.Setenv("ALLOWED_ORIGINS", "https://app.example")
	for _, path := range []string{"/api/v1/auth/logout", "/api/v1/auth/sessions/revoke-others", "/api/v1/auth/sessions/44444444-4444-4444-8444-444444444444/revoke"} {
		for _, origin := range []struct{ value, site string }{{"https://evil.example", ""}, {"null", ""}, {"https://app.example", "cross-site"}} {
			t.Run(path+"/"+origin.value+"/"+origin.site, func(t *testing.T) {
				mock := onlineMock(t) // No SQL expectations: origin must precede DB.
				app := fiber.New()
				Registrar(app)
				req := httptest.NewRequest("POST", path, nil)
				req.Header.Set("Origin", origin.value)
				req.Header.Set("Sec-Fetch-Site", origin.site)
				req.Header.Set("Authorization", "Bearer "+onlineToken(t, "company-a", "owner"))
				resp, err := app.Test(req)
				if err != nil {
					t.Fatal("origin request failed")
				}
				resp.Body.Close()
				if resp.StatusCode != 403 || resp.Header.Get("Set-Cookie") != "" {
					t.Fatalf("origin barrier: got %d, want 403 without cookie", resp.StatusCode)
				}
				if err = mock.ExpectationsWereMet(); err != nil {
					t.Fatal("origin rejection touched database")
				}
			})
		}
	}
}
