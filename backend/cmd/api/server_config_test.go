package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/apicontract"
	"titansystem-backend/internal/core/routes"
)

func TestOnlineListenAddressDefaultsAndRejectsAmbiguity(t *testing.T) {
	for _, tc := range []struct{ host, port, want string }{{"", "", "0.0.0.0:8080"}, {"127.0.0.1", "8081", "127.0.0.1:8081"}, {"::1", "8080", "[::1]:8080"}} {
		got, err := onlineListenAddress(tc.host, tc.port)
		if err != nil || got != tc.want {
			t.Fatalf("wrong address %s", got)
		}
	}
	for _, tc := range []struct{ host, port string }{{"localhost", "8080"}, {"127.0.0.1:8080", "8080"}, {" 127.0.0.1", "8080"}, {"127.0.0.1", "0"}, {"127.0.0.1", "65536"}, {"127.0.0.1", "+8080"}, {"127.0.0.1", "8080\n"}, {"127.0.0.1", "1e3"}} {
		if _, err := onlineListenAddress(tc.host, tc.port); err == nil {
			t.Fatal("ambiguous listener accepted")
		}
	}
}

func TestOnlineResourceConfigIsBounded(t *testing.T) {
	cfg := onlineServerConfig()
	if cfg.BodyLimit != 65536 || cfg.ReadBufferSize != 8192 || cfg.Concurrency != 256 || cfg.ReadTimeout != 10*time.Second || cfg.WriteTimeout != 30*time.Second || cfg.IdleTimeout != 60*time.Second || cfg.ProxyHeader != "" || cfg.ErrorHandler == nil {
		t.Fatal("resource bounds changed")
	}
}

func TestOnlineBodyLimitStopsBeforeHandlerAndDoesNotReflectInput(t *testing.T) {
	for _, format := range []string{"", "v1"} {
		app := fiber.New(onlineServerConfig())
		var invoked atomic.Bool
		app.Post("/input", func(c *fiber.Ctx) error { invoked.Store(true); return c.SendStatus(200) })
		base := serveOnlineFixture(t, app)
		req, _ := http.NewRequest("POST", base+"/input", strings.NewReader(strings.Repeat("private-body", 6000)))
		req.Header.Set(apicontract.ErrorFormatHeader, format)
		client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 413 || invoked.Load() || strings.Contains(string(body), "private-body") || resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("body limit failed")
		}
		if format == "v1" && !strings.Contains(string(body), "payload_too_large") {
			t.Fatal("negotiated error missing")
		}
	}
	app := fiber.New(onlineServerConfig())
	app.Post("/input", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	req := httptest.NewRequest("POST", "/input", strings.NewReader(strings.Repeat("a", 65536)))
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("exact body limit rejected")
	}
}

func serveOnlineFixture(t *testing.T, app *fiber.App) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	t.Cleanup(func() {
		listener.Close()
		app.ShutdownWithTimeout(time.Second)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("listener did not stop")
		}
	})
	return "http://" + listener.Addr().String()
}

func TestOnlineLargeHeaderIsRefusedBeforeHandler(t *testing.T) {
	app := fiber.New(onlineServerConfig())
	var invoked atomic.Bool
	app.Get("/", func(c *fiber.Ctx) error { invoked.Store(true); return c.SendStatus(200) })
	req, _ := http.NewRequest("GET", serveOnlineFixture(t, app)+"/", nil)
	req.Header.Set("X-Extra", strings.Repeat("private-header", 700))
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 431 || invoked.Load() || strings.Contains(string(body), "private-header") {
		t.Fatal("large header reached handler or leaked input")
	}
}

func TestOnlineFrameworkFailureNeverLeaksPrivateError(t *testing.T) {
	app := fiber.New(onlineServerConfig())
	app.Get("/fail", func(c *fiber.Ctx) error { return errors.New("private-database-password") })
	resp, err := app.Test(httptest.NewRequest("GET", "/fail", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 500 || strings.Contains(string(body), "private-database-password") {
		t.Fatal("framework error exposed diagnostics")
	}
}

func TestOnlinePartialHeaderIsClosedByReadTimeout(t *testing.T) {
	cfg := onlineServerConfig()
	cfg.ReadTimeout = 50 * time.Millisecond
	app := fiber.New(cfg)
	var invoked atomic.Bool
	app.Get("/", func(c *fiber.Ctx) error { invoked.Store(true); return c.SendStatus(200) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	defer func() {
		listener.Close()
		app.ShutdownWithTimeout(time.Second)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("listener did not stop")
		}
	}()
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: incomplete"); err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(conn)
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("server held incomplete request past its timeout")
	}
	if invoked.Load() {
		t.Fatal("incomplete request reached handler")
	}
}

func TestOnlineForgedForwardedHeaderDoesNotChangeClientIP(t *testing.T) {
	app := fiber.New(onlineServerConfig())
	app.Get("/ip", func(c *fiber.Ctx) error { return c.SendString(c.IP()) })
	req := httptest.NewRequest("GET", "/ip", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.55")
	req.Header.Set("X-Real-IP", "203.0.113.55")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) == "203.0.113.55" {
		t.Fatal("untrusted header controls IP-based limits")
	}
}

func TestOnlineHealthRemainsPublicWithConfiguredServer(t *testing.T) {
	app := fiber.New(onlineServerConfig())
	routes.Registrar(app)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/saude", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || string(body) != `{"status":"ativo"}` {
		t.Fatal("public liveness changed")
	}
}
