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

// A proposal records a human decision, not a purchase order or a stock movement.
type ReplenishmentProposal struct {
	ID            string            `json:"id"`
	ProductID     int64             `json:"product_id"`
	Revision      int64             `json:"revision"`
	State         string            `json:"state"`
	Snapshot      ReplenishmentItem `json:"snapshot"`
	QuantityUnits int64             `json:"quantity_units"`
	CreatorID     string            `json:"creator_id"`
	LastActorID   string            `json:"last_actor_id"`
	Reason        string            `json:"reason"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}
type ReplenishmentProposalInput struct {
	OperationID     string `json:"operation_id"`
	ProductID       int64  `json:"product_id"`
	ExpectedVersion int64  `json:"expected_version"`
	ReviewDays      int64  `json:"review_days"`
	QuantityUnits   int64  `json:"quantity_units"`
	Reason          string `json:"reason"`
}
type ReplenishmentDecisionInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}
type ReplenishmentProposalReceipt struct {
	OperationID string                 `json:"operation_id"`
	ProposalID  string                 `json:"proposal_id"`
	ActorID     string                 `json:"actor_id"`
	Kind        string                 `json:"kind"`
	Before      *ReplenishmentProposal `json:"before"`
	After       ReplenishmentProposal  `json:"after"`
}

func normalizeProposal(v ReplenishmentProposalInput) (ReplenishmentProposalInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || v.ProductID < 1 || v.ProductID > MaxVersion || v.ExpectedVersion < 1 || v.ExpectedVersion > MaxVersion || v.ReviewDays < 1 || v.ReviewDays > 365 || v.QuantityUnits < 1 || v.QuantityUnits > MaxStockUnits || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	return v, nil
}
func normalizeReplenishmentDecision(v ReplenishmentDecisionInput) (ReplenishmentDecisionInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || v.ExpectedRevision < 1 || v.ExpectedRevision >= MaxVersion || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	return v, nil
}
func proposalObject(content string, body []byte, fields map[string]bool) error {
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return ErrInput
	}
	f, e := batchObject(body, fields)
	if e != nil || len(f) != len(fields) {
		return ErrInput
	}
	return nil
}
func DecodeReplenishmentProposal(content string, body []byte) (ReplenishmentProposalInput, error) {
	var v ReplenishmentProposalInput
	if proposalObject(content, body, map[string]bool{"operation_id": true, "product_id": true, "expected_version": true, "review_days": true, "quantity_units": true, "reason": true}) != nil || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeProposal(v)
}
func DecodeReplenishmentDecision(content string, body []byte) (ReplenishmentDecisionInput, error) {
	var v ReplenishmentDecisionInput
	if proposalObject(content, body, map[string]bool{"operation_id": true, "expected_revision": true, "reason": true}) != nil || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeReplenishmentDecision(v)
}
func validProposal(p ReplenishmentProposal) bool {
	calc, e := replenish(p.Snapshot.StockPlan, p.Snapshot.ReviewDays)
	return e == nil && reflect.DeepEqual(calc, p.Snapshot) && p.Snapshot.State == "needs_replenishment" && ValidUUID(p.ID) && p.ProductID == p.Snapshot.Product.ID && p.Revision >= 1 && p.Revision <= MaxVersion && (p.State == "planned" || p.State == "approved" || p.State == "cancelled") && p.QuantityUnits >= 1 && p.QuantityUnits <= MaxStockUnits && ValidUUID(p.CreatorID) && ValidUUID(p.LastActorID) && clean(p.Reason, 500, true) && !p.CreatedAt.IsZero() && !p.UpdatedAt.Before(p.CreatedAt)
}
func validProposalReceipt(r ReplenishmentProposalReceipt) bool {
	if !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || r.ProposalID != r.After.ID || !validProposal(r.After) || r.After.LastActorID != r.ActorID {
		return false
	}
	if r.Kind == "create" {
		return r.Before == nil && r.After.ID == r.OperationID && r.After.Revision == 1 && r.After.State == "planned" && r.After.CreatorID == r.ActorID && r.After.CreatedAt.Equal(r.After.UpdatedAt)
	}
	if r.Before == nil || !validProposal(*r.Before) || r.Before.Revision >= MaxVersion {
		return false
	}
	want := *r.Before
	want.Revision++
	want.LastActorID = r.ActorID
	want.Reason = r.After.Reason
	want.UpdatedAt = r.After.UpdatedAt
	switch r.Kind {
	case "approve":
		if want.State != "planned" {
			return false
		}
		want.State = "approved"
	case "cancel":
		if want.State != "planned" && want.State != "approved" {
			return false
		}
		want.State = "cancelled"
	default:
		return false
	}
	return !want.UpdatedAt.Before(r.Before.UpdatedAt) && reflect.DeepEqual(want, r.After)
}

const proposalColumns = `proposal_id::text,product_id,revision,state,payload`
const proposalSQL = `SELECT ` + proposalColumns + ` FROM online_replenishment_proposals WHERE tenant_id=$1 AND proposal_id=$2`

func scanProposal(row scanner) (ReplenishmentProposal, error) {
	var p ReplenishmentProposal
	var id, state string
	var product, revision int64
	var b []byte
	e := row.Scan(&id, &product, &revision, &state, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &p) != nil || !validProposal(p) || id != p.ID || product != p.ProductID || revision != p.Revision || state != p.State {
		return ReplenishmentProposal{}, ErrUnavailable
	}
	return p, nil
}

const proposalReceiptSQL = `SELECT operation_id::text,proposal_id::text,actor_id::text,kind,payload_hash,receipt FROM online_replenishment_operations WHERE tenant_id=$1 AND operation_id=$2`

func scanProposalReceipt(row scanner) (ReplenishmentProposalReceipt, string, error) {
	var r ReplenishmentProposalReceipt
	var op, id, actor, kind, h string
	var b []byte
	e := row.Scan(&op, &id, &actor, &kind, &h, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &r) != nil || !validProposalReceipt(r) || op != r.OperationID || id != r.ProposalID || actor != r.ActorID || kind != r.Kind || len(h) != 64 {
		return r, "", ErrUnavailable
	}
	return r, h, nil
}
func proposalHash(id, kind string, input any) string {
	b, _ := json.Marshal(struct {
		ID, Kind string
		Input    any
	}{id, kind, input})
	return creationHash(b)
}
func proposalTransition(before ReplenishmentProposal, kind string, v ReplenishmentDecisionInput, actor string, at time.Time) (ReplenishmentProposal, error) {
	if before.Revision != v.ExpectedRevision || before.Revision >= MaxVersion {
		return ReplenishmentProposal{}, ErrConflict
	}
	after := before
	switch kind {
	case "approve":
		if before.State != "planned" {
			return after, ErrConflict
		}
		after.State = "approved"
	case "cancel":
		if before.State != "planned" && before.State != "approved" {
			return after, ErrConflict
		}
		after.State = "cancelled"
	default:
		return after, ErrInput
	}
	after.Revision++
	after.LastActorID = actor
	after.Reason = v.Reason
	after.UpdatedAt = at
	if !validProposal(after) || at.Before(before.UpdatedAt) {
		return after, ErrUnavailable
	}
	return after, nil
}
func (s Store) ReplenishmentProposal(ctx context.Context, a Actor, id string) (ReplenishmentProposal, error) {
	if !ValidUUID(id) {
		return ReplenishmentProposal{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return ReplenishmentProposal{}, e
	}
	defer tx.Rollback()
	p, e := scanProposal(tx.QueryRowContext(ctx, proposalSQL, a.Tenant, id))
	if e != nil {
		return p, e
	}
	if tx.Commit() != nil {
		return ReplenishmentProposal{}, ErrUnavailable
	}
	return p, nil
}
func (s Store) ReplenishmentProposalOperation(ctx context.Context, a Actor, op string) (ReplenishmentProposalReceipt, error) {
	if !ValidUUID(op) {
		return ReplenishmentProposalReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return ReplenishmentProposalReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanProposalReceipt(tx.QueryRowContext(ctx, proposalReceiptSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return ReplenishmentProposalReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) CreateReplenishmentProposal(ctx context.Context, a Actor, v ReplenishmentProposalInput) (ReplenishmentProposalReceipt, error) {
	v, e := normalizeProposal(v)
	if e != nil {
		return ReplenishmentProposalReceipt{}, e
	}
	return s.writeProposal(ctx, a, v.OperationID, "create", v.OperationID, proposalHash(v.OperationID, "create", v), &v, ReplenishmentDecisionInput{})
}
func (s Store) DecideReplenishmentProposal(ctx context.Context, a Actor, id, kind string, v ReplenishmentDecisionInput) (ReplenishmentProposalReceipt, error) {
	v, e := normalizeReplenishmentDecision(v)
	if e != nil || !ValidUUID(id) || (kind != "approve" && kind != "cancel") {
		return ReplenishmentProposalReceipt{}, ErrInput
	}
	return s.writeProposal(ctx, a, id, kind, v.OperationID, proposalHash(id, kind, v), nil, v)
}
func (s Store) writeProposal(ctx context.Context, a Actor, id, kind, op, h string, create *ReplenishmentProposalInput, v ReplenishmentDecisionInput) (ReplenishmentProposalReceipt, error) {
	var zero ReplenishmentProposalReceipt
	if !a.allowed("price") {
		return zero, ErrDenied
	}
	if s.DB == nil {
		return zero, ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return zero, ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return zero, e
	}
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":replenishment-proposal:"+op); e != nil {
		return zero, ErrUnavailable
	}
	prior, ph, e := scanProposalReceipt(tx.QueryRowContext(ctx, proposalReceiptSQL, a.Tenant, op))
	if e == nil {
		if prior.ActorID != a.User || prior.ProposalID != id || prior.Kind != kind || ph != h {
			return zero, ErrConflict
		}
		if tx.Commit() != nil {
			return zero, ErrUnavailable
		}
		return prior, nil
	}
	if e != ErrMissing {
		return zero, e
	}
	var before *ReplenishmentProposal
	var after ReplenishmentProposal
	if create != nil {
		plan, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL+` FOR UPDATE`, a.Tenant, create.ProductID))
		if e != nil {
			return zero, e
		}
		if plan.Product.Version != create.ExpectedVersion {
			return zero, ErrConflict
		}
		snapshot, e := replenish(plan, create.ReviewDays)
		if e != nil {
			return zero, e
		}
		if snapshot.State != "needs_replenishment" {
			return zero, ErrConflict
		}
		var at time.Time
		if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
			return zero, ErrUnavailable
		}
		at = at.UTC()
		after = ReplenishmentProposal{id, create.ProductID, 1, "planned", snapshot, create.QuantityUnits, a.User, a.User, create.Reason, at, at}
	} else {
		p, e := scanProposal(tx.QueryRowContext(ctx, proposalSQL+` FOR UPDATE`, a.Tenant, id))
		if e != nil {
			return zero, e
		}
		before = &p
		if kind == "approve" {
			plan, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL+` FOR UPDATE`, a.Tenant, p.ProductID))
			if e != nil {
				return zero, e
			}
			// Any changed stock, product revision or planning parameter invalidates approval.
			if !reflect.DeepEqual(plan, p.Snapshot.StockPlan) {
				return zero, ErrConflict
			}
		}
		var at time.Time
		if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
			return zero, ErrUnavailable
		}
		after, e = proposalTransition(p, kind, v, a.User, at.UTC())
		if e != nil {
			return zero, e
		}
	}
	r := ReplenishmentProposalReceipt{op, id, a.User, kind, before, after}
	if !validProposalReceipt(r) {
		return zero, ErrUnavailable
	}
	payload, _ := json.Marshal(after)
	var result sql.Result
	if create != nil {
		result, e = tx.ExecContext(ctx, `INSERT INTO online_replenishment_proposals(tenant_id,proposal_id,product_id,revision,state,payload) VALUES($1,$2,$3,$4,$5,$6)`, a.Tenant, id, after.ProductID, after.Revision, after.State, payload)
	} else {
		result, e = tx.ExecContext(ctx, `UPDATE online_replenishment_proposals SET revision=$3,state=$4,payload=$5 WHERE tenant_id=$1 AND proposal_id=$2 AND revision=$6`, a.Tenant, id, after.Revision, after.State, payload, before.Revision)
	}
	if e != nil {
		return zero, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	actual, e := scanProposal(tx.QueryRowContext(ctx, proposalSQL, a.Tenant, id))
	if e != nil || !reflect.DeepEqual(actual, after) {
		return zero, ErrUnavailable
	}
	receipt, _ := json.Marshal(r)
	result, e = tx.ExecContext(ctx, `INSERT INTO online_replenishment_operations(tenant_id,operation_id,proposal_id,actor_id,kind,payload_hash,receipt) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.Tenant, op, id, a.User, kind, h, receipt)
	if e != nil {
		return zero, conflict(e)
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	saved, sh, e := scanProposalReceipt(tx.QueryRowContext(ctx, proposalReceiptSQL, a.Tenant, op))
	if e != nil || sh != h || !reflect.DeepEqual(saved, r) {
		return zero, ErrUnavailable
	}
	result, e = tx.ExecContext(ctx, `INSERT INTO online_replenishment_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,$3,$4)`, a.Tenant, op, "replenishment.proposal."+kind, receipt)
	if e != nil {
		return zero, ErrUnavailable
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	var out []byte
	var event string
	if tx.QueryRowContext(ctx, `SELECT event_type,payload FROM online_replenishment_outbox WHERE tenant_id=$1 AND operation_id=$2`, a.Tenant, op).Scan(&event, &out) != nil {
		return zero, ErrUnavailable
	}
	var outReceipt ReplenishmentProposalReceipt
	if event != "replenishment.proposal."+kind || json.Unmarshal(out, &outReceipt) != nil || !reflect.DeepEqual(outReceipt, r) {
		return zero, ErrUnavailable
	}
	if tx.Commit() != nil {
		return zero, ErrUnavailable
	}
	return r, nil
}
