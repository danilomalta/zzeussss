package localapi

import (
	"encoding/json"
	"testing"
)

func TestHTTPNFe55CorrectionRequiresOwnerSessionAndContract(t *testing.T) {
	f := fiscalFixture(t, "55")
	p := &nfe55ContractProvider{}
	nfe55ContractApp(t, f, p, "homologation")
	path := "/local/v1/fiscal/nfe55/sales/missing/correction"
	body := `{"correction":"Synthetic correction only","confirmed_allowed_fields":true}`
	if code, _ := request(t, f.app, "POST", path, body, f.token); code != 409 {
		t.Fatal("owner must reach unsupported contract provider", code)
	}
	for _, role := range []string{"manager", "accountant", "cashier"} {
		code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"cce-`+role+`","identity_id":"cce-`+role+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
		if code != 201 {
			t.Fatal("staff", code)
		}
		code, raw := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"cce-`+role+`","password":"test-password-strong-1"}`, "")
		if code != 200 {
			t.Fatal("login", code)
		}
		var auth struct {
			Token string `json:"token"`
		}
		json.Unmarshal(raw, &auth)
		if code, _ = request(t, f.app, "POST", path, body, auth.Token); code != 403 {
			t.Fatal(role, code)
		}
	}
	if code, _ := request(t, f.app, "POST", path, body, ""); code != 401 {
		t.Fatal("session", code)
	}
	nfe55ContractApp(t, f, p, "production")
	if code, _ := request(t, f.app, "POST", path, body, f.token); code != 409 {
		t.Fatal("production", code)
	}
}
