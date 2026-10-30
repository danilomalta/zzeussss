package fiscalproduction

import "titansystem-backend/internal/localdb/fiscal"

// OnboardingDiagnostic is an allowlist of offline checks. It never contains
// credentials, paths, provider responses, or evidence of official authorization.
type OnboardingDiagnostic struct {
	ProfileRevision       int      `json:"profile_revision"`
	ConfigurationValid    bool     `json:"configuration_valid"`
	ProductionBlocked     bool     `json:"production_blocked"`
	TransmissionPerformed bool     `json:"transmission_performed"`
	OfficialAuthorization bool     `json:"official_authorization"`
	Blockers              []string `json:"blockers"`
}

func DiagnoseOnboarding(r *ProviderRegistry, runtime *Runtime, a RequestContext, p fiscal.Profile) OnboardingDiagnostic {
	out := OnboardingDiagnostic{ProfileRevision: p.Revision, ProductionBlocked: true, Blockers: []string{}}
	configured := false
	if r != nil {
		provider, b, err := r.Resolve(a)
		if err == nil {
			c := r.entries[a.Actor.TenantID].Configuration
			configured = b.ProfileID == p.Input.ProfileID && b.ProfileRevision == p.Revision && c.Environment == p.Input.Environment && c.CRT == p.Input.CRT
			own, ok := provider.(*OwnProvider)
			if !ok {
				configured = false
			} else {
				runtime = own.runtime
			}
		} else {
			runtime = nil
		}
	}
	b := runtime.Binding()
	if r == nil {
		configured = b.TenantID == a.Actor.TenantID && b.StoreID == a.Actor.StoreID && b.DeviceID == a.Device.DeviceID && a.Device.TenantID == a.Actor.TenantID && a.Device.StoreID == a.Actor.StoreID && b.ProfileID == p.Input.ProfileID && b.ProfileRevision == p.Revision && p.Input.Environment == "production"
	}
	out.ConfigurationValid = configured && b.Model == p.Input.Model && b.UF == p.Input.State && b.Series == p.Input.Series && b.Issuer == p.Input.CNPJ && runtime.Validate() == nil
	if !out.ConfigurationValid {
		out.Blockers = append(out.Blockers, "onboarding_configuration_required")
	}
	out.Blockers = append(out.Blockers, "onboarding_official_verification_required", "onboarding_production_blocked")
	return out
}
