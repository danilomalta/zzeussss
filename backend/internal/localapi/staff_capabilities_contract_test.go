package localapi

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
)

func staffContractSchema(t *testing.T, name string, body []byte) {
	t.Helper()
	domainSchemaFile(t, "staff-capabilities.openapi.json", name, body)
}

func TestStaffCapabilitiesOpenAPIRegistrationAndContractReplay(t *testing.T) {
	f := httpContractSetup(t)
	env := signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Staff})
	input, e := json.Marshal(env)
	if e != nil {
		t.Fatal(e)
	}
	staffContractSchema(t, "ContractEnvelope", input)
	for i := 0; i < 2; i++ {
		status, b := request(t, f.app, "POST", "/local/v1/module-contracts", string(input), f.token)
		if status != 200 {
			t.Fatal("contract status", status)
		}
		staffContractSchema(t, "ContractResult", b)
		var result struct {
			Repeated bool `json:"repeated"`
		}
		_ = json.Unmarshal(b, &result)
		if result.Repeated != (i == 1) {
			t.Fatal("wrong contract replay")
		}
	}
	staffContractSchema(t, "StaffInput", []byte(staffBody))
	for _, want := range []int{201, 200} {
		status, b := request(t, f.app, "POST", "/local/v1/staff", staffBody, f.token)
		if status != want {
			t.Fatal("staff status", status)
		}
		staffContractSchema(t, "StaffResult", b)
		if bytes.Contains(b, []byte("password")) || bytes.Contains(b, []byte("senha-forte")) {
			t.Fatal("credential in response")
		}
	}
	status, b := request(t, f.app, "GET", "/local/v1/staff", "", f.token)
	if status != 200 {
		t.Fatal(status)
	}
	staffContractSchema(t, "StaffPage", b)
	if bytes.Contains(b, []byte("password")) || bytes.Contains(b, []byte("$2")) {
		t.Fatal("credential in list")
	}
	status, b = request(t, f.app, "GET", "/local/v1/capabilities", "", f.token)
	if status != 200 {
		t.Fatal(status)
	}
	staffContractSchema(t, "Capabilities", b)
	var caps struct {
		Permissions []string `json:"permissions"`
		License     struct {
			Modules []string `json:"modules"`
		} `json:"license"`
	}
	if json.Unmarshal(b, &caps) != nil {
		t.Fatal("invalid capabilities")
	}
	sell := false
	for _, p := range caps.Permissions {
		if p == "sell" {
			sell = true
		}
	}
	if !sell {
		t.Fatal("owner permission missing")
	}
	for _, m := range caps.License.Modules {
		if m == "pos" {
			t.Fatal("invented POS contract")
		}
	}
	// Permission in capabilities must never bypass the actual POS contract.
	status, _ = request(t, f.app, "POST", "/local/v1/cash/open", `{"session_id":"no-pos","opening_cents":0}`, f.token)
	if status != 403 {
		t.Fatal("unlicensed cash accepted", status)
	}
}

func TestStaffCapabilitiesOpenAPILicenseStates(t *testing.T) {
	for _, state := range []string{"active", "not_installed", "unavailable", "invalid", "clock_blocked", "expired", "not_yet_valid"} {
		t.Run(state, func(t *testing.T) {
			f := httpContractSetup(t)
			app := f.app
			if state == "unavailable" {
				var e error
				app, e = New(f.db, f.device)
				if e != nil {
					t.Fatal(e)
				}
			}
			if state != "not_installed" && state != "unavailable" {
				f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Staff}), 200)
				if state == "invalid" {
					if _, e := f.db.Exec(`UPDATE module_contract_history SET signature=zeroblob(64)`); e != nil {
						t.Fatal(e)
					}
				}
				if state == "clock_blocked" {
					if _, e := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=last_observed_unix+10000`); e != nil {
						t.Fatal(e)
					}
				}
				if state == "expired" || state == "not_yet_valid" {
					now := time.Now().Unix()
					nb, exp := now-120, now-60
					if state == "not_yet_valid" {
						nb, exp = now+600, now+1200
					}
					payload, e := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1, IssuedAt: now - 180, NotBefore: nb, ExpiresAt: exp, Modules: []modules.ID{modules.Core, modules.Staff}})
					if e != nil {
						t.Fatal(e)
					}
					sig := ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload))
					if _, e = f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=?`, payload, sig); e != nil {
						t.Fatal(e)
					}
				}
			}
			status, b := request(t, app, "GET", "/local/v1/capabilities", "", f.token)
			if status != 200 {
				t.Fatal("capabilities status", status)
			}
			staffContractSchema(t, "Capabilities", b)
			var v struct {
				License struct {
					State     string   `json:"state"`
					Modules   []string `json:"modules"`
					Remaining int64    `json:"remaining_days"`
					Expires   int64    `json:"expires_unix"`
				} `json:"license"`
			}
			if json.Unmarshal(b, &v) != nil || v.License.State != state {
				t.Fatal("unexpected license state", v.License.State)
			}
			if state == "active" {
				if v.License.Remaining < 1 || v.License.Expires == 0 {
					t.Fatal("active validity missing")
				}
			} else if v.License.Remaining != 0 {
				t.Fatal("inactive remaining days")
			}
			trusted := state == "active" || state == "expired" || state == "not_yet_valid"
			if trusted && len(v.License.Modules) != 2 {
				t.Fatal("authenticated metadata missing")
			}
			if !trusted && (len(v.License.Modules) != 0 || v.License.Expires != 0) {
				t.Fatal("untrusted license metadata exposed")
			}
			status, _ = request(t, app, "GET", "/local/v1/staff", "", f.token)
			want := 403
			if state == "clock_blocked" {
				want = 409
			}
			if state == "active" {
				want = 200
			}
			if state == "unavailable" {
				want = 503
			}
			// Corruption is an internal error for the business endpoint, not an entitlement.
			if state == "invalid" {
				if status < 400 {
					t.Fatal("invalid license allowed staff")
				}
			} else if status != want {
				t.Fatal("staff gate", state, status, want)
			}
		})
	}
}
