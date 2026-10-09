package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime"
	"strings"
	"time"
	"unicode/utf8"
)

type UndoInput struct {
	OperationID       string `json:"operation_id"`
	SourceOperationID string `json:"source_operation_id"`
	Reason            string `json:"reason"`
	PreviewHash       string `json:"preview_hash,omitempty"`
}
type UndoPreview struct {
	OperationID       string             `json:"operation_id"`
	SourceOperationID string             `json:"source_operation_id"`
	Reason            string             `json:"reason"`
	PreviewHash       string             `json:"preview_hash"`
	Items             []BatchPreviewItem `json:"items"`
}
type UndoReceipt struct {
	OperationID       string       `json:"operation_id"`
	SourceOperationID string       `json:"source_operation_id"`
	ActorID           string       `json:"actor_id"`
	Reason            string       `json:"reason"`
	PreviewHash       string       `json:"preview_hash"`
	Batch             BatchReceipt `json:"batch"`
	CreatedAt         time.Time    `json:"created_at"`
}

func normalizeUndo(u UndoInput, apply bool) (UndoInput, error) {
	u.Reason = strings.TrimSpace(u.Reason)
	if !ValidUUID(u.OperationID) || !ValidUUID(u.SourceOperationID) || u.OperationID == u.SourceOperationID || !clean(u.Reason, 500, true) {
		return UndoInput{}, ErrInput
	}
	if apply {
		if len(u.PreviewHash) != 64 {
			return UndoInput{}, ErrInput
		}
		for _, c := range u.PreviewHash {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return UndoInput{}, ErrInput
			}
		}
	} else if u.PreviewHash != "" {
		return UndoInput{}, ErrInput
	}
	return u, nil
}
func DecodeUndo(content string, body []byte, apply bool) (UndoInput, error) {
	var u UndoInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return u, ErrInput
	}
	allowed := map[string]bool{"operation_id": true, "source_operation_id": true, "reason": true}
	if apply {
		allowed["preview_hash"] = true
	}
	fields, e := batchObject(body, allowed)
	if e != nil || len(fields) != len(allowed) || json.Unmarshal(body, &u) != nil {
		return u, ErrInput
	}
	return normalizeUndo(u, apply)
}
func reverseBatch(source BatchReceipt, op string) (BatchInput, error) {
	b := BatchInput{OperationID: op, Items: make([]BatchItem, 0, len(source.Items))}
	for _, r := range source.Items {
		if r.After.Version >= MaxVersion {
			return BatchInput{}, ErrConflict
		}
		v := BatchItem{ProductID: r.ProductID, Action: r.Action, ExpectedVersion: r.After.Version}
		if r.Action == "price" {
			p := r.Before.PriceCents
			v.PriceCents = &p
		} else {
			active := r.Before.Active
			v.Active = &active
		}
		b.Items = append(b.Items, v)
	}
	return normalizeBatch(b, false)
}
func undoPreview(u UndoInput, p BatchPreview) UndoPreview {
	result := UndoPreview{OperationID: u.OperationID, SourceOperationID: u.SourceOperationID, Reason: u.Reason, Items: p.Items}
	raw, _ := json.Marshal(struct {
		Undo      UndoInput
		BatchHash string
	}{UndoInput{OperationID: u.OperationID, SourceOperationID: u.SourceOperationID, Reason: u.Reason}, p.PreviewHash})
	result.PreviewHash = creationHash(raw)
	return result
}
func undoSource(ctx context.Context, tx *sql.Tx, a Actor, u UndoInput) (BatchReceipt, error) {
	source, _, e := scanBatch(tx.QueryRowContext(ctx, batchSQL, a.Tenant, u.SourceOperationID))
	if e != nil {
		return BatchReceipt{}, e
	}
	// No second compensation and no compensation chains. Both are explicit conflicts.
	var excluded bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM online_catalog_undos WHERE tenant_id=$1 AND (source_operation_id=$2 OR operation_id=$2))`, a.Tenant, u.SourceOperationID).Scan(&excluded)
	if e != nil {
		return BatchReceipt{}, ErrUnavailable
	}
	if excluded {
		return BatchReceipt{}, ErrConflict
	}
	return source, nil
}
func undoPreviewTx(ctx context.Context, tx *sql.Tx, a Actor, u UndoInput, source BatchReceipt, lock bool) (BatchInput, BatchPreview, error) {
	b, e := reverseBatch(source, u.OperationID)
	if e != nil {
		return b, BatchPreview{}, e
	}
	p, e := previewTx(ctx, tx, a, b, lock)
	if e != nil {
		return b, p, e
	}
	// Version alone is insufficient if external SQL rewrites a value without bumping it.
	for i, v := range p.Items {
		if v.Before != source.Items[i].After {
			return b, p, ErrConflict
		}
	}
	return b, p, nil
}
func (s Store) PreviewUndo(ctx context.Context, a Actor, u UndoInput) (UndoPreview, error) {
	if !a.allowed("price") {
		return UndoPreview{}, ErrDenied
	}
	u, e := normalizeUndo(u, false)
	if e != nil {
		return UndoPreview{}, e
	}
	if s.DB == nil {
		return UndoPreview{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return UndoPreview{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return UndoPreview{}, e
	}
	source, e := undoSource(ctx, tx, a, u)
	if e != nil {
		return UndoPreview{}, e
	}
	_, p, e := undoPreviewTx(ctx, tx, a, u, source, false)
	if e != nil {
		return UndoPreview{}, e
	}
	if tx.Commit() != nil {
		return UndoPreview{}, ErrUnavailable
	}
	return undoPreview(u, p), nil
}

const undoSQL = `SELECT operation_id::text,source_operation_id::text,actor_id::text,reason,snapshot,created_at,payload_hash FROM online_catalog_undos WHERE tenant_id=$1 AND operation_id=$2`

func scanUndo(row scanner) (UndoReceipt, string, error) {
	var r UndoReceipt
	var op, source, actor, reason, hash string
	var raw []byte
	var at time.Time
	e := row.Scan(&op, &source, &actor, &reason, &raw, &at, &hash)
	if errors.Is(e, sql.ErrNoRows) {
		return r, hash, ErrMissing
	}
	if e != nil || json.Unmarshal(raw, &r) != nil || r.OperationID != op || r.SourceOperationID != source || r.ActorID != actor || r.Reason != reason || !ValidUUID(op) || !ValidUUID(source) || !ValidUUID(actor) || op == source || len(r.PreviewHash) != 64 || r.Batch.OperationID != op || r.Batch.ActorID != actor || len(r.Batch.Items) < 1 || len(r.Batch.Items) > 100 {
		return UndoReceipt{}, "", ErrUnavailable
	}
	if !clean(reason, 500, true) || len(r.Batch.PreviewHash) != 64 {
		return UndoReceipt{}, "", ErrUnavailable
	}
	for i, item := range r.Batch.Items {
		if item.ActorID != actor || item.ProductID < 1 || item.ProductID > MaxVersion || item.Before.ID != item.ProductID || item.After.ID != item.ProductID || item.Before.Version < 1 || item.After.Version > MaxVersion || item.After.Version != item.Before.Version+1 || item.Before.PriceCents < 0 || item.Before.PriceCents > MaxPrice || item.After.PriceCents < 0 || item.After.PriceCents > MaxPrice || item.OperationID != batchChild(op, item.ProductID) || (item.Action != "price" && item.Action != "active") || (i > 0 && item.ProductID <= r.Batch.Items[i-1].ProductID) {
			return UndoReceipt{}, "", ErrUnavailable
		}
	}
	r.CreatedAt = at
	return r, hash, nil
}
func (s Store) ApplyUndo(ctx context.Context, a Actor, u UndoInput) (UndoReceipt, error) {
	if !a.allowed("price") {
		return UndoReceipt{}, ErrDenied
	}
	u, e := normalizeUndo(u, true)
	if e != nil {
		return UndoReceipt{}, e
	}
	if s.DB == nil {
		return UndoReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return UndoReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return UndoReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":undo:"+u.OperationID); e != nil {
		return UndoReceipt{}, ErrUnavailable
	}
	raw, _ := json.Marshal(u)
	hash := creationHash(raw)
	prior, old, e := scanUndo(tx.QueryRowContext(ctx, undoSQL, a.Tenant, u.OperationID))
	if e == nil {
		if prior.ActorID != a.User || old != hash {
			return UndoReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return UndoReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if !errors.Is(e, ErrMissing) {
		return UndoReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":undo-source:"+u.SourceOperationID); e != nil {
		return UndoReceipt{}, ErrUnavailable
	}
	source, e := undoSource(ctx, tx, a, u)
	if e != nil {
		return UndoReceipt{}, e
	}
	// Reserve the compensation batch identity before checking it has not been used.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":batch:"+u.OperationID); e != nil {
		return UndoReceipt{}, ErrUnavailable
	}
	_, _, e = scanBatch(tx.QueryRowContext(ctx, batchSQL, a.Tenant, u.OperationID))
	if e == nil {
		return UndoReceipt{}, ErrConflict
	}
	if !errors.Is(e, ErrMissing) {
		return UndoReceipt{}, e
	}
	// Same ordering as regular batches: child identities first, then product locks.
	for _, item := range source.Items {
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":"+batchChild(u.OperationID, item.ProductID)); e != nil {
			return UndoReceipt{}, ErrUnavailable
		}
	}
	b, p, e := undoPreviewTx(ctx, tx, a, u, source, true)
	if e != nil {
		return UndoReceipt{}, e
	}
	preview := undoPreview(u, p)
	if preview.PreviewHash != u.PreviewHash {
		return UndoReceipt{}, ErrConflict
	}
	b.PreviewHash = p.PreviewHash
	batch, e := s.applyBatchTx(ctx, tx, a, b)
	if e != nil {
		return UndoReceipt{}, e
	}
	r := UndoReceipt{OperationID: u.OperationID, SourceOperationID: u.SourceOperationID, ActorID: a.User, Reason: u.Reason, PreviewHash: u.PreviewHash, Batch: batch}
	snapshot, _ := json.Marshal(r)
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_undos(tenant_id,operation_id,source_operation_id,actor_id,reason,payload_hash,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb) RETURNING created_at`, a.Tenant, u.OperationID, u.SourceOperationID, a.User, u.Reason, hash, string(snapshot)).Scan(&r.CreatedAt)
	if e != nil {
		return UndoReceipt{}, conflict(e)
	}
	event, _ := json.Marshal(r)
	result, e := tx.ExecContext(ctx, `INSERT INTO online_catalog_undo_outbox(tenant_id,operation_id,event) VALUES($1,$2,$3::jsonb)`, a.Tenant, u.OperationID, string(event))
	if e != nil {
		return UndoReceipt{}, ErrUnavailable
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return UndoReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return UndoReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) Undo(ctx context.Context, a Actor, op string) (UndoReceipt, error) {
	if !a.allowed("price") {
		return UndoReceipt{}, ErrDenied
	}
	if !ValidUUID(op) {
		return UndoReceipt{}, ErrInput
	}
	if s.DB == nil {
		return UndoReceipt{}, ErrUnavailable
	}
	r, _, e := scanUndo(s.DB.QueryRowContext(ctx, undoSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	return r, e
}
func (s Store) UndoStatus(ctx context.Context, a Actor, source string) (UndoReceipt, error) {
	if !a.allowed("price") {
		return UndoReceipt{}, ErrDenied
	}
	if !ValidUUID(source) {
		return UndoReceipt{}, ErrInput
	}
	if s.DB == nil {
		return UndoReceipt{}, ErrUnavailable
	}
	r, _, e := scanUndo(s.DB.QueryRowContext(ctx, strings.Replace(undoSQL, "operation_id=$2", "source_operation_id=$2", 1), a.Tenant, source))
	return r, e
}
