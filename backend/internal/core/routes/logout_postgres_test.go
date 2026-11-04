package routes

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/onlinesessions"
)

// Uses only opt-in loopback *_test PostgreSQL and a fresh schema. No .env,
// installed database, production identity, or frontend fixture is involved.
func TestPostgresHTTPLogoutRevocationIsolationRollbackAndLostResponse(t *testing.T) {
	raw := os.Getenv("TITAN_SESSION_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("PostgreSQL descartável ausente: TITAN_SESSION_TEST_DATABASE_URL")
	}
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid disposable database configuration")
	}
	host := cfg.ConnConfig.Host
	ip := net.ParseIP(host)
	if !strings.HasSuffix(cfg.ConnConfig.Database, "_test") || (host != "localhost" && (ip == nil || !ip.IsLoopback())) {
		t.Fatal("requires loopback *_test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("disposable pool unavailable")
	}
	defer pool.Close()
	schema := "p03_logout_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = pool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal("isolated schema unavailable")
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	sqlDB := stdlib.OpenDB(*cfg.ConnConfig)
	defer sqlDB.Close()
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := sqlDB.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal("disposable fixture SQL failed")
		}
	}
	exec(`CREATE TABLE tenants(id UUID PRIMARY KEY,status TEXT NOT NULL); CREATE TABLE users(id UUID PRIMARY KEY,tenant_id UUID NOT NULL REFERENCES tenants(id),name TEXT NOT NULL,password_hash TEXT NOT NULL,role TEXT NOT NULL)`)
	if err = onlinesessions.Migrate(ctx, sqlDB); err != nil {
		t.Fatal("session migrations failed")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("disposable ORM unavailable")
	}
	old := database.DB
	database.DB = db
	defer func() { database.DB = old }()
	t.Setenv("JWT_SECRET", jwtFixtureKey)
	t.Setenv("ALLOWED_ORIGINS", "https://app.example")
	t.Setenv("COOKIE_SECURE", "true")
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO tenants VALUES ($1,'active'),($2,'active')`, tenantA, tenantB)
	ownerA, otherA, ownerB := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO users VALUES ($1,$2,'Owner A','fixture-hash','owner'),($3,$2,'Other A','fixture-hash','cashier'),($4,$5,'Owner B','fixture-hash','owner')`, ownerA, tenantA, otherA, ownerB, tenantB)
	store := onlinesessions.New(sqlDB, jwtFixtureKey)
	newSession := func(user, tenant, role string) onlinesessions.Tokens {
		t.Helper()
		pair, err := store.Create(ctx, onlinesessions.Scope{User: user, Tenant: tenant, Role: role}, "fixture-hash")
		if err != nil {
			t.Fatal("disposable session unavailable")
		}
		return pair
	}
	primary := newSession(ownerA, tenantA, "owner")
	peer := newSession(ownerA, tenantA, "owner")
	other := newSession(otherA, tenantA, "cashier")
	foreign := newSession(ownerB, tenantB, "owner")
	id := func(pair onlinesessions.Tokens) string {
		t.Helper()
		value, err := onlinesessions.ParseRefresh(pair.Refresh)
		if err != nil {
			t.Fatal("invalid disposable refresh")
		}
		return value
	}
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	Registrar(app)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal("disposable HTTP listener unavailable")
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	defer func() {
		if err := app.ShutdownWithTimeout(2 * time.Second); err != nil {
			t.Error("HTTP shutdown failed")
		}
		if err := <-done; err != nil {
			t.Error("HTTP listener failed")
		}
	}()
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	base := "http://" + listener.Addr().String()
	request := func(path, method, access, refresh, body string, want int) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal("invalid HTTP test request")
		}
		req.Header.Set("Origin", "https://app.example")
		if access != "" {
			req.Header.Set("Authorization", "Bearer "+access)
		}
		if refresh != "" {
			req.AddCookie(&http.Cookie{Name: "titan_session_rt", Value: refresh})
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal("HTTP test request failed")
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != want {
			t.Fatalf("%s: got %d want %d", path, resp.StatusCode, want)
		}
		for _, secret := range []string{jwtFixtureKey, "private-fixture-rejection"} {
			if strings.Contains(string(data), secret) {
				t.Fatal("private material in HTTP response")
			}
		}
		if want != 200 && (strings.Contains(string(data), primary.Access) || strings.Contains(string(data), primary.Refresh)) {
			t.Fatal("tokens in logout/failure response")
		}
		if want != 200 && want != 204 && resp.Header.Get("Set-Cookie") != "" {
			t.Fatal("failure changed refresh cookie")
		}
		return resp
	}
	state := func(pair onlinesessions.Tokens, revoked bool, auditCount int) {
		t.Helper()
		var gotRevoked bool
		var count int
		if sqlDB.QueryRowContext(ctx, `SELECT revoked_at IS NOT NULL FROM online_sessions WHERE id=$1`, id(pair)).Scan(&gotRevoked) != nil || gotRevoked != revoked {
			t.Fatal("incorrect session state")
		}
		if sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM online_session_audit WHERE session_id=$1 AND event='logout'`, id(pair)).Scan(&count) != nil || count != auditCount {
			t.Fatal("duplicated or missing logout audit")
		}
	}
	// Authenticate the actual refresh route before logout; compare the issued
	// cookie against the deletion returned by the actual logout route.
	originalRefresh := primary.Refresh
	issued := request("/api/v1/auth/refresh", "POST", "", primary.Refresh, "", 200).Cookies()
	if len(issued) != 1 {
		t.Fatal("refresh did not issue exactly one cookie")
	}
	primary.Refresh = issued[0].Value
	request("/api/v1/auth/logout", "POST", "", "", "", 401)
	for _, path := range []string{"/api/v1/auth/logout", "/api/v1/auth/sessions/revoke-others", "/api/v1/auth/sessions/" + id(peer) + "/revoke"} {
		request(path, "POST", primary.Access, "", `{"tenant_id":"foreign","role":"owner","others":true}`, 400)
	}
	for _, target := range []onlinesessions.Tokens{other, foreign} {
		request("/api/v1/auth/sessions/"+id(target)+"/revoke", "POST", primary.Access, "", "", 401)
		state(target, false, 0)
	}
	// Durable audit failure rolls back revocation. No cookie is cleared; the
	// same access/refresh identity remains available for an explicit retry.
	exec(`CREATE FUNCTION reject_p03_logout() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event='logout' THEN RAISE EXCEPTION 'private-fixture-rejection'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_p03_logout BEFORE INSERT ON online_session_audit FOR EACH ROW EXECUTE FUNCTION reject_p03_logout()`)
	request("/api/v1/auth/logout", "POST", primary.Access, "", "", 503)
	state(primary, false, 0)
	request("/api/v1/auth/sessions", "GET", primary.Access, "", "", 200)
	exec(`ALTER TABLE online_session_audit DISABLE TRIGGER reject_p03_logout`)
	cleared := request("/api/v1/auth/logout", "POST", primary.Access, "", "", 204)
	cookies := cleared.Cookies()
	if len(cookies) != 1 {
		t.Fatal("logout did not clear cookie")
	}
	a, b := issued[0], cookies[0]
	if a.Name != b.Name || a.Path != b.Path || a.Domain != b.Domain || a.Secure != b.Secure || a.HttpOnly != b.HttpOnly || a.SameSite != b.SameSite || b.Value != "" || b.MaxAge > 0 || !b.Expires.Before(time.Now()) || cleared.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("logout cookie incompatible with issued cookie")
	}
	state(primary, true, 1)
	request("/api/v1/auth/sessions", "GET", primary.Access, "", "", 401)
	request("/api/v1/auth/refresh", "POST", "", primary.Refresh, "", 401)
	request("/api/v1/auth/refresh", "POST", "", originalRefresh, "", 401)
	request("/api/v1/auth/logout", "POST", primary.Access, "", "", 401)
	state(primary, true, 1)
	// Drop a real, committed HTTP response at the client transport boundary.
	// Retrying the same identity must not produce another audit or mutation.
	lost := newSession(ownerA, tenantA, "owner")
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	dropClient := &http.Client{Timeout: 5 * time.Second, Transport: p03LostResponse{base: transport}}
	req, err := http.NewRequest("POST", base+"/api/v1/auth/logout", nil)
	if err != nil {
		t.Fatal("invalid lost-response request")
	}
	req.Header.Set("Authorization", "Bearer "+lost.Access) // Native request: no Origin.
	if _, err = dropClient.Do(req); err == nil {
		t.Fatal("response loss not exercised")
	}
	state(lost, true, 1)
	request("/api/v1/auth/logout", "POST", lost.Access, "", "", 401)
	request("/api/v1/auth/refresh", "POST", "", lost.Refresh, "", 401)
	state(lost, true, 1)
	for _, untouched := range []onlinesessions.Tokens{peer, other, foreign} {
		state(untouched, false, 0)
		request("/api/v1/auth/sessions", "GET", untouched.Access, "", "", 200)
		request("/api/v1/auth/refresh", "POST", "", untouched.Refresh, "", 200)
	}
}

type p03LostResponse struct{ base http.RoundTripper }

func (p p03LostResponse) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := p.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	_, copyErr := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if copyErr != nil || resp.StatusCode != 204 {
		return nil, errors.New("synthetic logout did not confirm before loss")
	}
	return nil, errors.New("synthetic response lost after commit")
}
