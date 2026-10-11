package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"mime"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type ReplenishmentQuery struct {
	Query      string `json:"q"`
	Active     string `json:"active"`
	State      string `json:"state"`
	ReviewDays int64  `json:"review_days"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}
type ReplenishmentItem struct {
	StockPlan
	ReviewDays       int64  `json:"review_days"`
	PlanningDays     int64  `json:"planning_days"`
	DemandUnits      int64  `json:"demand_units"`
	TargetUnits      int64  `json:"target_units"`
	GapUnits         int64  `json:"gap_units"`
	SuggestedUnits   int64  `json:"suggested_units"`
	NoPositiveDemand bool   `json:"no_positive_demand"`
	State            string `json:"state"`
	PlanningOnly     bool   `json:"planning_only"`
	RequiresApproval bool   `json:"requires_approval"`
}
type ReplenishmentPage struct {
	ReplenishmentQuery
	Items       []ReplenishmentItem `json:"items"`
	Total       int64               `json:"total"`
	HasMore     bool                `json:"has_more"`
	NextOffset  *int                `json:"next_offset,omitempty"`
	GeneratedAt time.Time           `json:"generated_at"`
	SourceHash  string              `json:"source_hash"`
}
type ReplenishmentSummary struct {
	Query                string    `json:"q"`
	Active               string    `json:"active"`
	ReviewDays           int64     `json:"review_days"`
	Products             int64     `json:"products"`
	NeedsReplenishment   int64     `json:"needs_replenishment"`
	WithinTarget         int64     `json:"within_target"`
	Inactive             int64     `json:"inactive"`
	QuantityExceedsLimit int64     `json:"quantity_exceeds_limit"`
	NoPositiveDemand     int64     `json:"no_positive_demand"`
	GeneratedAt          time.Time `json:"generated_at"`
}
type ReplenishmentSimulationInput struct {
	ExpectedVersion int64 `json:"expected_version"`
	ReviewDays      int64 `json:"review_days"`
	StockUnits      int64 `json:"stock_units"`
}
type ReplenishmentSimulation struct {
	Simulated bool              `json:"simulated"`
	Current   ReplenishmentItem `json:"current"`
	Scenario  ReplenishmentItem `json:"scenario"`
}

func ValidateReplenishmentQuery(q ReplenishmentQuery) error {
	if !clean(q.Query, 120, false) || (q.Active != "true" && q.Active != "false" && q.Active != "all") || q.ReviewDays < 1 || q.ReviewDays > 365 || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	switch q.State {
	case "all", "needs_replenishment", "within_target", "inactive", "quantity_exceeds_limit":
	default:
		return ErrInput
	}
	return nil
}
func replenish(p StockPlan, review int64) (ReplenishmentItem, error) {
	if !validStockPlan(p) || review < 1 || review > 365 {
		return ReplenishmentItem{}, ErrUnavailable
	}
	scaled, e := demandUnits(p.Settings.DailyDemand)
	if e != nil {
		return ReplenishmentItem{}, e
	}
	days := p.Settings.LeadTimeDays + review
	// NUMERIC(12,4) maximum scaled*730 is below int64; round upward to whole units.
	demand := (scaled*days + 9999) / 10000
	target := p.Settings.MinimumUnits
	if p.Settings.ReorderPointUnits > target {
		target = p.Settings.ReorderPointUnits
	}
	if demand > target {
		target = demand
	}
	gap := target - p.StockUnits
	if gap < 0 {
		gap = 0
	}
	v := ReplenishmentItem{StockPlan: p, ReviewDays: review, PlanningDays: days, DemandUnits: demand, TargetUnits: target, GapUnits: gap, NoPositiveDemand: scaled == 0, PlanningOnly: true, State: "within_target"}
	switch {
	case !p.Product.Active:
		v.State = "inactive"
	case target > MaxStockUnits:
		v.State = "quantity_exceeds_limit"
	case gap > 0:
		v.State = "needs_replenishment"
		v.SuggestedUnits = gap
		v.RequiresApproval = true
	}
	return v, nil
}
func DecodeReplenishmentSimulation(content string, body []byte) (ReplenishmentSimulationInput, error) {
	var v ReplenishmentSimulationInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 4096 || !utf8.Valid(body) {
		return v, ErrInput
	}
	fields, e := batchObject(body, map[string]bool{"expected_version": true, "review_days": true, "stock_units": true})
	if e != nil || len(fields) != 3 || json.Unmarshal(body, &v) != nil || v.ExpectedVersion < 1 || v.ExpectedVersion > MaxVersion || v.ReviewDays < 1 || v.ReviewDays > 365 || v.StockUnits < 0 || v.StockUnits > MaxStockUnits {
		return v, ErrInput
	}
	return v, nil
}
func (s Store) replenishmentTx(ctx context.Context, a Actor, q ReplenishmentQuery) (*sql.Tx, error) {
	if ValidateReplenishmentQuery(q) != nil {
		return nil, ErrInput
	}
	return s.stockReportTx(ctx, a, StockReportQuery{Query: q.Query, Active: q.Active, Issue: "all", HorizonDays: q.ReviewDays, Limit: q.Limit, Offset: q.Offset})
}

const replenishCTE = `WITH raw AS (
 SELECT id,nome,COALESCE(descricao,'') AS descricao,sku,preco::text AS price,ativo,catalog_version,estoque AS stock,estoque_minimo AS minimum,ponto_reposicao AS reorder_point,tempo_reposicao_dias AS lead,demanda_media_diaria AS demand,ultima_previsao_demanda_em AS forecast,
 (estoque<0 OR estoque_minimo<0 OR ponto_reposicao<0 OR tempo_reposicao_dias<0 OR tempo_reposicao_dias>365 OR demanda_media_diaria<0 OR demanda_media_diaria::text='NaN') AS invalid
 FROM products WHERE tenant_id=$1 AND deleted_at IS NULL AND ($2::text='' OR nome ILIKE $2 ESCAPE '\' OR sku ILIKE $2 ESCAPE '\') AND ($3::text='all' OR ativo=($3='true'))
), targets AS (SELECT *,GREATEST(minimum::numeric,reorder_point::numeric,ceil(demand*(lead+$4::integer))) AS target FROM raw), recommendations AS (
 SELECT *,CASE WHEN NOT ativo THEN 'inactive' WHEN target>2147483647 THEN 'quantity_exceeds_limit' WHEN target>stock THEN 'needs_replenishment' ELSE 'within_target' END AS state FROM targets
)`
const replenishColumns = `id,nome,descricao,sku,price,ativo,catalog_version,stock,minimum,reorder_point,lead,demand::text,forecast`
const replenishFilter = `($5::text='all' OR state=$5)`

func (s Store) ReplenishmentSummary(ctx context.Context, a Actor, q ReplenishmentQuery) (ReplenishmentSummary, error) {
	if q.State != "all" {
		return ReplenishmentSummary{}, ErrInput
	}
	tx, e := s.replenishmentTx(ctx, a, q)
	if e != nil {
		return ReplenishmentSummary{}, e
	}
	defer tx.Rollback()
	r := ReplenishmentSummary{Query: q.Query, Active: q.Active, ReviewDays: q.ReviewDays}
	var invalid int64
	e = tx.QueryRowContext(ctx, replenishCTE+` SELECT count(*),count(*) FILTER(WHERE state='needs_replenishment'),count(*) FILTER(WHERE state='within_target'),count(*) FILTER(WHERE state='inactive'),count(*) FILTER(WHERE state='quantity_exceeds_limit'),count(*) FILTER(WHERE demand=0),count(*) FILTER(WHERE invalid),transaction_timestamp() FROM recommendations`, a.Tenant, qualityPattern(q.Query), q.Active, q.ReviewDays).Scan(&r.Products, &r.NeedsReplenishment, &r.WithinTarget, &r.Inactive, &r.QuantityExceedsLimit, &r.NoPositiveDemand, &invalid, &r.GeneratedAt)
	if e != nil || invalid != 0 || r.Products < 0 || r.Products > MaxVersion || r.NeedsReplenishment < 0 || r.WithinTarget < 0 || r.Inactive < 0 || r.QuantityExceedsLimit < 0 || r.NoPositiveDemand < 0 || r.NoPositiveDemand > r.Products || r.NeedsReplenishment+r.WithinTarget+r.Inactive+r.QuantityExceedsLimit != r.Products || r.GeneratedAt.IsZero() {
		return ReplenishmentSummary{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return ReplenishmentSummary{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) Replenishments(ctx context.Context, a Actor, q ReplenishmentQuery) (ReplenishmentPage, error) {
	tx, e := s.replenishmentTx(ctx, a, q)
	if e != nil {
		return ReplenishmentPage{}, e
	}
	defer tx.Rollback()
	r := ReplenishmentPage{ReplenishmentQuery: q, Items: make([]ReplenishmentItem, 0)}
	var invalid int64
	e = tx.QueryRowContext(ctx, replenishCTE+` SELECT (SELECT count(*) FROM recommendations WHERE `+replenishFilter+`),(SELECT count(*) FROM raw WHERE invalid),transaction_timestamp()`, a.Tenant, qualityPattern(q.Query), q.Active, q.ReviewDays, q.State).Scan(&r.Total, &invalid, &r.GeneratedAt)
	if e != nil || invalid != 0 || r.Total < 0 || r.Total > MaxVersion || r.GeneratedAt.IsZero() {
		return ReplenishmentPage{}, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, replenishCTE+` SELECT `+replenishColumns+` FROM recommendations WHERE `+replenishFilter+` ORDER BY id ASC LIMIT $6 OFFSET $7`, a.Tenant, qualityPattern(q.Query), q.Active, q.ReviewDays, q.State, q.Limit, q.Offset)
	if e != nil {
		return ReplenishmentPage{}, ErrUnavailable
	}
	var previous int64
	for rows.Next() {
		p, er := scanStockPlan(rows)
		if er != nil {
			rows.Close()
			return ReplenishmentPage{}, ErrUnavailable
		}
		v, er := replenish(p, q.ReviewDays)
		if er != nil || v.Product.ID <= previous || (q.State != "all" && v.State != q.State) || (q.Active != "all" && v.Product.Active != (q.Active == "true")) {
			rows.Close()
			return ReplenishmentPage{}, ErrUnavailable
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
		return ReplenishmentPage{}, ErrUnavailable
	}
	r.HasMore = int64(q.Offset+len(r.Items)) < r.Total
	if r.HasMore {
		next := q.Offset + len(r.Items)
		r.NextOffset = &next
	}
	b, e := json.Marshal(struct {
		Query ReplenishmentQuery
		Total int64
		Items []ReplenishmentItem
	}{q, r.Total, r.Items})
	if e != nil {
		return ReplenishmentPage{}, ErrUnavailable
	}
	r.SourceHash = creationHash(b)
	if tx.Commit() != nil {
		return ReplenishmentPage{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) ProductReplenishment(ctx context.Context, a Actor, id, review int64) (ReplenishmentItem, error) {
	if id < 1 || id > MaxVersion || review < 1 || review > 365 {
		return ReplenishmentItem{}, ErrInput
	}
	tx, e := s.replenishmentTx(ctx, a, ReplenishmentQuery{Active: "all", State: "all", ReviewDays: review, Limit: 50})
	if e != nil {
		return ReplenishmentItem{}, e
	}
	defer tx.Rollback()
	p, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL, a.Tenant, id))
	if e != nil {
		return ReplenishmentItem{}, e
	}
	v, e := replenish(p, review)
	if e != nil {
		return ReplenishmentItem{}, e
	}
	if tx.Commit() != nil {
		return ReplenishmentItem{}, ErrUnavailable
	}
	return v, nil
}
func (s Store) SimulateReplenishment(ctx context.Context, a Actor, id int64, v ReplenishmentSimulationInput) (ReplenishmentSimulation, error) {
	if v.ExpectedVersion < 1 || v.ExpectedVersion > MaxVersion || v.StockUnits < 0 || v.StockUnits > MaxStockUnits {
		return ReplenishmentSimulation{}, ErrInput
	}
	current, e := s.ProductReplenishment(ctx, a, id, v.ReviewDays)
	if e != nil {
		return ReplenishmentSimulation{}, e
	}
	if current.Product.Version != v.ExpectedVersion {
		return ReplenishmentSimulation{}, ErrConflict
	}
	plan := current.StockPlan
	plan.StockUnits = v.StockUnits
	scenario, e := replenish(plan, v.ReviewDays)
	if e != nil {
		return ReplenishmentSimulation{}, e
	}
	return ReplenishmentSimulation{true, current, scenario}, nil
}
func ReplenishmentCSV(p ReplenishmentPage) (string, error) {
	if ValidateReplenishmentQuery(p.ReplenishmentQuery) != nil || len(p.Items) > p.Limit {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"product_id", "sku", "nome", "version", "ativo", "stock_units", "minimum_units", "reorder_point_units", "lead_time_days", "review_days", "daily_demand", "planning_days", "demand_units", "target_units", "gap_units", "suggested_units", "state", "no_positive_demand", "requires_approval", "planning_only"}) != nil {
		return "", ErrUnavailable
	}
	for _, v := range p.Items {
		expected, e := replenish(v.StockPlan, p.ReviewDays)
		if e != nil || !reflect.DeepEqual(expected, v) || (p.State != "all" && v.State != p.State) || (p.Active != "all" && v.Product.Active != (p.Active == "true")) {
			return "", ErrUnavailable
		}
		if w.Write([]string{strconv.FormatInt(v.Product.ID, 10), qualityCSVCell(v.Product.SKU), qualityCSVCell(v.Product.Name), strconv.FormatInt(v.Product.Version, 10), strconv.FormatBool(v.Product.Active), strconv.FormatInt(v.StockUnits, 10), strconv.FormatInt(v.Settings.MinimumUnits, 10), strconv.FormatInt(v.Settings.ReorderPointUnits, 10), strconv.FormatInt(v.Settings.LeadTimeDays, 10), strconv.FormatInt(v.ReviewDays, 10), v.Settings.DailyDemand, strconv.FormatInt(v.PlanningDays, 10), strconv.FormatInt(v.DemandUnits, 10), strconv.FormatInt(v.TargetUnits, 10), strconv.FormatInt(v.GapUnits, 10), strconv.FormatInt(v.SuggestedUnits, 10), v.State, strconv.FormatBool(v.NoPositiveDemand), strconv.FormatBool(v.RequiresApproval), "true"}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
