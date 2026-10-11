package onlinecatalog

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type DiscountReportQuery struct {
	Query  string `json:"q"`
	Active string `json:"active"`
	Status string `json:"status"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}
type DiscountReportItem struct {
	ID               int64     `json:"id"`
	ProductID        int64     `json:"product_id"`
	Name             string    `json:"nome"`
	SKU              string    `json:"sku"`
	Active           bool      `json:"ativo"`
	PriceCents       int64     `json:"current_price_cents"`
	ProductVersion   int64     `json:"current_product_version"`
	DiscountBPS      int64     `json:"discount_bps"`
	Status           string    `json:"status"`
	SuggestionReason string    `json:"suggestion_reason"`
	ReviewedBy       string    `json:"reviewed_by"`
	ReviewReason     string    `json:"review_reason"`
	OperationID      string    `json:"operation_id"`
	HasReceipt       bool      `json:"has_receipt"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type DiscountReportPage struct {
	DiscountReportQuery
	Total       int64                `json:"total"`
	Items       []DiscountReportItem `json:"items"`
	HasMore     bool                 `json:"has_more"`
	NextOffset  *int                 `json:"next_offset,omitempty"`
	GeneratedAt time.Time            `json:"generated_at"`
	SourceHash  string               `json:"source_hash"`
}
type DiscountReportSummary struct {
	Query               string    `json:"q"`
	Active              string    `json:"active"`
	Total               int64     `json:"total"`
	Pending             int64     `json:"pending"`
	Approved            int64     `json:"approved"`
	Rejected            int64     `json:"rejected"`
	FinalWithoutReceipt int64     `json:"final_without_receipt"`
	GeneratedAt         time.Time `json:"generated_at"`
}

func ValidateDiscountReport(q DiscountReportQuery) error {
	if !clean(q.Query, 120, false) || (q.Active != "all" && q.Active != "active" && q.Active != "inactive") ||
		(q.Status != "all" && q.Status != "PENDING" && q.Status != "APPROVED" && q.Status != "REJECTED") || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	return nil
}

const discountReportFrom = ` FROM discount_suggestions d JOIN products p ON p.tenant_id=d.tenant_id AND p.id=d.product_id
 LEFT JOIN online_discount_reviews r ON r.tenant_id=d.tenant_id AND r.suggestion_id=d.id
 WHERE d.tenant_id=$1 AND d.deleted_at IS NULL AND p.deleted_at IS NULL
 AND ($2::text='all' OR p.ativo=($2='active'))
 AND ($3::text='' OR p.nome ILIKE $3 ESCAPE '\' OR p.sku ILIKE $3 ESCAPE '\')`
const discountReportWhere = ` AND ($4::text='all' OR d.status=$4)`

func (s Store) DiscountReportSummary(ctx context.Context, a Actor, q DiscountReportQuery) (DiscountReportSummary, error) {
	if ValidateDiscountReport(q) != nil || q.Status != "all" {
		return DiscountReportSummary{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return DiscountReportSummary{}, e
	}
	defer tx.Rollback()
	r := DiscountReportSummary{Query: q.Query, Active: q.Active}
	e = tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE d.status='PENDING'),count(*) FILTER(WHERE d.status='APPROVED'),count(*) FILTER(WHERE d.status='REJECTED'),count(*) FILTER(WHERE d.status IN ('APPROVED','REJECTED') AND r.operation_id IS NULL),transaction_timestamp()`+discountReportFrom, a.Tenant, q.Active, qualityPattern(q.Query)).Scan(&r.Total, &r.Pending, &r.Approved, &r.Rejected, &r.FinalWithoutReceipt, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.Pending < 0 || r.Approved < 0 || r.Rejected < 0 || r.Pending > r.Total || r.Approved > r.Total || r.Rejected > r.Total || r.Pending+r.Approved+r.Rejected != r.Total || r.FinalWithoutReceipt < 0 || r.FinalWithoutReceipt > r.Approved+r.Rejected || r.GeneratedAt.IsZero() {
		return DiscountReportSummary{}, ErrUnavailable
	}
	if tx.Commit() != nil {
		return DiscountReportSummary{}, ErrUnavailable
	}
	return r, nil
}
func scanDiscountReport(row scanner) (DiscountReportItem, error) {
	var r DiscountReportItem
	var price, percent, action string
	e := row.Scan(&r.ID, &r.ProductID, &r.Name, &r.SKU, &r.Active, &price, &r.ProductVersion, &percent, &r.Status, &r.SuggestionReason, &r.ReviewedBy, &r.ReviewReason, &r.OperationID, &action, &r.CreatedAt, &r.UpdatedAt)
	if e != nil {
		return r, ErrUnavailable
	}
	r.PriceCents, e = cents(price)
	if e != nil {
		return r, ErrUnavailable
	}
	r.DiscountBPS, e = cents(percent)
	r.HasReceipt = r.OperationID != ""
	if e != nil || r.ID < 1 || r.ID > MaxVersion || r.ProductID < 1 || r.ProductID > MaxVersion || r.ProductVersion < 1 || r.ProductVersion > MaxVersion || r.DiscountBPS > 10000 || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || (r.Status != "PENDING" && r.Status != "APPROVED" && r.Status != "REJECTED") {
		return r, ErrUnavailable
	}
	if r.Status == "PENDING" && (r.ReviewedBy != "" || r.HasReceipt) {
		return r, ErrUnavailable
	}
	if r.Status != "PENDING" && strings.TrimSpace(r.ReviewedBy) == "" {
		return r, ErrUnavailable
	}
	if r.HasReceipt && (!ValidUUID(r.OperationID) || !clean(r.ReviewReason, 500, true) || (r.Status == "APPROVED" && action != "approve") || (r.Status == "REJECTED" && action != "reject")) {
		return r, ErrUnavailable
	}
	if !r.HasReceipt && (r.ReviewReason != "" || action != "") {
		return r, ErrUnavailable
	}
	r.CreatedAt = r.CreatedAt.UTC()
	r.UpdatedAt = r.UpdatedAt.UTC()
	return r, nil
}
func (s Store) DiscountReport(ctx context.Context, a Actor, q DiscountReportQuery) (DiscountReportPage, error) {
	if ValidateDiscountReport(q) != nil {
		return DiscountReportPage{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return DiscountReportPage{}, e
	}
	defer tx.Rollback()
	r := DiscountReportPage{DiscountReportQuery: q, Items: make([]DiscountReportItem, 0)}
	e = tx.QueryRowContext(ctx, `SELECT count(*),transaction_timestamp()`+discountReportFrom+discountReportWhere, a.Tenant, q.Active, qualityPattern(q.Query), q.Status).Scan(&r.Total, &r.GeneratedAt)
	if e != nil || r.Total < 0 || r.Total > MaxVersion || r.GeneratedAt.IsZero() {
		return r, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT d.id,p.id,p.nome,p.sku,p.ativo,p.preco::text,p.catalog_version,d.suggested_discount::text,d.status,d.reason,d.reviewed_by,COALESCE(r.reason,''),COALESCE(r.operation_id::text,''),COALESCE(r.action,''),d.created_at,d.updated_at`+discountReportFrom+discountReportWhere+` ORDER BY d.id ASC LIMIT $5 OFFSET $6`, a.Tenant, q.Active, qualityPattern(q.Query), q.Status, q.Limit, q.Offset)
	if e != nil {
		return r, ErrUnavailable
	}
	var prev int64
	for rows.Next() {
		item, e := scanDiscountReport(rows)
		if e != nil || item.ID <= prev || (q.Status != "all" && item.Status != q.Status) || (q.Active == "active" && !item.Active) || (q.Active == "inactive" && item.Active) {
			rows.Close()
			return DiscountReportPage{}, ErrUnavailable
		}
		prev = item.ID
		r.Items = append(r.Items, item)
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
		return DiscountReportPage{}, ErrUnavailable
	}
	r.HasMore = int64(q.Offset)+int64(len(r.Items)) < r.Total
	if r.HasMore {
		next := q.Offset + len(r.Items)
		r.NextOffset = &next
	}
	raw, e := json.Marshal(struct {
		Query DiscountReportQuery
		Total int64
		Items []DiscountReportItem
	}{q, r.Total, r.Items})
	if e != nil {
		return DiscountReportPage{}, ErrUnavailable
	}
	r.SourceHash = creationHash(raw)
	if tx.Commit() != nil {
		return DiscountReportPage{}, ErrUnavailable
	}
	return r, nil
}
func DiscountReportCSV(page DiscountReportPage) (string, error) {
	if ValidateDiscountReport(page.DiscountReportQuery) != nil || len(page.Items) > page.Limit {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"suggestion_id", "product_id", "sku", "nome", "ativo", "current_price_cents", "current_product_version", "discount_bps", "status", "suggestion_reason", "reviewed_by", "review_reason", "operation_id", "has_receipt", "created_at", "updated_at"}) != nil {
		return "", ErrUnavailable
	}
	for _, r := range page.Items {
		if w.Write([]string{strconv.FormatInt(r.ID, 10), strconv.FormatInt(r.ProductID, 10), qualityCSVCell(r.SKU), qualityCSVCell(r.Name), strconv.FormatBool(r.Active), strconv.FormatInt(r.PriceCents, 10), strconv.FormatInt(r.ProductVersion, 10), strconv.FormatInt(r.DiscountBPS, 10), r.Status, qualityCSVCell(r.SuggestionReason), qualityCSVCell(r.ReviewedBy), qualityCSVCell(r.ReviewReason), r.OperationID, strconv.FormatBool(r.HasReceipt), r.CreatedAt.Format(time.RFC3339Nano), r.UpdatedAt.Format(time.RFC3339Nano)}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
