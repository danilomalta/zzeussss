package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

// Planning declarations use integer units and exact NUMERIC(12,4) text.
// They do not constitute reservations, stock movements or demand forecasts.
type StockSettings struct {
	MinimumUnits      int64      `json:"minimum_units"`
	ReorderPointUnits int64      `json:"reorder_point_units"`
	LeadTimeDays      int64      `json:"lead_time_days"`
	DailyDemand       string     `json:"daily_demand"`
	ForecastAt        *time.Time `json:"forecast_at"`
}
type StockSettingsInput struct {
	OperationID       string `json:"operation_id"`
	ExpectedVersion   int64  `json:"expected_version"`
	MinimumUnits      int64  `json:"minimum_units"`
	ReorderPointUnits int64  `json:"reorder_point_units"`
	LeadTimeDays      int64  `json:"lead_time_days"`
	DailyDemand       string `json:"daily_demand"`
	Reason            string `json:"reason"`
}
type StockPlan struct {
	Product    Product       `json:"product"`
	StockUnits int64         `json:"stock_units"`
	Settings   StockSettings `json:"settings"`
}
type StockSettingsPreview struct {
	OperationID string          `json:"operation_id"`
	Before      StockPlan       `json:"before"`
	After       StockPlan       `json:"after"`
	HorizonDays int64           `json:"horizon_days"`
	Metrics     StockReportItem `json:"metrics"`
}
type StockSettingsReceipt struct {
	OperationID string    `json:"operation_id"`
	ProductID   int64     `json:"product_id"`
	ActorID     string    `json:"actor_id"`
	Reason      string    `json:"reason"`
	Before      StockPlan `json:"before"`
	After       StockPlan `json:"after"`
	CreatedAt   time.Time `json:"created_at"`
}
type StockSettingsHistory struct {
	Items      []StockSettingsReceipt `json:"items"`
	Total      int64                  `json:"total"`
	Limit      int                    `json:"limit"`
	Offset     int                    `json:"offset"`
	HasMore    bool                   `json:"has_more"`
	NextOffset *int                   `json:"next_offset,omitempty"`
}

func normalizeStockSettings(v StockSettingsInput) (StockSettingsInput, error) {
	n, e := demandUnits(v.DailyDemand)
	v.Reason = strings.TrimSpace(v.Reason)
	if e != nil || !ValidUUID(v.OperationID) || v.ExpectedVersion < 1 || v.ExpectedVersion >= MaxVersion || v.MinimumUnits < 0 || v.MinimumUnits > MaxStockUnits || v.ReorderPointUnits < 0 || v.ReorderPointUnits > MaxStockUnits || v.LeadTimeDays < 0 || v.LeadTimeDays > 365 || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	v.DailyDemand = fixedFour(n)
	return v, nil
}
func DecodeStockSettings(content string, body []byte) (StockSettingsInput, error) {
	var v StockSettingsInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return v, ErrInput
	}
	fields, e := batchObject(body, map[string]bool{"operation_id": true, "expected_version": true, "minimum_units": true, "reorder_point_units": true, "lead_time_days": true, "daily_demand": true, "reason": true})
	if e != nil || len(fields) != 7 || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeStockSettings(v)
}

const stockPlanSQL = `SELECT id,nome,COALESCE(descricao,''),sku,preco::text,ativo,catalog_version,estoque,estoque_minimo,ponto_reposicao,tempo_reposicao_dias,demanda_media_diaria::text,ultima_previsao_demanda_em FROM products WHERE tenant_id=$1 AND id=$2 AND deleted_at IS NULL`

func validStockPlan(p StockPlan) bool {
	s := p.Settings
	n, e := demandUnits(s.DailyDemand)
	return e == nil && fixedFour(n) == s.DailyDemand && s.MinimumUnits >= 0 && s.MinimumUnits <= MaxStockUnits && s.ReorderPointUnits >= 0 && s.ReorderPointUnits <= MaxStockUnits && s.LeadTimeDays >= 0 && s.LeadTimeDays <= 365 && (s.ForecastAt == nil || !s.ForecastAt.IsZero()) && p.StockUnits >= 0 && p.StockUnits <= MaxStockUnits && p.Product.ID >= 1 && p.Product.ID <= MaxVersion && p.Product.Version >= 1 && p.Product.Version <= MaxVersion && p.Product.PriceCents >= 0 && p.Product.PriceCents <= MaxPrice
}
func scanStockPlan(row scanner) (StockPlan, error) {
	var p StockPlan
	var price string
	var forecast sql.NullTime
	e := row.Scan(&p.Product.ID, &p.Product.Name, &p.Product.Description, &p.Product.SKU, &price, &p.Product.Active, &p.Product.Version, &p.StockUnits, &p.Settings.MinimumUnits, &p.Settings.ReorderPointUnits, &p.Settings.LeadTimeDays, &p.Settings.DailyDemand, &forecast)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrMissing
	}
	if e != nil {
		return p, ErrUnavailable
	}
	p.Product.PriceCents, e = cents(price)
	if e != nil {
		return p, ErrUnavailable
	}
	n, e := demandUnits(p.Settings.DailyDemand)
	if e != nil {
		return p, ErrUnavailable
	}
	p.Settings.DailyDemand = fixedFour(n)
	if forecast.Valid {
		stamp := forecast.Time.UTC()
		p.Settings.ForecastAt = &stamp
	}
	if !validStockPlan(p) {
		return StockPlan{}, ErrUnavailable
	}
	return p, nil
}
func proposedStockPlan(before StockPlan, v StockSettingsInput) StockPlan {
	after := before
	after.Product.Version++
	after.Settings = StockSettings{MinimumUnits: v.MinimumUnits, ReorderPointUnits: v.ReorderPointUnits, LeadTimeDays: v.LeadTimeDays, DailyDemand: v.DailyDemand, ForecastAt: before.Settings.ForecastAt}
	if before.Settings.DailyDemand != v.DailyDemand {
		after.Settings.ForecastAt = nil
	}
	return after
}
func stockSettingsHash(id int64, v StockSettingsInput) string {
	b, _ := json.Marshal(struct {
		ID    int64
		Input StockSettingsInput
	}{id, v})
	return creationHash(b)
}
func (s Store) ProductStockSettings(ctx context.Context, a Actor, id int64) (StockPlan, error) {
	if id < 1 || id > MaxVersion {
		return StockPlan{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return StockPlan{}, e
	}
	defer tx.Rollback()
	p, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL, a.Tenant, id))
	if e != nil {
		return p, e
	}
	if tx.Commit() != nil {
		return StockPlan{}, ErrUnavailable
	}
	return p, nil
}
func (s Store) PreviewStockSettings(ctx context.Context, a Actor, id int64, v StockSettingsInput) (StockSettingsPreview, error) {
	v, e := normalizeStockSettings(v)
	if e != nil {
		return StockSettingsPreview{}, e
	}
	before, e := s.ProductStockSettings(ctx, a, id)
	if e != nil {
		return StockSettingsPreview{}, e
	}
	if before.Product.Version != v.ExpectedVersion {
		return StockSettingsPreview{}, ErrConflict
	}
	after := proposedStockPlan(before, v)
	m, e := stockMetrics(StockReportItem{Product: after.Product, StockUnits: after.StockUnits, MinimumUnits: after.Settings.MinimumUnits, ReorderPointUnits: after.Settings.ReorderPointUnits, DailyDemand: after.Settings.DailyDemand, ForecastAt: after.Settings.ForecastAt}, 7)
	if e != nil {
		return StockSettingsPreview{}, e
	}
	return StockSettingsPreview{v.OperationID, before, after, 7, m}, nil
}

const stockSettingsReceiptColumns = `operation_id::text,product_id,actor_id::text,reason,before_state,after_state,created_at,payload_hash`
const stockSettingsReceiptSQL = `SELECT ` + stockSettingsReceiptColumns + ` FROM online_stock_setting_operations WHERE tenant_id=$1 AND operation_id=$2`

func scanStockSettingsReceipt(row scanner) (StockSettingsReceipt, string, error) {
	var r StockSettingsReceipt
	var b, af []byte
	var h string
	e := row.Scan(&r.OperationID, &r.ProductID, &r.ActorID, &r.Reason, &b, &af, &r.CreatedAt, &h)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil {
		return r, h, ErrUnavailable
	}
	if json.Unmarshal(b, &r.Before) != nil || json.Unmarshal(af, &r.After) != nil || !validStockPlan(r.Before) || !validStockPlan(r.After) || !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || !clean(r.Reason, 500, true) || len(h) != 64 || r.CreatedAt.IsZero() || r.ProductID != r.Before.Product.ID || r.ProductID != r.After.Product.ID || r.Before.Product.Version >= MaxVersion {
		return StockSettingsReceipt{}, "", ErrUnavailable
	}
	// Audit must describe exactly a planning-only change, including forecast invalidation.
	expected := proposedStockPlan(r.Before, StockSettingsInput{MinimumUnits: r.After.Settings.MinimumUnits, ReorderPointUnits: r.After.Settings.ReorderPointUnits, LeadTimeDays: r.After.Settings.LeadTimeDays, DailyDemand: r.After.Settings.DailyDemand})
	if !reflect.DeepEqual(expected, r.After) {
		return StockSettingsReceipt{}, "", ErrUnavailable
	}
	r.CreatedAt = r.CreatedAt.UTC()
	return r, h, nil
}
func (s Store) SetStockSettings(ctx context.Context, a Actor, id int64, v StockSettingsInput) (StockSettingsReceipt, error) {
	if !a.allowed("price") {
		return StockSettingsReceipt{}, ErrDenied
	}
	v, e := normalizeStockSettings(v)
	if e != nil || id < 1 || id > MaxVersion {
		return StockSettingsReceipt{}, ErrInput
	}
	if s.DB == nil {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return StockSettingsReceipt{}, e
	}
	h := stockSettingsHash(id, v)
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":stock-settings:"+v.OperationID); e != nil {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	prior, ph, e := scanStockSettingsReceipt(tx.QueryRowContext(ctx, stockSettingsReceiptSQL, a.Tenant, v.OperationID))
	if e == nil {
		if prior.ActorID != a.User || prior.ProductID != id || ph != h {
			return StockSettingsReceipt{}, ErrConflict
		}
		if tx.Commit() != nil {
			return StockSettingsReceipt{}, ErrUnavailable
		}
		return prior, nil
	}
	if e != ErrMissing {
		return StockSettingsReceipt{}, e
	}
	before, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL+" FOR UPDATE", a.Tenant, id))
	if e != nil {
		return StockSettingsReceipt{}, e
	}
	if before.Product.Version != v.ExpectedVersion {
		return StockSettingsReceipt{}, ErrConflict
	}
	expected := proposedStockPlan(before, v)
	result, e := tx.ExecContext(ctx, `UPDATE products SET estoque_minimo=$3,ponto_reposicao=$4,tempo_reposicao_dias=$5,ultima_previsao_demanda_em=CASE WHEN demanda_media_diaria<>$6::numeric THEN NULL ELSE ultima_previsao_demanda_em END,demanda_media_diaria=$6::numeric,catalog_version=catalog_version+1,updated_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND catalog_version=$7 AND deleted_at IS NULL`, a.Tenant, id, v.MinimumUnits, v.ReorderPointUnits, v.LeadTimeDays, v.DailyDemand, v.ExpectedVersion)
	if e != nil {
		return StockSettingsReceipt{}, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	after, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL, a.Tenant, id))
	if e != nil || !reflect.DeepEqual(after, expected) {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	b, _ := json.Marshal(before)
	af, _ := json.Marshal(after)
	if _, e = tx.ExecContext(ctx, `INSERT INTO online_stock_setting_operations(tenant_id,operation_id,product_id,actor_id,reason,payload_hash,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, a.Tenant, v.OperationID, id, a.User, v.Reason, h, b, af); e != nil {
		return StockSettingsReceipt{}, conflict(e)
	}
	r, ph, e := scanStockSettingsReceipt(tx.QueryRowContext(ctx, stockSettingsReceiptSQL, a.Tenant, v.OperationID))
	if e != nil || ph != h || r.OperationID != v.OperationID || r.ActorID != a.User || r.Reason != v.Reason || !reflect.DeepEqual(r.Before, before) || !reflect.DeepEqual(r.After, after) {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	payload, _ := json.Marshal(r)
	result, e = tx.ExecContext(ctx, `INSERT INTO online_stock_setting_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,'product.stock-settings.changed',$3)`, a.Tenant, v.OperationID, payload)
	if e != nil {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) StockSettingsOperation(ctx context.Context, a Actor, op string) (StockSettingsReceipt, error) {
	if !ValidUUID(op) {
		return StockSettingsReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return StockSettingsReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanStockSettingsReceipt(tx.QueryRowContext(ctx, stockSettingsReceiptSQL+" AND actor_id=$3", a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return StockSettingsReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) StockSettingsHistory(ctx context.Context, a Actor, id int64, limit, offset int) (StockSettingsHistory, error) {
	r := StockSettingsHistory{Items: make([]StockSettingsReceipt, 0), Limit: limit, Offset: offset}
	if id < 1 || id > MaxVersion || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	// Require current tenant ownership; soft-deleted rows still expose their audit history.
	var exists bool
	if tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE tenant_id=$1 AND id=$2)`, a.Tenant, id).Scan(&exists) != nil {
		return r, ErrUnavailable
	}
	if !exists {
		return r, ErrMissing
	}
	if tx.QueryRowContext(ctx, `SELECT count(*) FROM online_stock_setting_operations WHERE tenant_id=$1 AND product_id=$2`, a.Tenant, id).Scan(&r.Total) != nil || r.Total < 0 || r.Total > MaxVersion {
		return r, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+stockSettingsReceiptColumns+` FROM online_stock_setting_operations WHERE tenant_id=$1 AND product_id=$2 ORDER BY created_at DESC,operation_id DESC LIMIT $3 OFFSET $4`, a.Tenant, id, limit, offset)
	if e != nil {
		return r, ErrUnavailable
	}
	for rows.Next() {
		v, _, er := scanStockSettingsReceipt(rows)
		if er != nil || v.ProductID != id {
			rows.Close()
			return r, ErrUnavailable
		}
		r.Items = append(r.Items, v)
	}
	e = rows.Err()
	ce := rows.Close()
	want := r.Total - int64(offset)
	if want < 0 {
		want = 0
	}
	if want > int64(limit) {
		want = int64(limit)
	}
	if e != nil || ce != nil || int64(len(r.Items)) != want {
		return r, ErrUnavailable
	}
	r.HasMore = int64(offset+len(r.Items)) < r.Total
	if r.HasMore {
		next := offset + len(r.Items)
		r.NextOffset = &next
	}
	if tx.Commit() != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
