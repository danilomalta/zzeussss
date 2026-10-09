package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxImportBytes = 65536

type ImportInput struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
	CSV         string `json:"csv"`
	PreviewHash string `json:"preview_hash,omitempty"`
}
type ImportPreview struct {
	OperationID string             `json:"operation_id"`
	Reason      string             `json:"reason"`
	SourceHash  string             `json:"source_hash"`
	PreviewHash string             `json:"preview_hash"`
	Items       []BatchPreviewItem `json:"items"`
}
type ImportReceipt struct {
	OperationID string       `json:"operation_id"`
	ActorID     string       `json:"actor_id"`
	Reason      string       `json:"reason"`
	SourceHash  string       `json:"source_hash"`
	PreviewHash string       `json:"preview_hash"`
	Batch       BatchReceipt `json:"batch"`
	CreatedAt   time.Time    `json:"created_at"`
}
type importRow struct {
	SKU  string
	Item BatchItem
}

func importDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// A CSV is a bounded list of existing SKU/version/one-field changes. No row is skipped.
func parseImport(source string) ([]importRow, error) {
	if len(source) == 0 || len(source) > MaxImportBytes || !utf8.ValidString(source) {
		return nil, ErrInput
	}
	reader := csv.NewReader(strings.NewReader(source))
	reader.Comma = ';'
	reader.FieldsPerRecord = 4
	header, e := reader.Read()
	if e != nil || strings.Join(header, ";") != "sku;expected_version;preco;ativo" {
		return nil, ErrInput
	}
	rows := make([]importRow, 0)
	seen := map[string]bool{}
	for {
		row, e := reader.Read()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil || len(rows) >= MaxBatchItems {
			return nil, ErrInput
		}
		sku := strings.TrimSpace(row[0])
		version, e := ProductID(row[1])
		if e != nil || version >= MaxVersion || !clean(sku, 100, true) || seen[sku] {
			return nil, ErrInput
		}
		seen[sku] = true
		item := BatchItem{ExpectedVersion: version}
		price, active := row[2], row[3]
		if (price == "") == (active == "") {
			return nil, ErrInput
		}
		if price != "" {
			separators := 0
			for _, c := range price {
				if c == '.' || c == ',' {
					separators++
				} else if c < '0' || c > '9' {
					return nil, ErrInput
				}
			}
			if separators > 1 || len(price) > 13 || price[0] == '.' || price[0] == ',' || price[len(price)-1] == '.' || price[len(price)-1] == ',' {
				return nil, ErrInput
			}
			value, e := cents(strings.ReplaceAll(price, ",", "."))
			if e != nil {
				return nil, ErrInput
			}
			item.Action = "price"
			item.PriceCents = &value
		} else {
			if active != "true" && active != "false" {
				return nil, ErrInput
			}
			value := active == "true"
			item.Action = "active"
			item.Active = &value
		}
		rows = append(rows, importRow{sku, item})
	}
	if len(rows) == 0 {
		return nil, ErrInput
	}
	return rows, nil
}
func normalizeImport(v ImportInput, apply bool) (ImportInput, []importRow, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || !clean(v.Reason, 500, true) || (apply && !importDigest(v.PreviewHash)) || (!apply && v.PreviewHash != "") {
		return v, nil, ErrInput
	}
	rows, e := parseImport(v.CSV)
	return v, rows, e
}
func DecodeImport(content string, body []byte, apply bool) (ImportInput, error) {
	var v ImportInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8*MaxImportBytes || !utf8.Valid(body) {
		return v, ErrInput
	}
	allowed := map[string]bool{"operation_id": true, "reason": true, "csv": true}
	if apply {
		allowed["preview_hash"] = true
	}
	fields, e := batchObject(body, allowed)
	if e != nil || len(fields) != len(allowed) || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	v, _, e = normalizeImport(v, apply)
	return v, e
}

const importSKU = `SELECT id,nome,COALESCE(descricao,''),sku,preco::text,ativo,catalog_version FROM products WHERE tenant_id=$1 AND sku=$2 AND deleted_at IS NULL ORDER BY id LIMIT 2`

func resolveImport(ctx context.Context, tx *sql.Tx, a Actor, v ImportInput, rows []importRow) (BatchInput, error) {
	b := BatchInput{OperationID: v.OperationID, Items: make([]BatchItem, 0, len(rows))}
	for _, row := range rows {
		found, e := tx.QueryContext(ctx, importSKU, a.Tenant, row.SKU)
		if e != nil {
			return b, ErrUnavailable
		}
		if !found.Next() {
			e = found.Err()
			found.Close()
			if e != nil {
				return b, ErrUnavailable
			}
			return b, ErrMissing
		}
		p, e := scanProduct(found)
		if e != nil {
			found.Close()
			return b, e
		}
		if found.Next() {
			found.Close()
			return b, ErrConflict
		}
		e = found.Err()
		found.Close()
		if e != nil {
			return b, ErrUnavailable
		}
		if p.SKU != row.SKU {
			return b, ErrConflict
		}
		item := row.Item
		item.ProductID = p.ID
		b.Items = append(b.Items, item)
	}
	return normalizeBatch(b, false)
}
func importPreview(v ImportInput, p BatchPreview) ImportPreview {
	out := ImportPreview{OperationID: v.OperationID, Reason: v.Reason, SourceHash: creationHash([]byte(v.CSV)), Items: p.Items}
	raw, _ := json.Marshal(struct{ Operation, Reason, SourceHash, BatchHash string }{v.OperationID, v.Reason, out.SourceHash, p.PreviewHash})
	out.PreviewHash = creationHash(raw)
	return out
}
func (s Store) PreviewImport(ctx context.Context, a Actor, v ImportInput) (ImportPreview, error) {
	if !a.allowed("price") {
		return ImportPreview{}, ErrDenied
	}
	v, rows, e := normalizeImport(v, false)
	if e != nil {
		return ImportPreview{}, e
	}
	if s.DB == nil {
		return ImportPreview{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return ImportPreview{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return ImportPreview{}, e
	}
	b, e := resolveImport(ctx, tx, a, v, rows)
	if e != nil {
		return ImportPreview{}, e
	}
	p, e := previewTx(ctx, tx, a, b, false)
	if e != nil {
		return ImportPreview{}, e
	}
	if tx.Commit() != nil {
		return ImportPreview{}, ErrUnavailable
	}
	return importPreview(v, p), nil
}

const importSQL = `SELECT operation_id::text,actor_id::text,reason,source_hash,snapshot,created_at,payload_hash FROM online_catalog_imports WHERE tenant_id=$1 AND operation_id=$2`

func scanImport(row scanner) (ImportReceipt, string, error) {
	var r ImportReceipt
	var op, actor, reason, source, hash string
	var raw []byte
	var at time.Time
	e := row.Scan(&op, &actor, &reason, &source, &raw, &at, &hash)
	if errors.Is(e, sql.ErrNoRows) {
		return r, hash, ErrMissing
	}
	if e != nil || json.Unmarshal(raw, &r) != nil || r.OperationID != op || r.ActorID != actor || r.Reason != reason || r.SourceHash != source || !ValidUUID(op) || !ValidUUID(actor) || !clean(reason, 500, true) || !importDigest(source) || !importDigest(hash) || !importDigest(r.PreviewHash) || r.Batch.OperationID != op || r.Batch.ActorID != actor || !importDigest(r.Batch.PreviewHash) || len(r.Batch.Items) < 1 || len(r.Batch.Items) > MaxBatchItems {
		return ImportReceipt{}, "", ErrUnavailable
	}
	for i, item := range r.Batch.Items {
		if item.ActorID != actor || item.ProductID < 1 || item.ProductID > MaxVersion || item.Before.ID != item.ProductID || item.After.ID != item.ProductID || item.Before.Version < 1 || item.After.Version > MaxVersion || item.After.Version != item.Before.Version+1 || item.Before.PriceCents < 0 || item.Before.PriceCents > MaxPrice || item.After.PriceCents < 0 || item.After.PriceCents > MaxPrice || item.OperationID != batchChild(op, item.ProductID) || (item.Action != "price" && item.Action != "active") || (i > 0 && item.ProductID <= r.Batch.Items[i-1].ProductID) {
			return ImportReceipt{}, "", ErrUnavailable
		}
	}
	r.CreatedAt = at
	return r, hash, nil
}
func (s Store) ApplyImport(ctx context.Context, a Actor, v ImportInput) (ImportReceipt, error) {
	if !a.allowed("price") {
		return ImportReceipt{}, ErrDenied
	}
	v, rows, e := normalizeImport(v, true)
	if e != nil {
		return ImportReceipt{}, e
	}
	if s.DB == nil {
		return ImportReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return ImportReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return ImportReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":import:"+v.OperationID); e != nil {
		return ImportReceipt{}, ErrUnavailable
	}
	raw, _ := json.Marshal(v)
	hash := creationHash(raw)
	prior, old, e := scanImport(tx.QueryRowContext(ctx, importSQL, a.Tenant, v.OperationID))
	if e == nil {
		if prior.ActorID != a.User || old != hash {
			return ImportReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return ImportReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if !errors.Is(e, ErrMissing) {
		return ImportReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":batch:"+v.OperationID); e != nil {
		return ImportReceipt{}, ErrUnavailable
	}
	_, _, e = scanBatch(tx.QueryRowContext(ctx, batchSQL, a.Tenant, v.OperationID))
	if e == nil {
		return ImportReceipt{}, ErrConflict
	}
	if !errors.Is(e, ErrMissing) {
		return ImportReceipt{}, e
	}
	b, e := resolveImport(ctx, tx, a, v, rows)
	if e != nil {
		return ImportReceipt{}, e
	}
	for _, item := range b.Items {
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":"+batchChild(v.OperationID, item.ProductID)); e != nil {
			return ImportReceipt{}, ErrUnavailable
		}
	}
	p, e := previewTx(ctx, tx, a, b, true)
	if e != nil {
		return ImportReceipt{}, e
	}
	preview := importPreview(v, p)
	if preview.PreviewHash != v.PreviewHash {
		return ImportReceipt{}, ErrConflict
	}
	b.PreviewHash = p.PreviewHash
	batch, e := s.applyBatchTx(ctx, tx, a, b)
	if e != nil {
		return ImportReceipt{}, e
	}
	r := ImportReceipt{OperationID: v.OperationID, ActorID: a.User, Reason: v.Reason, SourceHash: preview.SourceHash, PreviewHash: v.PreviewHash, Batch: batch}
	snapshot, _ := json.Marshal(r)
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_imports(tenant_id,operation_id,actor_id,reason,source_hash,payload_hash,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb) RETURNING created_at`, a.Tenant, v.OperationID, a.User, v.Reason, r.SourceHash, hash, string(snapshot)).Scan(&r.CreatedAt)
	if e != nil {
		return ImportReceipt{}, conflict(e)
	}
	event, _ := json.Marshal(r)
	result, e := tx.ExecContext(ctx, `INSERT INTO online_catalog_import_outbox(tenant_id,operation_id,event) VALUES($1,$2,$3::jsonb)`, a.Tenant, v.OperationID, string(event))
	if e != nil {
		return ImportReceipt{}, ErrUnavailable
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return ImportReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return ImportReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) Import(ctx context.Context, a Actor, op string, administrative bool) (ImportReceipt, error) {
	if !a.allowed("price") {
		return ImportReceipt{}, ErrDenied
	}
	if !ValidUUID(op) {
		return ImportReceipt{}, ErrInput
	}
	if s.DB == nil {
		return ImportReceipt{}, ErrUnavailable
	}
	q := importSQL
	args := []any{a.Tenant, op}
	if !administrative {
		q += ` AND actor_id=$3`
		args = append(args, a.User)
	}
	r, _, e := scanImport(s.DB.QueryRowContext(ctx, q, args...))
	return r, e
}
