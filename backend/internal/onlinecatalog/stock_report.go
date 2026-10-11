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

const MaxStockUnits int64 = 2147483647

type StockReportQuery struct {
	Query       string `json:"q"`
	Active      string `json:"active"`
	Issue       string `json:"issue"`
	HorizonDays int64  `json:"horizon_days"`
	Limit       int    `json:"limit"`
	Offset      int    `json:"offset"`
}
type StockReportItem struct {
	Product
	StockUnits        int64      `json:"stock_units"`
	MinimumUnits      int64      `json:"minimum_units"`
	ReorderPointUnits int64      `json:"reorder_point_units"`
	DailyDemand       string     `json:"daily_demand"`
	ForecastAt        *time.Time `json:"forecast_at"`
	CoverageDays      *string    `json:"coverage_days"`
	CoverageState     string     `json:"coverage_state"`
	BelowMinimum      bool       `json:"below_minimum"`
	LowCoverage       bool       `json:"low_coverage"`
	MissingToMinimum  int64      `json:"missing_to_minimum"`
}
type StockReportPage struct {
	StockReportQuery
	Total       int64             `json:"total"`
	Items       []StockReportItem `json:"items"`
	HasMore     bool              `json:"has_more"`
	NextOffset  *int              `json:"next_offset,omitempty"`
	GeneratedAt time.Time         `json:"generated_at"`
	SourceHash  string            `json:"source_hash"`
}
type StockReportSummary struct {
	Query            string    `json:"q"`
	Active           string    `json:"active"`
	HorizonDays      int64     `json:"horizon_days"`
	Products         int64     `json:"products"`
	ZeroStock        int64     `json:"zero_stock"`
	BelowMinimum     int64     `json:"below_minimum"`
	LowCoverage      int64     `json:"low_coverage"`
	NoPositiveDemand int64     `json:"no_positive_demand"`
	GeneratedAt      time.Time `json:"generated_at"`
}

func ValidateStockReport(q StockReportQuery) error {
	if !clean(q.Query, 120, false) || (q.Active != "all" && q.Active != "true" && q.Active != "false") || q.HorizonDays < 1 || q.HorizonDays > 365 || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	switch q.Issue {
	case "all", "zero_stock", "below_minimum", "low_coverage", "no_positive_demand":
	default:
		return ErrInput
	}
	return nil
}

// NUMERIC(12,4) is parsed as fixed-point, never through float64.
func demandUnits(raw string) (int64, error) {
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || len(parts[0]) < 1 || len(parts[0]) > 8 {
		return 0, ErrUnavailable
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) < 1 || len(fraction) > 4 {
			return 0, ErrUnavailable
		}
	}
	for _, c := range parts[0] + fraction {
		if c < '0' || c > '9' {
			return 0, ErrUnavailable
		}
	}
	fraction += strings.Repeat("0", 4-len(fraction))
	n, e := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if e != nil || n < 0 || n > MaxPrice {
		return 0, ErrUnavailable
	}
	return n, nil
}
func fixedFour(n int64) string {
	return strconv.FormatInt(n/10000, 10) + "." + strconv.FormatInt(n%10000+10000, 10)[1:]
}
func stockMetrics(v StockReportItem, horizon int64) (StockReportItem, error) {
	if v.ID < 1 || v.ID > MaxVersion || v.Version < 1 || v.Version > MaxVersion || v.PriceCents < 0 || v.PriceCents > MaxPrice || v.StockUnits < 0 || v.StockUnits > MaxStockUnits || v.MinimumUnits < 0 || v.MinimumUnits > MaxStockUnits || v.ReorderPointUnits < 0 || v.ReorderPointUnits > MaxStockUnits || horizon < 1 || horizon > 365 {
		return StockReportItem{}, ErrUnavailable
	}
	units, e := demandUnits(v.DailyDemand)
	if e != nil {
		return StockReportItem{}, e
	}
	v.DailyDemand = fixedFour(units)
	v.CoverageDays = nil
	v.LowCoverage = false
	v.CoverageState = "no_positive_demand"
	v.BelowMinimum = v.StockUnits < v.MinimumUnits
	v.MissingToMinimum = 0
	if v.BelowMinimum {
		v.MissingToMinimum = v.MinimumUnits - v.StockUnits
	}
	if units > 0 {
		// Safe bounds: int32 stock*1e8 < MaxInt64; horizon*units < MaxInt64.
		days := fixedFour(v.StockUnits * 100000000 / units) // floor to four decimals for display only.
		v.CoverageDays = &days
		v.LowCoverage = v.StockUnits*10000 < horizon*units
		v.CoverageState = "at_or_above_horizon"
		if v.LowCoverage {
			v.CoverageState = "below_horizon"
		}
	}
	return v, nil
}

const stockReportCTE = `WITH stock_report AS (
 SELECT id,nome,COALESCE(descricao,'') AS descricao,sku,preco::text AS price,ativo,catalog_version,
 estoque AS stock,estoque_minimo AS minimum,ponto_reposicao AS reorder_point,demanda_media_diaria AS demand,ultima_previsao_demanda_em AS forecast_at,
 estoque=0 AS zero_stock,estoque<estoque_minimo AS below_minimum,
 demanda_media_diaria>0 AND estoque<$4::numeric*demanda_media_diaria AS low_coverage,
 demanda_media_diaria=0 AS no_positive_demand
 FROM products WHERE tenant_id=$1 AND deleted_at IS NULL
 AND ($2::text='' OR nome ILIKE $2 ESCAPE '\' OR sku ILIKE $2 ESCAPE '\')
 AND ($3::text='all' OR ativo=($3='true'))
)`
const stockReportColumns = `id,nome,descricao,sku,price,ativo,catalog_version,stock,minimum,reorder_point,demand::text,forecast_at`
const stockReportWhere = ` FROM stock_report WHERE ($5::text='all' OR ($5='zero_stock' AND zero_stock) OR ($5='below_minimum' AND below_minimum) OR ($5='low_coverage' AND low_coverage) OR ($5='no_positive_demand' AND no_positive_demand))`

func (s Store) stockReportTx(ctx context.Context, a Actor, q StockReportQuery) (*sql.Tx, error) {
	if ValidateStockReport(q) != nil {
		return nil, ErrInput
	}
	if !a.allowed("details") {
		return nil, ErrDenied
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
func (s Store) StockReportSummary(ctx context.Context, a Actor, q StockReportQuery) (StockReportSummary, error) {
	if q.Issue != "all" {
		return StockReportSummary{}, ErrInput
	}
	tx, e := s.stockReportTx(ctx, a, q)
	if e != nil {
		return StockReportSummary{}, e
	}
	defer tx.Rollback()
	r := StockReportSummary{Query: q.Query, Active: q.Active, HorizonDays: q.HorizonDays}
	var invalid int64
	e = tx.QueryRowContext(ctx, stockReportCTE+` SELECT count(*),count(*) FILTER(WHERE zero_stock),count(*) FILTER(WHERE below_minimum),count(*) FILTER(WHERE low_coverage),count(*) FILTER(WHERE no_positive_demand),count(*) FILTER(WHERE demand<0 OR demand::text='NaN' OR stock<0 OR minimum<0 OR reorder_point<0),transaction_timestamp() FROM stock_report`, a.Tenant, qualityPattern(q.Query), q.Active, q.HorizonDays).Scan(&r.Products, &r.ZeroStock, &r.BelowMinimum, &r.LowCoverage, &r.NoPositiveDemand, &invalid, &r.GeneratedAt)
	if e != nil || invalid != 0 || r.Products < 0 || r.Products > MaxVersion || r.ZeroStock < 0 || r.ZeroStock > r.Products || r.BelowMinimum < 0 || r.BelowMinimum > r.Products || r.LowCoverage < 0 || r.LowCoverage > r.Products || r.NoPositiveDemand < 0 || r.NoPositiveDemand > r.Products || r.LowCoverage+r.NoPositiveDemand > r.Products || r.GeneratedAt.IsZero() {
		return StockReportSummary{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return StockReportSummary{}, ErrUnavailable
	}
	return r, nil
}
func stockReportMatches(v StockReportItem, issue string) bool {
	switch issue {
	case "zero_stock":
		return v.StockUnits == 0
	case "below_minimum":
		return v.BelowMinimum
	case "low_coverage":
		return v.LowCoverage
	case "no_positive_demand":
		return v.CoverageState == "no_positive_demand"
	}
	return issue == "all"
}
func (s Store) StockReport(ctx context.Context, a Actor, q StockReportQuery) (StockReportPage, error) {
	tx, e := s.stockReportTx(ctx, a, q)
	if e != nil {
		return StockReportPage{}, e
	}
	defer tx.Rollback()
	r := StockReportPage{StockReportQuery: q, Items: make([]StockReportItem, 0)}
	e = tx.QueryRowContext(ctx, stockReportCTE+` SELECT count(*),transaction_timestamp()`+stockReportWhere, a.Tenant, qualityPattern(q.Query), q.Active, q.HorizonDays, q.Issue).Scan(&r.Total, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.GeneratedAt.IsZero() {
		return StockReportPage{}, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, stockReportCTE+` SELECT `+stockReportColumns+stockReportWhere+` ORDER BY id ASC LIMIT $6 OFFSET $7`, a.Tenant, qualityPattern(q.Query), q.Active, q.HorizonDays, q.Issue, q.Limit, q.Offset)
	if e != nil {
		return StockReportPage{}, ErrUnavailable
	}
	var previous int64
	for rows.Next() {
		var v StockReportItem
		var price string
		var forecast sql.NullTime
		er := rows.Scan(&v.ID, &v.Name, &v.Description, &v.SKU, &price, &v.Active, &v.Version, &v.StockUnits, &v.MinimumUnits, &v.ReorderPointUnits, &v.DailyDemand, &forecast)
		if er != nil {
			rows.Close()
			return StockReportPage{}, ErrUnavailable
		}
		v.PriceCents, er = cents(price)
		if er != nil {
			rows.Close()
			return StockReportPage{}, ErrUnavailable
		}
		if forecast.Valid {
			stamp := forecast.Time.UTC()
			v.ForecastAt = &stamp
		}
		v, er = stockMetrics(v, q.HorizonDays)
		if er != nil || v.ID <= previous || !stockReportMatches(v, q.Issue) || (q.Active != "all" && v.Active != (q.Active == "true")) {
			rows.Close()
			return StockReportPage{}, ErrUnavailable
		}
		previous = v.ID
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
		return StockReportPage{}, ErrUnavailable
	}
	r.HasMore = int64(q.Offset)+int64(len(r.Items)) < r.Total
	if r.HasMore {
		n := q.Offset + len(r.Items)
		r.NextOffset = &n
	}
	raw, e := json.Marshal(struct {
		Query StockReportQuery
		Total int64
		Items []StockReportItem
	}{q, r.Total, r.Items})
	if e != nil {
		return StockReportPage{}, ErrUnavailable
	}
	r.SourceHash = creationHash(raw)
	if tx.Commit() != nil {
		return StockReportPage{}, ErrUnavailable
	}
	return r, nil
}
func StockReportCSV(p StockReportPage) (string, error) {
	if ValidateStockReport(p.StockReportQuery) != nil || len(p.Items) > p.Limit {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"product_id", "sku", "nome", "ativo", "stock_units", "minimum_units", "missing_to_minimum", "reorder_point_units", "daily_demand", "forecast_at", "coverage_days", "coverage_state", "below_minimum", "low_coverage", "horizon_days"}) != nil {
		return "", ErrUnavailable
	}
	for _, v := range p.Items {
		expected, e := stockMetrics(v, p.HorizonDays)
		if e != nil || !stockReportMatches(expected, p.Issue) {
			return "", ErrUnavailable
		}
		forecast, coverage := "", ""
		if v.ForecastAt != nil {
			forecast = v.ForecastAt.UTC().Format(time.RFC3339Nano)
		}
		if expected.CoverageDays != nil {
			coverage = *expected.CoverageDays
		}
		if w.Write([]string{strconv.FormatInt(v.ID, 10), qualityCSVCell(v.SKU), qualityCSVCell(v.Name), strconv.FormatBool(v.Active), strconv.FormatInt(v.StockUnits, 10), strconv.FormatInt(v.MinimumUnits, 10), strconv.FormatInt(expected.MissingToMinimum, 10), strconv.FormatInt(v.ReorderPointUnits, 10), expected.DailyDemand, forecast, coverage, expected.CoverageState, strconv.FormatBool(expected.BelowMinimum), strconv.FormatBool(expected.LowCoverage), strconv.FormatInt(p.HorizonDays, 10)}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
