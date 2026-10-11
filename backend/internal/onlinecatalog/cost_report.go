package onlinecatalog

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Historical filters use the saved snapshot, including inactive/deleted products.
type CostReportQuery struct {
	Query     string `json:"q"`
	ActorID   string `json:"actor_id"`
	ProductID int64  `json:"product_id"`
	Change    string `json:"change"`
	From      string `json:"from"`
	Until     string `json:"until"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}
type CostReportEntry struct {
	CostReceipt
	Change     string `json:"change"`
	DeltaCents *int64 `json:"delta_cents"`
}
type CostReportPage struct {
	CostReportQuery
	Total       int64             `json:"total"`
	Items       []CostReportEntry `json:"items"`
	HasMore     bool              `json:"has_more"`
	NextOffset  *int              `json:"next_offset,omitempty"`
	GeneratedAt time.Time         `json:"generated_at"`
	SourceHash  string            `json:"source_hash"`
}
type CostReportSummary struct {
	CostReportQuery
	Total       int64     `json:"total"`
	Initial     int64     `json:"initial"`
	Increased   int64     `json:"increased"`
	Decreased   int64     `json:"decreased"`
	Unchanged   int64     `json:"unchanged"`
	Products    int64     `json:"products"`
	Actors      int64     `json:"actors"`
	GeneratedAt time.Time `json:"generated_at"`
}

func ValidateCostReport(q CostReportQuery) error {
	if !clean(q.Query, 120, false) || (q.ActorID != "" && !ValidUUID(q.ActorID)) || q.ProductID < 0 || q.ProductID > MaxVersion || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	switch q.Change {
	case "all", "initial", "increased", "decreased", "unchanged":
	default:
		return ErrInput
	}
	var from, until time.Time
	var e error
	if q.From != "" {
		from, e = time.Parse(time.RFC3339Nano, q.From)
		if e != nil || from.Year() < 2000 || from.Year() > 9999 {
			return ErrInput
		}
	}
	if q.Until != "" {
		until, e = time.Parse(time.RFC3339Nano, q.Until)
		if e != nil || until.Year() < 2000 || until.Year() > 9999 {
			return ErrInput
		}
	}
	if !from.IsZero() && !until.IsZero() && !from.Before(until) {
		return ErrInput
	}
	return nil
}
func costReportEntry(r CostReceipt) (CostReportEntry, error) {
	if !validCostState(r.Before) || !validCostState(r.After) || !r.After.Known || r.After.Revision != r.Before.Revision+1 {
		return CostReportEntry{}, ErrUnavailable
	}
	v := CostReportEntry{CostReceipt: r, Change: "initial"}
	if r.Before.Known {
		delta := *r.After.CostCents - *r.Before.CostCents
		v.DeltaCents = &delta
		v.Change = "unchanged"
		if delta > 0 {
			v.Change = "increased"
		} else if delta < 0 {
			v.Change = "decreased"
		}
	}
	return v, nil
}

const costReportCTE = `WITH history AS (
 SELECT operation_id,product_id,actor_id,reason,before_state,after_state,product_snapshot,created_at,payload_hash,
 CASE WHEN NOT (before_state->>'known')::boolean THEN 'initial'
 WHEN (after_state->>'cost_cents')::bigint>(before_state->>'cost_cents')::bigint THEN 'increased'
 WHEN (after_state->>'cost_cents')::bigint<(before_state->>'cost_cents')::bigint THEN 'decreased' ELSE 'unchanged' END AS change_kind
 FROM online_cost_operations WHERE tenant_id=$1
 AND ($2::text='' OR product_snapshot->>'nome' ILIKE $2 ESCAPE '\' OR product_snapshot->>'sku' ILIKE $2 ESCAPE '\')
 AND ($3::text='' OR actor_id::text=$3)
 AND ($4::bigint=0 OR product_id=$4)
 AND (NULLIF($5::text,'') IS NULL OR created_at>=NULLIF($5::text,'')::timestamptz)
 AND (NULLIF($6::text,'') IS NULL OR created_at<NULLIF($6::text,'')::timestamptz)
)`
const costReportWhere = ` FROM history WHERE ($7::text='all' OR change_kind=$7)`
const costReportColumns = `operation_id::text,product_id,actor_id::text,reason,before_state,after_state,product_snapshot,created_at,payload_hash`

func costReportArgs(a Actor, q CostReportQuery) []interface{} {
	return []interface{}{a.Tenant, qualityPattern(q.Query), q.ActorID, q.ProductID, q.From, q.Until, q.Change}
}
func (s Store) CostReportSummary(ctx context.Context, a Actor, q CostReportQuery) (CostReportSummary, error) {
	if ValidateCostReport(q) != nil {
		return CostReportSummary{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return CostReportSummary{}, e
	}
	defer tx.Rollback()
	r := CostReportSummary{CostReportQuery: q}
	e = tx.QueryRowContext(ctx, costReportCTE+` SELECT count(*),count(*) FILTER(WHERE change_kind='initial'),count(*) FILTER(WHERE change_kind='increased'),count(*) FILTER(WHERE change_kind='decreased'),count(*) FILTER(WHERE change_kind='unchanged'),count(DISTINCT product_id),count(DISTINCT actor_id),transaction_timestamp()`+costReportWhere, costReportArgs(a, q)...).Scan(&r.Total, &r.Initial, &r.Increased, &r.Decreased, &r.Unchanged, &r.Products, &r.Actors, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.Initial < 0 || r.Increased < 0 || r.Decreased < 0 || r.Unchanged < 0 || r.Initial+r.Increased+r.Decreased+r.Unchanged != r.Total || r.Products < 0 || r.Products > r.Total || r.Actors < 0 || r.Actors > r.Total || r.GeneratedAt.IsZero() {
		return CostReportSummary{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return CostReportSummary{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) CostReport(ctx context.Context, a Actor, q CostReportQuery) (CostReportPage, error) {
	if ValidateCostReport(q) != nil {
		return CostReportPage{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return CostReportPage{}, e
	}
	defer tx.Rollback()
	r := CostReportPage{CostReportQuery: q, Items: make([]CostReportEntry, 0)}
	args := costReportArgs(a, q)
	e = tx.QueryRowContext(ctx, costReportCTE+` SELECT count(*),transaction_timestamp()`+costReportWhere, args...).Scan(&r.Total, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.GeneratedAt.IsZero() {
		return CostReportPage{}, ErrUnavailable
	}
	args = append(args, q.Limit, q.Offset)
	rows, e := tx.QueryContext(ctx, costReportCTE+` SELECT `+costReportColumns+costReportWhere+` ORDER BY created_at DESC,operation_id DESC LIMIT $8 OFFSET $9`, args...)
	if e != nil {
		return CostReportPage{}, ErrUnavailable
	}
	var previous CostReportEntry
	for rows.Next() {
		receipt, _, er := scanCostReceipt(rows)
		v, ve := costReportEntry(receipt)
		if er != nil || ve != nil || (q.Change != "all" && v.Change != q.Change) || (q.ProductID != 0 && v.ProductID != q.ProductID) || (q.ActorID != "" && v.ActorID != q.ActorID) || (len(r.Items) > 0 && (v.CreatedAt.After(previous.CreatedAt) || (v.CreatedAt.Equal(previous.CreatedAt) && v.OperationID >= previous.OperationID))) {
			rows.Close()
			return CostReportPage{}, ErrUnavailable
		}
		previous = v
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
		return CostReportPage{}, ErrUnavailable
	}
	r.HasMore = int64(q.Offset)+int64(len(r.Items)) < r.Total
	if r.HasMore {
		n := q.Offset + len(r.Items)
		r.NextOffset = &n
	}
	raw, e := json.Marshal(struct {
		Query CostReportQuery
		Total int64
		Items []CostReportEntry
	}{q, r.Total, r.Items})
	if e != nil {
		return CostReportPage{}, ErrUnavailable
	}
	r.SourceHash = creationHash(raw)
	if tx.Commit() != nil {
		return CostReportPage{}, ErrUnavailable
	}
	return r, nil
}

// Managers can inspect all cost receipts in their company; own replay stays actor-scoped.
func (s Store) CostReportDetail(ctx context.Context, a Actor, op string) (CostReportEntry, error) {
	if !ValidUUID(op) {
		return CostReportEntry{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return CostReportEntry{}, e
	}
	defer tx.Rollback()
	r, _, e := scanCostReceipt(tx.QueryRowContext(ctx, costReceiptSQL, a.Tenant, op))
	if e != nil {
		return CostReportEntry{}, e
	}
	v, e := costReportEntry(r)
	if e != nil {
		return CostReportEntry{}, e
	}
	if tx.Commit() != nil {
		return CostReportEntry{}, ErrUnavailable
	}
	return v, nil
}
func CostReportCSV(p CostReportPage) (string, error) {
	if ValidateCostReport(p.CostReportQuery) != nil || len(p.Items) > p.Limit {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"operation_id", "product_id", "sku_snapshot", "nome_snapshot", "actor_id", "created_at", "change", "before_cost_cents", "after_cost_cents", "delta_cents", "revision", "reason"}) != nil {
		return "", ErrUnavailable
	}
	for _, v := range p.Items {
		expected, e := costReportEntry(v.CostReceipt)
		if e != nil || expected.Change != v.Change || !ValidUUID(v.OperationID) || !ValidUUID(v.ActorID) || v.CreatedAt.IsZero() {
			return "", ErrUnavailable
		}
		before, delta := "", ""
		if v.Before.Known {
			before = strconv.FormatInt(*v.Before.CostCents, 10)
			delta = strconv.FormatInt(*expected.DeltaCents, 10)
		}
		if w.Write([]string{v.OperationID, strconv.FormatInt(v.ProductID, 10), qualityCSVCell(v.Product.SKU), qualityCSVCell(v.Product.Name), v.ActorID, v.CreatedAt.UTC().Format(time.RFC3339Nano), v.Change, before, strconv.FormatInt(*v.After.CostCents, 10), delta, strconv.FormatInt(v.After.Revision, 10), qualityCSVCell(v.Reason)}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
