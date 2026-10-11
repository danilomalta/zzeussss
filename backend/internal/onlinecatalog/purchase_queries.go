package onlinecatalog

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type PurchaseQuery struct {
	Query      string `json:"q"`
	State      string `json:"state"`
	ProductID  int64  `json:"product_id"`
	SupplierID string `json:"supplier_id"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}
type PurchasePage struct {
	PurchaseQuery
	Items       []PurchaseDraft `json:"items"`
	Total       int64           `json:"total"`
	HasMore     bool            `json:"has_more"`
	NextOffset  *int            `json:"next_offset,omitempty"`
	GeneratedAt time.Time       `json:"generated_at"`
	SourceHash  string          `json:"source_hash"`
}
type PurchaseSummary struct {
	Query       string    `json:"q"`
	ProductID   int64     `json:"product_id"`
	SupplierID  string    `json:"supplier_id"`
	Total       int64     `json:"total"`
	Draft       int64     `json:"draft"`
	Approved    int64     `json:"approved"`
	Cancelled   int64     `json:"cancelled"`
	GeneratedAt time.Time `json:"generated_at"`
}
type PurchaseHistory struct {
	Items      []PurchaseDraftReceipt `json:"items"`
	Total      int64                  `json:"total"`
	Limit      int                    `json:"limit"`
	Offset     int                    `json:"offset"`
	HasMore    bool                   `json:"has_more"`
	NextOffset *int                   `json:"next_offset,omitempty"`
}
type PurchaseCheck struct {
	Purchase        PurchaseDraft          `json:"purchase"`
	CurrentProposal *ReplenishmentProposal `json:"current_proposal"`
	CurrentOffer    *SupplierOffer         `json:"current_offer"`
	CurrentSupplier *Supplier              `json:"current_supplier"`
	CurrentProduct  *StockPlan             `json:"current_product"`
	ProposalMatches bool                   `json:"proposal_matches"`
	OfferMatches    bool                   `json:"offer_matches"`
	SupplierMatches bool                   `json:"supplier_matches"`
	ProductMatches  bool                   `json:"product_matches"`
	SnapshotMatches bool                   `json:"snapshot_matches"`
	CanApprove      bool                   `json:"can_approve"`
	Status          string                 `json:"status"`
	CheckedAt       time.Time              `json:"checked_at"`
}

func validOptionalUUID(s string) bool { return s == "" || ValidUUID(s) }

func ValidatePurchaseQuery(q PurchaseQuery) error {
	if !clean(q.Query, 120, false) || !validOptionalUUID(q.SupplierID) || q.ProductID < 0 || q.ProductID > MaxVersion || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	switch q.State {
	case "all", "draft", "approved", "cancelled":
		return nil
	default:
		return ErrInput
	}
}

const purchaseWhere = ` FROM online_purchase_drafts WHERE tenant_id=$1 AND ($2::text='all' OR state=$2) AND ($3::text='' OR payload#>>'{proposal,snapshot,product,nome}' ILIKE $3 ESCAPE '\' OR payload#>>'{proposal,snapshot,product,sku}' ILIKE $3 ESCAPE '\' OR payload->>'reason' ILIKE $3 ESCAPE '\') AND ($4::bigint=0 OR (payload#>>'{proposal,product_id}')::bigint=$4) AND ($5::text='' OR payload#>>'{supplier,id}'=$5)`

func purchasePageHash(p PurchasePage) string {
	b, _ := json.Marshal(struct {
		Query PurchaseQuery
		Total int64
		Items []PurchaseDraft
	}{p.PurchaseQuery, p.Total, p.Items})
	return creationHash(b)
}
func (s Store) PurchaseDraftSummary(ctx context.Context, a Actor, q PurchaseQuery) (PurchaseSummary, error) {
	var r PurchaseSummary
	if ValidatePurchaseQuery(q) != nil || q.State != "all" {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	r.Query = q.Query
	r.ProductID = q.ProductID
	r.SupplierID = q.SupplierID
	if tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE state='draft'),count(*) FILTER(WHERE state='approved'),count(*) FILTER(WHERE state='cancelled'),transaction_timestamp()`+purchaseWhere, a.Tenant, q.State, qualityPattern(q.Query), q.ProductID, q.SupplierID).Scan(&r.Total, &r.Draft, &r.Approved, &r.Cancelled, &r.GeneratedAt) != nil || r.Total < 0 || r.Total > MaxVersion || r.Draft+r.Approved+r.Cancelled != r.Total {
		return r, ErrUnavailable
	}
	r.GeneratedAt = r.GeneratedAt.UTC()
	if tx.Commit() != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func (s Store) PurchaseDrafts(ctx context.Context, a Actor, q PurchaseQuery) (PurchasePage, error) {
	r := PurchasePage{PurchaseQuery: q, Items: make([]PurchaseDraft, 0)}
	if ValidatePurchaseQuery(q) != nil {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	if tx.QueryRowContext(ctx, `SELECT count(*),transaction_timestamp()`+purchaseWhere, a.Tenant, q.State, qualityPattern(q.Query), q.ProductID, q.SupplierID).Scan(&r.Total, &r.GeneratedAt) != nil || r.Total < 0 || r.Total > MaxVersion {
		return r, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+purchaseColumns+purchaseWhere+` ORDER BY (payload->>'created_at')::timestamptz DESC,purchase_id DESC LIMIT $6 OFFSET $7`, a.Tenant, q.State, qualityPattern(q.Query), q.ProductID, q.SupplierID, q.Limit, q.Offset)
	if e != nil {
		return r, ErrUnavailable
	}
	for rows.Next() {
		p, e := scanPurchaseDraft(rows)
		if e != nil || (q.State != "all" && p.State != q.State) || (q.ProductID != 0 && p.Proposal.ProductID != q.ProductID) || (q.SupplierID != "" && p.Supplier.ID != q.SupplierID) {
			rows.Close()
			return r, ErrUnavailable
		}
		r.Items = append(r.Items, p)
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
		return r, ErrUnavailable
	}
	r.HasMore = int64(q.Offset+len(r.Items)) < r.Total
	if r.HasMore {
		n := q.Offset + len(r.Items)
		r.NextOffset = &n
	}
	r.GeneratedAt = r.GeneratedAt.UTC()
	r.SourceHash = purchasePageHash(r)
	if tx.Commit() != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func (s Store) PurchaseDraftHistory(ctx context.Context, a Actor, id string, limit, offset int) (PurchaseHistory, error) {
	r := PurchaseHistory{Items: make([]PurchaseDraftReceipt, 0), Limit: limit, Offset: offset}
	if !ValidUUID(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	current, e := scanPurchaseDraft(tx.QueryRowContext(ctx, purchaseSQL, a.Tenant, id))
	if e != nil {
		return r, e
	}
	if tx.QueryRowContext(ctx, `SELECT count(*) FROM online_purchase_operations WHERE tenant_id=$1 AND purchase_id=$2`, a.Tenant, id).Scan(&r.Total) != nil || r.Total != current.Revision || r.Total > MaxVersion {
		return r, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT operation_id::text,purchase_id::text,actor_id::text,kind,payload_hash,receipt FROM online_purchase_operations WHERE tenant_id=$1 AND purchase_id=$2 ORDER BY (receipt#>>'{after,revision}')::bigint DESC,operation_id DESC LIMIT $3 OFFSET $4`, a.Tenant, id, limit, offset)
	if e != nil {
		return r, ErrUnavailable
	}
	for rows.Next() {
		v, _, e := scanPurchaseReceipt(rows)
		if e != nil || v.PurchaseID != id || v.After.Revision != current.Revision-int64(offset+len(r.Items)) || (offset == 0 && len(r.Items) == 0 && !reflect.DeepEqual(v.After, current)) || (len(r.Items) > 0 && (r.Items[len(r.Items)-1].Before == nil || !reflect.DeepEqual(*r.Items[len(r.Items)-1].Before, v.After))) {
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
		n := offset + len(r.Items)
		r.NextOffset = &n
	}
	if tx.Commit() != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func checkPurchaseSnapshot(p PurchaseDraft, proposal *ReplenishmentProposal, offer *SupplierOffer, supplier *Supplier, product *StockPlan, at time.Time) PurchaseCheck {
	r := PurchaseCheck{Purchase: p, CurrentProposal: proposal, CurrentOffer: offer, CurrentSupplier: supplier, CurrentProduct: product, Status: "parent_unavailable", CheckedAt: at.UTC()}
	r.ProposalMatches = proposal != nil && reflect.DeepEqual(p.Proposal, *proposal)
	r.OfferMatches = offer != nil && reflect.DeepEqual(p.Offer, *offer)
	r.SupplierMatches = supplier != nil && reflect.DeepEqual(p.Supplier, *supplier)
	r.ProductMatches = product != nil && reflect.DeepEqual(p.Proposal.Snapshot.StockPlan, *product)
	r.SnapshotMatches = r.ProposalMatches && r.OfferMatches && r.SupplierMatches && r.ProductMatches
	if proposal != nil && offer != nil && supplier != nil && product != nil {
		r.Status = "changed"
		if r.SnapshotMatches {
			r.Status = "matches"
		}
	}
	r.CanApprove = p.State == "draft" && p.Revision < MaxVersion && r.SnapshotMatches
	return r
}
func (s Store) CheckPurchaseDraft(ctx context.Context, a Actor, id string) (PurchaseCheck, error) {
	var zero PurchaseCheck
	if !ValidUUID(id) {
		return zero, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return zero, e
	}
	defer tx.Rollback()
	p, e := scanPurchaseDraft(tx.QueryRowContext(ctx, purchaseSQL, a.Tenant, id))
	if e != nil {
		return zero, e
	}
	var proposal *ReplenishmentProposal
	var offer *SupplierOffer
	var supplier *Supplier
	var product *StockPlan
	pp, e := scanProposal(tx.QueryRowContext(ctx, proposalSQL, a.Tenant, p.ProposalID))
	if e != nil && e != ErrMissing {
		return zero, e
	}
	if e == nil {
		proposal = &pp
	}
	po, e := scanSupplierOffer(tx.QueryRowContext(ctx, supplierOfferSQL, a.Tenant, p.Offer.ID))
	if e != nil && e != ErrMissing {
		return zero, e
	}
	if e == nil {
		offer = &po
	}
	ps, e := scanSupplier(tx.QueryRowContext(ctx, supplierSQL, a.Tenant, p.Supplier.ID))
	if e != nil && e != ErrMissing {
		return zero, e
	}
	if e == nil {
		supplier = &ps
	}
	pr, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL, a.Tenant, p.Proposal.ProductID))
	if e != nil && e != ErrMissing {
		return zero, e
	}
	if e == nil {
		product = &pr
	}
	var at time.Time
	if tx.QueryRowContext(ctx, `SELECT transaction_timestamp()`).Scan(&at) != nil {
		return zero, ErrUnavailable
	}
	result := checkPurchaseSnapshot(p, proposal, offer, supplier, product, at)
	if tx.Commit() != nil {
		return zero, ErrUnavailable
	}
	return result, nil
}
func PurchaseDraftCSV(p PurchasePage) (string, error) {
	if ValidatePurchaseQuery(p.PurchaseQuery) != nil || len(p.Items) > p.Limit || p.SourceHash != purchasePageHash(p) {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"purchase_id", "proposal_id", "supplier_id", "supplier_name_snapshot", "product_id", "sku_snapshot", "name_snapshot", "state", "revision", "quantity_units", "unit_price_cents_snapshot", "total_cents", "source", "creator_id", "last_actor_id", "reason", "created_at", "updated_at"}) != nil {
		return "", ErrUnavailable
	}
	for _, v := range p.Items {
		if !validPurchaseDraft(v) || (p.State != "all" && v.State != p.State) || (p.ProductID != 0 && v.Proposal.ProductID != p.ProductID) || (p.SupplierID != "" && v.Supplier.ID != p.SupplierID) {
			return "", ErrUnavailable
		}
		if w.Write([]string{v.ID, v.ProposalID, v.Supplier.ID, qualityCSVCell(v.Supplier.Name), strconv.FormatInt(v.Proposal.ProductID, 10), qualityCSVCell(v.Proposal.Snapshot.Product.SKU), qualityCSVCell(v.Proposal.Snapshot.Product.Name), v.State, strconv.FormatInt(v.Revision, 10), strconv.FormatInt(v.QuantityUnits, 10), strconv.FormatInt(v.Offer.UnitPriceCents, 10), strconv.FormatInt(v.TotalCents, 10), v.Source, v.CreatorID, v.LastActorID, qualityCSVCell(v.Reason), v.CreatedAt.UTC().Format(time.RFC3339Nano), v.UpdatedAt.UTC().Format(time.RFC3339Nano)}) != nil {
			return "", ErrUnavailable
		}
	}

	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
