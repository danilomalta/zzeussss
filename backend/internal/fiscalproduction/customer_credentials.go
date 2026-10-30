package fiscalproduction

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

// This endpoint is explicitly enabled by the infrastructure owner per tenant.
// The request cannot select a tenant, store, vault, key, URL or certificate file.
type CustomerCredentialInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int    `json:"expected_revision"`
	ProfileRevision  int    `json:"profile_revision"`
	Token            []byte `json:"-"`
	CSC              []byte `json:"-"`
}

func (CustomerCredentialInput) String() string   { return "[private fiscal credential input]" }
func (CustomerCredentialInput) GoString() string { return "[private fiscal credential input]" }

// Only this allowlist can be returned to an authenticated customer owner.
type CustomerCredentialStatus struct {
	Revision          int    `json:"revision"`
	OperationID       string `json:"operation_id"`
	ProfileID         string `json:"profile_id"`
	ProfileRevision   int    `json:"profile_revision"`
	Environment       string `json:"environment"`
	TokenPresent      bool   `json:"token_present"`
	CSCPresent        bool   `json:"csc_present"`
	ProductionBlocked bool   `json:"production_blocked"`
}
type customerCredentialRecord struct {
	Configuration TenantFiscalConfiguration `json:"configuration"`
	Revision      int                       `json:"revision"`
	OperationID   string                    `json:"operation_id"`
	RequestSHA256 string                    `json:"request_sha256"`
	Token         []byte                    `json:"token"`
	CSC           []byte                    `json:"csc"`
}

func (r *customerCredentialRecord) clear() { clear(r.Token); clear(r.CSC) }

var customerOperation = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func (r *ProviderRegistry) customerVault(a RequestContext) (*FiscalSecretVault, TenantFiscalConfiguration, error) {
	if r == nil {
		return nil, TenantFiscalConfiguration{}, ErrUnavailable
	}
	e, ok := r.entries[a.Actor.TenantID]
	c := e.Configuration
	b := c.Binding
	if !ok || !c.CustomerManaged || c.Provider != "external" || b.StoreID != a.Actor.StoreID || b.DeviceID != a.Device.DeviceID || a.Device.TenantID != a.Actor.TenantID || a.Device.StoreID != a.Actor.StoreID {
		return nil, c, ErrUnavailable
	}
	v, err := openTenantVault(c, r.vaultPath)
	return v, c, err
}
func authorizeCustomerCredentials(ctx context.Context, a RequestContext) error {
	if a.DB == nil || a.License == nil {
		return ErrUnavailable
	}
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if err = fiscal.ValidateSaleSessionTx(ctx, tx, a.Actor, a.Device, a.Token); err != nil {
		return err
	}
	if err = a.License.RequireTx(ctx, tx, a.Actor, a.Device, identity.ViewAccounting, modules.Fiscal); err != nil {
		return err
	}
	var role string
	if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.Actor.TenantID, a.Actor.IdentityID).Scan(&role); err != nil {
		return identity.ErrDenied
	}
	if role != "owner" {
		return identity.ErrDenied
	}
	return nil
}
func readCustomerRecord(ctx context.Context, v *FiscalSecretVault, tenant string) (customerCredentialRecord, []byte, error) {
	var record customerCredentialRecord
	var sealed []byte
	err := v.db.QueryRowContext(ctx, `SELECT ciphertext FROM fiscal_secret_vault WHERE tenant=? AND purpose='customer_provider'`, tenant).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return record, nil, nil
	}
	if err != nil || len(sealed) < v.aead.NonceSize()+v.aead.Overhead() || len(sealed) > 32768 {
		return record, nil, ErrUnavailable
	}
	aad, err := vaultAAD(tenant, "customer_provider")
	if err != nil {
		return record, nil, err
	}
	n := v.aead.NonceSize()
	raw, err := v.aead.Open(nil, sealed[:n], sealed[n:], aad)
	if err != nil {
		return record, nil, ErrUnavailable
	}
	defer clear(raw)
	if fiscalcore.DecodeInput(raw, &record) != nil || record.Revision < 1 || record.Configuration.Binding.TenantID != tenant || !customerOperation.MatchString(record.OperationID) {
		record.clear()
		return customerCredentialRecord{}, nil, ErrUnavailable
	}
	return record, sealed, nil
}
func customerStatus(record customerCredentialRecord) CustomerCredentialStatus {
	return CustomerCredentialStatus{Revision: record.Revision, OperationID: record.OperationID, ProfileID: record.Configuration.Binding.ProfileID, ProfileRevision: record.Configuration.Binding.ProfileRevision, Environment: record.Configuration.Environment, TokenPresent: len(record.Token) > 0, CSCPresent: len(record.CSC) > 0, ProductionBlocked: true}
}
func (r *ProviderRegistry) ReadCustomerCredentials(ctx context.Context, a RequestContext, profile string) (CustomerCredentialStatus, error) {
	if err := authorizeCustomerCredentials(ctx, a); err != nil {
		return CustomerCredentialStatus{}, err
	}
	if _, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, profile, 0); err != nil {
		return CustomerCredentialStatus{}, err
	}
	v, _, err := r.customerVault(a)
	if err != nil {
		return CustomerCredentialStatus{}, err
	}
	defer v.Close()
	record, _, err := readCustomerRecord(ctx, v, a.Actor.TenantID)
	defer record.clear()
	if err != nil {
		return CustomerCredentialStatus{}, err
	}
	if record.Revision == 0 {
		return CustomerCredentialStatus{ProfileID: profile, ProductionBlocked: true}, nil
	}
	if record.Configuration.Binding.ProfileID != profile || record.Configuration.Binding.StoreID != a.Actor.StoreID || record.Configuration.Binding.DeviceID != a.Device.DeviceID {
		return CustomerCredentialStatus{}, fiscal.ErrConflict
	}
	return customerStatus(record), nil
}
func (r *ProviderRegistry) SaveCustomerCredentials(ctx context.Context, a RequestContext, profile string, in CustomerCredentialInput) (CustomerCredentialStatus, error) {
	if !customerOperation.MatchString(in.OperationID) || in.ExpectedRevision < 0 || in.ExpectedRevision > 1000000 || in.ProfileRevision < 1 || len(in.Token) == 0 || len(in.Token) > 512 || len(in.CSC) > 128 {
		return CustomerCredentialStatus{}, fiscal.ErrInvalid
	}
	for _, b := range in.Token {
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			return CustomerCredentialStatus{}, fiscal.ErrInvalid
		}
	}
	for _, b := range in.CSC {
		if b < 33 || b > 126 {
			return CustomerCredentialStatus{}, fiscal.ErrInvalid
		}
	}
	if err := authorizeCustomerCredentials(ctx, a); err != nil {
		return CustomerCredentialStatus{}, err
	}
	p, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, profile, 0)
	if err != nil {
		return CustomerCredentialStatus{}, err
	}
	if p.Revision != in.ProfileRevision || p.Input.Model != "65" {
		return CustomerCredentialStatus{}, fiscal.ErrConflict
	}
	v, c, err := r.customerVault(a)
	if err != nil {
		return CustomerCredentialStatus{}, err
	}
	defer v.Close()
	old, sealed, err := readCustomerRecord(ctx, v, a.Actor.TenantID)
	if err != nil {
		return CustomerCredentialStatus{}, err
	}
	defer old.clear()
	if old.Revision > 0 && (old.Configuration.Binding.StoreID != a.Actor.StoreID || old.Configuration.Binding.DeviceID != a.Device.DeviceID) {
		return CustomerCredentialStatus{}, fiscal.ErrConflict
	}
	// Fingerprint is inside the encrypted record only. No token or hash is logged.
	body, _ := json.Marshal(struct {
		Operation                 string
		Expected, ProfileRevision int
		Profile                   string
		Token, CSC                []byte
	}{in.OperationID, in.ExpectedRevision, in.ProfileRevision, profile, in.Token, in.CSC})
	defer clear(body)
	sum := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(sum[:])
	if old.Revision > 0 && old.OperationID == in.OperationID {
		if old.RequestSHA256 != fingerprint {
			return CustomerCredentialStatus{}, fiscal.ErrConflict
		}
		return customerStatus(old), nil
	}
	if old.Revision != in.ExpectedRevision {
		return CustomerCredentialStatus{}, fiscal.ErrConflict
	}
	c.Binding.ProfileID = profile
	c.Binding.ProfileRevision = p.Revision
	c.Environment = p.Input.Environment
	c.CRT = p.Input.CRT
	c.CertificateType = "A1"
	c.FocusBaseURL = focusHomologation
	record := customerCredentialRecord{Configuration: c, Revision: old.Revision + 1, OperationID: in.OperationID, RequestSHA256: fingerprint, Token: in.Token, CSC: in.CSC}
	raw, _ := json.Marshal(record)
	defer clear(raw)
	aad, _ := vaultAAD(a.Actor.TenantID, "customer_provider")
	nonce := make([]byte, v.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return CustomerCredentialStatus{}, ErrUnavailable
	}
	encrypted := v.aead.Seal(nonce, nonce, raw, aad)
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return CustomerCredentialStatus{}, ErrUnavailable
	}
	defer tx.Rollback()
	if _, err = v.checkKeyTx(ctx, tx); err != nil {
		return CustomerCredentialStatus{}, ErrUnavailable
	}
	var result sql.Result
	if sealed == nil {
		result, err = tx.ExecContext(ctx, `INSERT INTO fiscal_secret_vault(tenant,purpose,ciphertext) VALUES (?,'customer_provider',?) ON CONFLICT(tenant,purpose) DO NOTHING`, a.Actor.TenantID, encrypted)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE fiscal_secret_vault SET ciphertext=? WHERE tenant=? AND purpose='customer_provider' AND ciphertext=?`, encrypted, a.Actor.TenantID, sealed)
	}
	if err != nil {
		return CustomerCredentialStatus{}, ErrUnavailable
	}
	n, err := result.RowsAffected()
	if err != nil {
		return CustomerCredentialStatus{}, ErrUnavailable
	}
	if n != 1 {
		return CustomerCredentialStatus{}, fiscal.ErrConflict
	}
	if err = tx.Commit(); err != nil {
		return CustomerCredentialStatus{}, ErrUnavailable
	}
	return customerStatus(record), nil
}
func (r *ProviderRegistry) customerRegistration(tenant string, base TenantFiscalConfiguration) (ProviderRegistration, error) {
	v, err := openTenantVault(base, r.vaultPath)
	if err != nil {
		return ProviderRegistration{}, err
	}
	defer v.Close()
	record, _, err := readCustomerRecord(context.Background(), v, tenant)
	defer record.clear()
	if err != nil || record.Revision == 0 {
		return ProviderRegistration{}, ErrUnavailable
	}
	c := record.Configuration
	if c.Binding.TenantID != base.Binding.TenantID || c.Binding.StoreID != base.Binding.StoreID || c.Binding.DeviceID != base.Binding.DeviceID || c.Provider != "external" || !c.CustomerManaged {
		return ProviderRegistration{}, ErrUnavailable
	}
	c.VaultPath = base.VaultPath
	c.MasterKeyFile = base.MasterKeyFile
	if c.Environment == "production" {
		return ProviderRegistration{}, ErrCheckoutProduction
	}
	p, err := newFocusProvider(c, record.Token, record.CSC)
	if err != nil {
		return ProviderRegistration{}, err
	}
	return ProviderRegistration{Configuration: c, Provider: p}, nil
}
