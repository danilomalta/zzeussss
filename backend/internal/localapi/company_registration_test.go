package localapi

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"titansystem-backend/internal/localdb/companyregistration"
)

func TestHTTPCompanyRegistrationManualStagesRecoveryAndIsolation(t *testing.T) {
	f := httpContractSetup(t)
	initial, body := request(t, f.app, "GET", "/local/v1/company/registration", "", f.token)
	if initial != 200 || !bytes.Contains(body, []byte(`"revision":0`)) {
		t.Fatal("initial", initial)
	}
	in := companyregistration.Input{OperationID: "minimum-registration", Stage: "pre_registration", CompanyName: "Synthetic company", Responsible: "Customer owner"}
	data, _ := json.Marshal(in)
	code, body := request(t, f.app, "POST", "/local/v1/company/registration", string(data), f.token)
	if code != 200 {
		t.Fatal("pre", code, string(body))
	}
	replay, repeated := request(t, f.app, "POST", "/local/v1/company/registration", string(data), f.token)
	if replay != 200 || !bytes.Contains(repeated, []byte(`"repeated":true`)) {
		t.Fatal("replay", replay)
	}
	in.OperationID = "full-registration"
	in.ExpectedRevision = 1
	in.Stage = "company_profile"
	in.CNPJ = "11222333000181"
	in.LegalName = "Synthetic Legal"
	in.UF = "SP"
	in.StateRegistration = "123456789"
	in.Municipality = "3550308"
	in.CRT = "1"
	in.Provider = "external"
	in.Environment = "homologation"
	data, _ = json.Marshal(in)
	code, body = request(t, f.app, "POST", "/local/v1/company/registration", string(data), f.token)
	if code != 200 || !bytes.Contains(body, []byte(`"revision":2`)) {
		t.Fatal("complete", code, string(body))
	}
	code, body = request(t, f.app, "GET", "/local/v1/company/registration?operation_id=minimum-registration", "", f.token)
	if code != 200 || !bytes.Contains(body, []byte(`"revision":1`)) {
		t.Fatal("original recovery", code)
	}
	other := httpContractSetup(t)
	code, body = request(t, other.app, "GET", "/local/v1/company/registration?operation_id=full-registration", "", other.token)
	if code != 404 || bytes.Contains(body, []byte("Synthetic Legal")) {
		t.Fatal("tenant isolation", code)
	}
	for _, query := range []string{`UPDATE company_registrations SET request_json='{}'`, `DELETE FROM company_registrations`} {
		if _, err := f.db.ExecContext(context.Background(), query); err == nil {
			t.Fatal("history mutated")
		}
	}
	code, _ = request(t, f.app, "POST", "/local/v1/fiscal/profiles", fiscalProfileBody, f.token)
	if code == 200 || code == 201 {
		t.Fatal("registration granted fiscal module")
	}
	in.OperationID = "stale-registration"
	in.ExpectedRevision = 0
	data, _ = json.Marshal(in)
	if code, _ := request(t, f.app, "POST", "/local/v1/company/registration", string(data), f.token); code != 409 {
		t.Fatal("stale", code)
	}
	for _, bad := range []string{`{"tenant_id":"other"}`, string(data[:len(data)-1]) + `,"token":"not-allowed"}`} {
		if code, _ := request(t, f.app, "POST", "/local/v1/company/registration", bad, f.token); code != 400 {
			t.Fatal("unknown field", code)
		}
	}
	request(t, f.app, "POST", "/local/v1/logout", `{}`, f.token)
	if code, _ := request(t, f.app, "POST", "/local/v1/company/registration", string(data), f.token); code != 401 {
		t.Fatal("logout", code)
	}
}
func TestHTTPCompanyRegistrationRejectsNonOwner(t *testing.T) {
	f := fiscalFixture(t)
	code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"company-manager","identity_id":"company-manager","name":"Synthetic Manager","role":"manager","password":"test-password-strong-1"}`, f.token)
	if code != 201 {
		t.Fatal(code)
	}
	code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"company-manager","password":"test-password-strong-1"}`, "")
	if code != 200 {
		t.Fatal(code)
	}
	var auth struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(body, &auth) != nil {
		t.Fatal("login")
	}
	input := companyregistration.Input{OperationID: "not-owner", Stage: "pre_registration", CompanyName: "Other", Responsible: "Manager"}
	raw, _ := json.Marshal(input)
	for _, method := range []string{"GET", "POST"} {
		data := ""
		if method == "POST" {
			data = string(raw)
		}
		if code, _ := request(t, f.app, method, "/local/v1/company/registration", data, auth.Token); code != 403 {
			t.Fatal("non-owner access", code)
		}
	}
}
