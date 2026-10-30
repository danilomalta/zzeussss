package fiscalproduction

import "titansystem-backend/internal/localdb/fiscal"

// ProviderDiagnostic exposes only local configuration and credential presence.
type ProviderDiagnostic struct {
	Provider        string `json:"provider"`
	Environment     string `json:"environment"`
	CertificateType string `json:"certificate_type"`
	TokenPresent    bool   `json:"token_present"`
	CSCPresent      bool   `json:"csc_present"`
}

// Credential presence is only local process state, never a certificate or an
// issuer authorization check. No secret, path, identifier or remote body escapes.
type OnboardingDiagnostic struct {
	ProviderConfiguration *ProviderDiagnostic `json:"provider_configuration,omitempty"`
	ProfileRevision       int                 `json:"profile_revision"`
	ConfigurationValid    bool                `json:"configuration_valid"`
	ProductionBlocked     bool                `json:"production_blocked"`
	TransmissionPerformed bool                `json:"transmission_performed"`
	OfficialAuthorization bool                `json:"official_authorization"`
	Blockers              []string            `json:"blockers"`
}

func DiagnoseOnboarding(r *ProviderRegistry, runtime *Runtime, a RequestContext, p fiscal.Profile) OnboardingDiagnostic {
	out := OnboardingDiagnostic{ProfileRevision: p.Revision, ProductionBlocked: true, Blockers: []string{}}
	configured := false
	if r != nil {
		provider, b, err := r.Resolve(a)
		if err == nil {
			c := r.entries[a.Actor.TenantID].Configuration
			if c.CustomerManaged {
				if focus, ok := provider.(*FocusProvider); ok && focus != nil {
					c = focus.configuration
				}
			}
			configured = b.ProfileID == p.Input.ProfileID && b.ProfileRevision == p.Revision && c.Environment == p.Input.Environment && c.CRT == p.Input.CRT
			switch selected := provider.(type) {
			case *OwnProvider:
				runtime = selected.runtime
			case *FocusProvider:
				// Never fall back to a different runtime/tenant or issue HTTP.
				expected := c
				if expected.FocusBaseURL == "" {
					expected.FocusBaseURL = focusHomologation
				}
				matches := configured && c.Provider == "external" && selected != nil && selected.configuration == expected &&
					p.Input.Environment == "homologation" && p.Input.Model == "65" &&
					c.CertificateType == "A1" && expected.FocusBaseURL == focusHomologation &&
					len(selected.token) > 0 && fiscal.ValidCNPJ(p.Input.CNPJ)
				if matches {
					out.ConfigurationValid = true
					out.ProviderConfiguration = &ProviderDiagnostic{
						Provider: "external", Environment: c.Environment,
						CertificateType: c.CertificateType,
						TokenPresent:    len(selected.token) > 0, CSCPresent: len(selected.csc) > 0,
					}
					out.Blockers = append(out.Blockers, "onboarding_official_verification_required", "onboarding_production_blocked")
					return out
				}
				configured = false
				runtime = nil
			default:
				configured = false
				runtime = nil
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
