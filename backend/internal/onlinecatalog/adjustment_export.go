package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const MaxAdjustmentExportBytes = 4 * 1024 * 1024

type AdjustmentExportQuery struct {
	ActorID   string `json:"actor_id"`
	ProductID int64  `json:"product_id"`
	From      string `json:"from"`
	Until     string `json:"until"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}
type AdjustmentExportPage struct {
	AdjustmentExportQuery
	Total       int64               `json:"total"`
	HasMore     bool                `json:"has_more"`
	NextOffset  *int                `json:"next_offset,omitempty"`
	RowCount    int                 `json:"row_count"`
	TextPrefix  string              `json:"text_prefix"`
	SourceHash  string              `json:"source_hash"`
	CSV         string              `json:"csv"`
	Items       []AdjustmentReceipt `json:"items"`
	GeneratedAt time.Time           `json:"generated_at"`
}

func ValidateAdjustmentExport(q AdjustmentExportQuery) error {
	if (q.ActorID != "" && !ValidUUID(q.ActorID)) || q.ProductID < 0 || q.ProductID > MaxVersion || q.Limit < 1 || q.Limit > MaxBatchHistoryPage || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	if (q.From == "") != (q.Until == "") {
		return ErrInput
	}
	if q.From != "" {
		from, e := time.Parse(time.RFC3339Nano, q.From)
		if e != nil || from.Year() < 2000 || from.Year() > 9999 {
			return ErrInput
		}
		until, e := time.Parse(time.RFC3339Nano, q.Until)
		if e != nil || until.Year() < 2000 || until.Year() > 9999 || !until.After(from) || until.Sub(from) > 366*24*time.Hour {
			return ErrInput
		}
	}
	return nil
}

// All filters are parameters; count and page read immutable receipts in one snapshot.
// Product filtering retains a parent only if its original batch contains that ID.
const adjustmentExportWhere = ` FROM online_catalog_adjustments WHERE tenant_id=$1 AND ($2::text='' OR actor_id::text=$2) AND ($3::timestamptz IS NULL OR created_at >= $3) AND ($4::timestamptz IS NULL OR created_at < $4) AND ($5::text='' OR EXISTS(SELECT 1 FROM jsonb_array_elements(snapshot->'batch'->'items') item WHERE item->>'product_id'=$5))`
const adjustmentExportSelect = `SELECT operation_id::text,actor_id::text,reason,rate_basis_points,snapshot,created_at,payload_hash`
const adjustmentExportHeader = "operation_id;item_operation_id;created_at_utc;actor_id;product_id;sku_before;nome_before;sku_after;nome_after;version_before;version_after;price_before_cents;price_after_cents;price_before;price_after;rate_basis_points;reason;rounding\n"

// This is an audit report, not an import file. Prefix every text field with an
// apostrophe so spreadsheet software cannot execute formulas or discard SKU zeros.
func buildAdjustmentExportCSV(receipts []AdjustmentReceipt, productID int64) (string, int, error) {
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write(strings.Split(strings.TrimSuffix(adjustmentExportHeader, "\n"), ";")) != nil {
		return "", 0, ErrUnavailable
	}
	count := 0
	for _, r := range receipts {
		if !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || r.CreatedAt.IsZero() || !clean(r.Reason, 500, true) || r.Rounding != "half_up" || len(r.Batch.Items) < 1 || len(r.Batch.Items) > MaxBatchItems {
			return "", 0, ErrUnavailable
		}
		for _, item := range r.Batch.Items {
			if !clean(item.Before.Name, 255, true) || !clean(item.Before.SKU, 100, true) || !clean(item.After.Name, 255, true) || !clean(item.After.SKU, 100, true) {
				return "", 0, ErrUnavailable
			}
			if productID != 0 && item.ProductID != productID {
				continue
			}
			text := func(v string) string { return "'" + v }
			num := func(v int64) string { return strconv.FormatInt(v, 10) }
			money := func(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
			row := []string{text(r.OperationID), text(item.OperationID), text(r.CreatedAt.UTC().Format(time.RFC3339Nano)), text(r.ActorID), text(num(item.ProductID)), text(item.Before.SKU), text(item.Before.Name), text(item.After.SKU), text(item.After.Name), text(num(item.Before.Version)), text(num(item.After.Version)), num(item.Before.PriceCents), num(item.After.PriceCents), money(item.Before.PriceCents), money(item.After.PriceCents), num(r.RateBasisPoints), text(r.Reason), text(r.Rounding)}
			if w.Write(row) != nil {
				return "", 0, ErrUnavailable
			}
			count++
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > MaxAdjustmentExportBytes {
		return "", 0, ErrUnavailable
	}
	return out.String(), count, nil
}

func (s Store) ExportAdjustments(ctx context.Context, a Actor, q AdjustmentExportQuery) (AdjustmentExportPage, error) {
	if !a.allowed("price") {
		return AdjustmentExportPage{}, ErrDenied
	}
	if e := ValidateAdjustmentExport(q); e != nil {
		return AdjustmentExportPage{}, e
	}
	if s.DB == nil {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return AdjustmentExportPage{}, e
	}
	var from, until interface{}
	if q.From != "" {
		f, _ := time.Parse(time.RFC3339Nano, q.From)
		u, _ := time.Parse(time.RFC3339Nano, q.Until)
		from = f.UTC()
		until = u.UTC()
	}
	product := ""
	if q.ProductID != 0 {
		product = strconv.FormatInt(q.ProductID, 10)
	}
	args := []interface{}{a.Tenant, q.ActorID, from, until, product}
	result := AdjustmentExportPage{AdjustmentExportQuery: q, Items: make([]AdjustmentReceipt, 0), TextPrefix: "'"}
	if tx.QueryRowContext(ctx, `SELECT count(*)`+adjustmentExportWhere, args...).Scan(&result.Total) != nil || result.Total < 0 || result.Total > 1000000000 {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	args = append(args, q.Limit, q.Offset)
	rows, e := tx.QueryContext(ctx, adjustmentExportSelect+adjustmentExportWhere+` ORDER BY created_at DESC,operation_id DESC LIMIT $6 OFFSET $7`, args...)
	if e != nil {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	for rows.Next() {
		receipt, _, e := scanAdjustment(rows)
		if e != nil || receipt.CreatedAt.IsZero() || receipt.CreatedAt.Year() < 2000 || receipt.CreatedAt.Year() > 9999 || len(result.Items) >= q.Limit {
			rows.Close()
			return AdjustmentExportPage{}, ErrUnavailable
		}
		if q.ActorID != "" && receipt.ActorID != q.ActorID {
			rows.Close()
			return AdjustmentExportPage{}, ErrUnavailable
		}
		if from != nil && (receipt.CreatedAt.Before(from.(time.Time)) || !receipt.CreatedAt.Before(until.(time.Time))) {
			rows.Close()
			return AdjustmentExportPage{}, ErrUnavailable
		}
		if q.ProductID != 0 {
			found := false
			for _, i := range receipt.Batch.Items {
				found = found || i.ProductID == q.ProductID
			}
			if !found {
				rows.Close()
				return AdjustmentExportPage{}, ErrUnavailable
			}
		}
		if n := len(result.Items); n > 0 {
			prev := result.Items[n-1]
			if receipt.CreatedAt.After(prev.CreatedAt) || (receipt.CreatedAt.Equal(prev.CreatedAt) && receipt.OperationID >= prev.OperationID) {
				rows.Close()
				return AdjustmentExportPage{}, ErrUnavailable
			}
		}
		result.Items = append(result.Items, receipt)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	expected := result.Total - int64(q.Offset)
	if expected < 0 {
		expected = 0
	}
	if expected > int64(q.Limit) {
		expected = int64(q.Limit)
	}
	if int64(len(result.Items)) != expected {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	result.CSV, result.RowCount, e = buildAdjustmentExportCSV(result.Items, q.ProductID)
	if e != nil {
		return AdjustmentExportPage{}, e
	}
	result.SourceHash = creationHash([]byte(result.CSV))
	result.HasMore = int64(q.Offset+len(result.Items)) < result.Total
	if result.HasMore {
		next := q.Offset + len(result.Items)
		result.NextOffset = &next
	}
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&result.GeneratedAt) != nil || result.GeneratedAt.IsZero() {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return AdjustmentExportPage{}, ErrUnavailable
	}
	return result, nil
}
