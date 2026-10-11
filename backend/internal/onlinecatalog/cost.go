package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type CostInput struct {
	OperationID            string `json:"operation_id"`
	ExpectedProductVersion int64  `json:"expected_product_version"`
	ExpectedCostRevision   int64  `json:"expected_cost_revision"`
	CostCents              int64  `json:"cost_cents"`
	Reason                 string `json:"reason"`
}

func normalizeCost(v CostInput) (CostInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || v.ExpectedProductVersion < 1 || v.ExpectedProductVersion > MaxVersion || v.ExpectedCostRevision < 0 || v.ExpectedCostRevision >= MaxVersion || v.CostCents < 0 || v.CostCents > MaxPrice || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	return v, nil
}
func DecodeCost(content string, body []byte) (CostInput, error) {
	var v CostInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return v, ErrInput
	}
	fields, e := batchObject(body, map[string]bool{"operation_id": true, "expected_product_version": true, "expected_cost_revision": true, "cost_cents": true, "reason": true})
	if e != nil || len(fields) != 5 || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeCost(v)
}

// Revision zero and null fields represent an unknown cost, never a fabricated zero.
type CostState struct {
	Known     bool       `json:"known"`
	Revision  int64      `json:"revision"`
	CostCents *int64     `json:"cost_cents"`
	ActorID   *string    `json:"actor_id"`
	UpdatedAt *time.Time `json:"updated_at"`
}
type ProductCost struct {
	Product Product   `json:"product"`
	Cost    CostState `json:"cost"`
}
type CostReceipt struct {
	OperationID string    `json:"operation_id"`
	ProductID   int64     `json:"product_id"`
	ActorID     string    `json:"actor_id"`
	Reason      string    `json:"reason"`
	Before      CostState `json:"before"`
	After       CostState `json:"after"`
	Product     Product   `json:"product"`
	CreatedAt   time.Time `json:"created_at"`
}
type CostMargin struct {
	ProductCost
	MarginCents *int64  `json:"margin_cents"`
	MarginBPS   *string `json:"margin_bps"`
	State       string  `json:"state"`
}

const costStateSQL = `SELECT cost_cents,revision,actor_id::text,updated_at FROM online_product_costs WHERE tenant_id=$1 AND product_id=$2`
const costReceiptSQL = `SELECT operation_id::text,product_id,actor_id::text,reason,before_state,after_state,product_snapshot,created_at,payload_hash FROM online_cost_operations WHERE tenant_id=$1 AND operation_id=$2`

func validCostState(v CostState) bool {
	if !v.Known {
		return v.Revision == 0 && v.CostCents == nil && v.ActorID == nil && v.UpdatedAt == nil
	}
	return v.Revision >= 1 && v.Revision <= MaxVersion && v.CostCents != nil && *v.CostCents >= 0 && *v.CostCents <= MaxPrice && v.ActorID != nil && ValidUUID(*v.ActorID) && v.UpdatedAt != nil && !v.UpdatedAt.IsZero()
}
func scanCostState(row scanner) (CostState, error) {
	var c, rev int64
	var actor string
	var updated time.Time
	e := row.Scan(&c, &rev, &actor, &updated)
	if errors.Is(e, sql.ErrNoRows) {
		return CostState{}, nil
	}
	if e != nil {
		return CostState{}, ErrUnavailable
	}
	updated = updated.UTC()
	v := CostState{true, rev, &c, &actor, &updated}
	if !validCostState(v) {
		return CostState{}, ErrUnavailable
	}
	return v, nil
}
func scanCostReceipt(row scanner) (CostReceipt, string, error) {
	var r CostReceipt
	var b, a, p []byte
	var h string
	e := row.Scan(&r.OperationID, &r.ProductID, &r.ActorID, &r.Reason, &b, &a, &p, &r.CreatedAt, &h)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil {
		return r, h, ErrUnavailable
	}
	if json.Unmarshal(b, &r.Before) != nil || json.Unmarshal(a, &r.After) != nil || json.Unmarshal(p, &r.Product) != nil || !validCostState(r.Before) || !validCostState(r.After) || !r.After.Known || r.After.Revision != r.Before.Revision+1 || *r.After.ActorID != r.ActorID || !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || !clean(r.Reason, 500, true) || len(h) != 64 || r.CreatedAt.IsZero() || r.ProductID < 1 || r.ProductID > MaxVersion || r.Product.ID != r.ProductID || r.Product.Version < 1 || r.Product.Version > MaxVersion || r.Product.PriceCents < 0 || r.Product.PriceCents > MaxPrice {
		return CostReceipt{}, "", ErrUnavailable
	}
	r.CreatedAt = r.CreatedAt.UTC()
	return r, h, nil
}
func equalCost(a, b CostState) bool {
	if a.Known != b.Known || a.Revision != b.Revision {
		return false
	}
	if !a.Known {
		return validCostState(a) && validCostState(b)
	}
	return validCostState(a) && validCostState(b) && *a.CostCents == *b.CostCents && *a.ActorID == *b.ActorID && a.UpdatedAt.Equal(*b.UpdatedAt)
}
func (s Store) ProductCost(ctx context.Context, a Actor, id int64) (ProductCost, error) {
	if id < 1 || id > MaxVersion {
		return ProductCost{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return ProductCost{}, e
	}
	defer tx.Rollback()
	p, e := scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, id))
	if e != nil {
		return ProductCost{}, e
	}
	c, e := scanCostState(tx.QueryRowContext(ctx, costStateSQL, a.Tenant, id))
	if e != nil {
		return ProductCost{}, e
	}
	if tx.Commit() != nil {
		return ProductCost{}, ErrUnavailable
	}
	return ProductCost{p, c}, nil
}

// Basis points use the sale price denominator; halves round away from zero.
// MaxPrice*10000 fits int64. A zero price deliberately has no percentage.
func calculateMargin(v ProductCost) (CostMargin, error) {
	r := CostMargin{ProductCost: v, State: "unknown_cost"}
	if !validCostState(v.Cost) || v.Product.PriceCents < 0 || v.Product.PriceCents > MaxPrice {
		return r, ErrUnavailable
	}
	if !v.Cost.Known {
		return r, nil
	}
	m := v.Product.PriceCents - *v.Cost.CostCents
	r.MarginCents = &m
	if v.Product.PriceCents == 0 {
		r.State = "zero_price"
		return r, nil
	}
	n := m * 10000
	price := v.Product.PriceCents
	bps := n / price
	rem := n % price
	if rem*2 >= price {
		bps++
	} else if -rem*2 >= price {
		bps--
	}
	bpsText := strconv.FormatInt(bps, 10)
	r.MarginBPS = &bpsText
	r.State = "nonnegative_margin"
	if m < 0 {
		r.State = "negative_margin"
	}
	return r, nil
}
func (s Store) ProductMargin(ctx context.Context, a Actor, id int64) (CostMargin, error) {
	v, e := s.ProductCost(ctx, a, id)
	if e != nil {
		return CostMargin{}, e
	}
	return calculateMargin(v)
}
func costHash(id int64, v CostInput) string {
	b, _ := json.Marshal(struct {
		ID    int64
		Input CostInput
	}{id, v})
	return creationHash(b)
}
func (s Store) SetCost(ctx context.Context, a Actor, id int64, v CostInput) (CostReceipt, error) {
	if !a.allowed("price") {
		return CostReceipt{}, ErrDenied
	}
	v, e := normalizeCost(v)
	if e != nil || id < 1 || id > MaxVersion {
		return CostReceipt{}, ErrInput
	}
	if s.DB == nil {
		return CostReceipt{}, ErrUnavailable
	}
	h := costHash(id, v)
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return CostReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return CostReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":cost:"+v.OperationID); e != nil {
		return CostReceipt{}, ErrUnavailable
	}
	prior, ph, e := scanCostReceipt(tx.QueryRowContext(ctx, costReceiptSQL, a.Tenant, v.OperationID))
	if e == nil {
		if prior.ActorID != a.User || prior.ProductID != id || ph != h {
			return CostReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return CostReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if e != ErrMissing {
		return CostReceipt{}, e
	}
	// The product lock also serializes the first cost registration, with no cost row yet.
	p, e := scanProduct(tx.QueryRowContext(ctx, productSQL+" FOR UPDATE", a.Tenant, id))
	if e != nil {
		return CostReceipt{}, e
	}
	before, e := scanCostState(tx.QueryRowContext(ctx, costStateSQL+" FOR UPDATE", a.Tenant, id))
	if e != nil {
		return CostReceipt{}, e
	}
	if p.Version != v.ExpectedProductVersion || before.Revision != v.ExpectedCostRevision {
		return CostReceipt{}, ErrConflict
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO online_product_costs(tenant_id,product_id,cost_cents,revision,actor_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,product_id) DO UPDATE SET cost_cents=EXCLUDED.cost_cents,revision=EXCLUDED.revision,actor_id=EXCLUDED.actor_id,updated_at=clock_timestamp()`, a.Tenant, id, v.CostCents, before.Revision+1, a.User)
	if e != nil {
		return CostReceipt{}, conflict(e)
	}
	after, e := scanCostState(tx.QueryRowContext(ctx, costStateSQL, a.Tenant, id))
	if e != nil || !after.Known || after.Revision != before.Revision+1 || *after.CostCents != v.CostCents || *after.ActorID != a.User {
		return CostReceipt{}, ErrUnavailable
	}
	b, _ := json.Marshal(before)
	af, _ := json.Marshal(after)
	pr, _ := json.Marshal(p)
	if _, e = tx.ExecContext(ctx, `INSERT INTO online_cost_operations(tenant_id,operation_id,product_id,actor_id,reason,payload_hash,before_state,after_state,product_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, a.Tenant, v.OperationID, id, a.User, v.Reason, h, b, af, pr); e != nil {
		return CostReceipt{}, conflict(e)
	}
	receipt, ph, e := scanCostReceipt(tx.QueryRowContext(ctx, costReceiptSQL, a.Tenant, v.OperationID))
	if e != nil || ph != h || !equalCost(receipt.Before, before) || !equalCost(receipt.After, after) || receipt.Product != p || receipt.ActorID != a.User || receipt.Reason != v.Reason {
		return CostReceipt{}, ErrUnavailable
	}
	payload, _ := json.Marshal(receipt)
	result, e := tx.ExecContext(ctx, `INSERT INTO online_cost_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,'product.cost.changed',$3)`, a.Tenant, v.OperationID, payload)
	if e != nil {
		return CostReceipt{}, ErrUnavailable
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return CostReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return CostReceipt{}, ErrUnavailable
	}
	return receipt, nil
}
func (s Store) CostOperation(ctx context.Context, a Actor, op string) (CostReceipt, error) {
	if !ValidUUID(op) {
		return CostReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return CostReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanCostReceipt(tx.QueryRowContext(ctx, costReceiptSQL+" AND actor_id=$3", a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return CostReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) CostHistory(ctx context.Context, a Actor, id int64, limit, offset int) ([]CostReceipt, error) {
	if id < 1 || id > MaxVersion || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return nil, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, id)); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT operation_id::text,product_id,actor_id::text,reason,before_state,after_state,product_snapshot,created_at,payload_hash FROM online_cost_operations WHERE tenant_id=$1 AND product_id=$2 ORDER BY created_at DESC,operation_id DESC LIMIT $3 OFFSET $4`, a.Tenant, id, limit, offset)
	if e != nil {
		return nil, ErrUnavailable
	}
	items := make([]CostReceipt, 0)
	for rows.Next() {
		r, _, e := scanCostReceipt(rows)
		if e != nil || r.ProductID != id {
			rows.Close()
			return nil, ErrUnavailable
		}
		items = append(items, r)
	}
	e = rows.Err()
	ce := rows.Close()
	if e != nil || ce != nil {
		return nil, ErrUnavailable
	}
	if tx.Commit() != nil {
		return nil, ErrUnavailable
	}
	return items, nil
}
