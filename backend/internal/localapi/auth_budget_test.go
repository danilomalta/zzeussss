package localapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"titansystem-backend/internal/authabuse"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func TestHTTPAuthDurableRestartAndUntrustedForwarding(t *testing.T) {
	db, app, o := fixture(t)
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest("POST", "/local/v1/login", strings.NewReader(`{"identity_id":"unknown","password":"wrong-password"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", string(rune('a'+i)))
		req.Header.Set("Forwarded", "for=untrusted")
		req.Header.Set("X-Real-IP", string(rune('a'+i)))
		r, e := app.Test(req, -1)
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		want := 401
		if i == 5 {
			want = 429
		}
		if r.StatusCode != want {
			t.Fatalf("attempt %d: %d", i, r.StatusCode)
		}
	}
	again, e := New(db, identity.DeviceContext{TenantID: o.TenantID, StoreID: o.StoreID, DeviceID: o.DeviceID})
	if e != nil {
		t.Fatal(e)
	}
	code, _ := request(t, again, "POST", "/local/v1/login", `{"identity_id":"unknown","password":"wrong-password"}`, "")
	if code != 429 {
		t.Fatal("server restart bypass", code)
	}
}
func TestHTTPAuthAccountPressureDoesNotLockVictim(t *testing.T) {
	db, app, o := fixture(t)
	s := authabuse.Store{DB: db}
	var generic []byte
	for _, account := range []string{o.OwnerID, "missing-owner"} {
		for i := 0; i < 5; i++ {
			if e := s.Take(context.Background(), o.TenantID, "login", "", account); e != nil {
				t.Fatal(e)
			}
		}
		raw, _ := json.Marshal(map[string]string{"identity_id": account, "password": "wrong-password"})
		code, body := request(t, app, "POST", "/local/v1/login", string(raw), "")
		if code != 429 || strings.Contains(string(body), account) {
			t.Fatal("nonuniform account pressure", code)
		}
		if generic == nil {
			generic = body
		} else if !bytes.Equal(body, generic) {
			t.Fatal("account enumeration under pressure")
		}
	}
	token := loginToken(t, app, o.OwnerID)
	code, _ := request(t, app, "GET", "/local/v1/me", "", token)
	if code != 200 {
		t.Fatal("victim locked out")
	}
}
func TestHTTPAuthOriginsDeniedBeforeBudget(t *testing.T) {
	db, app, _ := fixture(t)
	for _, path := range []string{"/local/v1/login", "/local/v1/account/recover"} {
		for _, origin := range []string{"https://untrusted.invalid", "null", "*"} {
			req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", origin)
			r, e := app.Test(req)
			if e != nil {
				t.Fatal(e)
			}
			r.Body.Close()
			if r.StatusCode != 403 {
				t.Fatal("untrusted origin accepted")
			}
		}
		req := httptest.NewRequest("POST", path, nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		r, e := app.Test(req)
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		if r.StatusCode != 403 {
			t.Fatal("cross-site accepted")
		}
	}
	var n int
	if e := db.QueryRow(`SELECT count(*) FROM auth_attempt_budgets`).Scan(&n); e != nil || n != 0 {
		t.Fatal("untrusted origin consumed victim budget")
	}
}
func TestHTTPAuthMissingBudgetSchemaFailsClosed(t *testing.T) {
	db, app, _ := fixture(t)
	if _, e := db.Exec(`ALTER TABLE auth_attempt_budgets RENAME TO fixture_missing_budgets`); e != nil {
		t.Fatal(e)
	}
	code, b := request(t, app, "POST", "/local/v1/login", `{"identity_id":"owner","password":"secret"}`, "")
	if code != 503 || strings.Contains(string(b), "secret") {
		t.Fatal("policy missing fail-open", code)
	}
}

func TestHTTPRecoveryPressureCannotConsumeValidKeyOrPreserveOldSession(t *testing.T) {
	db, app, o := fixture(t)
	token := loginToken(t, app, o.OwnerID)
	device := identity.DeviceContext{TenantID: o.TenantID, StoreID: o.StoreID, DeviceID: o.DeviceID}
	secret := bytes.Repeat([]byte{9}, 32)
	if err := localauth.IssueOwnerRecovery(context.Background(), db, device, o.OwnerID, "senha-forte-de-teste-1", secret, func() error { return nil }); err != nil {
		t.Fatal("recovery fixture unavailable")
	}
	s := authabuse.Store{DB: db}
	for i := 0; i < 5; i++ {
		if err := s.Take(context.Background(), o.TenantID, "recovery", "", o.OwnerID); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(map[string]string{"identity_id": o.OwnerID, "recovery_key": base64.RawURLEncoding.EncodeToString(secret), "new_password": "new-disposable-password"})
	code, body := request(t, app, "POST", "/local/v1/account/recover", string(raw), "")
	if code != 204 || bytes.Contains(body, secret) {
		t.Fatal("valid recovery locked or leaked", code)
	}
	code, _ = request(t, app, "GET", "/local/v1/me", "", token)
	if code != 401 {
		t.Fatal("recovered session remained active")
	}
	code, _ = request(t, app, "POST", "/local/v1/account/recover", string(raw), "")
	if code != 429 {
		t.Fatal("consumed recovery key accepted or pressure ignored", code)
	}
	code, _ = request(t, app, "POST", "/local/v1/login", `{"identity_id":"`+o.OwnerID+`","password":"new-disposable-password"}`, "")
	if code != 200 {
		t.Fatal("legitimate recovery did not persist")
	}
}
