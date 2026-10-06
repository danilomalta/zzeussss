package localapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/accesspolicy"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/localsetup"
)

func policyInput(op, kind, dept, target, permission, value string, revision int64) accesspolicy.Input {
	name := ""
	if kind == "department" {
		name = "Departamento " + dept
	}
	return accesspolicy.Input{OperationID: op, CurrentPassword: "senha-forte-de-teste-1", Kind: kind, DepartmentID: dept, TargetID: target, Permission: identity.Permission(permission), Value: value, Name: name, ExpectedRevision: revision, Reason: "Configuração solicitada pelo dono"}
}
func postPolicy(t *testing.T, app *fiber.App, token string, in accesspolicy.Input, want int) []byte {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	status, body := request(t, app, "POST", "/local/v1/access/policy", string(raw), token)
	if status != want {
		t.Fatalf("policy %s: %d expected %d: %s", in.Kind, status, want, body)
	}
	return body
}
func policyPerson(t *testing.T, db *sql.DB, o localsetup.Result, id, role string) {
	t.Helper()
	for _, q := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO identities VALUES (?,?,'now')`, []any{id, id}},
		{`INSERT INTO memberships VALUES (?,?,?,'active','now')`, []any{o.TenantID, id, role}},
		{`INSERT INTO membership_stores VALUES (?,?,?)`, []any{o.TenantID, id, o.StoreID}},
	} {
		if _, err := db.Exec(q.query, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := localauth.HashPassword("senha-forte-de-teste-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO local_passwords VALUES (?, ?,?,'now')`, o.TenantID, id, hash); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPPolicyEffectiveGrantDenyAndStoreIsolation(t *testing.T) {
	db, app, o := fixture(t)
	token := loginToken(t, app, o.OwnerID)
	policyPerson(t, db, o, "employee", "employee")
	employee := loginToken(t, app, "employee")
	postPolicy(t, app, token, policyInput("dept", "department", "rh", "", "", "active", 0), 200)
	postPolicy(t, app, token, policyInput("assign", "member", "rh", "employee", "", "assigned", 0), 200)
	status, _ := request(t, app, "GET", "/local/v1/products", "", employee)
	if status != 403 {
		t.Fatalf("ungranted read %d", status)
	}
	grant := policyInput("grant", "rule", "rh", "", "view_catalog", "allow", 0)
	postPolicy(t, app, token, grant, 200)
	status, _ = request(t, app, "GET", "/local/v1/products", "", employee)
	if status != 200 {
		t.Fatalf("department grant not effective %d", status)
	}
	status, body := request(t, app, "GET", "/local/v1/capabilities", "", employee)
	if status != 200 || !bytes.Contains(body, []byte("view_catalog")) {
		t.Fatal("capabilities omitted effective grant")
	}
	postPolicy(t, app, token, policyInput("deny", "rule", "rh", "employee", "view_catalog", "deny", 0), 200)
	status, _ = request(t, app, "GET", "/local/v1/products", "", employee)
	if status != 403 {
		t.Fatal("deny did not win")
	}
	postPolicy(t, app, token, policyInput("inherit", "rule", "rh", "employee", "view_catalog", "inherit", 1), 200)
	status, _ = request(t, app, "GET", "/local/v1/products", "", employee)
	if status != 200 {
		t.Fatal("inherit did not restore department grant")
	}
	// Store link is still required, even when the department grants an action.
	other := identity.Scope{TenantID: o.TenantID, StoreID: "foreign-store", IdentityID: "employee"}
	if err := identity.Can(context.Background(), db, other, identity.ViewCatalog); err == nil {
		t.Fatal("grant crossed store")
	}
	// A grant never creates a module contract or verifier.
	postPolicy(t, app, token, policyInput("stock", "rule", "rh", "employee", "manage_stock", "allow", 0), 200)
	noVerifier, err := New(db, identity.DeviceContext{TenantID: o.TenantID, StoreID: o.StoreID, DeviceID: o.DeviceID})
	if err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, noVerifier, "POST", "/local/v1/products", `{"sku":"x","name":"x","unit":"unit","price_cents":100,"cost_cents":50}`, employee)
	if status != 503 {
		t.Fatalf("grant bypassed verifier: %d", status)
	}
	// Existing session observes policy revocation without another login.
	postPolicy(t, app, token, policyInput("removegrant", "rule", "rh", "", "view_catalog", "inherit", 1), 200)
	status, _ = request(t, app, "GET", "/local/v1/products", "", employee)
	if status != 403 {
		t.Fatal("stale session retained authorization")
	}
}

func TestHTTPPolicyDelegationHasDepartmentAndPermissionCeiling(t *testing.T) {
	db, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	for _, id := range []string{"rh", "employee", "other"} {
		policyPerson(t, db, o, id, "employee")
	}
	for _, dept := range []string{"rh", "production"} {
		postPolicy(t, app, owner, policyInput("create-"+dept, "department", dept, "", "", "active", 0), 200)
	}
	postPolicy(t, app, owner, policyInput("assign-rh", "member", "rh", "rh", "", "assigned", 0), 200)
	postPolicy(t, app, owner, policyInput("grant-rh", "rule", "rh", "rh", "view_catalog", "allow", 0), 200)
	postPolicy(t, app, owner, policyInput("delegate", "delegation", "rh", "rh", "", "active", 0), 200)
	rh := loginToken(t, app, "rh")
	postPolicy(t, app, rh, policyInput("assign-employee", "member", "rh", "employee", "", "assigned", 0), 200)
	postPolicy(t, app, rh, policyInput("grant-employee", "rule", "rh", "employee", "view_catalog", "allow", 0), 200)
	postPolicy(t, app, rh, policyInput("too-much", "rule", "rh", "employee", "manage_stock", "allow", 0), 403)
	postPolicy(t, app, rh, policyInput("staff-root", "rule", "rh", "employee", "manage_staff", "allow", 0), 403)
	postPolicy(t, app, rh, policyInput("self", "rule", "rh", "rh", "view_catalog", "deny", 1), 403)
	postPolicy(t, app, rh, policyInput("newdelegate", "delegation", "rh", "employee", "", "active", 0), 403)
	postPolicy(t, app, rh, policyInput("allgroup", "rule", "rh", "", "view_catalog", "allow", 0), 403)
	postPolicy(t, app, rh, policyInput("otherdept", "member", "production", "other", "", "assigned", 0), 403)
	postPolicy(t, app, owner, policyInput("placeother", "member", "production", "other", "", "assigned", 0), 200)
	postPolicy(t, app, rh, policyInput("moveother", "member", "rh", "other", "", "assigned", 1), 403)
	for _, path := range []string{"/access/policy", "/access/audit", "/access/policy?department_id=production", "/access/audit?department_id=production"} {
		status, _ := request(t, app, "GET", "/local/v1"+path, "", rh)
		if status != 403 {
			t.Fatalf("delegate read %s: %d", path, status)
		}
	}
	status, body := request(t, app, "GET", "/local/v1/access/policy?department_id=rh", "", rh)
	if status != 200 || bytes.Contains(body, []byte("production")) || bytes.Contains(body, []byte(`"other"`)) {
		t.Fatal("department configuration leaked")
	}
	postPolicy(t, app, owner, policyInput("revoke-delegate", "delegation", "rh", "rh", "", "revoked", 1), 200)
	postPolicy(t, app, rh, policyInput("after-revoke", "rule", "rh", "employee", "view_catalog", "deny", 1), 403)
	status, _ = request(t, app, "GET", "/local/v1/access/audit?department_id=rh", "", rh)
	if status != 403 {
		t.Fatal("revoked delegate read audit")
	}
}

func TestHTTPPolicyRevisionRetryAndAuditRollback(t *testing.T) {
	db, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	first := policyInput("create", "department", "rh", "", "", "active", 0)
	postPolicy(t, app, owner, first, 200)
	body := postPolicy(t, app, owner, first, 200)
	if !bytes.Contains(body, []byte(`"repeated":true`)) {
		t.Fatal("retry not detected")
	}
	modified := first
	modified.Name = "Altered"
	postPolicy(t, app, owner, modified, 409)
	stale := policyInput("stale", "department", "rh", "", "", "inactive", 0)
	postPolicy(t, app, owner, stale, 409)
	for _, mode := range []string{"ABORT,'forced'", "IGNORE"} {
		if _, err := db.Exec(`CREATE TEMP TRIGGER fail_policy BEFORE INSERT ON access_policy_events BEGIN SELECT RAISE(` + mode + `); END`); err != nil {
			t.Fatal(err)
		}
		postPolicy(t, app, owner, policyInput("failure", "department", "rh", "", "", "inactive", 1), 500)
		var status string
		var rev int
		if err := db.QueryRow(`SELECT status,revision FROM access_departments WHERE id='rh'`).Scan(&status, &rev); err != nil || status != "active" || rev != 1 {
			t.Fatal("policy survived audit rollback")
		}
		if _, err := db.Exec(`DROP TRIGGER fail_policy`); err != nil {
			t.Fatal(err)
		}
	}
	postPolicy(t, app, owner, policyInput("update", "department", "rh", "", "", "inactive", 1), 200)
	postPolicy(t, app, owner, first, 200) // original retry returns original revision; no rewrite
	var revision int
	if err := db.QueryRow(`SELECT revision FROM access_departments WHERE id='rh'`).Scan(&revision); err != nil || revision != 2 {
		t.Fatal("retry rewrote policy")
	}
	wrong := policyInput("wrong", "department", "new", "", "", "active", 0)
	wrong.CurrentPassword = "incorrect-password"
	postPolicy(t, app, owner, wrong, 401)
	var count int
	db.QueryRow(`SELECT count(*) FROM access_policy_events`).Scan(&count)
	if count != 2 {
		t.Fatalf("unexpected audit count %d", count)
	}
	status, body := request(t, app, "GET", "/local/v1/access/audit?limit=1", "", owner)
	if status != 200 || bytes.Contains(body, []byte(owner)) || bytes.Contains(body, []byte(first.CurrentPassword)) || bytes.Contains(body, []byte("request_hash")) {
		t.Fatalf("unsafe audit %d %s", status, body)
	}
	var page accesspolicy.AuditPage
	if json.Unmarshal(body, &page) != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatal("audit pagination missing")
	}
	status, body = request(t, app, "GET", "/local/v1/access/audit?limit=1&cursor="+page.NextCursor, "", owner)
	var second accesspolicy.AuditPage
	if status != 200 || json.Unmarshal(body, &second) != nil || len(second.Items) != 1 || second.Items[0].Reference == page.Items[0].Reference || second.NextCursor != "" {
		t.Fatal("audit duplicate/skip")
	}
	status, _ = request(t, app, "GET", "/local/v1/access/audit?department_id=rh&cursor="+page.NextCursor, "", owner)
	if status != 400 {
		t.Fatal("cross-scope cursor accepted")
	}
}

func TestHTTPPolicyRejectsAmbiguityForeignTargetsAndUnknownQueries(t *testing.T) {
	db, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	in := policyInput("create", "department", "rh", "", "", "active", 0)
	raw, _ := json.Marshal(in)
	for _, body := range []string{string(bytes.Replace(raw, []byte(`"kind":`), []byte(`"kind":"rule","kind":`), 1)), string(bytes.Replace(raw, []byte(`"expected_revision":0`), []byte(`"expected_revision":null`), 1)), string(bytes.Replace(raw, []byte(`"reason":`), []byte(`"tenant_id":"foreign","reason":`), 1)), string(raw) + "{}"} {
		status, _ := request(t, app, "POST", "/local/v1/access/policy", body, owner)
		if status != 400 {
			t.Fatalf("bad JSON %d", status)
		}
	}
	postPolicy(t, app, owner, in, 200)
	postPolicy(t, app, owner, policyInput("owner", "member", "rh", o.OwnerID, "", "assigned", 0), 403)
	postPolicy(t, app, owner, policyInput("foreign", "member", "rh", "absent", "", "assigned", 0), 403)
	postPolicy(t, app, owner, policyInput("unknown", "rule", "rh", "", "root", "allow", 0), 400)
	for _, path := range []string{"/access/audit?limit=01", "/access/audit?limit=101", "/access/audit?limit=1&limit=2", "/access/policy?tenant_id=x", "/access/audit?cursor=bad"} {
		status, _ := request(t, app, "GET", "/local/v1"+path, "", owner)
		if status != 400 {
			t.Fatalf("bad query %s %d", path, status)
		}
	}
	if _, err := db.Exec(`UPDATE device_pairings SET status='revoked' WHERE device_id=?`, o.DeviceID); err != nil {
		t.Fatal(err)
	}
	status, _ := request(t, app, "GET", "/local/v1/access/policy", "", owner)
	if status != 401 {
		t.Fatal("revoked device read policy")
	}
}

func TestPolicyConcurrentSameRevisionCannotOverwrite(t *testing.T) {
	db, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	device := identity.DeviceContext{TenantID: o.TenantID, StoreID: o.StoreID, DeviceID: o.DeviceID}
	postPolicy(t, app, owner, policyInput("create", "department", "rh", "", "", "active", 0), 200)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := accesspolicy.Mutate(context.Background(), db, device, owner, policyInput(fmt.Sprint("update", i), "department", "rh", "", "", "inactive", 1))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if err == accesspolicy.ErrConflict {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent revision overwrite")
	}
}

func TestHTTPPolicyPersistsAfterReopenAndInactiveDepartmentKeepsDenial(t *testing.T) {
	db, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	policyPerson(t, db, o, "cashier", "cashier")
	cashier := loginToken(t, app, "cashier")
	postPolicy(t, app, owner, policyInput("create", "department", "cash", "", "", "active", 0), 200)
	postPolicy(t, app, owner, policyInput("assign", "member", "cash", "cashier", "", "assigned", 0), 200)
	postPolicy(t, app, owner, policyInput("deny", "rule", "cash", "", "sell", "deny", 0), 200)
	postPolicy(t, app, owner, policyInput("inactive", "department", "cash", "", "", "inactive", 1), 200)
	actor := identity.Scope{TenantID: o.TenantID, StoreID: o.StoreID, IdentityID: "cashier"}
	if err := identity.Can(context.Background(), db, actor, identity.Sell); err == nil {
		t.Fatal("deactivation removed denial over role")
	}
	var sequence int
	var name, path string
	if err := db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	device := identity.DeviceContext{TenantID: o.TenantID, StoreID: o.StoreID, DeviceID: o.DeviceID}
	next, err := New(reopened, device)
	if err != nil {
		t.Fatal(err)
	}
	status, body := request(t, next, "GET", "/local/v1/capabilities", "", cashier)
	if status != 200 || bytes.Contains(body, []byte(`"sell"`)) {
		t.Fatal("reopen lost denial")
	}
	status, body = request(t, next, "GET", "/local/v1/access/audit", "", owner)
	var page accesspolicy.AuditPage
	if status != 200 || json.Unmarshal(body, &page) != nil || len(page.Items) != 4 {
		t.Fatal("audit did not survive reopen")
	}
	postPolicy(t, next, owner, policyInput("reactivate", "department", "cash", "", "", "active", 2), 200)
	postPolicy(t, next, owner, policyInput("inherit", "rule", "cash", "", "sell", "inherit", 1), 200)
	if err := identity.Can(context.Background(), reopened, actor, identity.Sell); err != nil {
		t.Fatal("role not restored after explicit inherit")
	}
}

func TestHTTPPolicyIgnoredMutationNeverCreatesSuccessfulAudit(t *testing.T) {
	db, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	if _, err := db.Exec(`CREATE TEMP TRIGGER ignore_department BEFORE INSERT ON access_departments BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	postPolicy(t, app, owner, policyInput("ignored", "department", "rh", "", "", "active", 0), 500)
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM access_policy_events`).Scan(&count); err != nil || count != 0 {
		t.Fatal("ignored mutation emitted audit")
	}
}

func TestHTTPPolicyAuditSourcesAreSanitizedAndStoreScoped(t *testing.T) {
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Staff}), 200)
	db, app, o, owner := f.db, f.app, f.owner, f.token
	if _, err := db.Exec(`INSERT INTO account_security_events VALUES ('event',?,?,?,?,'sessions_revoked',0,123)`, o.TenantID, o.StoreID, o.DeviceID, o.OwnerID); err != nil {
		t.Fatal(err)
	}
	status, body := request(t, app, "POST", "/local/v1/staff", `{"operation_id":"staff1","identity_id":"staff1","name":"Staff","role":"employee","password":"staff-test-password"}`, owner)
	if status != 201 {
		t.Fatalf("staff setup %d %s", status, body)
	}
	postPolicy(t, app, owner, policyInput("dept", "department", "rh", "", "", "active", 0), 200)
	status, body = request(t, app, "POST", "/local/v1/staff/staff1/sessions/revoke", `{"operation_id":"revoke1","current_password":"senha-forte-de-teste-1","reason":"Revoke access"}`, owner)
	if status != 200 {
		t.Fatalf("revoke setup %d %s", status, body)
	}
	status, body = request(t, app, "GET", "/local/v1/access/audit", "", owner)
	var page accesspolicy.AuditPage
	if status != 200 || json.Unmarshal(body, &page) != nil || len(page.Items) != 4 {
		t.Fatalf("audit union %d %s", status, body)
	}
	for _, secret := range []string{"staff-test-password", "senha-forte-de-teste-1", "request_hash", "password_hash", "token_sha256", owner} {
		if bytes.Contains(body, []byte(secret)) {
			t.Fatal("secret exposed in audit")
		}
	}
	// A second store's events never appear, even for an owner with a global role.
	if _, err := db.Exec(`INSERT INTO stores VALUES (?,'other','Other')`, o.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO devices VALUES (?,'other','other-device','Other')`, o.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO account_security_events VALUES ('other-event',?,'other','other-device',?,'sessions_revoked',0,123)`, o.TenantID, o.OwnerID); err != nil {
		t.Fatal(err)
	}
	status, body = request(t, app, "GET", "/local/v1/access/audit", "", owner)
	if status != 200 || bytes.Contains(body, []byte("other-event")) {
		t.Fatal("audit crossed store")
	}
}

func TestHTTPPolicyBodyLimitsAndMutationRate(t *testing.T) {
	_, app, o := fixture(t)
	owner := loginToken(t, app, o.OwnerID)
	raw, _ := json.Marshal(policyInput("create", "department", "rh", "", "", "active", 0))
	req := httptest.NewRequest("POST", "/local/v1/access/policy", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+owner)
	req.Header.Set("Content-Type", "text/plain")
	response, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 415 {
		t.Fatal("incorrect media type accepted")
	}
	status, _ := request(t, app, "POST", "/local/v1/access/policy", string(bytes.Repeat([]byte("x"), 4097)), owner)
	if status != 413 {
		t.Fatal("body limit ignored")
	}
	status, _ = request(t, app, "POST", "/local/v1/access/policy", string(raw), "")
	if status != 401 {
		t.Fatal("anonymous mutation accepted")
	}
	// The first two authenticated requests already used two of the twenty slots.
	for i := 0; i < 18; i++ {
		status, _ = request(t, app, "POST", "/local/v1/access/policy", "{}", owner)
		if status != 400 {
			t.Fatalf("mutation limit changed at %d: %d", i, status)
		}
	}
	status, _ = request(t, app, "POST", "/local/v1/access/policy", string(raw), owner)
	if status != 429 {
		t.Fatal("mutation rate not limited")
	}
}
