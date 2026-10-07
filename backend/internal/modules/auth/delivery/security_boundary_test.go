package delivery

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/modules/auth/usecase"
)

type failedLogin struct{ err error }

func (f failedLogin) Execute(usecase.LoginInput) (*usecase.LoginOutput, error) { return nil, f.err }

func TestLoginDoesNotReturnInternalErrorsOrSubmittedSecrets(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{errors.New("internal password=private host=private"), 401},
		{usecase.ErrSessionUnavailable, 503},
	} {
		app := fiber.New()
		app.Post("/login", NewAuthHandler(failedLogin{tc.err}).Login)
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"test@example.com","password":"submitted-secret"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.status || strings.Contains(string(body), "private") || strings.Contains(string(body), "submitted-secret") || resp.Header.Get("Set-Cookie") != "" {
			t.Fatal("resposta de falha incorreta")
		}
	}
}

func TestRefreshRejectsInvalidTokenBeforeDatabaseLookup(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-secret")
	old := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = old })
	for _, bad := range []string{"HS384", "HS512", "missing-exp", "expired", "numeric-tenant", "access"} {
		t.Run(bad, func(t *testing.T) {
			claims := jwt.MapClaims{"sub": "operator", "tenant_id": "company", "role": "owner", "name": "Operator", "type": "refresh", "exp": time.Now().Add(time.Minute).Unix()}
			method := jwt.SigningMethodHS256
			switch bad {
			case "HS384":
				method = jwt.SigningMethodHS384
			case "HS512":
				method = jwt.SigningMethodHS512
			case "missing-exp":
				delete(claims, "exp")
			case "expired":
				claims["exp"] = time.Now().Add(-time.Minute).Unix()
			case "numeric-tenant":
				claims["tenant_id"] = 123
			case "access":
				claims["type"] = "access"
			}
			raw, err := jwt.NewWithClaims(method, claims).SignedString([]byte("test-only-secret"))
			if err != nil {
				t.Fatal(err)
			}
			app := fiber.New()
			app.Post("/refresh", NewAuthHandler(nil).RefreshToken)
			req := httptest.NewRequest("POST", "/refresh", nil)
			req.AddCookie(&http.Cookie{Name: "titan_session_rt", Value: raw})
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 401 || resp.Header.Get("Set-Cookie") != "" {
				t.Fatal("token inválido não bloqueado")
			}
		})
	}
}
