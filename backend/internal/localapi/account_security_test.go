package localapi

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAccountPasswordChangeRevokesAllSessionsWithoutLeakingSecrets(t *testing.T) {
	db, app, owner := fixture(t)
	token := loginToken(t, app, owner.OwnerID)
	other := loginToken(t, app, owner.OwnerID)
	status, body := request(t, app, "GET", "/local/v1/account/sessions", "", token)
	if status != 200 || bytes.Contains(body, []byte(token)) || bytes.Contains(body, []byte(other)) || bytes.Contains(body, []byte("sha256")) {
		t.Fatalf("sessions: %d %s", status, body)
	}
	var listing struct {
		Items []struct {
			Current bool `json:"current"`
		} `json:"items"`
	}
	if json.Unmarshal(body, &listing) != nil || len(listing.Items) != 2 {
		t.Fatal("missing sessions")
	}
	status, body = request(t, app, "POST", "/local/v1/account/password", `{"current_password":"incorrect-secret","new_password":"another-strong-password"}`, token)
	if status != 401 || bytes.Contains(body, []byte("secret")) {
		t.Fatalf("incorrect password: %d %s", status, body)
	}
	status, _ = request(t, app, "POST", "/local/v1/account/password", `{"current_password":"senha-forte-de-teste-1","new_password":"another-strong-password"}`, token)
	if status != 204 {
		t.Fatalf("change: %d", status)
	}
	for _, old := range []string{token, other} {
		status, _ = request(t, app, "GET", "/local/v1/me", "", old)
		if status != 401 {
			t.Fatal("old session survived")
		}
	}
	encoded, _ := json.Marshal(map[string]string{"identity_id": owner.OwnerID, "password": "another-strong-password"})
	status, _ = request(t, app, "POST", "/local/v1/login", string(encoded), "")
	if status != 200 {
		t.Fatalf("new login: %d", status)
	}
	var events int
	if err := db.QueryRow(`SELECT count(*) FROM account_security_events WHERE kind='password_changed'`).Scan(&events); err != nil || events != 1 {
		t.Fatalf("audit: %d %v", events, err)
	}
}

func TestAccountRevokeOthersKeepsCurrentAndAuditFailureRollsBack(t *testing.T) {
	db, app, owner := fixture(t)
	token := loginToken(t, app, owner.OwnerID)
	other := loginToken(t, app, owner.OwnerID)
	status, _ := request(t, app, "POST", "/local/v1/account/sessions/revoke-others", `{"current_password":"senha-forte-de-teste-1"}`, token)
	if status != 204 {
		t.Fatalf("revoke: %d", status)
	}
	status, _ = request(t, app, "GET", "/local/v1/me", "", token)
	if status != 200 {
		t.Fatal("current revoked")
	}
	status, _ = request(t, app, "GET", "/local/v1/me", "", other)
	if status != 401 {
		t.Fatal("other survived")
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_security BEFORE INSERT ON account_security_events BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, app, "POST", "/local/v1/account/password", `{"current_password":"senha-forte-de-teste-1","new_password":"another-strong-password"}`, token)
	if status != 500 {
		t.Fatalf("missing audit must be internal failure: %d", status)
	}
	status, _ = request(t, app, "GET", "/local/v1/me", "", token)
	if status != 200 {
		t.Fatal("revocation escaped rollback")
	}
	_ = loginToken(t, app, owner.OwnerID)
}

func TestAccountRejectsAmbiguousOrForeignInputs(t *testing.T) {
	for _, body := range []string{
		`{"current_password":"a","current_password":"b","new_password":"long-enough-password"}`,
		`{"current_password":"senha-forte-de-teste-1","new_password":"long-enough-password","identity_id":"other"}`,
		`{"current_password":null,"new_password":"long-enough-password"}`,
		`{"current_password":"senha-forte-de-teste-1","new_password":"long-enough-password"} {}`,
	} {
		_, app, owner := fixture(t)
		token := loginToken(t, app, owner.OwnerID)
		status, _ := request(t, app, "POST", "/local/v1/account/password", body, token)
		if status != 400 {
			t.Fatalf("ambiguous: %d", status)
		}
	}
	_, app, _ := fixture(t)
	status, _ := request(t, app, "GET", "/local/v1/account/sessions", "", "")
	if status != 401 {
		t.Fatal("anonymous listing")
	}
}
