package fiscalproduction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

// CheckoutNFCeDANFE renders the existing archived XML, never requesting another
// emission or downloading a caller-controlled URL. A nil registry still supports own.
func (r *ProviderRegistry) CheckoutNFCeDANFE(ctx context.Context, a RequestContext, id string, opts fiscalcore.PrintOptions) (fiscalcore.DANFEArtifact, error) {
	empty := fiscalcore.DANFEArtifact{}
	if a.DB == nil || (opts.WidthMM != 58 && opts.WidthMM != 80) || fiscalcore.ValidatePrintOptions(opts) != nil {
		return empty, fiscal.ErrInvalid
	}
	session, err := localauth.Resolve(ctx, a.DB, a.Token)
	if err != nil || session.Actor != a.Actor || session.Device != a.Device {
		return empty, identity.ErrDenied
	}
	plan, err := sale.ReadPlan(ctx, a.DB, a.Actor, a.Device, id)
	if err != nil {
		return empty, fiscal.ErrDocumentUnavailable
	}
	valid := func(p sale.Plan) bool {
		return p.Completion != nil && p.Completion.SaleStatus == "committed" && p.Fiscal != nil && p.Fiscal.State == "authorized_homologation" && fiscalcore.ValidAccessKey(p.Fiscal.Key) && p.Fiscal.Key[20:22] == "65"
	}
	if !valid(plan) {
		return empty, fiscal.ErrDocumentUnavailable
	}
	intent, err := fiscal.ReadIntent(ctx, a.DB, a.Actor, a.Device, plan.Fiscal.IntentID, false)
	if err != nil || intent.Input.SaleID != id || intent.Snapshot.Profile.Model != "65" || intent.Snapshot.Profile.Environment != "homologation" {
		return empty, fiscal.ErrDocumentUnavailable
	}
	out, nativeErr := fiscal.ReadHomologationDANFE(ctx, a.DB, a.Actor, a.Device, intent.ID, a.Token, opts)
	if nativeErr == nil {
		if out.Key != plan.Fiscal.Key {
			return empty, fiscal.ErrDocumentUnavailable
		}
		return out, nil
	}
	if r == nil || !errors.Is(nativeErr, fiscal.ErrNotFound) {
		return empty, fiscal.ErrDocumentUnavailable
	}
	entry, ok := r.entries[a.Actor.TenantID]
	b := entry.Configuration.Binding
	if !ok || entry.Configuration.Provider != "external" || entry.Configuration.Environment != "homologation" || entry.Configuration.CertificateType != "A1" || b.StoreID != a.Actor.StoreID || b.DeviceID != a.Device.DeviceID {
		return empty, fiscal.ErrDocumentUnavailable
	}
	// A durable cancellation claim blocks an active-looking copy even before
	// the provider returns or records its final response.
	var cancelling int
	if err = a.DB.QueryRowContext(ctx, `SELECT count(*) FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND event_type='checkout.focus_cancel'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, id).Scan(&cancelling); err != nil || cancelling != 0 {
		return empty, fiscal.ErrDocumentUnavailable
	}
	// The archive belongs to the customer's selected DB and private backup key.
	rows, err := a.DB.QueryContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND event_type='checkout.focus_xml_archive'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, id)
	if err != nil {
		return empty, fiscal.ErrDocumentUnavailable
	}
	var selected *focusArchiveRecord
	for rows.Next() {
		var raw string
		var record focusArchiveRecord
		if rows.Scan(&raw) != nil || json.Unmarshal([]byte(raw), &record) != nil {
			rows.Close()
			return empty, fiscal.ErrDocumentUnavailable
		}
		if record.Kind == "cancellation" {
			rows.Close()
			return empty, fiscal.ErrDocumentUnavailable
		}
		if record.Kind != "invoice" {
			continue
		}
		if selected != nil || record.Version != 1 || record.Sale != id || record.Intent != intent.ID || record.Key != plan.Fiscal.Key || record.Number != 0 {
			rows.Close()
			return empty, fiscal.ErrDocumentUnavailable
		}
		selected = &record
	}
	err = rows.Err()
	rows.Close()
	if err != nil || selected == nil {
		return empty, fiscal.ErrDocumentUnavailable
	}
	out, err = renderCheckoutArchive(a, *selected, entry.Configuration.XMLArchiveKeyFile, opts)
	if err != nil {
		return empty, err
	}
	// A cancellation or new uncertain status must not fall back to an old copy.
	latest, err := sale.ReadPlan(ctx, a.DB, a.Actor, a.Device, id)
	if err != nil || !valid(latest) || latest.Fiscal.Key != plan.Fiscal.Key || latest.Fiscal.OperationID != plan.Fiscal.OperationID {
		return empty, fiscal.ErrDocumentUnavailable
	}
	return out, nil
}

// Existing archive decryption and existing fiscal renderer; no new fiscal logic.
func renderCheckoutArchive(a RequestContext, record focusArchiveRecord, keyFile string, opts fiscalcore.PrintOptions) (fiscalcore.DANFEArtifact, error) {
	empty := fiscalcore.DANFEArtifact{}
	crypt, err := archiveCrypt(keyFile)
	if err != nil || len(record.Ciphertext) < crypt.NonceSize() {
		return empty, fiscal.ErrDocumentUnavailable
	}
	n := crypt.NonceSize()
	data, err := crypt.Open(nil, record.Ciphertext[:n], record.Ciphertext[n:], archiveAAD(a, record))
	if err != nil {
		return empty, fiscal.ErrDocumentUnavailable
	}
	defer clear(data)
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != record.SHA256 || validateFocusArchiveXML(data, record.Key, "invoice", 0) != nil {
		return empty, fiscal.ErrDocumentUnavailable
	}
	out, err := fiscalcore.RenderHomologationNFCe(data, opts)
	if err != nil {
		return empty, fiscal.ErrDocumentUnavailable
	}
	return out, nil
}
