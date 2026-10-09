package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Barcode struct {
	ID            string    `json:"id"`
	ProductID     int64     `json:"product_id"`
	Code          string    `json:"code"`
	CanonicalCode string    `json:"canonical_code"`
	Active        bool      `json:"ativo"`
	Version       int64     `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type BarcodeReceipt struct {
	OperationID   string    `json:"operation_id"`
	ActorID       string    `json:"actor_id"`
	Action        string    `json:"action"`
	Reason        string    `json:"reason"`
	Before        *Barcode  `json:"before"`
	After         Barcode   `json:"after"`
	ProductBefore Product   `json:"product_before"`
	ProductAfter  Product   `json:"product_after"`
	CreatedAt     time.Time `json:"created_at"`
}

const barcodeColumns = `id::text,product_id,code,canonical_code,ativo,code_version,created_at,updated_at`
const barcodeSQL = `SELECT ` + barcodeColumns + ` FROM online_catalog_barcodes WHERE tenant_id=$1 AND product_id=$2 AND id=$3`
const barcodeOperationSQL = `SELECT operation_id::text,actor_id::text,product_id,barcode_id::text,action,reason,snapshot,created_at,payload_hash FROM online_catalog_barcode_operations WHERE tenant_id=$1 AND operation_id=$2`

func validBarcode(b Barcode) bool {
	k, e := CanonicalBarcode(b.Code)
	return e == nil && k == b.CanonicalCode && ValidUUID(b.ID) && b.ProductID >= 1 && b.ProductID <= MaxVersion && b.Version >= 1 && b.Version <= MaxVersion && !b.CreatedAt.IsZero() && !b.UpdatedAt.IsZero() && !b.UpdatedAt.Before(b.CreatedAt)
}
func scanBarcode(row scanner) (Barcode, error) {
	var b Barcode
	e := row.Scan(&b.ID, &b.ProductID, &b.Code, &b.CanonicalCode, &b.Active, &b.Version, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(e, sql.ErrNoRows) {
		return b, ErrMissing
	}
	if e != nil || !validBarcode(b) {
		return Barcode{}, ErrUnavailable
	}
	return b, nil
}
func scanBarcodeOperation(row scanner) (BarcodeReceipt, string, error) {
	var r BarcodeReceipt
	var op, actor, codeID, action, reason, hash string
	var product int64
	var raw []byte
	var at time.Time
	e := row.Scan(&op, &actor, &product, &codeID, &action, &reason, &raw, &at, &hash)
	if errors.Is(e, sql.ErrNoRows) {
		return r, hash, ErrMissing
	}
	if e != nil || json.Unmarshal(raw, &r) != nil || !ValidUUID(op) || !ValidUUID(actor) || !importDigest(hash) || r.OperationID != op || r.ActorID != actor || r.After.ID != codeID || r.After.ProductID != product || r.Action != action || r.Reason != reason || !clean(reason, 500, true) || !validBarcode(r.After) || at.IsZero() {
		return BarcodeReceipt{}, "", ErrUnavailable
	}
	before := r.ProductBefore
	want := before
	want.Version++
	if before.ID != product || before.Version < 1 || before.Version >= MaxVersion || before.PriceCents < 0 || before.PriceCents > MaxPrice || r.ProductAfter != want {
		return BarcodeReceipt{}, "", ErrUnavailable
	}
	switch action {
	case "add":
		if r.Before != nil || r.After.ID != op || r.After.Version != 1 || !r.After.Active {
			return BarcodeReceipt{}, "", ErrUnavailable
		}
	case "active":
		if r.Before == nil || !validBarcode(*r.Before) || r.Before.Version >= MaxVersion {
			return BarcodeReceipt{}, "", ErrUnavailable
		}
		w := *r.Before
		w.Version++
		w.Active = !w.Active
		w.UpdatedAt = r.After.UpdatedAt
		if r.After != w {
			return BarcodeReceipt{}, "", ErrUnavailable
		}
	default:
		return BarcodeReceipt{}, "", ErrUnavailable
	}
	r.CreatedAt = at
	return r, hash, nil
}
func (s Store) ChangeBarcode(ctx context.Context, a Actor, product int64, codeID, action string, v BarcodeInput) (BarcodeReceipt, error) {
	if !a.allowed("details") {
		return BarcodeReceipt{}, ErrDenied
	}
	v, e := normalizeBarcode(v, action)
	if e != nil || product < 1 || product > MaxVersion || (action == "active" && !ValidUUID(codeID)) || (action == "add" && codeID != "") {
		return BarcodeReceipt{}, ErrInput
	}
	if s.DB == nil {
		return BarcodeReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return BarcodeReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BarcodeReceipt{}, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":barcode-op:"+v.OperationID); e != nil {
		return BarcodeReceipt{}, ErrUnavailable
	}
	payload, _ := json.Marshal(struct {
		Product         int64
		Barcode, Action string
		Input           BarcodeInput
	}{product, codeID, action, v})
	hash := creationHash(payload)
	prior, old, e := scanBarcodeOperation(tx.QueryRowContext(ctx, barcodeOperationSQL, a.Tenant, v.OperationID))
	if e == nil {
		if prior.ActorID != a.User || old != hash {
			return BarcodeReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return BarcodeReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if !errors.Is(e, ErrMissing) {
		return BarcodeReceipt{}, e
	}
	key := ""
	if action == "add" {
		key, _ = CanonicalBarcode(v.Code)
		if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":barcode-key:"+key); e != nil {
			return BarcodeReceipt{}, ErrUnavailable
		}
	}
	beforeProduct, e := scanProduct(tx.QueryRowContext(ctx, productSQL+` FOR UPDATE`, a.Tenant, product))
	if e != nil {
		return BarcodeReceipt{}, e
	}
	if beforeProduct.Version != v.ExpectedVersion {
		return BarcodeReceipt{}, ErrConflict
	}
	receipt := BarcodeReceipt{OperationID: v.OperationID, ActorID: a.User, Action: action, Reason: v.Reason, ProductBefore: beforeProduct, ProductAfter: beforeProduct}
	receipt.ProductAfter.Version++
	if action == "add" {
		var count int
		if tx.QueryRowContext(ctx, `SELECT count(*) FROM online_catalog_barcodes WHERE tenant_id=$1 AND product_id=$2`, a.Tenant, product).Scan(&count) != nil || count < 0 {
			return BarcodeReceipt{}, ErrUnavailable
		}
		if count >= MaxProductBarcodes {
			return BarcodeReceipt{}, ErrConflict
		}
		_, e = scanBarcode(tx.QueryRowContext(ctx, `SELECT `+barcodeColumns+` FROM online_catalog_barcodes WHERE tenant_id=$1 AND canonical_code=$2`, a.Tenant, key))
		if e == nil {
			return BarcodeReceipt{}, ErrConflict
		}
		if !errors.Is(e, ErrMissing) {
			return BarcodeReceipt{}, e
		}
		receipt.After, e = scanBarcode(tx.QueryRowContext(ctx, `INSERT INTO online_catalog_barcodes(tenant_id,id,product_id,code,canonical_code) VALUES($1,$2,$3,$4,$5) RETURNING `+barcodeColumns, a.Tenant, v.OperationID, product, v.Code, key))
		if e != nil {
			return BarcodeReceipt{}, conflict(e)
		}
		if receipt.After.ID != v.OperationID || receipt.After.ProductID != product || receipt.After.Code != v.Code || receipt.After.CanonicalCode != key || receipt.After.Version != 1 || !receipt.After.Active {
			return BarcodeReceipt{}, ErrUnavailable
		}
	} else {
		before, e := scanBarcode(tx.QueryRowContext(ctx, barcodeSQL+` FOR UPDATE`, a.Tenant, product, codeID))
		if e != nil {
			return BarcodeReceipt{}, e
		}
		if before.Version != v.ExpectedCodeVersion || before.Active == *v.Active {
			return BarcodeReceipt{}, ErrConflict
		}
		receipt.Before = &before
		receipt.After, e = scanBarcode(tx.QueryRowContext(ctx, `UPDATE online_catalog_barcodes SET ativo=$4,code_version=code_version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND product_id=$2 AND id=$3 AND code_version=$5 RETURNING `+barcodeColumns, a.Tenant, product, codeID, *v.Active, v.ExpectedCodeVersion))
		if e != nil {
			return BarcodeReceipt{}, ErrUnavailable
		}
		want := before
		want.Version++
		want.Active = *v.Active
		want.UpdatedAt = receipt.After.UpdatedAt
		if receipt.After != want {
			return BarcodeReceipt{}, ErrUnavailable
		}
	}
	changed, e := tx.ExecContext(ctx, `UPDATE products SET catalog_version=catalog_version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND catalog_version=$3`, a.Tenant, product, v.ExpectedVersion)
	if e != nil {
		return BarcodeReceipt{}, ErrUnavailable
	}
	if n, e := changed.RowsAffected(); e != nil || n != 1 {
		return BarcodeReceipt{}, ErrUnavailable
	}
	persisted, e := scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, product))
	if e != nil || persisted != receipt.ProductAfter {
		return BarcodeReceipt{}, ErrUnavailable
	}
	raw, _ := json.Marshal(receipt)
	e = tx.QueryRowContext(ctx, `INSERT INTO online_catalog_barcode_operations(tenant_id,operation_id,product_id,barcode_id,actor_id,action,reason,payload_hash,snapshot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb) RETURNING created_at`, a.Tenant, v.OperationID, product, receipt.After.ID, a.User, action, v.Reason, hash, string(raw)).Scan(&receipt.CreatedAt)
	if e != nil {
		return BarcodeReceipt{}, conflict(e)
	}
	if receipt.CreatedAt.IsZero() {
		return BarcodeReceipt{}, ErrUnavailable
	}
	event, _ := json.Marshal(receipt)
	out, e := tx.ExecContext(ctx, `INSERT INTO online_catalog_barcode_outbox(tenant_id,operation_id,event) VALUES($1,$2,$3::jsonb)`, a.Tenant, v.OperationID, string(event))
	if e != nil {
		return BarcodeReceipt{}, ErrUnavailable
	}
	if n, e := out.RowsAffected(); e != nil || n != 1 {
		return BarcodeReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return BarcodeReceipt{}, ErrUnavailable
	}
	return receipt, nil
}
func (s Store) BarcodeOperation(ctx context.Context, a Actor, op string) (BarcodeReceipt, error) {
	if !a.allowed("details") {
		return BarcodeReceipt{}, ErrDenied
	}
	if !ValidUUID(op) {
		return BarcodeReceipt{}, ErrInput
	}
	if s.DB == nil {
		return BarcodeReceipt{}, ErrUnavailable
	}
	r, _, e := scanBarcodeOperation(s.DB.QueryRowContext(ctx, barcodeOperationSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	return r, e
}
