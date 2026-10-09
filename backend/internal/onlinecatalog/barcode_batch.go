package onlinecatalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

type BarcodeBatchPreviewItem struct {
	Code          string  `json:"code"`
	CanonicalCode string  `json:"canonical_code"`
	Before        Product `json:"before"`
	After         Product `json:"after"`
}
type BarcodeBatchPreview struct {
	OperationID string                    `json:"operation_id"`
	Reason      string                    `json:"reason"`
	PreviewHash string                    `json:"preview_hash"`
	Items       []BarcodeBatchPreviewItem `json:"items"`
}
type BarcodeBatchReceipt struct {
	OperationID string           `json:"operation_id"`
	ActorID     string           `json:"actor_id"`
	Reason      string           `json:"reason"`
	PreviewHash string           `json:"preview_hash"`
	Items       []BarcodeReceipt `json:"items"`
	CreatedAt   time.Time        `json:"created_at"`
}

const barcodeBatchSQL = `SELECT operation_id::text,actor_id::text,snapshot,created_at,payload_hash FROM online_catalog_barcode_batches WHERE tenant_id=$1 AND operation_id=$2`

func barcodeBatchPreviewTx(ctx context.Context, tx *sql.Tx, a Actor, b BarcodeBatchInput, lock bool) (BarcodeBatchPreview, error) {
	p := BarcodeBatchPreview{OperationID: b.OperationID, Reason: b.Reason, Items: make([]BarcodeBatchPreviewItem, 0, len(b.Items))}
	query := productSQL
	if lock {
		query += " FOR UPDATE"
	}
	for _, v := range b.Items {
		before, e := scanProduct(tx.QueryRowContext(ctx, query, a.Tenant, v.ProductID))
		if e != nil {
			return p, e
		}
		if before.Version != v.ExpectedVersion {
			return p, ErrConflict
		}
		var count int
		if tx.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcodes WHERE tenant_id=$1 AND product_id=$2`, a.Tenant, v.ProductID).Scan(&count) != nil || count < 0 {
			return p, ErrUnavailable
		}
		if count >= MaxProductBarcodes {
			return p, ErrConflict
		}
		key, _ := CanonicalBarcode(v.Code)
		_, e = scanBarcode(tx.QueryRowContext(ctx, `SELECT `+barcodeColumns+` FROM online_catalog_barcodes WHERE tenant_id=$1 AND canonical_code=$2`, a.Tenant, key))
		if e == nil {
			return p, ErrConflict
		}
		if !errors.Is(e, ErrMissing) {
			return p, e
		}
		after := before
		after.Version++
		p.Items = append(p.Items, BarcodeBatchPreviewItem{Code: v.Code, CanonicalCode: key, Before: before, After: after})
	}
	raw, _ := json.Marshal(struct {
		Tenant, Actor string
		Preview       BarcodeBatchPreview
	}{a.Tenant, a.User, p})
	p.PreviewHash = creationHash(raw)
	return p, nil
}
func (s Store) PreviewBarcodeBatch(ctx context.Context, a Actor, b BarcodeBatchInput) (BarcodeBatchPreview, error) {
	if !a.allowed("details") {
		return BarcodeBatchPreview{}, ErrDenied
	}
	b, e := normalizeBarcodeBatch(b, false)
	if e != nil {
		return BarcodeBatchPreview{}, e
	}
	if s.DB == nil {
		return BarcodeBatchPreview{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return BarcodeBatchPreview{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BarcodeBatchPreview{}, e
	}
	p, e := barcodeBatchPreviewTx(ctx, tx, a, b, false)
	if e != nil {
		return BarcodeBatchPreview{}, e
	}
	if tx.Commit() != nil {
		return BarcodeBatchPreview{}, ErrUnavailable
	}
	return p, nil
}
func scanBarcodeBatch(row scanner) (BarcodeBatchReceipt, string, error) {
	var r BarcodeBatchReceipt
	var op, actor, hash string
	var raw []byte
	var at time.Time
	e := row.Scan(&op, &actor, &raw, &at, &hash)
	if errors.Is(e, sql.ErrNoRows) {
		return r, hash, ErrMissing
	}
	if e != nil || json.Unmarshal(raw, &r) != nil || !ValidUUID(op) || !ValidUUID(actor) || !importDigest(hash) || !importDigest(r.PreviewHash) || r.OperationID != op || r.ActorID != actor || !clean(r.Reason, 500, true) || at.IsZero() || len(r.Items) < 1 || len(r.Items) > MaxBarcodeBatchItems {
		return BarcodeBatchReceipt{}, "", ErrUnavailable
	}
	seen := map[string]bool{}
	for i, v := range r.Items {
		before := v.ProductBefore
		after := before
		after.Version++
		if before.ID < 1 || before.ID > MaxVersion || before.Version < 1 || before.Version >= MaxVersion || before.PriceCents < 0 || before.PriceCents > MaxPrice || v.ProductAfter != after || v.OperationID != barcodeBatchChild(op, before.ID) || v.ActorID != actor || v.Reason != r.Reason || v.Action != "add" || v.Before != nil || !validBarcode(v.After) || v.After.ID != v.OperationID || v.After.ProductID != before.ID || v.After.Version != 1 || !v.After.Active || v.CreatedAt.IsZero() || seen[v.After.CanonicalCode] || (i > 0 && before.ID <= r.Items[i-1].ProductBefore.ID) {
			return BarcodeBatchReceipt{}, "", ErrUnavailable
		}
		seen[v.After.CanonicalCode] = true
	}
	r.CreatedAt = at
	return r, hash, nil
}
func (s Store) ApplyBarcodeBatch(ctx context.Context, a Actor, b BarcodeBatchInput) (BarcodeBatchReceipt, error) {
	if !a.allowed("details") {
		return BarcodeBatchReceipt{}, ErrDenied
	}
	b, e := normalizeBarcodeBatch(b, true)
	if e != nil {
		return BarcodeBatchReceipt{}, e
	}
	if s.DB == nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BarcodeBatchReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":barcode-batch:"+b.OperationID); e != nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	raw, _ := json.Marshal(b)
	hash := creationHash(raw)
	prior, old, e := scanBarcodeBatch(tx.QueryRowContext(ctx, barcodeBatchSQL, a.Tenant, b.OperationID))
	if e == nil {
		if old != hash || prior.ActorID != a.User {
			return BarcodeBatchReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return BarcodeBatchReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if !errors.Is(e, ErrMissing) {
		return BarcodeBatchReceipt{}, e
	}
	// Child operation locks first, then every canonical key, then sorted products.
	// A single write also locks operation -> key -> product. No batch holds a
	// product while waiting for another key, preventing lock inversions.
	for _, v := range b.Items {
		child := barcodeBatchChild(b.OperationID, v.ProductID)
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":barcode-op:"+child); e != nil {
			return BarcodeBatchReceipt{}, ErrUnavailable
		}
		_, _, e = scanBarcodeOperation(tx.QueryRowContext(ctx, barcodeOperationSQL, a.Tenant, child))
		if e == nil {
			return BarcodeBatchReceipt{}, ErrConflict
		}
		if !errors.Is(e, ErrMissing) {
			return BarcodeBatchReceipt{}, e
		}
	}
	keys := make([]string, 0, len(b.Items))
	for _, v := range b.Items {
		k, _ := CanonicalBarcode(v.Code)
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":barcode-key:"+key); e != nil {
			return BarcodeBatchReceipt{}, ErrUnavailable
		}
	}
	p, e := barcodeBatchPreviewTx(ctx, tx, a, b, true)
	if e != nil {
		return BarcodeBatchReceipt{}, e
	}
	if p.PreviewHash != b.PreviewHash {
		return BarcodeBatchReceipt{}, ErrConflict
	}
	r := BarcodeBatchReceipt{OperationID: b.OperationID, ActorID: a.User, Reason: b.Reason, PreviewHash: b.PreviewHash, Items: make([]BarcodeReceipt, 0, len(b.Items))}
	for i, v := range b.Items {
		item, e := s.changeBarcodeTx(ctx, tx, a, v.ProductID, "", "add", BarcodeInput{OperationID: barcodeBatchChild(b.OperationID, v.ProductID), ExpectedVersion: v.ExpectedVersion, Code: v.Code, Reason: b.Reason})
		if e != nil {
			return BarcodeBatchReceipt{}, e
		}
		if item.ProductBefore != p.Items[i].Before || item.ProductAfter != p.Items[i].After {
			return BarcodeBatchReceipt{}, ErrConflict
		}
		r.Items = append(r.Items, item)
	}
	snapshot, _ := json.Marshal(r)
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_barcode_batches(tenant_id,operation_id,actor_id,payload_hash,snapshot) VALUES($1,$2,$3,$4,$5::jsonb) RETURNING created_at`, a.Tenant, b.OperationID, a.User, hash, string(snapshot)).Scan(&r.CreatedAt)
	if e != nil {
		return BarcodeBatchReceipt{}, conflict(e)
	}
	if r.CreatedAt.IsZero() {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	persisted, persistedHash, e := scanBarcodeBatch(tx.QueryRowContext(ctx, barcodeBatchSQL, a.Tenant, b.OperationID))
	expectedJSON, _ := json.Marshal(r)
	persistedJSON, _ := json.Marshal(persisted)
	if e != nil || persistedHash != hash || !bytes.Equal(expectedJSON, persistedJSON) {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	event, _ := json.Marshal(r)
	out, e := tx.ExecContext(ctx, `INSERT INTO online_catalog_barcode_batch_outbox(tenant_id,operation_id,event) VALUES($1,$2,$3::jsonb)`, a.Tenant, b.OperationID, string(event))
	if e != nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	if n, e := out.RowsAffected(); e != nil || n != 1 {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) BarcodeBatch(ctx context.Context, a Actor, op string) (BarcodeBatchReceipt, error) {
	if !a.allowed("details") {
		return BarcodeBatchReceipt{}, ErrDenied
	}
	if !ValidUUID(op) {
		return BarcodeBatchReceipt{}, ErrInput
	}
	if s.DB == nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BarcodeBatchReceipt{}, e
	}
	r, _, e := scanBarcodeBatch(tx.QueryRowContext(ctx, barcodeBatchSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	if e != nil {
		return BarcodeBatchReceipt{}, e
	}
	if tx.Commit() != nil {
		return BarcodeBatchReceipt{}, ErrUnavailable
	}
	return r, nil
}
