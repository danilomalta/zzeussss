package fiscal

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/sqlitewrite"
)

// NFeOrigin is an immutable commercial snapshot. It is not a sale flag.
type NFeOrigin struct {
	Kind           string          `json:"kind"`
	SourceID       string          `json:"source_id"`
	ReferencedKey  string          `json:"referenced_access_key"`
	FreightMode    string          `json:"freight_mode"`
	RecipientCNPJ  string          `json:"recipient_cnpj,omitempty"`
	RecipientCPF   string          `json:"recipient_cpf,omitempty"`
	RecipientName  string          `json:"recipient_name"`
	RecipientUF    string          `json:"recipient_uf"`
}

type OriginRecord struct {
	ID             string     `json:"origin_id"`
	OperationID    string     `json:"operation_id"`
	Snapshot       Snapshot   `json:"snapshot"`
	SnapshotSHA256 string     `json:"snapshot_sha256"`
	Repeated       bool       `json:"repeated"`
}

type OriginBindInput struct {
	OperationID     string `json:"operation_id"`
	Kind            string `json:"kind"`
	SourceID        string `json:"source_id"`
	ReferencedKey   string `json:"referenced_access_key"`
	FreightMode     string `json:"freight_mode"`
	ProfileID          string   `json:"-"`
	ProfileRevision    int      `json:"-"`
	TaxQuoteOperations []string `json:"tax_quote_operations,omitempty"`
	Confirmed          bool     `json:"confirmed"`
	Recipient      struct {
		CNPJ string `json:"cnpj"`
		CPF  string `json:"cpf"`
		Name string `json:"name"`
		UF   string `json:"uf"`
	} `json:"recipient"`
	Items []SaleItem `json:"items"`
}

func (o NFeOrigin) valid() bool {
	if (o.Kind != "inbound_receipt" && o.Kind != "customer_return") || !idPattern.MatchString(o.SourceID) || !fiscalcore.ValidAccessKey(o.ReferencedKey) || o.ReferencedKey[20:22] != "55" {
		return false
	}
	if o.FreightMode != "9" && o.FreightMode != "0" && o.FreightMode != "1" && o.FreightMode != "2" {
		return false
	}
	if o.RecipientName == "" || len(o.RecipientUF) != 2 || (o.RecipientCNPJ == "") == (o.RecipientCPF == "") {
		return false
	}
	return true
}

func bindOriginTaxesTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, in *OriginBindInput) error {
	if in == nil {
		return ErrInvalid
	}
	refs := in.TaxQuoteOperations
	if len(refs) == 0 {
		refs = make([]string, len(in.Items))
		resolve := false
		for i, item := range in.Items {
			if item.Tax != nil && item.Tax.QuoteOperationID != "" && len(item.Tax.Taxes) == 0 {
				refs[i] = item.Tax.QuoteOperationID
				resolve = true
				continue
			}
			if resolve {
				return ErrInvalid
			}
		}
		if !resolve {
			return nil
		}
	}
	if len(refs) != len(in.Items) {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for i, ref := range refs {
		if seen[ref] || ref == "" || (in.Items[i].Tax != nil && len(in.Items[i].Tax.Taxes) > 0) {
			return ErrInvalid
		}
		seen[ref] = true
		item := in.Items[i]
		var unit string
		if err := tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, item.ProductID).Scan(&unit); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		binding, err := ResolveSaleTaxTx(ctx, tx, a, d, ref, item.ProductID, unit, item.QuantityMilli, item.UnitPriceCents, item.TotalCents)
		if err != nil {
			return err
		}
		in.Items[i].Tax = &binding
	}
	return nil
}

func BindNFeOrigin(ctx context.Context, db *sql.DB, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in OriginBindInput, token string) (OriginRecord, error) {
	var out OriginRecord
	err := sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		var e error
		out, e = bindNFeOriginTx(ctx, tx, license, a, d, in, token)
		return e
	})
	return out, err
}

func bindNFeOriginTx(ctx context.Context, tx *sql.Tx, license *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in OriginBindInput, token string) (OriginRecord, error) {
	var out OriginRecord
	if !in.Confirmed || !idPattern.MatchString(in.OperationID) || len(in.Items) == 0 || len(in.Items) > 100 {
		return out, ErrInvalid
	}
	origin := NFeOrigin{Kind: in.Kind, SourceID: in.SourceID, ReferencedKey: in.ReferencedKey, FreightMode: in.FreightMode, RecipientCNPJ: in.Recipient.CNPJ, RecipientCPF: in.Recipient.CPF, RecipientName: in.Recipient.Name, RecipientUF: in.Recipient.UF}
	if !origin.valid() {
		return out, ErrInvalid
	}
	if err := fiscalSessionTx(ctx, tx, a, d, token); err != nil {
		return out, err
	}
	if err := license.RequireTx(ctx, tx, a, d, identity.ViewAccounting, modules.Fiscal); err != nil {
		return out, err
	}
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		return out, err
	}
	if role != "owner" {
		return out, identity.ErrDenied
	}
	body, err := json.Marshal(in)
	if err != nil {
		return out, ErrInvalid
	}
	var existing, oldReq string
	err = tx.QueryRowContext(ctx, `SELECT origin_id,request_json FROM fiscal_nfe_origins WHERE tenant_id=? AND device_id=? AND operation_id=?`, a.TenantID, d.DeviceID, in.OperationID).Scan(&existing, &oldReq)
	if err == nil {
		if oldReq != string(body) {
			return out, ErrConflict
		}
		replay, e := ReadNFeOriginTx(ctx, tx, a, existing)
		if e != nil {
			return out, e
		}
		replay.Repeated = true
		return replay, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if err = assertOriginSourceTx(ctx, tx, a, origin, in.Items); err != nil {
		return out, err
	}
	profile, err := profileTx(ctx, tx, a, in.ProfileID, in.ProfileRevision)
	if err != nil {
		return out, err
	}
	if profile.Input.Model != "55" || profile.Input.Environment != "homologation" {
		return out, ErrInvalid
	}
	if err := bindOriginTaxesTx(ctx, tx, a, d, &in); err != nil {
		return out, err
	}
	snap := Snapshot{Version: 1, SaleID: "", Origin: &origin, Profile: profile.Input, ProfileRevision: profile.Revision, Items: in.Items, Payments: nil}
	for i := range snap.Items {
		item := &snap.Items[i]
		if item.Tax == nil || item.QuantityMilli <= 0 || item.UnitPriceCents < 0 {
			return out, ErrInvalid
		}
		q, e := taxQuoteTx(ctx, tx, a, d, item.Tax.QuoteOperationID)
		if e != nil {
			return out, e
		}
		if q.Input.Item.ProductID != item.ProductID {
			return out, ErrConflict
		}
		snap.TotalCents += item.TotalCents
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return out, ErrInvalid
	}
	sum := sha256.Sum256(raw)
	id, err := localdb.NewID()
	if err != nil {
		return out, err
	}
	digest := hex.EncodeToString(sum[:])
	_, err = tx.ExecContext(ctx, `INSERT INTO fiscal_nfe_origins(tenant_id,store_id,origin_id,kind,source_id,referenced_access_key,device_id,actor_id,operation_id,request_json,snapshot_json,snapshot_sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, origin.Kind, origin.SourceID, origin.ReferencedKey, d.DeviceID, a.IdentityID, in.OperationID, string(body), string(raw), digest, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return out, err
	}
	out = OriginRecord{ID: id, OperationID: in.OperationID, Snapshot: snap, SnapshotSHA256: digest}
	return out, nil
}

func assertOriginSourceTx(ctx context.Context, tx *sql.Tx, a identity.Scope, origin NFeOrigin, items []SaleItem) error {
	switch origin.Kind {
	case "inbound_receipt":
		var product string
		var accepted int64
		err := tx.QueryRowContext(ctx, `SELECT product_id,accepted_milli FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, origin.SourceID).Scan(&product, &accepted)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var used int64
		for _, item := range items {
			if item.ProductID != product {
				return ErrConflict
			}
			used += item.QuantityMilli
		}
		if used <= 0 || used > accepted {
			return ErrConflict
		}
		return nil
	case "customer_return":
		var sale string
		err := tx.QueryRowContext(ctx, `SELECT sale_id FROM customer_returns WHERE tenant_id=? AND store_id=? AND return_id=?`, a.TenantID, a.StoreID, origin.SourceID).Scan(&sale)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var key string
		_ = tx.QueryRowContext(ctx, `SELECT json_extract(payload_json,'$.key') FROM outbox WHERE tenant_id=? AND store_id=? AND aggregate_id=? AND event_type='checkout.fiscal_status' AND json_extract(payload_json,'$.key') IS NOT NULL ORDER BY created_at DESC LIMIT 1`, a.TenantID, a.StoreID, sale).Scan(&key)
		if key != origin.ReferencedKey {
			return ErrConflict
		}
		for _, item := range items {
			var returned int64
			e := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_milli),0) FROM customer_return_items WHERE tenant_id=? AND return_id=? AND product_id=?`, a.TenantID, origin.SourceID, item.ProductID).Scan(&returned)
			if e != nil {
				return e
			}
			if item.QuantityMilli <= 0 || item.QuantityMilli > returned {
				return ErrConflict
			}
		}
		return nil
	}
	return ErrInvalid
}

func ReadNFeOrigin(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (OriginRecord, error) {
	if !idPattern.MatchString(id) {
		return OriginRecord{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return OriginRecord{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); err != nil {
		return OriginRecord{}, err
	}
	out, err := ReadNFeOriginTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func ReadNFeOriginTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (OriginRecord, error) {
	var out OriginRecord
	var raw, op string
	err := tx.QueryRowContext(ctx, `SELECT origin_id,operation_id,snapshot_json,snapshot_sha256 FROM fiscal_nfe_origins WHERE tenant_id=? AND store_id=? AND origin_id=?`, a.TenantID, a.StoreID, id).Scan(&out.ID, &op, &raw, &out.SnapshotSHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256([]byte(raw))
	if hex.EncodeToString(sum[:]) != out.SnapshotSHA256 || json.Unmarshal([]byte(raw), &out.Snapshot) != nil || out.Snapshot.SaleID != "" || out.Snapshot.Origin == nil {
		return OriginRecord{}, ErrIntegrity
	}
	out.OperationID = op
	return out, nil
}
