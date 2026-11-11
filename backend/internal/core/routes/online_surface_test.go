package routes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestOnlineSurfaceContractsWithoutInventingOperations(t *testing.T) {
	const secret = "segredo-apenas-do-teste"
	t.Setenv("JWT_SECRET", secret)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sid": "11111111-1111-4111-8111-111111111112", "sub": "11111111-1111-4111-8111-111111111111",
		"tenant_id": "22222222-2222-4222-8222-222222222222", "role": "admin", "type": "access",
		"exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	Registrar(app)
	auth := func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+token) }

	for _, item := range []struct{ method, path string }{
		{"GET", "/api/v1/analises/produtos-parados"},
		{"GET", "/api/v1/recompensas/indicacoes/saldo"},
		{"GET", "/api/v1/accounting/sped/status/123"},
		{"POST", "/api/v1/fiscal/emit"},
	} {
		req := httptest.NewRequest(item.method, item.path, nil)
		resp, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		resp.Body.Close()
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatal("public", item.path, resp.StatusCode)
		}
	}

	for _, item := range []struct{ method, path, code string }{
		{"POST", "/api/v1/fiscal/emit", "production_blocked"},
		{"GET", "/api/v1/analises/produtos-parados", "online_sales_absent"},
		{"POST", "/api/v1/recompensas/indicacoes", "loyalty_unavailable"},
		{"POST", "/api/v1/recompensas/indicacoes/recompensar", "loyalty_unavailable"},
		{"GET", "/api/v1/recompensas/indicacoes/saldo", "loyalty_unavailable"},
		{"POST", "/api/v1/accounting/sped/request", "sped_unavailable"},
		{"GET", "/api/v1/accounting/sped/status/123", "sped_unavailable"},
	} {
		req := httptest.NewRequest(item.method, item.path, nil)
		auth(req)
		resp, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var body map[string]string
		if resp.StatusCode != fiber.StatusConflict || json.Unmarshal(raw, &body) != nil || body["error"] != item.code {
			t.Fatal(item.path, resp.StatusCode, string(raw))
		}
	}

	req := httptest.NewRequest("GET", "/api/v1/pos/sales/recent", nil)
	auth(req)
	resp, e := app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK || resp.Header.Get("X-Titan-PDV-Available") != "false" {
		t.Fatal("sales", resp.StatusCode, string(raw))
	}
	var sales []any
	if json.Unmarshal(raw, &sales) != nil || len(sales) != 0 {
		t.Fatal("invented sales", string(raw))
	}

	for _, path := range []string{"/api/v1/cash/sessions", "/api/v1/hr/employees"} {
		req = httptest.NewRequest("GET", path, nil)
		auth(req)
		resp, e = app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		var body struct {
			Items     []any `json:"items"`
			Available bool  `json:"available"`
		}
		if resp.StatusCode != fiber.StatusOK || json.Unmarshal(raw, &body) != nil || body.Available || len(body.Items) != 0 {
			t.Fatal(path, resp.StatusCode, string(raw))
		}
	}

	req = httptest.NewRequest("GET", "/api/v1/metrics/overview", nil)
	auth(req)
	resp, e = app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatal("metrics without catalog", resp.StatusCode)
	}

	req = httptest.NewRequest("GET", "/api/v1/tempo-real/chat", nil)
	resp, e = app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatal("chat public", resp.StatusCode)
	}
	req = httptest.NewRequest("GET", "/api/v1/tempo-real/chat", nil)
	auth(req)
	resp, e = app.Test(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusUpgradeRequired {
		t.Fatal("chat upgrade", resp.StatusCode)
	}
}
