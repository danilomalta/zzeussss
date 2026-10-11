package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type DiscountCheckQuery struct {
	Query  string `json:"q"`
	Issue  string `json:"issue"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}
type DiscountCheckItem struct {
	ID              int64    `json:"id"`
	ProductID       int64    `json:"product_id"`
	Name            string   `json:"nome"`
	SKU             string   `json:"sku"`
	Active          bool     `json:"ativo"`
	CurrentPrice    int64    `json:"current_price_cents"`
	CurrentVersion  int64    `json:"current_product_version"`
	DiscountBPS     int64    `json:"discount_bps"`
	ApprovedPrice   *int64   `json:"approved_price_cents,omitempty"`
	ApprovedVersion *int64   `json:"approved_product_version,omitempty"`
	OperationID     string   `json:"operation_id"`
	HasReceipt      bool     `json:"has_receipt"`
	Issues          []string `json:"issues"`
}
type DiscountCheckPage struct {
	DiscountCheckQuery
	Total       int64               `json:"total"`
	Items       []DiscountCheckItem `json:"items"`
	HasMore     bool                `json:"has_more"`
	NextOffset  *int                `json:"next_offset,omitempty"`
	GeneratedAt time.Time           `json:"generated_at"`
	SourceHash  string              `json:"source_hash"`
}
type DiscountCheckSummary struct {
	Query          string    `json:"q"`
	Total          int64     `json:"total"`
	Attention      int64     `json:"attention"`
	PriceChanged   int64     `json:"price_changed"`
	VersionChanged int64     `json:"version_changed"`
	Inactive       int64     `json:"inactive"`
	NoReceipt      int64     `json:"no_receipt"`
	GeneratedAt    time.Time `json:"generated_at"`
}

func ValidateDiscountCheck(q DiscountCheckQuery) error {
	if !clean(q.Query, 120, false) || (q.Issue != "all" && q.Issue != "any" && q.Issue != "price_changed" && q.Issue != "inactive") || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	return nil
}

const discountCheckCTE = `WITH checks AS (
 SELECT d.id,p.id AS product_id,p.nome,p.sku,p.ativo,p.preco::text AS price,p.catalog_version,d.suggested_discount::text AS percent,d.reviewed_by,
 COALESCE(r.operation_id::text,'') AS operation_id,COALESCE(r.after_state,'{}'::jsonb) AS snapshot,
 r.operation_id IS NULL AS no_receipt,NOT p.ativo AS inactive,
 COALESCE(p.preco*100<>(r.after_state->'product'->>'price_cents')::bigint,FALSE) AS price_changed,
 COALESCE(p.catalog_version<>(r.after_state->'product'->>'version')::bigint,FALSE) AS version_changed,
 COALESCE(r.actor_id::text,'') AS receipt_actor,COALESCE(r.action,'') AS receipt_action
 FROM discount_suggestions d JOIN products p ON p.tenant_id=d.tenant_id AND p.id=d.product_id
 LEFT JOIN online_discount_reviews r ON r.tenant_id=d.tenant_id AND r.suggestion_id=d.id
 WHERE d.tenant_id=$1 AND d.deleted_at IS NULL AND p.deleted_at IS NULL AND d.status='APPROVED'
 AND ($2::text='' OR p.nome ILIKE $2 ESCAPE '\' OR p.sku ILIKE $2 ESCAPE '\')
)`
const discountCheckColumns = `id,product_id,nome,sku,ativo,price,catalog_version,percent,reviewed_by,operation_id,snapshot,no_receipt,inactive,price_changed,version_changed,receipt_actor,receipt_action`
const discountCheckWhere = ` FROM checks WHERE ($3::text='all' OR ($3='any' AND (no_receipt OR inactive OR price_changed OR version_changed)) OR ($3='price_changed' AND price_changed) OR ($3='inactive' AND inactive))`

func scanDiscountCheck(row scanner) (DiscountCheckItem, error) {
	var r DiscountCheckItem
	var price, percent, reviewer, actor, action string
	var raw []byte
	var missing, inactive, changed, version bool
	e := row.Scan(&r.ID, &r.ProductID, &r.Name, &r.SKU, &r.Active, &price, &r.CurrentVersion, &percent, &reviewer, &r.OperationID, &raw, &missing, &inactive, &changed, &version, &actor, &action)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrMissing
	}
	if e != nil {
		return r, ErrUnavailable
	}
	r.CurrentPrice, e = cents(price)
	if e != nil {
		return r, ErrUnavailable
	}
	r.DiscountBPS, e = cents(percent)
	r.HasReceipt = !missing
	r.Issues = make([]string, 0, 4)
	if e != nil || r.ID < 1 || r.ID > MaxVersion || r.ProductID < 1 || r.ProductID > MaxVersion || r.CurrentVersion < 1 || r.CurrentVersion > MaxVersion || r.DiscountBPS > 10000 || strings.TrimSpace(reviewer) == "" || inactive == r.Active {
		return r, ErrUnavailable
	}
	if missing {
		if r.OperationID != "" || changed || version || actor != "" || action != "" {
			return r, ErrUnavailable
		}
		r.Issues = append(r.Issues, "no_receipt")
	} else {
		var snapshot DiscountSuggestion
		if !ValidUUID(r.OperationID) || !ValidUUID(actor) || action != "approve" || actor != reviewer || json.Unmarshal(raw, &snapshot) != nil || snapshot.ID != r.ID || snapshot.ProductID != r.ProductID || snapshot.Status != "APPROVED" || snapshot.ReviewedBy != actor || snapshot.DiscountBPS != r.DiscountBPS || snapshot.Product.ID != r.ProductID || !snapshot.Product.Active || snapshot.Product.PriceCents < 0 || snapshot.Product.PriceCents > MaxPrice || snapshot.Product.Version < 1 || snapshot.Product.Version > MaxVersion || changed != (r.CurrentPrice != snapshot.Product.PriceCents) || version != (r.CurrentVersion != snapshot.Product.Version) {
			return r, ErrUnavailable
		}
		r.ApprovedPrice = &snapshot.Product.PriceCents
		r.ApprovedVersion = &snapshot.Product.Version
	}
	if inactive {
		r.Issues = append(r.Issues, "inactive")
	}
	if changed {
		r.Issues = append(r.Issues, "price_changed")
	}
	if version {
		r.Issues = append(r.Issues, "version_changed")
	}
	return r, nil
}
func (s Store) DiscountCheckSummary(ctx context.Context, a Actor, q DiscountCheckQuery) (DiscountCheckSummary, error) {
	if ValidateDiscountCheck(q) != nil || q.Issue != "all" {
		return DiscountCheckSummary{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return DiscountCheckSummary{}, e
	}
	defer tx.Rollback()
	r := DiscountCheckSummary{Query: q.Query}
	e = tx.QueryRowContext(ctx, discountCheckCTE+` SELECT count(*),count(*) FILTER(WHERE no_receipt OR inactive OR price_changed OR version_changed),count(*) FILTER(WHERE price_changed),count(*) FILTER(WHERE version_changed),count(*) FILTER(WHERE inactive),count(*) FILTER(WHERE no_receipt),transaction_timestamp() FROM checks`, a.Tenant, qualityPattern(q.Query)).Scan(&r.Total, &r.Attention, &r.PriceChanged, &r.VersionChanged, &r.Inactive, &r.NoReceipt, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.Attention < 0 || r.Attention > r.Total || r.PriceChanged < 0 || r.PriceChanged > r.Attention || r.VersionChanged < 0 || r.VersionChanged > r.Attention || r.Inactive < 0 || r.Inactive > r.Attention || r.NoReceipt < 0 || r.NoReceipt > r.Attention || r.GeneratedAt.IsZero() {
		return DiscountCheckSummary{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return DiscountCheckSummary{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) DiscountCheck(ctx context.Context, a Actor, id int64) (DiscountCheckItem, error) {
	if id < 1 || id > MaxVersion {
		return DiscountCheckItem{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return DiscountCheckItem{}, e
	}
	defer tx.Rollback()
	r, e := scanDiscountCheck(tx.QueryRowContext(ctx, discountCheckCTE+` SELECT `+discountCheckColumns+` FROM checks WHERE id=$3`, a.Tenant, "", id))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return DiscountCheckItem{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) DiscountChecks(ctx context.Context, a Actor, q DiscountCheckQuery) (DiscountCheckPage, error) {
	if ValidateDiscountCheck(q) != nil {
		return DiscountCheckPage{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return DiscountCheckPage{}, e
	}
	defer tx.Rollback()
	r := DiscountCheckPage{DiscountCheckQuery: q, Items: make([]DiscountCheckItem, 0)}
	e = tx.QueryRowContext(ctx, discountCheckCTE+` SELECT count(*),transaction_timestamp()`+discountCheckWhere, a.Tenant, qualityPattern(q.Query), q.Issue).Scan(&r.Total, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.GeneratedAt.IsZero() {
		return r, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, discountCheckCTE+` SELECT `+discountCheckColumns+discountCheckWhere+` ORDER BY id ASC LIMIT $4 OFFSET $5`, a.Tenant, qualityPattern(q.Query), q.Issue, q.Limit, q.Offset)
	if e != nil {
		return r, ErrUnavailable
	}
	var prev int64
	for rows.Next() {
		v, e := scanDiscountCheck(rows)
		if e != nil || v.ID <= prev || (q.Issue == "any" && len(v.Issues) == 0) || (q.Issue != "all" && q.Issue != "any" && !qualityHas(v.Issues, q.Issue)) {
			rows.Close()
			return DiscountCheckPage{}, ErrUnavailable
		}
		prev = v.ID
		r.Items = append(r.Items, v)
	}
	re := rows.Err()
	ce := rows.Close()
	want := r.Total - int64(q.Offset)
	if want < 0 {
		want = 0
	}
	if want > int64(q.Limit) {
		want = int64(q.Limit)
	}
	if re != nil || ce != nil || int64(len(r.Items)) != want {
		return DiscountCheckPage{}, ErrUnavailable
	}
	r.HasMore = int64(q.Offset)+int64(len(r.Items)) < r.Total
	if r.HasMore {
		n := q.Offset + len(r.Items)
		r.NextOffset = &n
	}
	raw, e := json.Marshal(struct {
		Query DiscountCheckQuery
		Total int64
		Items []DiscountCheckItem
	}{q, r.Total, r.Items})
	if e != nil {
		return DiscountCheckPage{}, ErrUnavailable
	}
	r.SourceHash = creationHash(raw)
	if tx.Commit() != nil {
		return DiscountCheckPage{}, ErrUnavailable
	}
	return r, nil
}
func DiscountChecksCSV(p DiscountCheckPage) (string, error) {
	if ValidateDiscountCheck(p.DiscountCheckQuery) != nil || len(p.Items) > p.Limit {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"suggestion_id", "product_id", "sku", "nome", "ativo", "current_price_cents", "current_product_version", "discount_bps", "approved_price_cents", "approved_product_version", "operation_id", "has_receipt", "issues"}) != nil {
		return "", ErrUnavailable
	}
	for _, v := range p.Items {
		price, version := "", ""
		if v.ApprovedPrice != nil {
			price = strconv.FormatInt(*v.ApprovedPrice, 10)
		}
		if v.ApprovedVersion != nil {
			version = strconv.FormatInt(*v.ApprovedVersion, 10)
		}
		if w.Write([]string{strconv.FormatInt(v.ID, 10), strconv.FormatInt(v.ProductID, 10), qualityCSVCell(v.SKU), qualityCSVCell(v.Name), strconv.FormatBool(v.Active), strconv.FormatInt(v.CurrentPrice, 10), strconv.FormatInt(v.CurrentVersion, 10), strconv.FormatInt(v.DiscountBPS, 10), price, version, v.OperationID, strconv.FormatBool(v.HasReceipt), strings.Join(v.Issues, "|")}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
