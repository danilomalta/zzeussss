package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Operational checks on the online catalog, not fiscal validation or POS stock.
type QualityQuery struct {
	Query  string `json:"q"`
	Active string `json:"active"`
	Issue  string `json:"issue"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}
type QualityItem struct {
	Product
	Stock  int64    `json:"estoque"`
	Issues []string `json:"issues"`
}
type QualityPage struct {
	QualityQuery
	Total       int64         `json:"total"`
	Items       []QualityItem `json:"items"`
	HasMore     bool          `json:"has_more"`
	NextOffset  *int          `json:"next_offset,omitempty"`
	GeneratedAt time.Time     `json:"generated_at"`
	SourceHash  string        `json:"source_hash"`
}
type QualitySummary struct {
	Query           string    `json:"q"`
	Active          string    `json:"active"`
	Products        int64     `json:"products"`
	PendingProducts int64     `json:"pending_products"`
	NoActiveGTIN    int64     `json:"no_active_gtin"`
	NoDescription   int64     `json:"no_description"`
	ZeroStock       int64     `json:"zero_stock"`
	GeneratedAt     time.Time `json:"generated_at"`
}

func ValidateQuality(q QualityQuery) error {
	if !clean(q.Query, 120, false) || (q.Active != "all" && q.Active != "active" && q.Active != "inactive") ||
		(q.Issue != "any" && q.Issue != "no_active_gtin" && q.Issue != "no_description" && q.Issue != "zero_stock") ||
		q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	return nil
}
func qualityPattern(q string) string {
	if q == "" {
		return ""
	}
	return "%" + strings.NewReplacer("\u005c", "\u005c\u005c", "%", "\u005c%", "_", "\u005c_").Replace(q) + "%"
}

const qualityCTE = `WITH quality AS (
 SELECT p.id,p.nome,COALESCE(p.descricao,'') AS descricao,p.sku,p.preco::text AS price,p.ativo,p.catalog_version,p.estoque,
 NOT EXISTS(SELECT 1 FROM online_catalog_barcodes b WHERE b.tenant_id=p.tenant_id AND b.product_id=p.id AND b.ativo=TRUE) AS no_active_gtin,
 btrim(COALESCE(p.descricao,''), E' \t\r\n')='' AS no_description,
 p.estoque=0 AS zero_stock
 FROM products p WHERE p.tenant_id=$1 AND p.deleted_at IS NULL
 AND ($2::text='all' OR p.ativo=($2='active'))
 AND ($3::text='' OR p.nome ILIKE $3 ESCAPE '\' OR p.sku ILIKE $3 ESCAPE '\')
)`
const qualityWhere = ` FROM quality WHERE
 ($4::text='any' AND (no_active_gtin OR no_description OR zero_stock))
 OR ($4='no_active_gtin' AND no_active_gtin)
 OR ($4='no_description' AND no_description)
 OR ($4='zero_stock' AND zero_stock)`

func qualityIssues(gtin, description, stock bool) []string {
	r := make([]string, 0, 3)
	if gtin {
		r = append(r, "no_active_gtin")
	}
	if description {
		r = append(r, "no_description")
	}
	if stock {
		r = append(r, "zero_stock")
	}
	return r
}
func (s Store) qualityTx(ctx context.Context, a Actor, q QualityQuery) (*sql.Tx, error) {
	if !a.allowed("details") {
		return nil, ErrDenied
	}
	if e := ValidateQuality(q); e != nil {
		return nil, e
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
func (s Store) QualitySummary(ctx context.Context, a Actor, q QualityQuery) (QualitySummary, error) {
	tx, e := s.qualityTx(ctx, a, q)
	if e != nil {
		return QualitySummary{}, e
	}
	defer tx.Rollback()
	r := QualitySummary{Query: q.Query, Active: q.Active}
	e = tx.QueryRowContext(ctx, qualityCTE+` SELECT count(*),
 count(*) FILTER(WHERE no_active_gtin OR no_description OR zero_stock),
 count(*) FILTER(WHERE no_active_gtin),count(*) FILTER(WHERE no_description),
 count(*) FILTER(WHERE zero_stock),transaction_timestamp() FROM quality`,
		a.Tenant, q.Active, qualityPattern(q.Query)).Scan(&r.Products, &r.PendingProducts, &r.NoActiveGTIN, &r.NoDescription, &r.ZeroStock, &r.GeneratedAt)
	if e != nil || r.Products < 0 || r.Products > MaxVersion || r.PendingProducts < 0 || r.PendingProducts > r.Products ||
		r.NoActiveGTIN < 0 || r.NoActiveGTIN > r.PendingProducts || r.NoDescription < 0 || r.NoDescription > r.PendingProducts ||
		r.ZeroStock < 0 || r.ZeroStock > r.PendingProducts || r.GeneratedAt.IsZero() {
		return QualitySummary{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return QualitySummary{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) Quality(ctx context.Context, a Actor, q QualityQuery) (QualityPage, error) {
	tx, e := s.qualityTx(ctx, a, q)
	if e != nil {
		return QualityPage{}, e
	}
	defer tx.Rollback()
	r := QualityPage{QualityQuery: q, Items: make([]QualityItem, 0)}
	e = tx.QueryRowContext(ctx, qualityCTE+" SELECT count(*),transaction_timestamp()"+qualityWhere,
		a.Tenant, q.Active, qualityPattern(q.Query), q.Issue).Scan(&r.Total, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.GeneratedAt.IsZero() {
		return QualityPage{}, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, qualityCTE+` SELECT id,nome,descricao,sku,price,ativo,catalog_version,estoque,no_active_gtin,no_description,zero_stock`+qualityWhere+" ORDER BY id ASC LIMIT $5 OFFSET $6",
		a.Tenant, q.Active, qualityPattern(q.Query), q.Issue, q.Limit, q.Offset)
	if e != nil {
		return QualityPage{}, ErrUnavailable
	}
	var previous int64
	for rows.Next() {
		var p QualityItem
		var price string
		var g, d, z bool
		if rows.Scan(&p.ID, &p.Name, &p.Description, &p.SKU, &price, &p.Active, &p.Version, &p.Stock, &g, &d, &z) != nil {
			rows.Close()
			return QualityPage{}, ErrUnavailable
		}
		p.PriceCents, e = cents(price)
		p.Issues = qualityIssues(g, d, z)
		if e != nil || p.ID <= previous || p.ID > MaxVersion || p.Version < 1 || p.Version > MaxVersion ||
			p.Stock < 0 || p.Stock > MaxVersion || len(p.Issues) == 0 ||
			(q.Issue != "any" && !qualityHas(p.Issues, q.Issue)) || (q.Active == "active" && !p.Active) || (q.Active == "inactive" && p.Active) ||
			d != (strings.Trim(p.Description, " \t\r\n") == "") || z != (p.Stock == 0) {
			rows.Close()
			return QualityPage{}, ErrUnavailable
		}
		previous = p.ID
		r.Items = append(r.Items, p)
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	remaining := r.Total - int64(q.Offset)
	if remaining < 0 {
		remaining = 0
	}
	want := remaining
	if want > int64(q.Limit) {
		want = int64(q.Limit)
	}
	if rowErr != nil || closeErr != nil || int64(len(r.Items)) != want {
		return QualityPage{}, ErrUnavailable
	}
	r.HasMore = int64(q.Offset)+int64(len(r.Items)) < r.Total
	if r.HasMore {
		next := q.Offset + len(r.Items)
		r.NextOffset = &next
	}
	raw, e := json.Marshal(struct {
		Query QualityQuery
		Total int64
		Items []QualityItem
	}{q, r.Total, r.Items})
	if e != nil {
		return QualityPage{}, ErrUnavailable
	}
	r.SourceHash = creationHash(raw)
	if tx.Commit() != nil {
		return QualityPage{}, ErrUnavailable
	}
	return r, nil
}
func qualityHas(issues []string, want string) bool {
	for _, s := range issues {
		if s == want {
			return true
		}
	}
	return false
}

// Report only: escape formula prefixes without altering stored product data.
func qualityCSVCell(s string) string {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if trimmed != "" && strings.ContainsAny(trimmed[:1], "=+-@") {
		return "'" + s
	}
	if strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") || strings.HasPrefix(s, "\n") {
		return "'" + s
	}
	return s
}
func QualityCSV(page QualityPage) (string, error) {
	if ValidateQuality(page.QualityQuery) != nil || len(page.Items) > page.Limit {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"product_id", "version", "sku", "nome", "ativo", "price_cents", "estoque_online", "pendencias"}) != nil {
		return "", ErrUnavailable
	}
	for _, p := range page.Items {
		if w.Write([]string{strconv.FormatInt(p.ID, 10), strconv.FormatInt(p.Version, 10), qualityCSVCell(p.SKU), qualityCSVCell(p.Name), strconv.FormatBool(p.Active), strconv.FormatInt(p.PriceCents, 10), strconv.FormatInt(p.Stock, 10), strings.Join(p.Issues, "|")}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
