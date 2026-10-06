package localapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

const adminReset = `{"operation_id":"reset-1","current_password":"senha-forte-de-teste-1","new_password":"staff-new-strong-password","reason":"Employee requested reset"}`

func TestHTTPAdminAccessResetRevokesTargetAndRepeatsWithoutSecrets(t *testing.T) {
	db, app, owner := fixture(t)
	for _, q := range []string{
		`INSERT INTO identities VALUES ('employee','Employee','now')`,
		`INSERT INTO memberships VALUES ('` + owner.TenantID + `','employee','employee','active','now')`,
		`INSERT INTO membership_stores VALUES ('` + owner.TenantID + `','employee','` + owner.StoreID + `')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	hash, _ := localauth.HashPassword("employee-old-password")
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES (?, 'employee',?,'now')`, owner.TenantID, hash); err != nil {
		t.Fatal(err)
	}
	login, _ := json.Marshal(map[string]string{"identity_id": "employee", "password": "employee-old-password"})
	status, body := request(t, app, "POST", "/local/v1/login", string(login), "")
	if status != 200 {
		t.Fatal("employee login failed")
	}
	var employeeSession struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &employeeSession); err != nil {
		t.Fatal(err)
	}
	token := loginToken(t, app, owner.OwnerID)
	for _, repeated := range []bool{false, true} {
		status, body = request(t, app, "POST", "/local/v1/staff/employee/password-reset", adminReset, token)
		if status != 200 || bytes.Contains(body, []byte("password")) || bytes.Contains(body, []byte(token)) {
			t.Fatalf("reset: %d %s", status, body)
		}
		var response localauth.AccessResult
		if err := json.Unmarshal(body, &response); err != nil || response.Repeated != repeated || response.AffectedSessions != 1 {
			t.Fatal("invalid reset result")
		}
	}
	status, _ = request(t, app, "GET", "/local/v1/me", "", employeeSession.Token)
	if status != 401 {
		t.Fatal("employee session survived")
	}
	status, _ = request(t, app, "GET", "/local/v1/me", "", token)
	if status != 200 {
		t.Fatal("administrator session revoked")
	}
	// A later password change cannot be overwritten by retrying the old reset.
	hash, _ = localauth.HashPassword("employee-later-password")
	if _, err := db.Exec(`UPDATE local_passwords SET password_hash=? WHERE tenant_id=? AND identity_id='employee'`, hash, owner.TenantID); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, app, "POST", "/local/v1/staff/employee/password-reset", adminReset, token)
	if status != 409 {
		t.Fatalf("stale reset: %d", status)
	}
}

func TestHTTPAdminAccessRejectsOwnerForeignStoreAndBadJSON(t *testing.T) {
	db, app, owner := fixture(t)
	token := loginToken(t, app, owner.OwnerID)
	status, _ := request(t, app, "POST", "/local/v1/staff/"+owner.OwnerID+"/password-reset", adminReset, token)
	if status != 403 {
		t.Fatalf("owner reset: %d", status)
	}
	for _, q := range []string{
		`INSERT INTO stores VALUES ('` + owner.TenantID + `','other-store','Other')`,
		`INSERT INTO identities VALUES ('other','Other','now')`,
		`INSERT INTO memberships VALUES ('` + owner.TenantID + `','other','employee','active','now')`,
		`INSERT INTO membership_stores VALUES ('` + owner.TenantID + `','other','other-store')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	status, _ = request(t, app, "POST", "/local/v1/staff/other/password-reset", adminReset, token)
	if status != 403 {
		t.Fatalf("foreign store: %d", status)
	}
	status, _ = request(t, app, "POST", "/local/v1/staff/other/password-reset", adminReset, "")
	if status != 401 {
		t.Fatalf("anonymous: %d", status)
	}
	// Duplicated or injected scope is rejected before any write.
	bad := string(bytes.Replace([]byte(adminReset), []byte(`"reason":`), []byte(`"tenant_id":"foreign","reason":`), 1))
	status, _ = request(t, app, "POST", "/local/v1/staff/other/password-reset", bad, token)
	if status != 400 {
		t.Fatalf("injected scope: %d", status)
	}
}

func TestHTTPOwnerRecoveryRequiresKeyAndPreservesLoginWithoutVerifier(t *testing.T) {
	db, _, owner := fixture(t)
	device := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
	app, err := New(db, device)
	if err != nil {
		t.Fatal(err)
	}
	secret := bytes.Repeat([]byte{4}, 32)
	if err := localauth.IssueOwnerRecovery(context.Background(), db, device, owner.OwnerID, "senha-forte-de-teste-1", secret, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"identity_id": owner.OwnerID, "recovery_key": base64.RawURLEncoding.EncodeToString(secret), "new_password": "recovered-owner-password"})
	status, response := request(t, app, "POST", "/local/v1/account/recover", string(body), "")
	if status != 204 || len(response) != 0 {
		t.Fatalf("recover: %d", status)
	}
	status, response = request(t, app, "POST", "/local/v1/account/recover", string(body), "")
	if status != 401 || bytes.Contains(response, secret) || bytes.Contains(response, []byte("recovered-owner-password")) {
		t.Fatal("recovery key reused or leaked")
	}
	login, _ := json.Marshal(map[string]string{"identity_id": owner.OwnerID, "password": "recovered-owner-password"})
	status, _ = request(t, app, "POST", "/local/v1/login", string(login), "")
	if status != 200 {
		t.Fatal("recovered login blocked by licensing")
	}
}

func TestHTTPRecoveryRateLimitAndMalformedBodies(t *testing.T) {
	_, app, _ := fixture(t)
	for i := 0; i < 5; i++ {
		status, _ := request(t, app, "POST", "/local/v1/account/recover", `{"identity_id":"a","recovery_key":"invalid","new_password":"long-test-password"}`, "")
		if status != 401 {
			t.Fatalf("invalid recovery: %d", status)
		}
	}
	status, _ := request(t, app, "POST", "/local/v1/account/recover", `{}`, "")
	if status != 429 {
		t.Fatalf("limiter: %d", status)
	}
	_, fresh, _ := fixture(t)
	status, _ = request(t, fresh, "POST", "/local/v1/account/recover", `{"identity_id":"a","identity_id":"b","recovery_key":"invalid","new_password":"long-test-password"}`, "")
	if status != 400 {
		t.Fatalf("duplicate input: %d", status)
	}
}
