package fiscalproduction

import (
	"context"
	"os"
	"path/filepath"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// Trusted process configuration. Secret references are rejected; secrets live in the vault.
type TenantFiscalConfiguration struct {
	CustomerManaged      bool            `json:"customer_managed,omitempty"`
	CertificateType      string          `json:"certificate_type,omitempty"`
	FocusBaseURL         string          `json:"focus_base_url,omitempty"`
	Binding              ProviderBinding `json:"binding"`
	Provider             string          `json:"provider"`
	Environment          string          `json:"environment"`
	CRT                  string          `json:"crt"`
	CSCReference         string          `json:"csc_reference,omitempty"`
	CertificateReference string          `json:"certificate_reference,omitempty"`
	TokenReference       string          `json:"token_reference,omitempty"`
}
type ProviderRegistration struct {
	Configuration TenantFiscalConfiguration
	Provider      FiscalProvider
}
type ProviderRegistry struct {
	entries   map[string]ProviderRegistration
	vaultPath string
}

func privateReference(path string) bool {
	if path == "" {
		return true
	}
	st, err := os.Lstat(path)
	return filepath.IsAbs(path) && err == nil && st.Mode().IsRegular() && st.Mode().Perm() == 0600
}
func NewProviderRegistry(entries []ProviderRegistration) (*ProviderRegistry, error) {
	r := &ProviderRegistry{entries: map[string]ProviderRegistration{}}
	for _, entry := range entries {
		c := entry.Configuration
		b := c.Binding
		if b.TenantID == "" || b.StoreID == "" || b.DeviceID == "" || b.ProfileID == "" || b.ProfileRevision < 1 ||
			(c.Provider != "own" && c.Provider != "external") || (c.Environment != "homologation" && c.Environment != "production") ||
			(c.CRT != "1" && c.CRT != "2" && c.CRT != "3" && c.CRT != "4") || c.CSCReference != "" || c.CertificateReference != "" || c.TokenReference != "" {
			return nil, ErrUnavailable
		}
		if c.CustomerManaged && (c.Provider != "external" || (c.CertificateType != "" && c.CertificateType != "A1") || (c.FocusBaseURL != "" && c.FocusBaseURL != focusHomologation)) {
			return nil, ErrUnavailable
		}
		if _, ok := r.entries[b.TenantID]; ok {
			return nil, ErrUnavailable
		}
		// Own bindings cannot be relabeled as a different tenant/store/device.
		if own, ok := entry.Provider.(*OwnProvider); ok {
			if own == nil || own.runtime == nil || bindingOf(own.runtime) != b || c.Provider != "own" {
				return nil, ErrUnavailable
			}
		}
		r.entries[b.TenantID] = entry
	}
	return r, nil
}
func bindingOf(r *Runtime) ProviderBinding {
	b := r.Binding()
	return ProviderBinding{TenantID: b.TenantID, StoreID: b.StoreID, DeviceID: b.DeviceID, ProfileID: b.ProfileID, ProfileRevision: b.ProfileRevision}
}
func (r *ProviderRegistry) Resolve(a RequestContext) (FiscalProvider, ProviderBinding, error) {
	if r == nil {
		return nil, ProviderBinding{}, ErrUnavailable
	}
	e, ok := r.entries[a.Actor.TenantID]
	original := e.Configuration.Binding
	if !ok || a.Actor.TenantID != a.Device.TenantID || a.Actor.StoreID != a.Device.StoreID || original.StoreID != a.Actor.StoreID || original.DeviceID != a.Device.DeviceID {
		return nil, ProviderBinding{}, ErrUnavailable
	}
	if e.Configuration.CustomerManaged {
		var err error
		e, err = r.customerRegistration(a.Actor.TenantID, e.Configuration)
		if err != nil {
			return nil, ProviderBinding{}, err
		}
	}
	b := e.Configuration.Binding
	if !ok || e.Provider == nil || a.Actor.TenantID != a.Device.TenantID || a.Actor.StoreID != a.Device.StoreID || b.StoreID != a.Actor.StoreID || b.DeviceID != a.Device.DeviceID {
		return nil, ProviderBinding{}, ErrUnavailable
	}
	return e.Provider, b, nil
}
func (r *ProviderRegistry) ValidateCheckoutConfiguration(ctx context.Context, a RequestContext, environment int) error {
	if environment != 2 {
		return ErrCheckoutProduction
	}
	provider, b, err := r.Resolve(a)
	if err != nil {
		return err
	}
	c := r.entries[a.Actor.TenantID].Configuration
	if c.CustomerManaged {
		if focus, ok := provider.(*FocusProvider); ok {
			c = focus.configuration
		} else {
			return ErrUnavailable
		}
	}
	if c.Environment != "homologation" {
		return ErrCheckoutProduction
	}
	profile, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, b.ProfileID, 0)
	if err != nil {
		return err
	}
	if profile.Revision != b.ProfileRevision || profile.Input.CRT != c.CRT || profile.Input.Environment != c.Environment || profile.Input.Model != "65" {
		return ErrUnavailable
	}
	return nil
}

// LoadProviderRegistry reads encrypted tenant credentials and never resolves an external
// provider to own. External uses Focus NFC-e homologation with a tenant vault token.
func LoadProviderRegistry(path string) (*ProviderRegistry, error) {
	raw, err := privateFile(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer clear(raw)
	var config struct {
		Version   int                         `json:"version"`
		VaultPath string                      `json:"vault_path"`
		Tenants   []TenantFiscalConfiguration `json:"tenants"`
	}
	if fiscalcore.DecodeInput(raw, &config) != nil || config.Version != 1 || len(config.Tenants) == 0 || len(config.Tenants) > 1000 {
		return nil, ErrUnavailable
	}
	vault, err := OpenFiscalSecretVault(config.VaultPath)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer vault.Close()
	entries := make([]ProviderRegistration, 0, len(config.Tenants))
	for _, c := range config.Tenants {
		e := ProviderRegistration{Configuration: c}
		if c.CustomerManaged {
			if c.Provider != "external" {
				return nil, ErrUnavailable
			}
			entries = append(entries, e)
			continue
		}
		if c.Provider == "own" {
			runtime, err := vault.loadOwn(c.Binding.TenantID)
			if err != nil {
				return nil, ErrUnavailable
			}
			e.Provider = NewOwnProvider(runtime)
		}
		if c.Provider == "external" && (c.CertificateType != "" || c.FocusBaseURL != "") {
			token, err := vault.read(context.Background(), c.Binding.TenantID, "token")
			if err != nil {
				return nil, ErrUnavailable
			}
			csc, _ := vault.read(context.Background(), c.Binding.TenantID, "csc")
			provider, e2 := newFocusProvider(c, token, csc)
			clear(token)
			clear(csc)
			if e2 != nil {
				return nil, e2
			}
			e.Provider = provider
		}
		entries = append(entries, e)
	}
	registry, err := NewProviderRegistry(entries)
	if err != nil {
		return nil, err
	}
	registry.vaultPath = config.VaultPath
	return registry, nil
}
