package localapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocalSessionOpenAPILifecycleAndNoStore(t *testing.T) {
	_, app, owner := fixture(t)
	status, b := request(t, app, "GET", "/local/v1/health", "", "")
	if status != 200 {
		t.Fatal(status)
	}
	domainSchemaFile(t, "local-session.openapi.json", "Health", b)
	input, e := json.Marshal(map[string]string{"identity_id": owner.OwnerID, "password": "senha-forte-de-teste-1"})
	if e != nil {
		t.Fatal(e)
	}
	domainSchemaFile(t, "local-session.openapi.json", "LoginInput", input)
	req := httptest.NewRequest("POST", "/local/v1/login", bytes.NewReader(input))
	req.Header.Set("Content-Type", "application/json")
	response, e := app.Test(req, -1)
	if e != nil {
		t.Fatal(e)
	}
	b, e = io.ReadAll(response.Body)
	response.Body.Close()
	if e != nil {
		t.Fatal(e)
	}
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("login status or cache policy")
	}
	domainSchemaFile(t, "local-session.openapi.json", "LoginResult", b)
	var session struct {
		Token   string `json:"token"`
		Expires int64  `json:"expires_unix"`
	}
	if json.Unmarshal(b, &session) != nil {
		t.Fatal("invalid login")
	}
	if session.Expires <= time.Now().Unix() || session.Expires > time.Now().Add(8*time.Hour+time.Minute).Unix() {
		t.Fatal("wrong session validity")
	}
	req = httptest.NewRequest("GET", "/local/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+session.Token)
	response, e = app.Test(req, -1)
	if e != nil {
		t.Fatal(e)
	}
	b, e = io.ReadAll(response.Body)
	response.Body.Close()
	if e != nil {
		t.Fatal(e)
	}
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("me status or cache policy")
	}
	domainSchemaFile(t, "local-session.openapi.json", "Session", b)
	var me struct {
		Tenant   string `json:"tenant_id"`
		Store    string `json:"store_id"`
		Device   string `json:"device_id"`
		Identity string `json:"identity_id"`
	}
	if json.Unmarshal(b, &me) != nil || me.Tenant != owner.TenantID || me.Store != owner.StoreID || me.Device != owner.DeviceID || me.Identity != owner.OwnerID {
		t.Fatal("session context mismatch")
	}
	status, b = request(t, app, "POST", "/local/v1/logout", "", session.Token)
	if status != 204 || len(b) != 0 {
		t.Fatal("logout must be empty 204")
	}
	for _, path := range []string{"/local/v1/me", "/local/v1/logout"} {
		method := "GET"
		if strings.HasSuffix(path, "logout") {
			method = "POST"
		}
		status, _ = request(t, app, method, path, "", session.Token)
		if status != 401 {
			t.Fatal("revoked token accepted")
		}
	}
}

func TestLocalSessionOpenAPIRejectsAmbiguousLoginWithoutSession(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
	}{
		{"duplicate", `{"identity_id":"ID","password":"senha-forte-de-teste-1","identity_id":"ID"}`, 400},
		{"extra", `{"identity_id":"ID","password":"senha-forte-de-teste-1","tenant_id":"foreign"}`, 400},
		{"null", `{"identity_id":"ID","password":null}`, 400},
		{"missing", `{"identity_id":"ID"}`, 400},
		{"nested", `{"identity_id":"ID","password":{"value":"bad"}}`, 400},
		{"trailing", `{"identity_id":"ID","password":"senha-forte-de-teste-1"}{}`, 400},
		{"array", `[]`, 400},
		{"large", `{"identity_id":"ID","password":"` + strings.Repeat("x", 4096) + `"}`, 413},
		{"wrong-password", `{"identity_id":"ID","password":"senha-incorreta-de-teste"}`, 401},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, app, owner := fixture(t)
			body := strings.ReplaceAll(c.body, "ID", owner.OwnerID)
			var before, after int
			if e := db.QueryRow(`SELECT count(*) FROM local_sessions`).Scan(&before); e != nil {
				t.Fatal(e)
			}
			status, b := request(t, app, "POST", "/local/v1/login", body, "")
			if status != c.want {
				t.Fatal("unexpected rejection", status, c.want)
			}
			if e := db.QueryRow(`SELECT count(*) FROM local_sessions`).Scan(&after); e != nil || after != before {
				t.Fatal("rejected request created session")
			}
			if bytes.Contains(b, []byte("senha-")) || bytes.Contains(b, []byte(owner.OwnerID)) {
				t.Fatal("request data leaked")
			}
		})
	}
}
