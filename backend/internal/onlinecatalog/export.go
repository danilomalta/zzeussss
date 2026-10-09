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

type ExportQuery struct {
	Action string `json:"action"`
	Query  string `json:"q"`
	Active string `json:"ativo"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}
type ExportPage struct {
	ExportQuery
	Total       int64     `json:"total"`
	HasMore     bool      `json:"has_more"`
	NextOffset  *int      `json:"next_offset,omitempty"`
	SourceHash  string    `json:"source_hash"`
	CSV         string    `json:"csv"`
	Items       []Product `json:"items"`
	GeneratedAt time.Time `json:"generated_at"`
}

func ValidateExport(q ExportQuery) error {
	if (q.Action != "price" && q.Action != "active") || !clean(q.Query, 120, false) || (q.Active != "all" && q.Active != "active" && q.Active != "inactive") || q.Limit < 1 || q.Limit > MaxBatchItems || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	return nil
}

const exportWhere = ` FROM products WHERE tenant_id=$1 AND deleted_at IS NULL AND ($2::text='all' OR ativo=($2='active')) AND ($3::text='' OR nome ILIKE $3 ESCAPE '\' OR sku ILIKE $3 ESCAPE '\')`

// Do not prefix/alter a SKU to neutralize a spreadsheet formula: that would break
// exact-SKU round trips. Refuse the entire page and let an authorised person fix it.
func exportable(p Product) bool {
	if !clean(p.SKU, 100, true) || p.SKU != strings.TrimSpace(p.SKU) || p.Version >= MaxVersion {
		return false
	}
	return !strings.ContainsAny(p.SKU[:1], "=+-@")
}
func buildExportCSV(items []Product, action string) (string, error) {
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"sku", "expected_version", "preco", "ativo"}) != nil {
		return "", ErrUnavailable
	}
	for _, p := range items {
		if !exportable(p) {
			return "", ErrConflict
		}
		row := []string{p.SKU, strconv.FormatInt(p.Version, 10), "", ""}
		if action == "price" {
			row[2] = fmt.Sprintf("%d.%02d", p.PriceCents/100, p.PriceCents%100)
		} else {
			row[3] = strconv.FormatBool(p.Active)
		}
		if w.Write(row) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > MaxImportBytes {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
func (s Store) Export(ctx context.Context, a Actor, q ExportQuery) (ExportPage, error) {
	if !a.allowed("price") {
		return ExportPage{}, ErrDenied
	}
	if e := ValidateExport(q); e != nil {
		return ExportPage{}, e
	}
	if s.DB == nil {
		return ExportPage{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return ExportPage{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return ExportPage{}, e
	}
	pattern := ""
	if q.Query != "" {
		pattern = "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q.Query) + "%"
	}
	result := ExportPage{ExportQuery: q, Items: make([]Product, 0)}
	// Count and page are in the same snapshot; a concurrent edit cannot mix them.
	e = tx.QueryRowContext(ctx, `SELECT count(*)`+exportWhere, a.Tenant, q.Active, pattern).Scan(&result.Total)
	if e != nil || result.Total < 0 || result.Total > 1000000000 {
		return ExportPage{}, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,nome,COALESCE(descricao,''),sku,preco::text,ativo,catalog_version`+exportWhere+` ORDER BY id ASC LIMIT $4 OFFSET $5`, a.Tenant, q.Active, pattern, q.Limit, q.Offset)
	if e != nil {
		return ExportPage{}, ErrUnavailable
	}
	var previous int64
	for rows.Next() {
		p, e := scanProduct(rows)
		if e != nil {
			rows.Close()
			return ExportPage{}, e
		}
		if p.ID <= previous || len(result.Items) >= q.Limit {
			rows.Close()
			return ExportPage{}, ErrUnavailable
		}
		previous = p.ID
		result.Items = append(result.Items, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return ExportPage{}, ErrUnavailable
	}
	expected := result.Total - int64(q.Offset)
	if expected < 0 {
		expected = 0
	}
	if expected > int64(q.Limit) {
		expected = int64(q.Limit)
	}
	if int64(len(result.Items)) != expected {
		return ExportPage{}, ErrUnavailable
	}
	result.CSV, e = buildExportCSV(result.Items, q.Action)
	if e != nil {
		return ExportPage{}, e
	}
	result.SourceHash = creationHash([]byte(result.CSV))
	result.HasMore = int64(q.Offset+len(result.Items)) < result.Total
	if result.HasMore {
		next := q.Offset + len(result.Items)
		result.NextOffset = &next
	}
	if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&result.GeneratedAt) != nil || result.GeneratedAt.IsZero() {
		return ExportPage{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return ExportPage{}, ErrUnavailable
	}
	return result, nil
}
