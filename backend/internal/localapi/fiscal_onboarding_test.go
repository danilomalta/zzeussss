package localapi

import (
	"encoding/json"
	"testing"
	"titansystem-backend/internal/fiscalproduction"
)

func TestHTTPFiscalOnboarding(t *testing.T) {
	f := fiscalFixture(t)
	for _, tc := range []struct {
		path, body, token string
		want              int
	}{
		{"/local/v1/fiscal/onboarding/issuer-one", "", f.token, 200},
		{"/local/v1/fiscal/onboarding/issuer-one?token=secret", "", f.token, 400},
		{"/local/v1/fiscal/onboarding/issuer-one", `{"secret":"sensitive"}`, f.token, 400},
		{"/local/v1/fiscal/onboarding/missing", "", f.token, 404},
		{"/local/v1/fiscal/onboarding/issuer-one", "", "", 401},
	} {
		code, body := request(t, f.app, "GET", tc.path, tc.body, tc.token)
		if code != tc.want {
			t.Fatalf("status %d want %d", code, tc.want)
		}
		if code == 200 {
			var out fiscalproduction.OnboardingDiagnostic
			if json.Unmarshal(body, &out) != nil || out.ConfigurationValid || !out.ProductionBlocked || out.TransmissionPerformed || out.OfficialAuthorization || out.ProfileRevision != 1 || len(out.Blockers) != 3 {
				t.Fatal("unsafe diagnostic")
			}
			var fields map[string]any
			json.Unmarshal(body, &fields)
			if len(fields) != 6 {
				t.Fatal("unexpected response fields")
			}
		}
	}
	code, _ := request(t, f.app, "POST", "/local/v1/fiscal/onboarding", fiscalProfileBody, f.token)
	if code != 200 {
		t.Fatalf("replay status %d", code)
	}
	code, _ = request(t, f.app, "POST", "/local/v1/fiscal/onboarding", `{"token":"secret"}`, f.token)
	if code != 400 {
		t.Fatalf("secret input status %d", code)
	}
}

func TestHTTPFiscalOnboardingPermissions(t *testing.T) {
	f := fiscalFixture(t)
	for _, role := range []string{"manager", "accountant", "cashier"} {
		code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"onboarding-`+role+`","identity_id":"`+role+`","name":"Test staff","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
		if code != 201 {
			t.Fatalf("staff %d", code)
		}
		code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"`+role+`","password":"test-password-strong-1"}`, "")
		if code != 200 {
			t.Fatalf("login %d", code)
		}
		var auth struct {
			Token string `json:"token"`
		}
		json.Unmarshal(body, &auth)
		code, _ = request(t, f.app, "POST", "/local/v1/fiscal/onboarding", fiscalProfileBody, auth.Token)
		if code != 403 {
			t.Fatalf("%s configured onboarding: %d", role, code)
		}
		code, _ = request(t, f.app, "GET", "/local/v1/fiscal/onboarding/issuer-one", "", auth.Token)
		want := 200
		if role == "cashier" {
			want = 403
		}
		if code != want {
			t.Fatalf("%s diagnostic: %d want %d", role, code, want)
		}
	}
}

func TestHTTPFiscalOnboardingDoesNotTransmit(t *testing.T) {
	p := newProductionFixture(t, "55")
	code, body := request(t, p.f.app, "GET", "/local/v1/fiscal/onboarding/issuer-one", "", p.f.token)
	if code != 200 {
		t.Fatalf("diagnostic %d", code)
	}
	var out fiscalproduction.OnboardingDiagnostic
	if json.Unmarshal(body, &out) != nil || !out.ConfigurationValid || !out.ProductionBlocked || out.OfficialAuthorization || out.TransmissionPerformed {
		t.Fatal("invalid offline diagnostic")
	}
	if p.issues.Load() != 0 {
		t.Fatal("diagnostic transmitted a document")
	}
}
