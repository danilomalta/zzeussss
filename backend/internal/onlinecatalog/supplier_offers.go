package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
)

// SupplierOffer records internal declared conditions; it is not a supplier-confirmed offer.
type SupplierOffer struct {
	ID             string    `json:"id"`
	SupplierID     string    `json:"supplier_id"`
	ProductID      int64     `json:"product_id"`
	SupplierSKU    string    `json:"supplier_sku"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	MinimumUnits   int64     `json:"minimum_units"`
	LeadTimeDays   int64     `json:"lead_time_days"`
	Active         bool      `json:"active"`
	Version        int64     `json:"version"`
	CreatorID      string    `json:"creator_id"`
	LastActorID    string    `json:"last_actor_id"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type SupplierOfferInput struct {
	OperationID     string `json:"operation_id"`
	ExpectedVersion int64  `json:"expected_version,omitempty"`
	SupplierID      string `json:"supplier_id"`
	ProductID       int64  `json:"product_id"`
	SupplierSKU     string `json:"supplier_sku"`
	UnitPriceCents  int64  `json:"unit_price_cents"`
	MinimumUnits    int64  `json:"minimum_units"`
	LeadTimeDays    int64  `json:"lead_time_days"`
	Active          bool   `json:"active"`
	Reason          string `json:"reason"`
}
type SupplierOfferReceipt struct {
	OperationID string         `json:"operation_id"`
	OfferID     string         `json:"offer_id"`
	ActorID     string         `json:"actor_id"`
	Kind        string         `json:"kind"`
	Before      *SupplierOffer `json:"before"`
	After       SupplierOffer  `json:"after"`
}

func offerTerms(sku, reason string, price, minimum, lead int64) bool {
	return clean(sku, 120, false) && sku == strings.TrimSpace(sku) && clean(reason, 500, true) && reason == strings.TrimSpace(reason) && price >= 0 && price <= MaxPrice && minimum >= 1 && minimum <= MaxStockUnits && lead >= 0 && lead <= 365
}
func normalizeSupplierOffer(v SupplierOfferInput, update bool) (SupplierOfferInput, error) {
	v.SupplierSKU = strings.TrimSpace(v.SupplierSKU)
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || !ValidUUID(v.SupplierID) || v.ProductID < 1 || v.ProductID > MaxVersion || !offerTerms(v.SupplierSKU, v.Reason, v.UnitPriceCents, v.MinimumUnits, v.LeadTimeDays) || (!update && v.ExpectedVersion != 0) || (update && (v.ExpectedVersion < 1 || v.ExpectedVersion >= MaxVersion)) {
		return v, ErrInput
	}
	return v, nil
}
func DecodeSupplierOffer(content string, body []byte, update bool) (SupplierOfferInput, error) {
	var v SupplierOfferInput
	fields := map[string]bool{"operation_id": true, "supplier_id": true, "product_id": true, "supplier_sku": true, "unit_price_cents": true, "minimum_units": true, "lead_time_days": true, "active": true, "reason": true}
	if update {
		fields["expected_version"] = true
	}
	if proposalObject(content, body, fields) != nil || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeSupplierOffer(v, update)
}
func validSupplierOffer(p SupplierOffer) bool {
	return ValidUUID(p.ID) && ValidUUID(p.SupplierID) && p.ProductID >= 1 && p.ProductID <= MaxVersion && ValidUUID(p.CreatorID) && ValidUUID(p.LastActorID) && p.Version >= 1 && p.Version <= MaxVersion && offerTerms(p.SupplierSKU, p.Reason, p.UnitPriceCents, p.MinimumUnits, p.LeadTimeDays) && !p.CreatedAt.IsZero() && !p.UpdatedAt.Before(p.CreatedAt)
}
func validSupplierOfferReceipt(r SupplierOfferReceipt) bool {
	if !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || !validSupplierOffer(r.After) || r.OfferID != r.After.ID || r.ActorID != r.After.LastActorID {
		return false
	}
	if r.Kind == "create" {
		return r.Before == nil && r.After.ID == r.OperationID && r.After.Version == 1 && r.After.CreatorID == r.ActorID && r.After.CreatedAt.Equal(r.After.UpdatedAt)
	}
	if r.Kind != "update" || r.Before == nil || !validSupplierOffer(*r.Before) || r.Before.Version >= MaxVersion {
		return false
	}
	return r.Before.ID == r.After.ID && r.Before.SupplierID == r.After.SupplierID && r.Before.ProductID == r.After.ProductID && r.Before.CreatorID == r.After.CreatorID && r.Before.CreatedAt.Equal(r.After.CreatedAt) && r.After.Version == r.Before.Version+1 && !r.After.UpdatedAt.Before(r.Before.UpdatedAt)
}

const supplierOfferColumns = `offer_id::text,supplier_id::text,product_id,version,active,payload`
const supplierOfferSQL = `SELECT ` + supplierOfferColumns + ` FROM online_supplier_offers WHERE tenant_id=$1 AND offer_id=$2`

func scanSupplierOffer(row scanner) (SupplierOffer, error) {
	var p SupplierOffer
	var id, supplier string
	var product, version int64
	var active bool
	var b []byte
	e := row.Scan(&id, &supplier, &product, &version, &active, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &p) != nil || !validSupplierOffer(p) || id != p.ID || supplier != p.SupplierID || product != p.ProductID || version != p.Version || active != p.Active {
		return SupplierOffer{}, ErrUnavailable
	}
	return p, nil
}

const supplierOfferReceiptSQL = `SELECT operation_id::text,offer_id::text,actor_id::text,kind,payload_hash,receipt FROM online_supplier_offer_operations WHERE tenant_id=$1 AND operation_id=$2`

func scanSupplierOfferReceipt(row scanner) (SupplierOfferReceipt, string, error) {
	var r SupplierOfferReceipt
	var op, id, actor, kind, h string
	var b []byte
	e := row.Scan(&op, &id, &actor, &kind, &h, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &r) != nil || !validSupplierOfferReceipt(r) || op != r.OperationID || id != r.OfferID || actor != r.ActorID || kind != r.Kind || len(h) != 64 {
		return r, "", ErrUnavailable
	}
	return r, h, nil
}
func (s Store) SupplierOffer(ctx context.Context, a Actor, id string) (SupplierOffer, error) {
	if !ValidUUID(id) {
		return SupplierOffer{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return SupplierOffer{}, e
	}
	defer tx.Rollback()
	p, e := scanSupplierOffer(tx.QueryRowContext(ctx, supplierOfferSQL, a.Tenant, id))
	if e != nil {
		return p, e
	}
	if tx.Commit() != nil {
		return SupplierOffer{}, ErrUnavailable
	}
	return p, nil
}
func (s Store) SupplierOfferOperation(ctx context.Context, a Actor, op string) (SupplierOfferReceipt, error) {
	if !ValidUUID(op) {
		return SupplierOfferReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return SupplierOfferReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanSupplierOfferReceipt(tx.QueryRowContext(ctx, supplierOfferReceiptSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return SupplierOfferReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) CreateSupplierOffer(ctx context.Context, a Actor, v SupplierOfferInput) (SupplierOfferReceipt, error) {
	v, e := normalizeSupplierOffer(v, false)
	if e != nil {
		return SupplierOfferReceipt{}, e
	}
	return s.writeSupplierOffer(ctx, a, v.OperationID, "create", v.OperationID, proposalHash(v.OperationID, "create", v), &v, SupplierOfferInput{})
}
func (s Store) UpdateSupplierOffer(ctx context.Context, a Actor, id string, v SupplierOfferInput) (SupplierOfferReceipt, error) {
	v, e := normalizeSupplierOffer(v, true)
	if e != nil || !ValidUUID(id) {
		return SupplierOfferReceipt{}, ErrInput
	}
	return s.writeSupplierOffer(ctx, a, id, "update", v.OperationID, proposalHash(id, "update", v), nil, v)
}

type SupplierOfferQuery struct {
	SupplierID string `json:"supplier_id"`
	ProductID  int64  `json:"product_id"`
	Active     string `json:"active"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}
type SupplierOfferPage struct {
	SupplierOfferQuery
	Items      []SupplierOffer `json:"items"`
	Total      int64           `json:"total"`
	HasMore    bool            `json:"has_more"`
	NextOffset *int            `json:"next_offset,omitempty"`
}

func ValidateSupplierOfferQuery(q SupplierOfferQuery) error {
	if (q.SupplierID != "" && !ValidUUID(q.SupplierID)) || q.ProductID < 0 || q.ProductID > MaxVersion || (q.Active != "all" && q.Active != "true" && q.Active != "false") || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	return nil
}

const supplierOfferWhere = ` FROM online_supplier_offers WHERE tenant_id=$1 AND ($2='all' OR active=($2='true')) AND ($3::uuid IS NULL OR supplier_id=$3::uuid) AND ($4::bigint=0 OR product_id=$4)`

func (s Store) SupplierOffers(ctx context.Context, a Actor, q SupplierOfferQuery) (SupplierOfferPage, error) {
	var zero SupplierOfferPage
	if ValidateSupplierOfferQuery(q) != nil {
		return zero, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return zero, e
	}
	defer tx.Rollback()
	r := SupplierOfferPage{SupplierOfferQuery: q, Items: []SupplierOffer{}}
	var supplier any
	if q.SupplierID != "" {
		supplier = q.SupplierID
	}
	args := []any{a.Tenant, q.Active, supplier, q.ProductID}
	if tx.QueryRowContext(ctx, `SELECT count(*)`+supplierOfferWhere, args...).Scan(&r.Total) != nil || r.Total < 0 || r.Total > MaxVersion {
		return zero, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+supplierOfferColumns+supplierOfferWhere+` ORDER BY product_id,supplier_id,offer_id LIMIT $5 OFFSET $6`, append(args, q.Limit, q.Offset)...)
	if e != nil {
		return zero, ErrUnavailable
	}
	for rows.Next() {
		p, e := scanSupplierOffer(rows)
		if e != nil {
			rows.Close()
			return zero, e
		}
		r.Items = append(r.Items, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return zero, ErrUnavailable
	}
	want := r.Total - int64(q.Offset)
	if want < 0 {
		want = 0
	}
	if want > int64(q.Limit) {
		want = int64(q.Limit)
	}
	if int64(len(r.Items)) != want {
		return zero, ErrUnavailable
	}
	r.HasMore = int64(q.Offset+len(r.Items)) < r.Total
	if r.HasMore {
		n := q.Offset + len(r.Items)
		r.NextOffset = &n
	}
	if tx.Commit() != nil {
		return zero, ErrUnavailable
	}
	return r, nil
}
func (s Store) writeSupplierOffer(ctx context.Context, a Actor, id, kind, op, h string, create *SupplierOfferInput, v SupplierOfferInput) (SupplierOfferReceipt, error) {
	var zero SupplierOfferReceipt
	if !a.allowed("price") {
		return zero, ErrDenied
	}
	if s.DB == nil {
		return zero, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return zero, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return zero, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":supplier-offer:"+op); e != nil {
		return zero, ErrUnavailable
	}
	prior, ph, e := scanSupplierOfferReceipt(tx.QueryRowContext(ctx, supplierOfferReceiptSQL, a.Tenant, op))
	if e == nil {
		if prior.ActorID != a.User || prior.OfferID != id || prior.Kind != kind || ph != h {
			return zero, ErrConflict
		}
		if tx.Commit() != nil {
			return zero, ErrUnavailable
		}
		return prior, nil
	}
	if e != ErrMissing {
		return zero, e
	}
	var before *SupplierOffer
	var after SupplierOffer
	input := v
	if create != nil {
		input = *create
	} else {
		p, err := scanSupplierOffer(tx.QueryRowContext(ctx, supplierOfferSQL+` FOR UPDATE`, a.Tenant, id))
		if err != nil {
			return zero, err
		}
		if p.Version != v.ExpectedVersion || p.Version >= MaxVersion || p.SupplierID != v.SupplierID || p.ProductID != v.ProductID {
			return zero, ErrConflict
		}
		before = &p
		after = p
	}
	// Active declared conditions require both records currently active in this tenant.
	// Inactivation remains possible when the product was later soft-deleted.
	if input.Active {
		supplier, err := scanSupplier(tx.QueryRowContext(ctx, supplierSQL+` FOR SHARE`, a.Tenant, input.SupplierID))
		if err != nil {
			return zero, err
		}
		if !supplier.Active {
			return zero, ErrConflict
		}
		product, err := scanProduct(tx.QueryRowContext(ctx, productSQL+` FOR SHARE`, a.Tenant, input.ProductID))
		if err != nil {
			return zero, err
		}
		if !product.Active {
			return zero, ErrConflict
		}
	} else if create != nil {
		if _, err := scanSupplier(tx.QueryRowContext(ctx, supplierSQL+` FOR SHARE`, a.Tenant, input.SupplierID)); err != nil {
			return zero, err
		}
		if _, err := scanProduct(tx.QueryRowContext(ctx, productSQL+` FOR SHARE`, a.Tenant, input.ProductID)); err != nil {
			return zero, err
		}
	}
	var at time.Time
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
		return zero, ErrUnavailable
	}
	at = at.UTC()
	if create != nil {
		after = SupplierOffer{ID: id, SupplierID: input.SupplierID, ProductID: input.ProductID, Version: 1, CreatorID: a.User, CreatedAt: at}
	} else {
		after.Version++
	}
	after.SupplierSKU = input.SupplierSKU
	after.UnitPriceCents = input.UnitPriceCents
	after.MinimumUnits = input.MinimumUnits
	after.LeadTimeDays = input.LeadTimeDays
	after.Active = input.Active
	after.Reason = input.Reason
	after.LastActorID = a.User
	after.UpdatedAt = at

	r := SupplierOfferReceipt{op, id, a.User, kind, before, after}
	if !validSupplierOfferReceipt(r) {
		return zero, ErrUnavailable
	}
	payload, _ := json.Marshal(after)
	var result sql.Result
	if create != nil {
		result, e = tx.ExecContext(ctx, `INSERT INTO online_supplier_offers(tenant_id,offer_id,supplier_id,product_id,version,active,payload) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.Tenant, id, after.SupplierID, after.ProductID, after.Version, after.Active, payload)
	} else {
		result, e = tx.ExecContext(ctx, `UPDATE online_supplier_offers SET version=$3,active=$4,payload=$5 WHERE tenant_id=$1 AND offer_id=$2 AND version=$6`, a.Tenant, id, after.Version, after.Active, payload, before.Version)
	}
	if e != nil {
		return zero, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	actual, e := scanSupplierOffer(tx.QueryRowContext(ctx, supplierOfferSQL, a.Tenant, id))
	if e != nil || !reflect.DeepEqual(actual, after) {
		return zero, ErrUnavailable
	}
	receipt, _ := json.Marshal(r)
	result, e = tx.ExecContext(ctx, `INSERT INTO online_supplier_offer_operations(tenant_id,operation_id,offer_id,actor_id,kind,payload_hash,receipt) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.Tenant, op, id, a.User, kind, h, receipt)
	if e != nil {
		return zero, conflict(e)
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	saved, sh, e := scanSupplierOfferReceipt(tx.QueryRowContext(ctx, supplierOfferReceiptSQL, a.Tenant, op))
	if e != nil || sh != h || !reflect.DeepEqual(saved, r) {
		return zero, ErrUnavailable
	}
	result, e = tx.ExecContext(ctx, `INSERT INTO online_supplier_offer_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,$3,$4)`, a.Tenant, op, "supplier.offer."+kind, receipt)
	if e != nil {
		return zero, ErrUnavailable
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	var out []byte
	var event string
	if tx.QueryRowContext(ctx, `SELECT event_type,payload FROM online_supplier_offer_outbox WHERE tenant_id=$1 AND operation_id=$2`, a.Tenant, op).Scan(&event, &out) != nil {
		return zero, ErrUnavailable
	}
	var outReceipt SupplierOfferReceipt
	if event != "supplier.offer."+kind || json.Unmarshal(out, &outReceipt) != nil || !reflect.DeepEqual(outReceipt, r) {
		return zero, ErrUnavailable
	}
	if tx.Commit() != nil {
		return zero, ErrUnavailable
	}
	return r, nil
}
