package localapi

import (
	"bytes"
	"testing"
	"titansystem-backend/internal/core/modules"
)

const staffBody = `{"operation_id":"staff-op","identity_id":"staff-one","name":"Funcionario","role":"employee","password":"senha-forte-de-teste"}`

func TestHTTPStaffCreationRepeatsWithoutLeakingCredentials(t *testing.T) {
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Staff}), 200)
	for _, want := range []int{201, 200} {
		status, body := request(t, f.app, "POST", "/local/v1/staff", staffBody, f.token)
		if status != want || bytes.Contains(body, []byte("password")) {
			t.Fatalf("staff: %d %s", status, body)
		}
	}
	status, body := request(t, f.app, "GET", "/local/v1/staff", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte("Funcionario")) || bytes.Contains(body, []byte("password")) {
		t.Fatalf("list: %d %s", status, body)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/staff", string(bytes.ReplaceAll([]byte(staffBody), []byte(`"employee"`), []byte(`"owner"`))), f.token)
	if status != 403 {
		t.Fatalf("owner escalation: %d", status)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/staff", staffBody, "")
	if status != 401 {
		t.Fatalf("anonymous: %d", status)
	}
}
func TestHTTPStaffMissingModuleAndFailedAuditCannotCreateMember(t *testing.T) {
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory}), 200)
	status, _ := request(t, f.app, "POST", "/local/v1/staff", staffBody, f.token)
	if status != 403 {
		t.Fatalf("missing staff: %d", status)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Staff}), 200)
	if _, err := f.db.Exec(`CREATE TRIGGER fail_staff BEFORE INSERT ON staff_registrations BEGIN SELECT RAISE(ABORT,'test'); END`); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/staff", staffBody, f.token)
	if status == 201 || status == 200 {
		t.Fatal("failed audit accepted")
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM identities WHERE id='staff-one'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial identity: %d %v", n, err)
	}
}
