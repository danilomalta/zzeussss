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

type ProposalQuery struct {
	Query     string `json:"q"`
	State     string `json:"state"`
	ProductID int64  `json:"product_id"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}
type ProposalPage struct {
	ProposalQuery
	Items       []ReplenishmentProposal `json:"items"`
	Total       int64                   `json:"total"`
	HasMore     bool                    `json:"has_more"`
	NextOffset  *int                    `json:"next_offset,omitempty"`
	GeneratedAt time.Time               `json:"generated_at"`
	SourceHash  string                  `json:"source_hash"`
}
type ProposalSummary struct {
	Query       string    `json:"q"`
	ProductID   int64     `json:"product_id"`
	Total       int64     `json:"total"`
	Planned     int64     `json:"planned"`
	Approved    int64     `json:"approved"`
	Cancelled   int64     `json:"cancelled"`
	GeneratedAt time.Time `json:"generated_at"`
}
type ProposalHistory struct {
	Items      []ReplenishmentProposalReceipt `json:"items"`
	Total      int64                          `json:"total"`
	Limit      int                            `json:"limit"`
	Offset     int                            `json:"offset"`
	HasMore    bool                           `json:"has_more"`
	NextOffset *int                           `json:"next_offset,omitempty"`
}
type ProposalCheck struct {
	Proposal        ReplenishmentProposal `json:"proposal"`
	Current         *StockPlan            `json:"current"`
	Status          string                `json:"status"`
	SnapshotMatches bool                  `json:"snapshot_matches"`
	CanApprove      bool                  `json:"can_approve"`
	CheckedAt       time.Time             `json:"checked_at"`
}

func ValidateProposalQuery(q ProposalQuery) error {
	if !clean(q.Query, 120, false) || q.ProductID < 0 || q.ProductID > MaxVersion || q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 1000000000 {
		return ErrInput
	}
	switch q.State {
	case "all", "planned", "approved", "cancelled":
		return nil
	default:
		return ErrInput
	}
}

const proposalWhere = ` FROM online_replenishment_proposals WHERE tenant_id=$1 AND ($2::text='all' OR state=$2) AND ($3::text='' OR payload#>>'{snapshot,product,nome}' ILIKE $3 ESCAPE '\' OR payload#>>'{snapshot,product,sku}' ILIKE $3 ESCAPE '\' OR payload->>'reason' ILIKE $3 ESCAPE '\') AND ($4::bigint=0 OR product_id=$4)`

func proposalPageHash(p ProposalPage) string {
	b, _ := json.Marshal(struct {
		Query ProposalQuery
		Total int64
		Items []ReplenishmentProposal
	}{p.ProposalQuery, p.Total, p.Items})
	return creationHash(b)
}
func (s Store) ReplenishmentProposalSummary(ctx context.Context, a Actor, q ProposalQuery) (ProposalSummary, error) {
	var r ProposalSummary
	if ValidateProposalQuery(q) != nil || q.State != "all" {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	r.Query = q.Query
	r.ProductID = q.ProductID
	if tx.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER(WHERE state='planned'),count(*) FILTER(WHERE state='approved'),count(*) FILTER(WHERE state='cancelled'),transaction_timestamp()`+proposalWhere, a.Tenant, q.State, qualityPattern(q.Query), q.ProductID).Scan(&r.Total, &r.Planned, &r.Approved, &r.Cancelled, &r.GeneratedAt) != nil || r.Total < 0 || r.Total > MaxVersion || r.Planned+r.Approved+r.Cancelled != r.Total {
		return r, ErrUnavailable
	}
	r.GeneratedAt = r.GeneratedAt.UTC()
	if tx.Commit() != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func (s Store) ReplenishmentProposals(ctx context.Context, a Actor, q ProposalQuery) (ProposalPage, error) {
	r := ProposalPage{ProposalQuery: q, Items: make([]ReplenishmentProposal, 0)}
	if ValidateProposalQuery(q) != nil {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	if tx.QueryRowContext(ctx, `SELECT count(*),transaction_timestamp()`+proposalWhere, a.Tenant, q.State, qualityPattern(q.Query), q.ProductID).Scan(&r.Total, &r.GeneratedAt) != nil || r.Total < 0 || r.Total > MaxVersion {
		return r, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+proposalColumns+proposalWhere+` ORDER BY (payload->>'created_at')::timestamptz DESC,proposal_id DESC LIMIT $5 OFFSET $6`, a.Tenant, q.State, qualityPattern(q.Query), q.ProductID, q.Limit, q.Offset)
	if e != nil {
		return r, ErrUnavailable
	}
	for rows.Next() {
		p, e := scanProposal(rows)
		if e != nil || (q.State != "all" && p.State != q.State) || (q.ProductID != 0 && p.ProductID != q.ProductID) {
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
	r.SourceHash = proposalPageHash(r)
	if tx.Commit() != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func (s Store) ReplenishmentProposalHistory(ctx context.Context, a Actor, id string, limit, offset int) (ProposalHistory, error) {
	r := ProposalHistory{Items: make([]ReplenishmentProposalReceipt, 0), Limit: limit, Offset: offset}
	if !ValidUUID(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000000 {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	if _, e = scanProposal(tx.QueryRowContext(ctx, proposalSQL, a.Tenant, id)); e != nil {
		return r, e
	}
	if tx.QueryRowContext(ctx, `SELECT count(*) FROM online_replenishment_operations WHERE tenant_id=$1 AND proposal_id=$2`, a.Tenant, id).Scan(&r.Total) != nil || r.Total < 1 || r.Total > MaxVersion {
		return r, ErrUnavailable
	}
	rows, e := tx.QueryContext(ctx, `SELECT operation_id::text,proposal_id::text,actor_id::text,kind,payload_hash,receipt FROM online_replenishment_operations WHERE tenant_id=$1 AND proposal_id=$2 ORDER BY (receipt#>>'{after,revision}')::bigint DESC,operation_id DESC LIMIT $3 OFFSET $4`, a.Tenant, id, limit, offset)
	if e != nil {
		return r, ErrUnavailable
	}
	for rows.Next() {
		v, _, e := scanProposalReceipt(rows)
		if e != nil || v.ProposalID != id {
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
func checkProposalSnapshot(p ReplenishmentProposal, current *StockPlan, at time.Time) ProposalCheck {
	r := ProposalCheck{Proposal: p, Current: current, Status: "product_unavailable", CheckedAt: at.UTC()}
	if current != nil {
		r.SnapshotMatches = reflect.DeepEqual(p.Snapshot.StockPlan, *current)
		r.Status = "changed"
		if r.SnapshotMatches {
			r.Status = "matches"
		}
		r.CanApprove = p.State == "planned" && r.SnapshotMatches
	}
	return r
}
func (s Store) CheckReplenishmentProposal(ctx context.Context, a Actor, id string) (ProposalCheck, error) {
	var r ProposalCheck
	if !ValidUUID(id) {
		return r, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return r, e
	}
	defer tx.Rollback()
	p, e := scanProposal(tx.QueryRowContext(ctx, proposalSQL, a.Tenant, id))
	if e != nil {
		return r, e
	}
	plan, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL, a.Tenant, p.ProductID))
	if e != nil && e != ErrMissing {
		return r, e
	}
	var at time.Time
	if tx.QueryRowContext(ctx, `SELECT transaction_timestamp()`).Scan(&at) != nil {
		return r, ErrUnavailable
	}
	var current *StockPlan
	if e == nil {
		current = &plan
	}
	r = checkProposalSnapshot(p, current, at)
	if tx.Commit() != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func ReplenishmentProposalCSV(p ProposalPage) (string, error) {
	if ValidateProposalQuery(p.ProposalQuery) != nil || len(p.Items) > p.Limit || p.SourceHash != proposalPageHash(p) {
		return "", ErrInput
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write([]string{"proposal_id", "product_id", "sku_snapshot", "nome_snapshot", "state", "revision", "quantity_units", "suggested_units_snapshot", "product_version_snapshot", "creator_id", "last_actor_id", "reason", "created_at", "updated_at"}) != nil {
		return "", ErrUnavailable
	}
	for _, v := range p.Items {
		if !validProposal(v) || (p.State != "all" && v.State != p.State) || (p.ProductID != 0 && v.ProductID != p.ProductID) {
			return "", ErrUnavailable
		}
		if w.Write([]string{v.ID, strconv.FormatInt(v.ProductID, 10), qualityCSVCell(v.Snapshot.Product.SKU), qualityCSVCell(v.Snapshot.Product.Name), v.State, strconv.FormatInt(v.Revision, 10), strconv.FormatInt(v.QuantityUnits, 10), strconv.FormatInt(v.Snapshot.SuggestedUnits, 10), strconv.FormatInt(v.Snapshot.Product.Version, 10), v.CreatorID, v.LastActorID, qualityCSVCell(v.Reason), v.CreatedAt.UTC().Format(time.RFC3339Nano), v.UpdatedAt.UTC().Format(time.RFC3339Nano)}) != nil {
			return "", ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > 262144 {
		return "", ErrUnavailable
	}
	return out.String(), nil
}
