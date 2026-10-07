package delivery

import (
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"titansystem-backend/internal/core/database"
)

func TestRecoveryStrictInputNoSecretsOrCookiesOnFailure(t *testing.T) {
	old := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = old })
	for _, tc := range []struct {
		path, body, media string
		status            int
	}{
		{"/issue", `{"current_password":"submitted-secret"}`, "application/json", 503},
		{"/issue", `{"current_password":"a","current_password":"b"}`, "application/json", 400},
		{"/issue", `{"current_password":null}`, "application/json", 400},
		{"/issue", `{"current_password":"a","user_id":"foreign"}`, "application/json", 400},
		{"/reset", `{"recovery_key":"submitted-key","new_password":"submitted-secret"}`, "application/json", 503},
		{"/reset", `{"recovery_key":"a","new_password":"b","tenant_id":"foreign"}`, "application/json", 400},
		{"/reset", `{"recovery_key":"a","new_password":"b"} {}`, "application/json", 400},
		{"/reset", `{"recovery_key":"a","new_password":{}}`, "application/json", 400},
		{"/reset", `{"recovery_key":"a"}`, "application/json", 400},
		{"/reset?recovery_key=untrusted", `{"recovery_key":"a","new_password":"b"}`, "application/json", 400},
		{"/reset", strings.Repeat("x", 2049), "application/json", 413},
		{"/reset", string([]byte{255}), "application/json", 400},
		{"/reset", `{}`, "text/plain", 415},
	} {
		app := fiber.New()
		h := NewAuthHandler(nil)
		app.Post("/issue", h.IssueRecovery)
		app.Post("/reset", h.Recover)
		req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", tc.media)
		resp, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.status || resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Set-Cookie") != "" || strings.Contains(string(body), "submitted-secret") || strings.Contains(string(body), "submitted-key") {
			t.Fatal("invalid recovery HTTP response")
		}
	}
}
