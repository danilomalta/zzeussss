package onlinecatalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io"
	"mime"
	"sort"
	"time"
	"unicode/utf8"
)

const MaxBatchItems = 100
const MaxBatchHistoryPage = 10

type BatchItem struct {
	ProductID       int64  `json:"product_id"`
	Action          string `json:"action"`
	ExpectedVersion int64  `json:"expected_version"`
	PriceCents      *int64 `json:"price_cents,omitempty"`
	Active          *bool  `json:"ativo,omitempty"`
}
type BatchInput struct {
	OperationID string      `json:"operation_id"`
	Items       []BatchItem `json:"items"`
	PreviewHash string      `json:"preview_hash,omitempty"`
}
type BatchPreviewItem struct {
	Action string  `json:"action"`
	Before Product `json:"before"`
	After  Product `json:"after"`
}
type BatchPreview struct {
	OperationID string             `json:"operation_id"`
	PreviewHash string             `json:"preview_hash"`
	Items       []BatchPreviewItem `json:"items"`
}
type BatchReceipt struct {
	OperationID string    `json:"operation_id"`
	ActorID     string    `json:"actor_id"`
	PreviewHash string    `json:"preview_hash"`
	Items       []Receipt `json:"items"`
	CreatedAt   time.Time `json:"created_at"`
}

// Strict objects at both levels: duplicates, unknown fields and nulls fail closed.
func batchObject(raw []byte, allowed map[string]bool) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return nil, ErrInput
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		t, e = d.Token()
		k, ok := t.(string)
		if e != nil || !ok || !allowed[k] || fields[k] != nil {
			return nil, ErrInput
		}
		var v json.RawMessage
		if d.Decode(&v) != nil || bytes.Equal(v, []byte("null")) {
			return nil, ErrInput
		}
		fields[k] = v
	}
	if t, e = d.Token(); e != nil || t != json.Delim('}') {
		return nil, ErrInput
	}
	if d.Decode(new(interface{})) != io.EOF {
		return nil, ErrInput
	}
	return fields, nil
}
func DecodeBatch(content string, body []byte, apply bool) (BatchInput, error) {
	var b BatchInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 65536 || !utf8.Valid(body) {
		return b, ErrInput
	}
	allowed := map[string]bool{"operation_id": true, "items": true}
	if apply {
		allowed["preview_hash"] = true
	}
	fields, e := batchObject(body, allowed)
	if e != nil || len(fields) != len(allowed) {
		return b, ErrInput
	}
	if json.Unmarshal(fields["operation_id"], &b.OperationID) != nil {
		return b, ErrInput
	}
	if apply && json.Unmarshal(fields["preview_hash"], &b.PreviewHash) != nil {
		return b, ErrInput
	}
	var items []json.RawMessage
	if json.Unmarshal(fields["items"], &items) != nil || len(items) < 1 || len(items) > MaxBatchItems {
		return b, ErrInput
	}
	for _, raw := range items {
		f, e := batchObject(raw, map[string]bool{"product_id": true, "action": true, "expected_version": true, "price_cents": true, "ativo": true})
		if e != nil || len(f) != 4 {
			return b, ErrInput
		}
		var item BatchItem
		if json.Unmarshal(raw, &item) != nil {
			return b, ErrInput
		}
		b.Items = append(b.Items, item)
	}
	return normalizeBatch(b, apply)
}
func normalizeBatch(b BatchInput, apply bool) (BatchInput, error) {
	if !ValidUUID(b.OperationID) || len(b.Items) < 1 || len(b.Items) > MaxBatchItems {
		return BatchInput{}, ErrInput
	}
	if apply {
		if len(b.PreviewHash) != 64 {
			return BatchInput{}, ErrInput
		}
		for _, c := range b.PreviewHash {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return BatchInput{}, ErrInput
			}
		}
	} else if b.PreviewHash != "" {
		return BatchInput{}, ErrInput
	}
	b.Items = append([]BatchItem(nil), b.Items...)
	sort.Slice(b.Items, func(i, j int) bool { return b.Items[i].ProductID < b.Items[j].ProductID })
	for i, v := range b.Items {
		if v.ProductID < 1 || v.ProductID > MaxVersion || v.ExpectedVersion < 1 || v.ExpectedVersion >= MaxVersion || (i > 0 && v.ProductID == b.Items[i-1].ProductID) {
			return BatchInput{}, ErrInput
		}
		if v.Action == "price" {
			if v.PriceCents == nil || v.Active != nil || *v.PriceCents < 0 || *v.PriceCents > MaxPrice {
				return BatchInput{}, ErrInput
			}
		} else if v.Action == "active" {
			if v.Active == nil || v.PriceCents != nil {
				return BatchInput{}, ErrInput
			}
		} else {
			return BatchInput{}, ErrInput
		}
	}
	return b, nil
}
func batchChild(op string, id int64) string {
	// Deterministic, separate namespace; a retry cannot produce new item identities.
	return uuid.NewSHA1(uuid.MustParse(op), []byte("catalog-batch:"+jsonID(id))).String()
}
func jsonID(id int64) string { b, _ := json.Marshal(id); return string(b) }
func batchLive(ctx context.Context, tx *sql.Tx, a Actor) error {
	var live string
	e := tx.QueryRowContext(ctx, liveCatalogSQL, a.Session, a.Tenant, a.User, a.Role).Scan(&live)
	if errors.Is(e, sql.ErrNoRows) {
		return ErrDenied
	}
	if e != nil {
		return ErrUnavailable
	}
	return nil
}
func previewTx(ctx context.Context, tx *sql.Tx, a Actor, b BatchInput, lock bool) (BatchPreview, error) {
	result := BatchPreview{OperationID: b.OperationID, Items: make([]BatchPreviewItem, 0, len(b.Items))}
	query := productSQL
	if lock {
		query += " FOR UPDATE"
	}
	for _, v := range b.Items {
		p, e := scanProduct(tx.QueryRowContext(ctx, query, a.Tenant, v.ProductID))
		if e != nil {
			return BatchPreview{}, e
		}
		if p.Version != v.ExpectedVersion {
			return BatchPreview{}, ErrConflict
		}
		after := p
		after.Version++
		if v.Action == "price" {
			after.PriceCents = *v.PriceCents
		} else {
			after.Active = *v.Active
		}
		// A no-op is an input error, not an invented business change.
		if after.PriceCents == p.PriceCents && after.Active == p.Active {
			return BatchPreview{}, ErrInput
		}
		result.Items = append(result.Items, BatchPreviewItem{Action: v.Action, Before: p, After: after})
	}
	data, _ := json.Marshal(struct {
		Tenant, Actor, Operation string
		Items                    []BatchPreviewItem
	}{a.Tenant, a.User, b.OperationID, result.Items})
	result.PreviewHash = creationHash(data)
	return result, nil
}
func (s Store) PreviewBatch(ctx context.Context, a Actor, b BatchInput) (BatchPreview, error) {
	if !a.allowed("price") {
		return BatchPreview{}, ErrDenied
	}
	b, e := normalizeBatch(b, false)
	if e != nil {
		return BatchPreview{}, e
	}
	if s.DB == nil {
		return BatchPreview{}, ErrUnavailable
	}
	// Repeatable snapshot; no product, audit or outbox writes.
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return BatchPreview{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BatchPreview{}, e
	}
	p, e := previewTx(ctx, tx, a, b, false)
	if e != nil {
		return BatchPreview{}, e
	}
	if tx.Commit() != nil {
		return BatchPreview{}, ErrUnavailable
	}
	return p, nil
}

const batchSQL = `SELECT operation_id::text,actor_id::text,snapshot,created_at,payload_hash FROM online_catalog_batches WHERE tenant_id=$1 AND operation_id=$2`

func scanBatch(row scanner) (BatchReceipt, string, error) {
	var r BatchReceipt
	var raw []byte
	var hash string
	var op, actor string
	var at time.Time
	e := row.Scan(&op, &actor, &raw, &at, &hash)
	if errors.Is(e, sql.ErrNoRows) {
		return r, hash, ErrMissing
	}
	if e != nil || json.Unmarshal(raw, &r) != nil || r.OperationID != op || r.ActorID != actor || !ValidUUID(op) || !ValidUUID(actor) || len(r.Items) < 1 || len(r.Items) > MaxBatchItems || len(r.PreviewHash) != 64 {
		return BatchReceipt{}, "", ErrUnavailable
	}
	r.CreatedAt = at
	for i, item := range r.Items {
		if item.Before.ID < 1 || item.Before.ID > MaxVersion || item.Before.Version < 1 || item.After.Version > MaxVersion || item.Before.PriceCents < 0 || item.Before.PriceCents > MaxPrice || item.After.PriceCents < 0 || item.After.PriceCents > MaxPrice {
			return BatchReceipt{}, "", ErrUnavailable
		}
		if item.ActorID != actor || item.ProductID != item.Before.ID || item.ProductID != item.After.ID || item.OperationID != batchChild(op, item.ProductID) || item.After.Version != item.Before.Version+1 || (item.Action != "price" && item.Action != "active") || i > 0 && item.ProductID <= r.Items[i-1].ProductID {
			return BatchReceipt{}, "", ErrUnavailable
		}
	}
	return r, hash, nil
}
func (s Store) ApplyBatch(ctx context.Context, a Actor, b BatchInput) (BatchReceipt, error) {
	if !a.allowed("price") {
		return BatchReceipt{}, ErrDenied
	}
	b, e := normalizeBatch(b, true)
	if e != nil {
		return BatchReceipt{}, e
	}
	if s.DB == nil {
		return BatchReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return BatchReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	r, e := s.applyBatchTx(ctx, tx, a, b)
	if e != nil {
		return BatchReceipt{}, e
	}
	if tx.Commit() != nil {
		return BatchReceipt{}, ErrUnavailable
	}
	return r, nil
}

// applyBatchTx leaves commit ownership to the transaction caller.
func (s Store) applyBatchTx(ctx context.Context, tx *sql.Tx, a Actor, b BatchInput) (BatchReceipt, error) {
	var e error
	if e = batchLive(ctx, tx, a); e != nil {
		return BatchReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":batch:"+b.OperationID); e != nil {
		return BatchReceipt{}, ErrUnavailable
	}
	raw, _ := json.Marshal(b)
	hash := creationHash(raw)
	prior, old, e := scanBatch(tx.QueryRowContext(ctx, batchSQL, a.Tenant, b.OperationID))
	if e == nil {
		if prior.ActorID != a.User || hash != old {
			return BatchReceipt{}, ErrConflict
		}
		return prior, nil
	}
	if !errors.Is(e, ErrMissing) {
		return BatchReceipt{}, e
	}
	// Acquire ALL child operation locks before product locks, in product order.
	// This matches individual edit lock ordering and prevents lock inversions.
	for _, v := range b.Items {
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":"+batchChild(b.OperationID, v.ProductID)); e != nil {
			return BatchReceipt{}, ErrUnavailable
		}
	}
	p, e := previewTx(ctx, tx, a, b, true)
	if e != nil {
		return BatchReceipt{}, e
	}
	if p.PreviewHash != b.PreviewHash {
		return BatchReceipt{}, ErrConflict
	}
	r := BatchReceipt{OperationID: b.OperationID, ActorID: a.User, PreviewHash: b.PreviewHash, Items: make([]Receipt, 0, len(b.Items))}
	for i, v := range b.Items {
		c := Change{OperationID: batchChild(b.OperationID, v.ProductID), ExpectedVersion: v.ExpectedVersion}
		if v.PriceCents != nil {
			c.PriceCents = *v.PriceCents
		}
		if v.Active != nil {
			c.Active = *v.Active
		}
		item, e := s.mutateTx(ctx, tx, a, v.ProductID, v.Action, c)
		if e != nil {
			return BatchReceipt{}, e
		}
		if item.Before != p.Items[i].Before || item.After != p.Items[i].After {
			return BatchReceipt{}, ErrConflict
		}
		r.Items = append(r.Items, item)
	}
	snapshot, _ := json.Marshal(r)
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_batches(tenant_id,operation_id,actor_id,payload_hash,snapshot) VALUES($1,$2,$3,$4,$5::jsonb) RETURNING created_at`, a.Tenant, b.OperationID, a.User, hash, string(snapshot)).Scan(&r.CreatedAt)
	if e != nil {
		return BatchReceipt{}, conflict(e)
	}
	// Item outboxes and immutable audit records are already in this transaction.
	return r, nil
}
func (s Store) Batch(ctx context.Context, a Actor, op string) (BatchReceipt, error) {
	if !a.allowed("price") {
		return BatchReceipt{}, ErrDenied
	}
	if !ValidUUID(op) {
		return BatchReceipt{}, ErrInput
	}
	if s.DB == nil {
		return BatchReceipt{}, ErrUnavailable
	}
	r, _, e := scanBatch(s.DB.QueryRowContext(ctx, batchSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	return r, e
}
func (s Store) BatchHistory(ctx context.Context, a Actor, limit, offset int) ([]BatchReceipt, error) {
	if !a.allowed("price") {
		return nil, ErrDenied
	}
	if limit < 1 || limit > MaxBatchHistoryPage || offset < 0 || offset > 1000000000 {
		return nil, ErrInput
	}
	if s.DB == nil {
		return nil, ErrUnavailable
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT operation_id::text,actor_id::text,snapshot,created_at,payload_hash FROM online_catalog_batches WHERE tenant_id=$1 ORDER BY created_at DESC,operation_id DESC LIMIT $2 OFFSET $3`, a.Tenant, limit, offset)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	result := make([]BatchReceipt, 0)
	for rows.Next() {
		r, _, e := scanBatch(rows)
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
