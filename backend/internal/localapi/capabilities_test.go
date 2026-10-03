package localapi

import (
	"bytes"
	"testing"
	"titansystem-backend/internal/core/modules"
)

func TestHTTPCapabilitiesDoNotGrantPOSWithProductionContract(t *testing.T) {
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory, modules.Production}), 200)
	status, body := request(t, f.app, "GET", "/local/v1/capabilities", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"production"`)) || bytes.Contains(body, []byte(`"pos"`)) {
		t.Fatalf("production capabilities: %d %s", status, body)
	}
	if !bytes.Contains(body, []byte(`"state":"active"`)) || !bytes.Contains(body, []byte(`"remaining_days"`)) {
		t.Fatalf("license missing: %s", body)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=?`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	status, body = request(t, f.app, "GET", "/local/v1/capabilities", "", f.token)
	if status != 200 || bytes.Contains(body, []byte(`"manage_staff"`)) || bytes.Contains(body, []byte(`"manage_stock"`)) || bytes.Contains(body, []byte(`"view_accounting"`)) {
		t.Fatalf("cashier privileges: %d %s", status, body)
	}
	status, _ = request(t, f.app, "GET", "/local/v1/capabilities", "", "")
	if status != 401 {
		t.Fatalf("anonymous: %d", status)
	}
}

func TestHTTPCapabilitiesWithoutVerifierNeverInventsModules(t *testing.T) {
	f := httpContractSetup(t)
	app, err := New(f.db, f.device)
	if err != nil {
		t.Fatal(err)
	}
	status, body := request(t, app, "GET", "/local/v1/capabilities", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"modules":[]`)) || !bytes.Contains(body, []byte(`"state":"unavailable"`)) {
		t.Fatalf("no verifier: %d %s", status, body)
	}
}

func TestHTTPCapabilitiesTamperedSignatureAndClockCannotEnableAreas(t *testing.T) {
	for _, query := range []string{`UPDATE module_contract_history SET signature=zeroblob(64)`, `UPDATE module_contract_state SET last_observed_unix=last_observed_unix+10000`} {
		f := httpContractSetup(t)
		f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS}), 200)
		if _, err := f.db.Exec(query); err != nil {
			t.Fatal(err)
		}
		status, body := request(t, f.app, "GET", "/local/v1/capabilities", "", f.token)
		if status != 200 || !bytes.Contains(body, []byte(`"modules":[]`)) {
			t.Fatalf("untrusted capabilities: %d %s", status, body)
		}
	}
}
