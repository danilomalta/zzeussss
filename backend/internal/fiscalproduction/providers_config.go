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
	entries map[string]ProviderRegistration
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
	_, b, err := r.Resolve(a)
	if err != nil {
		return err
	}
	c := r.entries[a.Actor.TenantID].Configuration
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
// provider to own. External configurations remain unavailable until an adapter
// is explicitly installed by trusted process wiring.
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
		if c.Provider == "own" {
			runtime, err := vault.loadOwn(c.Binding.TenantID)
			if err != nil {
				return nil, ErrUnavailable
			}
			e.Provider = NewOwnProvider(runtime)
		}
		entries = append(entries, e)
	}
	return NewProviderRegistry(entries)
}
