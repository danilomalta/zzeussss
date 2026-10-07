package apicontract

import (
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNegotiatedErrorsPreserveStatusAndHeadersWithoutSecrets(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 405, 409, 413, 415, 422, 429, 500, 501, 503} {
		for _, returned := range []bool{false, true} {
			app := fiber.New()
			app.Use(Errors(), Errors()) // startup and router may both install it.
			app.Get("/test", func(c *fiber.Ctx) error {
				c.Set("Retry-After", "60")
				c.Set("WWW-Authenticate", "Bearer")
				c.Cookie(&fiber.Cookie{Name: "test", Value: "fixed"})
				if returned {
					return fiber.NewError(status, "private database password token")
				}
				return c.Status(status).SendString("private database password token")
			})
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(ErrorFormatHeader, "v1")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var body ErrorBody
			if json.Unmarshal(raw, &body) != nil || body.Error.Code == "" || strings.Contains(string(raw), "private") || resp.StatusCode != status {
				t.Fatalf("unsafe error status %d", status)
			}
			if resp.Header.Get("Retry-After") != "60" || resp.Header.Get("WWW-Authenticate") != "Bearer" || len(resp.Cookies()) != 1 || resp.Header.Get("Cache-Control") != "no-store" {
				t.Fatal("error headers lost")
			}
		}
	}
}

func TestLegacySuccessUnknownRouteAndInternalError(t *testing.T) {
	app := fiber.New()
	app.Use(Errors())
	app.Get("/legacy", func(c *fiber.Ctx) error { return c.Status(409).SendString("legacy") })
	app.Get("/ok", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"items": []string{}}) })
	app.Get("/empty", func(c *fiber.Ctx) error { return c.SendStatus(204) })
	app.Get("/internal", func(c *fiber.Ctx) error { return errors.New("secret") })
	for _, item := range []struct {
		path, header string
		status       int
		body         string
	}{
		{"/legacy", "", 409, "legacy"}, {"/legacy", "unknown", 409, "legacy"},
		{"/ok", "v1", 200, `{"items":[]}`}, {"/empty", "v1", 204, ""},
	} {
		req := httptest.NewRequest("GET", item.path, nil)
		req.Header.Set(ErrorFormatHeader, item.header)
		resp, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != item.status || string(raw) != item.body || !strings.Contains(resp.Header.Get("Vary"), ErrorFormatHeader) {
			t.Fatal("legacy or success contract changed")
		}
	}
	for _, path := range []string{"/missing", "/internal"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set(ErrorFormatHeader, "v1")
		resp, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if !strings.Contains(string(raw), `"error"`) || strings.Contains(string(raw), "secret") {
			t.Fatal("unsafe default error")
		}
	}
}
