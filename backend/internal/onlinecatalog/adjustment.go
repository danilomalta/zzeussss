package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type AdjustmentItem struct {
	ProductID       int64 `json:"product_id"`
	ExpectedVersion int64 `json:"expected_version"`
}
type AdjustmentInput struct {
	OperationID     string           `json:"operation_id"`
	Reason          string           `json:"reason"`
	RateBasisPoints int64            `json:"rate_basis_points"`
	Items           []AdjustmentItem `json:"items"`
	PreviewHash     string           `json:"preview_hash,omitempty"`
}
type AdjustmentPreview struct {
	OperationID     string             `json:"operation_id"`
	Reason          string             `json:"reason"`
	RateBasisPoints int64              `json:"rate_basis_points"`
	Rounding        string             `json:"rounding"`
	PreviewHash     string             `json:"preview_hash"`
	Items           []BatchPreviewItem `json:"items"`
}
type AdjustmentReceipt struct {
	OperationID     string       `json:"operation_id"`
	ActorID         string       `json:"actor_id"`
	Reason          string       `json:"reason"`
	RateBasisPoints int64        `json:"rate_basis_points"`
	Rounding        string       `json:"rounding"`
	PreviewHash     string       `json:"preview_hash"`
	Batch           BatchReceipt `json:"batch"`
	CreatedAt       time.Time    `json:"created_at"`
}

func normalizeAdjustment(v AdjustmentInput, apply bool) (AdjustmentInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || !clean(v.Reason, 500, true) || v.RateBasisPoints == 0 || v.RateBasisPoints < -10000 || v.RateBasisPoints > 10000 || len(v.Items) < 1 || len(v.Items) > MaxBatchItems || (apply && !importDigest(v.PreviewHash)) || (!apply && v.PreviewHash != "") {
		return v, ErrInput
	}
	v.Items = append([]AdjustmentItem(nil), v.Items...)
	sort.Slice(v.Items, func(i, j int) bool { return v.Items[i].ProductID < v.Items[j].ProductID })
	for i, item := range v.Items {
		if item.ProductID < 1 || item.ProductID > MaxVersion || item.ExpectedVersion < 1 || item.ExpectedVersion >= MaxVersion || (i > 0 && v.Items[i-1].ProductID == item.ProductID) {
			return v, ErrInput
		}
	}
	return v, nil
}
func DecodeAdjustment(content string, body []byte, apply bool) (AdjustmentInput, error) {
	var v AdjustmentInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 65536 || !utf8.Valid(body) {
		return v, ErrInput
	}
	allowed := map[string]bool{"operation_id": true, "reason": true, "rate_basis_points": true, "items": true}
	if apply {
		allowed["preview_hash"] = true
	}
	fields, e := batchObject(body, allowed)
	if e != nil || len(fields) != len(allowed) {
		return v, ErrInput
	}
	if json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	var rawItems []json.RawMessage
	if json.Unmarshal(fields["items"], &rawItems) != nil || len(rawItems) < 1 || len(rawItems) > MaxBatchItems {
		return v, ErrInput
	}
	for _, raw := range rawItems {
		f, e := batchObject(raw, map[string]bool{"product_id": true, "expected_version": true})
		if e != nil || len(f) != 2 {
			return v, ErrInput
		}
	}
	return normalizeAdjustment(v, apply)
}

// 100 basis points = 1%. Round the final nonnegative price half up to a cent.
// The permitted product/rate bounds keep multiplication within signed int64.
func adjustedPrice(price, rate int64) (int64, error) {
	if price < 0 || price > MaxPrice || rate < -10000 || rate > 10000 || rate == 0 {
		return 0, ErrInput
	}
	after := (price*(10000+rate) + 5000) / 10000
	if after > MaxPrice {
		return 0, ErrInput
	}
	return after, nil
}
func adjustmentBatch(ctx context.Context, tx *sql.Tx, a Actor, v AdjustmentInput, lock bool) (BatchInput, BatchPreview, error) {
	b := BatchInput{OperationID: v.OperationID, Items: make([]BatchItem, 0, len(v.Items))}
	query := productSQL
	if lock {
		query += " FOR UPDATE"
	}
	for _, item := range v.Items {
		p, e := scanProduct(tx.QueryRowContext(ctx, query, a.Tenant, item.ProductID))
		if e != nil {
			return b, BatchPreview{}, e
		}
		if p.Version != item.ExpectedVersion {
			return b, BatchPreview{}, ErrConflict
		}
		price, e := adjustedPrice(p.PriceCents, v.RateBasisPoints)
		if e != nil {
			return b, BatchPreview{}, e
		}
		b.Items = append(b.Items, BatchItem{ProductID: item.ProductID, ExpectedVersion: item.ExpectedVersion, Action: "price", PriceCents: &price})
	}
	p, e := previewTx(ctx, tx, a, b, lock)
	return b, p, e
}
func adjustmentPreview(v AdjustmentInput, p BatchPreview) AdjustmentPreview {
	v.PreviewHash = ""
	raw, _ := json.Marshal(struct {
		Input     AdjustmentInput
		BatchHash string
	}{v, p.PreviewHash})
	return AdjustmentPreview{OperationID: v.OperationID, Reason: v.Reason, RateBasisPoints: v.RateBasisPoints, Rounding: "half_up", PreviewHash: creationHash(raw), Items: p.Items}
}
func (s Store) PreviewAdjustment(ctx context.Context, a Actor, v AdjustmentInput) (AdjustmentPreview, error) {
	if !a.allowed("price") {
		return AdjustmentPreview{}, ErrDenied
	}
	v, e := normalizeAdjustment(v, false)
	if e != nil {
		return AdjustmentPreview{}, e
	}
	if s.DB == nil {
		return AdjustmentPreview{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return AdjustmentPreview{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return AdjustmentPreview{}, e
	}
	_, p, e := adjustmentBatch(ctx, tx, a, v, false)
	if e != nil {
		return AdjustmentPreview{}, e
	}
	if tx.Commit() != nil {
		return AdjustmentPreview{}, ErrUnavailable
	}
	return adjustmentPreview(v, p), nil
}

const adjustmentSQL = `SELECT operation_id::text,actor_id::text,reason,rate_basis_points,snapshot,created_at,payload_hash FROM online_catalog_adjustments WHERE tenant_id=$1 AND operation_id=$2`

func scanAdjustment(row scanner) (AdjustmentReceipt, string, error) {
	var r AdjustmentReceipt
	var op, actor, reason, hash string
	var rate int64
	var raw []byte
	var at time.Time
	e := row.Scan(&op, &actor, &reason, &rate, &raw, &at, &hash)
	if errors.Is(e, sql.ErrNoRows) {
		return r, hash, ErrMissing
	}
	if e != nil || json.Unmarshal(raw, &r) != nil || r.OperationID != op || r.ActorID != actor || r.Reason != reason || r.RateBasisPoints != rate || r.Rounding != "half_up" || !ValidUUID(op) || !ValidUUID(actor) || !clean(reason, 500, true) || rate == 0 || rate < -10000 || rate > 10000 || !importDigest(hash) || !importDigest(r.PreviewHash) || r.Batch.OperationID != op || r.Batch.ActorID != actor || !importDigest(r.Batch.PreviewHash) || len(r.Batch.Items) < 1 || len(r.Batch.Items) > MaxBatchItems {
		return AdjustmentReceipt{}, "", ErrUnavailable
	}
	for i, item := range r.Batch.Items {
		price, e := adjustedPrice(item.Before.PriceCents, rate)
		want := item.Before
		want.Version++
		want.PriceCents = price
		if e != nil || item.ActorID != actor || item.ProductID < 1 || item.ProductID > MaxVersion || item.Before.ID != item.ProductID || item.Before.Version < 1 || item.Before.Version >= MaxVersion || item.After != want || item.Before.PriceCents == item.After.PriceCents || item.OperationID != batchChild(op, item.ProductID) || item.Action != "price" || (i > 0 && item.ProductID <= r.Batch.Items[i-1].ProductID) {
			return AdjustmentReceipt{}, "", ErrUnavailable
		}
	}
	r.CreatedAt = at
	return r, hash, nil
}
func (s Store) ApplyAdjustment(ctx context.Context, a Actor, v AdjustmentInput) (AdjustmentReceipt, error) {
	if !a.allowed("price") {
		return AdjustmentReceipt{}, ErrDenied
	}
	v, e := normalizeAdjustment(v, true)
	if e != nil {
		return AdjustmentReceipt{}, e
	}
	if s.DB == nil {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return AdjustmentReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":adjustment:"+v.OperationID); e != nil {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	raw, _ := json.Marshal(v)
	hash := creationHash(raw)
	prior, old, e := scanAdjustment(tx.QueryRowContext(ctx, adjustmentSQL, a.Tenant, v.OperationID))
	if e == nil {
		if prior.ActorID != a.User || old != hash {
			return AdjustmentReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return AdjustmentReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if !errors.Is(e, ErrMissing) {
		return AdjustmentReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":batch:"+v.OperationID); e != nil {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	_, _, e = scanBatch(tx.QueryRowContext(ctx, batchSQL, a.Tenant, v.OperationID))
	if e == nil {
		return AdjustmentReceipt{}, ErrConflict
	}
	if !errors.Is(e, ErrMissing) {
		return AdjustmentReceipt{}, e
	}
	for _, item := range v.Items {
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":"+batchChild(v.OperationID, item.ProductID)); e != nil {
			return AdjustmentReceipt{}, ErrUnavailable
		}
	}
	b, p, e := adjustmentBatch(ctx, tx, a, v, true)
	if e != nil {
		return AdjustmentReceipt{}, e
	}
	preview := adjustmentPreview(v, p)
	if preview.PreviewHash != v.PreviewHash {
		return AdjustmentReceipt{}, ErrConflict
	}
	b.PreviewHash = p.PreviewHash
	batch, e := s.applyBatchTx(ctx, tx, a, b)
	if e != nil {
		return AdjustmentReceipt{}, e
	}
	r := AdjustmentReceipt{OperationID: v.OperationID, ActorID: a.User, Reason: v.Reason, RateBasisPoints: v.RateBasisPoints, Rounding: "half_up", PreviewHash: v.PreviewHash, Batch: batch}
	snapshot, _ := json.Marshal(r)
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_adjustments(tenant_id,operation_id,actor_id,reason,rate_basis_points,payload_hash,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb) RETURNING created_at`, a.Tenant, v.OperationID, a.User, v.Reason, v.RateBasisPoints, hash, string(snapshot)).Scan(&r.CreatedAt)
	if e != nil {
		return AdjustmentReceipt{}, conflict(e)
	}
	event, _ := json.Marshal(r)
	result, e := tx.ExecContext(ctx, `INSERT INTO online_catalog_adjustment_outbox(tenant_id,operation_id,event) VALUES($1,$2,$3::jsonb)`, a.Tenant, v.OperationID, string(event))
	if e != nil {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) Adjustment(ctx context.Context, a Actor, op string) (AdjustmentReceipt, error) {
	if !a.allowed("price") {
		return AdjustmentReceipt{}, ErrDenied
	}
	if !ValidUUID(op) {
		return AdjustmentReceipt{}, ErrInput
	}
	if s.DB == nil {
		return AdjustmentReceipt{}, ErrUnavailable
	}
	r, _, e := scanAdjustment(s.DB.QueryRowContext(ctx, adjustmentSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	return r, e
}
func (s Store) AdjustmentHistory(ctx context.Context, a Actor, limit, offset int) ([]AdjustmentReceipt, error) {
	if !a.allowed("price") {
		return nil, ErrDenied
	}
	if limit < 1 || limit > MaxBatchHistoryPage || offset < 0 || offset > 1000000000 {
		return nil, ErrInput
	}
	if s.DB == nil {
		return nil, ErrUnavailable
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT operation_id::text,actor_id::text,reason,rate_basis_points,snapshot,created_at,payload_hash FROM online_catalog_adjustments WHERE tenant_id=$1 ORDER BY created_at DESC,operation_id DESC LIMIT $2 OFFSET $3`, a.Tenant, limit, offset)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	result := make([]AdjustmentReceipt, 0)
	for rows.Next() {
		r, _, e := scanAdjustment(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, r)
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	return result, nil
}
