package routes

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestUnfinishedBusinessRoutesAreUnavailable(t *testing.T) {
	const secret = "segredo-apenas-do-teste"
	t.Setenv("JWT_SECRET", secret)
	claims := jwt.MapClaims{"sid": "11111111-1111-4111-8111-111111111112",
		"sub":       "11111111-1111-4111-8111-111111111111",
		"tenant_id": "22222222-2222-4222-8222-222222222222",
		"role":      "admin",
		"type":      "access",
		"exp":       time.Now().Add(time.Minute).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	Registrar(app)
	cases := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/analises/produtos-parados"},
		{"POST", "/api/v1/recompensas/indicacoes"},
		{"POST", "/api/v1/recompensas/indicacoes/recompensar"},
		{"GET", "/api/v1/recompensas/indicacoes/saldo"},
		{"POST", "/api/v1/accounting/sped/request"},
		{"GET", "/api/v1/accounting/sped/status/123"},
		{"POST", "/api/v1/discounts/suggestions/123/review"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			for _, authenticated := range []bool{false, true} {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				want := fiber.StatusUnauthorized
				if authenticated {
					req.Header.Set("Authorization", "Bearer "+token)
					want = fiber.StatusNotImplemented
				}
				resp, err := app.Test(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != want {
					t.Fatalf("autenticado=%v: esperado %d, recebido %d", authenticated, want, resp.StatusCode)
				}
			}
		})
	}

	req := httptest.NewRequest("GET", "/api/v1/tempo-real/chat", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("chat público: esperado 401, recebido %d", resp.StatusCode)
	}

	req = httptest.NewRequest("GET", "/api/v1/tempo-real/chat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("chat autenticado: esperado 404, recebido %d", resp.StatusCode)
	}
}
