package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type DiscountSuggestion struct {
	ID             int64     `json:"id"`
	ProductID      int64     `json:"product_id"`
	DiscountBPS    int64     `json:"discount_bps"`
	SuggestedRange string    `json:"suggested_range"`
	Reason         string    `json:"reason"`
	Criteria       string    `json:"criteria"`
	Status         string    `json:"status"`
	ReviewedBy     string    `json:"reviewed_by"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Product        Product   `json:"product"`
}
type DiscountReviewReceipt struct {
	OperationID  string             `json:"operation_id"`
	SuggestionID int64              `json:"suggestion_id"`
	ActorID      string             `json:"actor_id"`
	Action       string             `json:"action"`
	Reason       string             `json:"reason"`
	Before       DiscountSuggestion `json:"before"`
	After        DiscountSuggestion `json:"after"`
	CreatedAt    time.Time          `json:"created_at"`
}

const discountSuggestionSQL = `SELECT id,product_id,suggested_discount::text,suggested_range,reason,criteria,status,reviewed_by,created_at,updated_at FROM discount_suggestions WHERE tenant_id=$1 AND id=$2 AND deleted_at IS NULL`
const discountReceiptSQL = `SELECT operation_id::text,suggestion_id,actor_id::text,action,reason,before_state,after_state,created_at,payload_hash FROM online_discount_reviews WHERE tenant_id=$1 AND operation_id=$2`

func scanDiscountSuggestion(row scanner) (DiscountSuggestion, error) {
	var r DiscountSuggestion
	var percent string
	e := row.Scan(&r.ID, &r.ProductID, &percent, &r.SuggestedRange, &r.Reason, &r.Criteria, &r.Status, &r.ReviewedBy, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrMissing
	}
	if e != nil {
		return r, ErrUnavailable
	}
	r.DiscountBPS, e = cents(percent)
	r.CreatedAt = r.CreatedAt.UTC()
	r.UpdatedAt = r.UpdatedAt.UTC()
	if e != nil || r.DiscountBPS > 10000 || r.ID < 1 || r.ID > MaxVersion || r.ProductID < 1 || r.ProductID > MaxVersion ||
		r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || (r.Status != "PENDING" && r.Status != "APPROVED" && r.Status != "REJECTED") ||
		(r.Status == "PENDING" && r.ReviewedBy != "") || (r.Status != "PENDING" && strings.TrimSpace(r.ReviewedBy) == "") {
		return DiscountSuggestion{}, ErrUnavailable
	}
	return r, nil
}
func scanDiscountReceipt(row scanner) (DiscountReviewReceipt, string, error) {
	var r DiscountReviewReceipt
	var b, a []byte
	var h string
	e := row.Scan(&r.OperationID, &r.SuggestionID, &r.ActorID, &r.Action, &r.Reason, &b, &a, &r.CreatedAt, &h)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil {
		return r, h, ErrUnavailable
	}
	status := "REJECTED"
	if r.Action == "approve" {
		status = "APPROVED"
	}
	if json.Unmarshal(b, &r.Before) != nil || json.Unmarshal(a, &r.After) != nil || !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) ||
		(r.Action != "approve" && r.Action != "reject") || !clean(r.Reason, 500, true) || r.SuggestionID < 1 || r.SuggestionID > MaxVersion ||
		r.CreatedAt.IsZero() || len(h) != 64 || r.Before.ID != r.SuggestionID || r.After.ID != r.SuggestionID ||
		r.Before.Status != "PENDING" || r.After.Status != status || r.After.ReviewedBy != r.ActorID ||
		r.Before.ProductID != r.After.ProductID || r.Before.DiscountBPS != r.After.DiscountBPS ||
		r.Before.Product.ID != r.Before.ProductID || r.After.Product != r.Before.Product {
		return DiscountReviewReceipt{}, "", ErrUnavailable
	}
	return r, h, nil
}
func (s Store) discountReadTx(ctx context.Context, a Actor) (*sql.Tx, error) {
	if !a.allowed("price") {
		return nil, ErrDenied
	}
	if s.DB == nil {
		return nil, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, ErrUnavailable
	}
	if e = batchLive(ctx, tx, a); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func (s Store) DiscountSuggestion(ctx context.Context, a Actor, id int64) (DiscountSuggestion, error) {
	if id < 1 || id > MaxVersion {
		return DiscountSuggestion{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return DiscountSuggestion{}, e
	}
	defer tx.Rollback()
	r, e := scanDiscountSuggestion(tx.QueryRowContext(ctx, discountSuggestionSQL, a.Tenant, id))
	if e != nil {
		return r, e
	}
	r.Product, e = scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, r.ProductID))
	if e != nil {
		return DiscountSuggestion{}, e
	}
	if tx.Commit() != nil {
		return DiscountSuggestion{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) ReviewDiscount(ctx context.Context, a Actor, id int64, action string, v DiscountReviewInput) (DiscountReviewReceipt, error) {
	if !a.allowed("price") {
		return DiscountReviewReceipt{}, ErrDenied
	}
	v, e := normalizeDiscountReview(v)
	if e != nil || id < 1 || id > MaxVersion || (action != "approve" && action != "reject") {
		return DiscountReviewReceipt{}, ErrInput
	}
	if s.DB == nil {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	raw, _ := json.Marshal(struct {
		ID     int64
		Action string
		Input  DiscountReviewInput
	}{id, action, v})
	h := creationHash(raw)
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return DiscountReviewReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":discount-review:"+v.OperationID); e != nil {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	prior, ph, e := scanDiscountReceipt(tx.QueryRowContext(ctx, discountReceiptSQL, a.Tenant, v.OperationID))
	if e == nil {
		if prior.ActorID != a.User || ph != h || prior.SuggestionID != id || prior.Action != action {
			return DiscountReviewReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return DiscountReviewReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if e != ErrMissing {
		return DiscountReviewReceipt{}, e
	}
	before, e := scanDiscountSuggestion(tx.QueryRowContext(ctx, discountSuggestionSQL+" FOR UPDATE", a.Tenant, id))
	if e != nil {
		return DiscountReviewReceipt{}, e
	}
	before.Product, e = scanProduct(tx.QueryRowContext(ctx, productSQL+" FOR SHARE", a.Tenant, before.ProductID))
	if e != nil {
		return DiscountReviewReceipt{}, e
	}
	if before.Status != "PENDING" || before.Product.Version != v.ExpectedProductVersion || before.DiscountBPS != v.ExpectedDiscountBPS ||
		(action == "approve" && !before.Product.Active) {
		return DiscountReviewReceipt{}, ErrConflict
	}
	status := "REJECTED"
	if action == "approve" {
		status = "APPROVED"
	}
	result, e := tx.ExecContext(ctx, `UPDATE discount_suggestions SET status=$3,reviewed_by=$4,updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND status='PENDING' AND deleted_at IS NULL`, a.Tenant, id, status, a.User)
	if e != nil {
		return DiscountReviewReceipt{}, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return DiscountReviewReceipt{}, ErrConflict
	}
	after, e := scanDiscountSuggestion(tx.QueryRowContext(ctx, discountSuggestionSQL, a.Tenant, id))
	if e != nil {
		return DiscountReviewReceipt{}, e
	}
	after.Product = before.Product
	if after.Status != status || after.ReviewedBy != a.User || after.ID != before.ID || after.ProductID != before.ProductID || after.DiscountBPS != before.DiscountBPS {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	b, _ := json.Marshal(before)
	af, _ := json.Marshal(after)
	_, e = tx.ExecContext(ctx, `INSERT INTO online_discount_reviews(tenant_id,operation_id,suggestion_id,actor_id,action,reason,payload_hash,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, a.Tenant, v.OperationID, id, a.User, action, v.Reason, h, b, af)
	if e != nil {
		return DiscountReviewReceipt{}, conflict(e)
	}
	receipt, ph, e := scanDiscountReceipt(tx.QueryRowContext(ctx, discountReceiptSQL, a.Tenant, v.OperationID))
	if e != nil || ph != h || receipt.Before != before || receipt.After != after || receipt.ActorID != a.User || receipt.Action != action || receipt.Reason != v.Reason {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	payload, _ := json.Marshal(receipt)
	if _, e = tx.ExecContext(ctx, `INSERT INTO online_discount_review_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,'discount.reviewed',$3)`, a.Tenant, v.OperationID, payload); e != nil {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	return receipt, nil
}
func (s Store) DiscountReview(ctx context.Context, a Actor, op string) (DiscountReviewReceipt, error) {
	if !ValidUUID(op) {
		return DiscountReviewReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return DiscountReviewReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanDiscountReceipt(tx.QueryRowContext(ctx, discountReceiptSQL+" AND actor_id=$3", a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return DiscountReviewReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) DiscountReviewHistory(ctx context.Context, a Actor, id int64, limit, offset int) ([]DiscountReviewReceipt, error) {
	if id < 1 || id > MaxVersion || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return nil, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = scanDiscountSuggestion(tx.QueryRowContext(ctx, discountSuggestionSQL, a.Tenant, id)); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT operation_id::text,suggestion_id,actor_id::text,action,reason,before_state,after_state,created_at,payload_hash FROM online_discount_reviews WHERE tenant_id=$1 AND suggestion_id=$2 ORDER BY created_at DESC,operation_id DESC LIMIT $3 OFFSET $4`, a.Tenant, id, limit, offset)
	if e != nil {
		return nil, ErrUnavailable
	}
	items := make([]DiscountReviewReceipt, 0)
	for rows.Next() {
		r, _, e := scanDiscountReceipt(rows)
		if e != nil || r.SuggestionID != id {
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
