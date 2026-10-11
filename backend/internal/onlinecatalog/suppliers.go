package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/mail"
	"reflect"
	"strings"
	"time"
	"unicode"
)

// Supplier is a tenant's declared contact; it does not authenticate another company.
type Supplier struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Active      bool      `json:"active"`
	Version     int64     `json:"version"`
	CreatorID   string    `json:"creator_id"`
	LastActorID string    `json:"last_actor_id"`
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type SupplierInput struct {
	OperationID     string `json:"operation_id"`
	ExpectedVersion int64  `json:"expected_version,omitempty"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	Active          bool   `json:"active"`
	Reason          string `json:"reason"`
}
type SupplierReceipt struct {
	OperationID string    `json:"operation_id"`
	SupplierID  string    `json:"supplier_id"`
	ActorID     string    `json:"actor_id"`
	Kind        string    `json:"kind"`
	Before      *Supplier `json:"before"`
	After       Supplier  `json:"after"`
}

func supplierContact(name, email, phone, reason string) bool {
	if !clean(name, 120, true) || !clean(email, 254, false) || !clean(phone, 30, false) || !clean(reason, 500, true) {
		return false
	}
	if email != "" {
		a, e := mail.ParseAddress(email)
		if e != nil || a.Address != email || strings.ContainsAny(email, " \t\r\n") {
			return false
		}
	}
	if phone != "" {
		digits := 0
		for _, c := range phone {
			if c >= '0' && c <= '9' {
				digits++
			} else if !strings.ContainsRune("+ ()-", c) {
				return false
			}
		}
		if digits < 6 || digits > 20 {
			return false
		}
	}
	return true
}
func normalizeSupplier(v SupplierInput, update bool) (SupplierInput, error) {
	v.Name = strings.TrimSpace(v.Name)
	v.Email = strings.TrimSpace(v.Email)
	v.Phone = strings.TrimSpace(v.Phone)
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || !supplierContact(v.Name, v.Email, v.Phone, v.Reason) || (!update && v.ExpectedVersion != 0) || (update && (v.ExpectedVersion < 1 || v.ExpectedVersion >= MaxVersion)) {
		return v, ErrInput
	}
	return v, nil
}
func DecodeSupplier(content string, body []byte, update bool) (SupplierInput, error) {
	var v SupplierInput
	fields := map[string]bool{"operation_id": true, "name": true, "email": true, "phone": true, "active": true, "reason": true}
	if update {
		fields["expected_version"] = true
	}
	if proposalObject(content, body, fields) != nil || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeSupplier(v, update)
}
func validSupplier(p Supplier) bool {
	return ValidUUID(p.ID) && ValidUUID(p.CreatorID) && ValidUUID(p.LastActorID) && p.Version >= 1 && p.Version <= MaxVersion && supplierContact(p.Name, p.Email, p.Phone, p.Reason) && p.Name == strings.TrimSpace(p.Name) && p.Email == strings.TrimSpace(p.Email) && p.Phone == strings.TrimSpace(p.Phone) && p.Reason == strings.TrimSpace(p.Reason) && !p.CreatedAt.IsZero() && !p.UpdatedAt.Before(p.CreatedAt)
}
func validSupplierReceipt(r SupplierReceipt) bool {
	if !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || !validSupplier(r.After) || r.SupplierID != r.After.ID || r.ActorID != r.After.LastActorID {
		return false
	}
	if r.Kind == "create" {
		return r.Before == nil && r.After.ID == r.OperationID && r.After.Version == 1 && r.After.CreatorID == r.ActorID && r.After.CreatedAt.Equal(r.After.UpdatedAt)
	}
	if r.Kind != "update" || r.Before == nil || !validSupplier(*r.Before) || r.Before.Version >= MaxVersion {
		return false
	}
	return r.Before.ID == r.After.ID && r.Before.CreatorID == r.After.CreatorID && r.Before.CreatedAt.Equal(r.After.CreatedAt) && r.After.Version == r.Before.Version+1 && !r.After.UpdatedAt.Before(r.Before.UpdatedAt)
}

const supplierColumns = `supplier_id::text,version,active,payload`
const supplierSQL = `SELECT ` + supplierColumns + ` FROM online_suppliers WHERE tenant_id=$1 AND supplier_id=$2`

func scanSupplier(row scanner) (Supplier, error) {
	var p Supplier
	var id string
	var version int64
	var active bool
	var b []byte
	e := row.Scan(&id, &version, &active, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &p) != nil || !validSupplier(p) || id != p.ID || version != p.Version || active != p.Active {
		return Supplier{}, ErrUnavailable
	}
	return p, nil
}

const supplierReceiptSQL = `SELECT operation_id::text,supplier_id::text,actor_id::text,kind,payload_hash,receipt FROM online_supplier_operations WHERE tenant_id=$1 AND operation_id=$2`

func scanSupplierReceipt(row scanner) (SupplierReceipt, string, error) {
	var r SupplierReceipt
	var op, id, actor, kind, h string
	var b []byte
	e := row.Scan(&op, &id, &actor, &kind, &h, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &r) != nil || !validSupplierReceipt(r) || op != r.OperationID || id != r.SupplierID || actor != r.ActorID || kind != r.Kind || len(h) != 64 {
		return r, "", ErrUnavailable
	}
	return r, h, nil
}
func (s Store) Supplier(ctx context.Context, a Actor, id string) (Supplier, error) {
	if !ValidUUID(id) {
		return Supplier{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return Supplier{}, e
	}
	defer tx.Rollback()
	p, e := scanSupplier(tx.QueryRowContext(ctx, supplierSQL, a.Tenant, id))
	if e != nil {
		return p, e
	}
	if tx.Commit() != nil {
		return Supplier{}, ErrUnavailable
	}
	return p, nil
}
func (s Store) SupplierOperation(ctx context.Context, a Actor, op string) (SupplierReceipt, error) {
	if !ValidUUID(op) {
		return SupplierReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return SupplierReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanSupplierReceipt(tx.QueryRowContext(ctx, supplierReceiptSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return SupplierReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) CreateSupplier(ctx context.Context, a Actor, v SupplierInput) (SupplierReceipt, error) {
	v, e := normalizeSupplier(v, false)
	if e != nil {
		return SupplierReceipt{}, e
	}
	return s.writeSupplier(ctx, a, v.OperationID, "create", v.OperationID, proposalHash(v.OperationID, "create", v), &v, SupplierInput{})
}
func (s Store) UpdateSupplier(ctx context.Context, a Actor, id string, v SupplierInput) (SupplierReceipt, error) {
	v, e := normalizeSupplier(v, true)
	if e != nil || !ValidUUID(id) {
		return SupplierReceipt{}, ErrInput
	}
	return s.writeSupplier(ctx, a, id, "update", v.OperationID, proposalHash(id, "update", v), nil, v)
}

type SupplierQuery struct {
	Query  string `json:"q"`
	Active string `json:"active"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}
type SupplierPage struct {
	SupplierQuery
	Items      []Supplier `json:"items"`
	Total      int64      `json:"total"`
	HasMore    bool       `json:"has_more"`
	NextOffset *int       `json:"next_offset,omitempty"`
}

func ValidateSupplierQuery(q SupplierQuery) error {
	if !clean(q.Query, 120, false) || strings.IndexFunc(q.Query, unicode.IsControl) >= 0 || (q.Active != "all" && q.Active != "true" && q.Active != "false") || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	return nil
}

const supplierWhere = ` FROM online_suppliers WHERE tenant_id=$1 AND ($2='all' OR active=($2='true')) AND ($3='' OR payload->>'name' ILIKE $3 ESCAPE '\' OR payload->>'email' ILIKE $3 ESCAPE '\' OR payload->>'phone' ILIKE $3 ESCAPE '\')`

func (s Store) Suppliers(ctx context.Context, a Actor, q SupplierQuery) (SupplierPage, error) {
	var zero SupplierPage
	if ValidateSupplierQuery(q) != nil {
		return zero, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return zero, e
	}
	defer tx.Rollback()
	r := SupplierPage{SupplierQuery: q, Items: []Supplier{}}
	args := []any{a.Tenant, q.Active, qualityPattern(q.Query)}
	if tx.QueryRowContext(ctx, `SELECT count(*)`+supplierWhere, args...).Scan(&r.Total) != nil || r.Total < 0 || r.Total > MaxVersion {
		return zero, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+supplierColumns+supplierWhere+` ORDER BY payload->>'name' COLLATE "C",supplier_id LIMIT $4 OFFSET $5`, append(args, q.Limit, q.Offset)...)
	if e != nil {
		return zero, ErrUnavailable
	}
	for rows.Next() {
		p, e := scanSupplier(rows)
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
func (s Store) writeSupplier(ctx context.Context, a Actor, id, kind, op, h string, create *SupplierInput, v SupplierInput) (SupplierReceipt, error) {
	var zero SupplierReceipt
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
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":supplier:"+op); e != nil {
		return zero, ErrUnavailable
	}
	prior, ph, e := scanSupplierReceipt(tx.QueryRowContext(ctx, supplierReceiptSQL, a.Tenant, op))
	if e == nil {
		if prior.ActorID != a.User || prior.SupplierID != id || prior.Kind != kind || ph != h {
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
	var before *Supplier
	var after Supplier
	var at time.Time
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
		return zero, ErrUnavailable
	}
	at = at.UTC()
	if create != nil {
		after = Supplier{ID: id, Name: create.Name, Email: create.Email, Phone: create.Phone, Active: create.Active, Version: 1, CreatorID: a.User, LastActorID: a.User, Reason: create.Reason, CreatedAt: at, UpdatedAt: at}
	} else {
		p, err := scanSupplier(tx.QueryRowContext(ctx, supplierSQL+` FOR UPDATE`, a.Tenant, id))
		if err != nil {
			return zero, err
		}
		before = &p
		if p.Version != v.ExpectedVersion || p.Version >= MaxVersion {
			return zero, ErrConflict
		}
		after = p
		after.Name, after.Email, after.Phone, after.Active = v.Name, v.Email, v.Phone, v.Active
		after.Version++
		after.LastActorID = a.User
		after.Reason = v.Reason
		after.UpdatedAt = at
	}

	r := SupplierReceipt{op, id, a.User, kind, before, after}
	if !validSupplierReceipt(r) {
		return zero, ErrUnavailable
	}
	payload, _ := json.Marshal(after)
	var result sql.Result
	if create != nil {
		result, e = tx.ExecContext(ctx, `INSERT INTO online_suppliers(tenant_id,supplier_id,version,active,payload) VALUES($1,$2,$3,$4,$5)`, a.Tenant, id, after.Version, after.Active, payload)
	} else {
		result, e = tx.ExecContext(ctx, `UPDATE online_suppliers SET version=$3,active=$4,payload=$5 WHERE tenant_id=$1 AND supplier_id=$2 AND version=$6`, a.Tenant, id, after.Version, after.Active, payload, before.Version)
	}
	if e != nil {
		return zero, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	actual, e := scanSupplier(tx.QueryRowContext(ctx, supplierSQL, a.Tenant, id))
	if e != nil || !reflect.DeepEqual(actual, after) {
		return zero, ErrUnavailable
	}
	receipt, _ := json.Marshal(r)
	result, e = tx.ExecContext(ctx, `INSERT INTO online_supplier_operations(tenant_id,operation_id,supplier_id,actor_id,kind,payload_hash,receipt) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.Tenant, op, id, a.User, kind, h, receipt)
	if e != nil {
		return zero, conflict(e)
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	saved, sh, e := scanSupplierReceipt(tx.QueryRowContext(ctx, supplierReceiptSQL, a.Tenant, op))
	if e != nil || sh != h || !reflect.DeepEqual(saved, r) {
		return zero, ErrUnavailable
	}
	result, e = tx.ExecContext(ctx, `INSERT INTO online_supplier_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,$3,$4)`, a.Tenant, op, "supplier."+kind, receipt)
	if e != nil {
		return zero, ErrUnavailable
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	var out []byte
	var event string
	if tx.QueryRowContext(ctx, `SELECT event_type,payload FROM online_supplier_outbox WHERE tenant_id=$1 AND operation_id=$2`, a.Tenant, op).Scan(&event, &out) != nil {
		return zero, ErrUnavailable
	}
	var outReceipt SupplierReceipt
	if event != "supplier."+kind || json.Unmarshal(out, &outReceipt) != nil || !reflect.DeepEqual(outReceipt, r) {
		return zero, ErrUnavailable
	}
	if tx.Commit() != nil {
		return zero, ErrUnavailable
	}
	return r, nil
}
