package onlinecatalog

import (
	"context"
	"database/sql"
)

type BarcodeLookup struct {
	Barcode Barcode `json:"barcode"`
	Product Product `json:"product"`
}
type BarcodePage struct {
	Product Product   `json:"product"`
	Items   []Barcode `json:"items"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
	Total   int       `json:"total"`
	HasMore bool      `json:"has_more"`
}
type BarcodeHistoryPage struct {
	Items   []BarcodeReceipt `json:"items"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	Total   int64            `json:"total"`
	HasMore bool             `json:"has_more"`
}

func barcodeReader(a Actor) bool {
	return a.allowed("details") || (a.Role == "cashier" && ValidUUID(a.Tenant) && ValidUUID(a.User) && ValidUUID(a.Session))
}
func barcodePageInput(product int64, limit, offset int) error {
	if product < 1 || product > MaxVersion || limit < 1 || limit > 50 || offset < 0 || offset > 1000000000 {
		return ErrInput
	}
	return nil
}
func (s Store) LookupBarcode(ctx context.Context, a Actor, code string) (BarcodeLookup, error) {
	if !barcodeReader(a) {
		return BarcodeLookup{}, ErrDenied
	}
	key, e := CanonicalBarcode(code)
	if e != nil {
		return BarcodeLookup{}, e
	}
	if s.DB == nil {
		return BarcodeLookup{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return BarcodeLookup{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BarcodeLookup{}, e
	}
	b, e := scanBarcode(tx.QueryRowContext(ctx, `SELECT `+barcodeColumns+` FROM online_catalog_barcodes WHERE tenant_id=$1 AND canonical_code=$2 AND ativo=TRUE`, a.Tenant, key))
	if e != nil {
		return BarcodeLookup{}, e
	}
	if b.CanonicalCode != key || !b.Active {
		return BarcodeLookup{}, ErrUnavailable
	}
	p, e := scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, b.ProductID))
	if e != nil {
		return BarcodeLookup{}, e
	}
	if !p.Active {
		return BarcodeLookup{}, ErrMissing
	}
	if tx.Commit() != nil {
		return BarcodeLookup{}, ErrUnavailable
	}
	return BarcodeLookup{Barcode: b, Product: p}, nil
}
func (s Store) ListBarcodes(ctx context.Context, a Actor, product int64, state string, limit, offset int) (BarcodePage, error) {
	if !barcodeReader(a) {
		return BarcodePage{}, ErrDenied
	}
	if barcodePageInput(product, limit, offset) != nil || (state != "all" && state != "active" && state != "inactive") {
		return BarcodePage{}, ErrInput
	}
	if s.DB == nil {
		return BarcodePage{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return BarcodePage{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BarcodePage{}, e
	}
	p, e := scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, product))
	if e != nil {
		return BarcodePage{}, e
	}
	page := BarcodePage{Product: p, Items: make([]Barcode, 0), Limit: limit, Offset: offset}
	where := ` FROM online_catalog_barcodes WHERE tenant_id=$1 AND product_id=$2 AND ($3::text='all' OR ativo=($3='active'))`
	if tx.QueryRowContext(ctx, `SELECT count(*)`+where, a.Tenant, product, state).Scan(&page.Total) != nil || page.Total < 0 || page.Total > MaxProductBarcodes {
		return BarcodePage{}, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+barcodeColumns+where+` ORDER BY created_at,id LIMIT $4 OFFSET $5`, a.Tenant, product, state, limit, offset)
	if e != nil {
		return BarcodePage{}, ErrUnavailable
	}
	for rows.Next() {
		b, e := scanBarcode(rows)
		if e != nil || b.ProductID != product || len(page.Items) >= limit || (state == "active" && !b.Active) || (state == "inactive" && b.Active) {
			rows.Close()
			return BarcodePage{}, ErrUnavailable
		}
		if n := len(page.Items); n > 0 {
			prev := page.Items[n-1]
			if b.CreatedAt.Before(prev.CreatedAt) || (b.CreatedAt.Equal(prev.CreatedAt) && b.ID <= prev.ID) {
				rows.Close()
				return BarcodePage{}, ErrUnavailable
			}
		}
		page.Items = append(page.Items, b)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return BarcodePage{}, ErrUnavailable
	}
	expected := page.Total - offset
	if expected < 0 {
		expected = 0
	}
	if expected > limit {
		expected = limit
	}
	if len(page.Items) != expected {
		return BarcodePage{}, ErrUnavailable
	}
	page.HasMore = offset+len(page.Items) < page.Total
	if tx.Commit() != nil {
		return BarcodePage{}, ErrUnavailable
	}
	return page, nil
}
func (s Store) BarcodeHistory(ctx context.Context, a Actor, product int64, limit, offset int) (BarcodeHistoryPage, error) {
	if !a.allowed("price") {
		return BarcodeHistoryPage{}, ErrDenied
	}
	if barcodePageInput(product, limit, offset) != nil {
		return BarcodeHistoryPage{}, ErrInput
	}
	if s.DB == nil {
		return BarcodeHistoryPage{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return BarcodeHistoryPage{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return BarcodeHistoryPage{}, e
	}
	if _, e = scanProduct(tx.QueryRowContext(ctx, productSQL, a.Tenant, product)); e != nil {
		return BarcodeHistoryPage{}, e
	}
	page := BarcodeHistoryPage{Items: make([]BarcodeReceipt, 0), Limit: limit, Offset: offset}
	where := ` FROM online_catalog_barcode_operations WHERE tenant_id=$1 AND product_id=$2`
	if tx.QueryRowContext(ctx, `SELECT count(*)`+where, a.Tenant, product).Scan(&page.Total) != nil || page.Total < 0 || page.Total > 1000000000 {
		return BarcodeHistoryPage{}, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT operation_id::text,actor_id::text,product_id,barcode_id::text,action,reason,snapshot,created_at,payload_hash`+where+` ORDER BY created_at DESC,operation_id DESC LIMIT $3 OFFSET $4`, a.Tenant, product, limit, offset)
	if e != nil {
		return BarcodeHistoryPage{}, ErrUnavailable
	}
	for rows.Next() {
		receipt, _, e := scanBarcodeOperation(rows)
		if e != nil || receipt.After.ProductID != product || len(page.Items) >= limit {
			rows.Close()
			return BarcodeHistoryPage{}, ErrUnavailable
		}
		if n := len(page.Items); n > 0 {
			prev := page.Items[n-1]
			if receipt.CreatedAt.After(prev.CreatedAt) || (receipt.CreatedAt.Equal(prev.CreatedAt) && receipt.OperationID >= prev.OperationID) {
				rows.Close()
				return BarcodeHistoryPage{}, ErrUnavailable
			}
		}
		page.Items = append(page.Items, receipt)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return BarcodeHistoryPage{}, ErrUnavailable
	}
	expected := page.Total - int64(offset)
	if expected < 0 {
		expected = 0
	}
	if expected > int64(limit) {
		expected = int64(limit)
	}
	if int64(len(page.Items)) != expected {
		return BarcodeHistoryPage{}, ErrUnavailable
	}
	page.HasMore = int64(offset+len(page.Items)) < page.Total
	if tx.Commit() != nil {
		return BarcodeHistoryPage{}, ErrUnavailable
	}
	return page, nil
}
