package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

func TestHTTPFiscalProfilesLatestPaginationIsolationAndNoAuthority(t *testing.T) {
	f := fiscalFixture(t)
	write := func(body string) {
		t.Helper()
		code, _ := request(t, f.app, "POST", "/local/v1/fiscal/profiles", body, f.token)
		if code != 201 {
			t.Fatal("fixture profile", code)
		}
	}
	write(strings.NewReplacer("issuer-create", "profile-two-create", "issuer-one", "issuer-two").Replace(fiscalProfileBody))
	write(strings.NewReplacer("issuer-create", "profile-three-create", "issuer-one", "issuer-three").Replace(fiscalProfileBody))
	write(strings.NewReplacer("issuer-create", "profile-one-revise", `"expected_revision":0`, `"expected_revision":1`, "Synthetic test issuer", "Revised issuer").Replace(fiscalProfileBody))
	// Populate another tenant and another store in the same database with valid FKs.
	for _, scope := range []struct{ tenant, store, device string }{{"profiles-foreign", "profiles-store", "profiles-device"}, {f.owner.TenantID, "profiles-other-store", "profiles-other-device"}} {
		if scope.tenant != f.owner.TenantID {
			if _, err := f.db.Exec(`INSERT INTO tenants(id,name,created_at) VALUES(?, 'Foreign fixture','fixture')`, scope.tenant); err != nil {
				t.Fatal(err)
			}
		}
		for _, stmt := range []struct {
			q    string
			args []any
		}{
			{`INSERT INTO stores(tenant_id,id,name) VALUES(?,?,'Other fixture')`, []any{scope.tenant, scope.store}},
			{`INSERT INTO devices(tenant_id,store_id,id,name) VALUES(?,?,?,'Other fixture')`, []any{scope.tenant, scope.store, scope.device}},
			{`INSERT INTO users(tenant_id,id,display_name) VALUES(?,?,'Other fixture') ON CONFLICT DO NOTHING`, []any{scope.tenant, f.owner.OwnerID}},
			{`INSERT INTO fiscal_profiles SELECT ?,?,'foreign-profile',revision,?,actor_id,operation_id,request_json,created_at FROM fiscal_profiles WHERE tenant_id=? AND store_id=? AND profile_id='issuer-one' AND revision=1`, []any{scope.tenant, scope.store, scope.device, f.owner.TenantID, f.owner.StoreID}},
		} {
			if _, err := f.db.Exec(stmt.q, stmt.args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	after := ""
	ids := []string{}
	for i := 0; i < 3; i++ {
		path := "/local/v1/fiscal/profiles?limit=1"
		if after != "" {
			path += "&after=" + after
		}
		code, raw := request(t, f.app, "GET", path, "", f.token)
		var page fiscal.ProfilePage
		if code != 200 || json.Unmarshal(raw, &page) != nil || len(page.Rows) != 1 {
			t.Fatal("page", code)
		}
		p := page.Rows[0]
		ids = append(ids, p.Input.ProfileID)
		if p.Input.ProfileID == "issuer-one" && (p.Revision != 2 || p.Input.LegalName != "Revised issuer") {
			t.Fatal("old revision shown")
		}
		if p.Readiness.OfficialAuthorization || p.Readiness.NumberAllocated || bytes.Contains(raw, []byte("foreign-profile")) {
			t.Fatal("scope or authority fabricated")
		}
		if page.HasMore != (i < 2) {
			t.Fatal("truncation hidden")
		}
		after = page.NextAfter
	}
	if strings.Join(ids, ",") != "issuer-one,issuer-three,issuer-two" || after != "" {
		t.Fatal("duplicate/missing profiles", ids)
	}
	if code, raw := request(t, f.app, "GET", "/local/v1/fiscal/profiles?after=zzzz", "", f.token); code != 200 || !bytes.Contains(raw, []byte(`"rows":[]`)) {
		t.Fatal("empty page", code)
	}
	// A context forged at the store boundary still cannot use the owner's session.
	actor := identity.Scope{TenantID: "profiles-foreign", StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	license, err := entitlementstore.New(f.db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fiscal.ListProfiles(context.Background(), f.db, license, actor, f.device, f.token, "", 20); err == nil {
		t.Fatal("foreign context accepted")
	}
}

func TestHTTPFiscalProfilesRejectOverridesRolesExpiredContractAndSession(t *testing.T) {
	f := fiscalFixture(t)
	for _, query := range []string{"?tenant_id=other", "?store_id=other", "?limit=0", "?limit=51", "?limit=1&limit=2", "?after=", "?after=../x", "?after=x&after=y"} {
		if code, _ := request(t, f.app, "GET", "/local/v1/fiscal/profiles"+query, "", f.token); code != 400 {
			t.Fatal("query accepted", query, code)
		}
	}
	if code, _ := request(t, f.app, "GET", "/local/v1/fiscal/profiles", "{}", f.token); code != 400 {
		t.Fatal("body accepted", code)
	}
	for _, role := range []string{"manager", "accountant", "cashier"} {
		id := "profiles-" + role
		code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"`+id+`","identity_id":"`+id+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
		if code != 201 {
			t.Fatal("staff", code)
		}
		code, b := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"`+id+`","password":"test-password-strong-1"}`, "")
		var auth struct {
			Token string `json:"token"`
		}
		if code != 200 || json.Unmarshal(b, &auth) != nil || auth.Token == "" {
			t.Fatal("login", code)
		}
		if code, _ = request(t, f.app, "GET", "/local/v1/fiscal/profiles", "", auth.Token); code != 403 {
			t.Fatal("non-owner listed", role, code)
		}
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
	if code, _ := request(t, f.app, "GET", "/local/v1/fiscal/profiles", "", f.token); code != 403 {
		t.Fatal("missing fiscal contract", code)
	}
	if code, _ := request(t, f.app, "POST", "/local/v1/logout", "{}", f.token); code != 204 {
		t.Fatal("logout", code)
	}
	if code, _ := request(t, f.app, "GET", "/local/v1/fiscal/profiles", "", f.token); code != 401 {
		t.Fatal("revoked session", code)
	}
}
