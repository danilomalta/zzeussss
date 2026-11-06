package localapi

import (
	"bytes"
	"encoding/json"
	"testing"

	"titansystem-backend/internal/core/modules"
)

// A signed removal disables new commands, not the customer's historical cash
// journal, authenticated reads, company registration, or account access.
func TestHTTPLicenseWithdrawalPreservesCashAndPrivateDiagnostic(t *testing.T) {
	f := movementSetup(t)
	cashRequest(t, f, "movements", supplyBody, 200)
	code, original := request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if code != 200 {
		t.Fatal("initial cash read", code)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core}), 200)
	cashRequest(t, f, "movements", withdrawalBody, 403)
	movementCounts(t, f, 1)
	code, after := request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if code != 200 || !bytes.Equal(original, after) {
		t.Fatal("license removal changed cash history or blocked read", code)
	}
	code, body := request(t, f.app, "GET", "/local/v1/capabilities", "", f.token)
	if code != 200 {
		t.Fatal("diagnostic", code)
	}
	var result struct {
		License map[string]json.RawMessage `json:"license"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.License) != 4 {
		t.Fatal("unexpected license metadata")
	}
	for _, name := range []string{"state", "modules", "expires_unix", "remaining_days"} {
		if _, ok := result.License[name]; !ok {
			t.Fatal("missing metadata", name)
		}
	}
	if bytes.Contains(body, []byte(f.token)) || bytes.Contains(body, []byte("Troco inicial")) || bytes.Contains(body, []byte("signature")) || bytes.Contains(body, []byte("password")) {
		t.Fatal("diagnostic leaked non-metadata")
	}
	if code, _ = request(t, f.app, "GET", "/local/v1/capabilities", "", ""); code != 401 {
		t.Fatal("public diagnostic", code)
	}
	if code, _ = request(t, f.app, "GET", "/local/v1/company/registration", "", f.token); code != 200 {
		t.Fatal("registration requires license renewal", code)
	}
	// Another installation's session never supplies a role or company context.
	other := httpContractSetup(t)
	if code, body = request(t, f.app, "GET", "/local/v1/capabilities", "", other.token); code != 401 || bytes.Contains(body, []byte(f.owner.TenantID)) {
		t.Fatal("foreign session diagnostic", code)
	}
}

func TestHTTPIgnoredLicenseWriteFailsClosedWithoutCashMutation(t *testing.T) {
	f := movementSetup(t)
	if _, err := f.db.Exec(`CREATE TRIGGER ignore_license BEFORE UPDATE ON module_contract_state BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core}), 500)
	cashRequest(t, f, "movements", supplyBody, 500)
	movementCounts(t, f, 0)
	assertHTTPCounts(t, f, 0, 1)
	code, body := request(t, f.app, "GET", "/local/v1/capabilities", "", f.token)
	if code != 200 || !bytes.Contains(body, []byte(`"state":"invalid"`)) || bytes.Contains(body, []byte(`"state":"active"`)) || bytes.Contains(body, []byte(f.token)) {
		t.Fatal("ignored observation announced active license or leaked token", code)
	}
}
