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

type MarginQuery struct {
	Query     string `json:"q"`
	Active    string `json:"active"`
	Issue     string `json:"issue"`
	TargetBPS int64  `json:"target_bps"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}
type MarginPage struct {
	MarginQuery
	Total       int64        `json:"total"`
	Items       []CostMargin `json:"items"`
	HasMore     bool         `json:"has_more"`
	NextOffset  *int         `json:"next_offset,omitempty"`
	GeneratedAt time.Time    `json:"generated_at"`
	SourceHash  string       `json:"source_hash"`
}
type MarginSummary struct {
	Query       string    `json:"q"`
	Active      string    `json:"active"`
	TargetBPS   int64     `json:"target_bps"`
	Total       int64     `json:"total"`
	KnownCost   int64     `json:"known_cost"`
	MissingCost int64     `json:"missing_cost"`
	Negative    int64     `json:"negative_margin"`
	BelowTarget int64     `json:"below_target"`
	ZeroPrice   int64     `json:"zero_price"`
	GeneratedAt time.Time `json:"generated_at"`
}

func ValidateMarginQuery(q MarginQuery) error {
	if !clean(q.Query, 120, false) || (q.Active != "all" && q.Active != "true" && q.Active != "false") || (q.Issue != "all" && q.Issue != "missing_cost" && q.Issue != "negative" && q.Issue != "below_target") || q.TargetBPS < 0 || q.TargetBPS > 10000 || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	return nil
}

// NUMERIC arithmetic compares the unrounded margin. Unknown cost is excluded
// from negative/below-target classifications; zero price has no percentage.
const marginReportCTE = `WITH margins AS (
 SELECT p.id,p.nome,COALESCE(p.descricao,'') AS descricao,p.sku,p.preco::text AS price,p.ativo,p.catalog_version,
 c.cost_cents,c.revision,c.actor_id::text,c.updated_at,
 c.product_id IS NULL AS missing_cost,
 c.product_id IS NOT NULL AND p.preco*100<c.cost_cents AS negative,
 c.product_id IS NOT NULL AND p.preco>0 AND (p.preco*100-c.cost_cents)*10000<$4::numeric*p.preco*100 AS below_target
 FROM products p LEFT JOIN online_product_costs c ON c.tenant_id=p.tenant_id AND c.product_id=p.id
 WHERE p.tenant_id=$1 AND p.deleted_at IS NULL
 AND ($2::text='' OR p.nome ILIKE $2 ESCAPE '\' OR p.sku ILIKE $2 ESCAPE '\')
 AND ($3::text='all' OR ($3='true' AND p.ativo) OR ($3='false' AND NOT p.ativo))
)`
const marginReportColumns = `id,nome,descricao,sku,price,ativo,catalog_version,cost_cents,revision,actor_id,updated_at`
const marginReportWhere = ` FROM margins WHERE ($5::text='all' OR ($5='missing_cost' AND missing_cost) OR ($5='negative' AND negative) OR ($5='below_target' AND below_target))`

func scanMarginItem(row scanner) (CostMargin, error) {
	var v ProductCost
	var price string
	var cost, revision sql.NullInt64
	var actor sql.NullString
	var updated sql.NullTime
	e := row.Scan(&v.Product.ID, &v.Product.Name, &v.Product.Description, &v.Product.SKU, &price, &v.Product.Active, &v.Product.Version, &cost, &revision, &actor, &updated)
	if e != nil {
		return CostMargin{}, ErrUnavailable
	}
	v.Product.PriceCents, e = cents(price)
	if e != nil || v.Product.ID < 1 || v.Product.ID > MaxVersion || v.Product.Version < 1 || v.Product.Version > MaxVersion {
		return CostMargin{}, ErrUnavailable
	}
	if cost.Valid {
		if !revision.Valid || !actor.Valid || !updated.Valid {
			return CostMargin{}, ErrUnavailable
		}
		stamp := updated.Time.UTC()
		v.Cost = CostState{true, revision.Int64, &cost.Int64, &actor.String, &stamp}
	} else if revision.Valid || actor.Valid || updated.Valid {
		return CostMargin{}, ErrUnavailable
	}
	return calculateMargin(v)
}
func marginMatches(v CostMargin, q MarginQuery) bool {
	switch q.Issue {
	case "missing_cost":
		return !v.Cost.Known
	case "negative":
		return v.MarginCents != nil && *v.MarginCents < 0
	case "below_target":
		return v.Cost.Known && v.Product.PriceCents > 0 && *v.MarginCents*10000 < q.TargetBPS*v.Product.PriceCents
	}
	return true
}
func (s Store) MarginSummary(ctx context.Context, a Actor, q MarginQuery) (MarginSummary, error) {
	if ValidateMarginQuery(q) != nil || q.Issue != "all" {
		return MarginSummary{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return MarginSummary{}, e
	}
	defer tx.Rollback()
	r := MarginSummary{Query: q.Query, Active: q.Active, TargetBPS: q.TargetBPS}
	e = tx.QueryRowContext(ctx, marginReportCTE+` SELECT count(*),count(*) FILTER(WHERE NOT missing_cost),count(*) FILTER(WHERE missing_cost),count(*) FILTER(WHERE negative),count(*) FILTER(WHERE below_target),count(*) FILTER(WHERE price::numeric=0),transaction_timestamp() FROM margins`, a.Tenant, qualityPattern(q.Query), q.Active, q.TargetBPS).Scan(&r.Total, &r.KnownCost, &r.MissingCost, &r.Negative, &r.BelowTarget, &r.ZeroPrice, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.KnownCost < 0 || r.MissingCost < 0 || r.KnownCost+r.MissingCost != r.Total || r.Negative < 0 || r.Negative > r.KnownCost || r.BelowTarget < 0 || r.BelowTarget > r.KnownCost || r.ZeroPrice < 0 || r.ZeroPrice > r.Total || r.GeneratedAt.IsZero() {
		return MarginSummary{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return MarginSummary{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) Margins(ctx context.Context, a Actor, q MarginQuery) (MarginPage, error) {
	if ValidateMarginQuery(q) != nil {
		return MarginPage{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return MarginPage{}, e
	}
	defer tx.Rollback()
	r := MarginPage{MarginQuery: q, Items: make([]CostMargin, 0)}
	e = tx.QueryRowContext(ctx, marginReportCTE+` SELECT count(*),transaction_timestamp()`+marginReportWhere, a.Tenant, qualityPattern(q.Query), q.Active, q.TargetBPS, q.Issue).Scan(&r.Total, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.GeneratedAt.IsZero() {
		return MarginPage{}, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, marginReportCTE+` SELECT `+marginReportColumns+marginReportWhere+` ORDER BY id ASC LIMIT $6 OFFSET $7`, a.Tenant, qualityPattern(q.Query), q.Active, q.TargetBPS, q.Issue, q.Limit, q.Offset)
	if e != nil {
		return MarginPage{}, ErrUnavailable
	}
	var previous int64
	for rows.Next() {
		v, e := scanMarginItem(rows)
		if e != nil || v.Product.ID <= previous || !marginMatches(v, q) || (q.Active != "all" && v.Product.Active != (q.Active == "true")) {
			rows.Close()
			return MarginPage{}, ErrUnavailable
		}
		previous = v.Product.ID
		r.Items = append(r.Items, v)
	}
	e = rows.Err()
	ce := rows.Close()
	want := r.Total - int64(q.Offset)
	if want < 0 {
		want = 0
	}
	if want > int64(q.Limit) {
		want = int64(q.Limit)
	}
	if e != nil || ce != nil || int64(len(r.Items)) != want {
		return MarginPage{}, ErrUnavailable
	}
	r.HasMore = int64(q.Offset)+int64(len(r.Items)) < r.Total
	if r.HasMore {
		n := q.Offset + len(r.Items)
		r.NextOffset = &n
	}
	raw, e := json.Marshal(struct {
		Query MarginQuery
		Total int64
		Items []CostMargin
	}{q, r.Total, r.Items})
	if e != nil {
		return MarginPage{}, ErrUnavailable
	}
	r.SourceHash = creationHash(raw)
	if tx.Commit() != nil {
		return MarginPage{}, ErrUnavailable
	}
	return r, nil
}
func MarginsCSV(p MarginPage) (string, error) {
	if ValidateMarginQuery(p.MarginQuery) != nil || len(p.Items) > p.Limit {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"product_id", "sku", "nome", "ativo", "product_version", "price_cents", "cost_known", "cost_revision", "cost_cents", "margin_cents", "margin_bps", "state"}) != nil {
		return "", ErrUnavailable
	}
	for _, v := range p.Items {
		expected, e := calculateMargin(v.ProductCost)
		if e != nil || v.State != expected.State || !marginMatches(expected, p.MarginQuery) {
			return "", ErrUnavailable
		}
		cost, margin, bps := "", "", ""
		if v.Cost.Known {
			cost = strconv.FormatInt(*v.Cost.CostCents, 10)
		}
		if expected.MarginCents != nil {
			margin = strconv.FormatInt(*expected.MarginCents, 10)
		}
		if expected.MarginBPS != nil {
			bps = *expected.MarginBPS
		}
		if w.Write([]string{strconv.FormatInt(v.Product.ID, 10), qualityCSVCell(v.Product.SKU), qualityCSVCell(v.Product.Name), strconv.FormatBool(v.Product.Active), strconv.FormatInt(v.Product.Version, 10), strconv.FormatInt(v.Product.PriceCents, 10), strconv.FormatBool(v.Cost.Known), strconv.FormatInt(v.Cost.Revision, 10), cost, margin, bps, v.State}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
