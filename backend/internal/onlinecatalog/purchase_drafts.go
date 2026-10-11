package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
)

// PurchaseDraft is an internal purchasing decision; no external supplier acceptance.
type PurchaseDraft struct {
	ID            string                `json:"id"`
	ProposalID    string                `json:"proposal_id"`
	Revision      int64                 `json:"revision"`
	State         string                `json:"state"`
	Proposal      ReplenishmentProposal `json:"proposal"`
	Offer         SupplierOffer         `json:"offer"`
	Supplier      Supplier              `json:"supplier"`
	QuantityUnits int64                 `json:"quantity_units"`
	TotalCents    int64                 `json:"total_cents"`
	Source        string                `json:"source"`
	CreatorID     string                `json:"creator_id"`
	LastActorID   string                `json:"last_actor_id"`
	Reason        string                `json:"reason"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}
type PurchaseDraftInput struct {
	OperationID              string `json:"operation_id"`
	ProposalID               string `json:"proposal_id"`
	OfferID                  string `json:"offer_id"`
	ExpectedProposalRevision int64  `json:"expected_proposal_revision"`
	ExpectedOfferVersion     int64  `json:"expected_offer_version"`
	Reason                   string `json:"reason"`
}
type PurchaseDraftReceipt struct {
	OperationID string         `json:"operation_id"`
	PurchaseID  string         `json:"purchase_id"`
	ActorID     string         `json:"actor_id"`
	Kind        string         `json:"kind"`
	Before      *PurchaseDraft `json:"before"`
	After       PurchaseDraft  `json:"after"`
}

func normalizePurchaseDraft(v PurchaseDraftInput) (PurchaseDraftInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || !ValidUUID(v.ProposalID) || !ValidUUID(v.OfferID) || v.ExpectedProposalRevision < 1 || v.ExpectedProposalRevision > MaxVersion || v.ExpectedOfferVersion < 1 || v.ExpectedOfferVersion > MaxVersion || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	return v, nil
}
func DecodePurchaseDraft(content string, body []byte) (PurchaseDraftInput, error) {
	var v PurchaseDraftInput
	if proposalObject(content, body, map[string]bool{"operation_id": true, "proposal_id": true, "offer_id": true, "expected_proposal_revision": true, "expected_offer_version": true, "reason": true}) != nil || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizePurchaseDraft(v)
}
func purchaseTotal(quantity int64, offer SupplierOffer) (int64, error) {
	if quantity < 1 || quantity > MaxStockUnits || quantity < offer.MinimumUnits || offer.UnitPriceCents < 0 || offer.UnitPriceCents > MaxPrice {
		return 0, ErrConflict
	}
	if offer.UnitPriceCents > 0 && quantity > MaxVersion/offer.UnitPriceCents {
		return 0, ErrInput
	}
	return quantity * offer.UnitPriceCents, nil
}
func validPurchaseDraft(p PurchaseDraft) bool {
	total, e := purchaseTotal(p.QuantityUnits, p.Offer)
	return e == nil && ValidUUID(p.ID) && p.ProposalID == p.Proposal.ID && validProposal(p.Proposal) && p.Proposal.State == "approved" && validSupplierOffer(p.Offer) && p.Offer.Active && validSupplier(p.Supplier) && p.Supplier.Active && p.Offer.SupplierID == p.Supplier.ID && p.Offer.ProductID == p.Proposal.ProductID && p.QuantityUnits == p.Proposal.QuantityUnits && p.TotalCents == total && p.Source == "internal_declaration" && p.Revision >= 1 && p.Revision <= MaxVersion && (p.State == "draft" || p.State == "approved" || p.State == "cancelled") && ValidUUID(p.CreatorID) && ValidUUID(p.LastActorID) && clean(p.Reason, 500, true) && !p.CreatedAt.IsZero() && !p.UpdatedAt.Before(p.CreatedAt)
}
func validPurchaseReceipt(r PurchaseDraftReceipt) bool {
	if !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || !validPurchaseDraft(r.After) || r.PurchaseID != r.After.ID || r.ActorID != r.After.LastActorID {
		return false
	}
	if r.Kind == "create" {
		return r.Before == nil && r.After.ID == r.OperationID && r.After.Revision == 1 && r.After.State == "draft" && r.After.CreatorID == r.ActorID && r.After.CreatedAt.Equal(r.After.UpdatedAt)
	}
	if r.Before == nil || !validPurchaseDraft(*r.Before) {
		return false
	}
	v := ReplenishmentDecisionInput{OperationID: r.OperationID, ExpectedRevision: r.Before.Revision, Reason: r.After.Reason}
	want, e := purchaseTransition(*r.Before, r.Kind, v, r.ActorID, r.After.UpdatedAt)
	return e == nil && reflect.DeepEqual(want, r.After)
}
func purchaseTransition(before PurchaseDraft, kind string, v ReplenishmentDecisionInput, actor string, at time.Time) (PurchaseDraft, error) {
	if before.Revision != v.ExpectedRevision || before.Revision >= MaxVersion {
		return PurchaseDraft{}, ErrConflict
	}
	after := before
	switch kind {
	case "approve":
		if before.State != "draft" {
			return after, ErrConflict
		}
		after.State = "approved"
	case "cancel":
		if before.State != "draft" && before.State != "approved" {
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
	if !validPurchaseDraft(after) || at.Before(before.UpdatedAt) {
		return after, ErrUnavailable
	}
	return after, nil
}

const purchaseColumns = `purchase_id::text,proposal_id::text,offer_id::text,revision,state,payload`
const purchaseSQL = `SELECT ` + purchaseColumns + ` FROM online_purchase_drafts WHERE tenant_id=$1 AND purchase_id=$2`

func scanPurchaseDraft(row scanner) (PurchaseDraft, error) {
	var p PurchaseDraft
	var id, proposal, offer, state string
	var revision int64
	var b []byte
	e := row.Scan(&id, &proposal, &offer, &revision, &state, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &p) != nil || !validPurchaseDraft(p) || id != p.ID || proposal != p.ProposalID || offer != p.Offer.ID || revision != p.Revision || state != p.State {
		return PurchaseDraft{}, ErrUnavailable
	}
	return p, nil
}

const purchaseReceiptSQL = `SELECT operation_id::text,purchase_id::text,actor_id::text,kind,payload_hash,receipt FROM online_purchase_operations WHERE tenant_id=$1 AND operation_id=$2`

func scanPurchaseReceipt(row scanner) (PurchaseDraftReceipt, string, error) {
	var r PurchaseDraftReceipt
	var op, id, actor, kind, h string
	var b []byte
	e := row.Scan(&op, &id, &actor, &kind, &h, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &r) != nil || !validPurchaseReceipt(r) || op != r.OperationID || id != r.PurchaseID || actor != r.ActorID || kind != r.Kind || len(h) != 64 {
		return r, "", ErrUnavailable
	}
	return r, h, nil
}
func (s Store) PurchaseDraft(ctx context.Context, a Actor, id string) (PurchaseDraft, error) {
	if !ValidUUID(id) {
		return PurchaseDraft{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return PurchaseDraft{}, e
	}
	defer tx.Rollback()
	p, e := scanPurchaseDraft(tx.QueryRowContext(ctx, purchaseSQL, a.Tenant, id))
	if e != nil {
		return p, e
	}
	if tx.Commit() != nil {
		return PurchaseDraft{}, ErrUnavailable
	}
	return p, nil
}
func (s Store) PurchaseOperation(ctx context.Context, a Actor, op string) (PurchaseDraftReceipt, error) {
	if !ValidUUID(op) {
		return PurchaseDraftReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return PurchaseDraftReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanPurchaseReceipt(tx.QueryRowContext(ctx, purchaseReceiptSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return PurchaseDraftReceipt{}, ErrUnavailable
	}
	return r, nil
}
func (s Store) CreatePurchaseDraft(ctx context.Context, a Actor, v PurchaseDraftInput) (PurchaseDraftReceipt, error) {
	v, e := normalizePurchaseDraft(v)
	if e != nil {
		return PurchaseDraftReceipt{}, e
	}
	return s.writePurchaseDraft(ctx, a, v.OperationID, "create", v.OperationID, proposalHash(v.OperationID, "create", v), &v, ReplenishmentDecisionInput{})
}
func (s Store) DecidePurchaseDraft(ctx context.Context, a Actor, id, kind string, v ReplenishmentDecisionInput) (PurchaseDraftReceipt, error) {
	v, e := normalizeReplenishmentDecision(v)
	if e != nil || !ValidUUID(id) || (kind != "approve" && kind != "cancel") {
		return PurchaseDraftReceipt{}, ErrInput
	}
	return s.writePurchaseDraft(ctx, a, id, kind, v.OperationID, proposalHash(id, kind, v), nil, v)
}
func (s Store) writePurchaseDraft(ctx context.Context, a Actor, id, kind, op, h string, create *PurchaseDraftInput, v ReplenishmentDecisionInput) (PurchaseDraftReceipt, error) {
	var zero PurchaseDraftReceipt
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
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":purchase-draft:"+op); e != nil {
		return zero, ErrUnavailable
	}
	prior, ph, e := scanPurchaseReceipt(tx.QueryRowContext(ctx, purchaseReceiptSQL, a.Tenant, op))
	if e == nil {
		if prior.ActorID != a.User || prior.PurchaseID != id || prior.Kind != kind || ph != h {
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
	var before *PurchaseDraft
	var after PurchaseDraft
	if create != nil {
		proposal, err := scanProposal(tx.QueryRowContext(ctx, proposalSQL+` FOR SHARE`, a.Tenant, create.ProposalID))
		if err != nil {
			return zero, err
		}
		offer, err := scanSupplierOffer(tx.QueryRowContext(ctx, supplierOfferSQL+` FOR SHARE`, a.Tenant, create.OfferID))
		if err != nil {
			return zero, err
		}
		if proposal.Revision != create.ExpectedProposalRevision || offer.Version != create.ExpectedOfferVersion || proposal.State != "approved" || !offer.Active || proposal.ProductID != offer.ProductID {
			return zero, ErrConflict
		}
		supplier, err := scanSupplier(tx.QueryRowContext(ctx, supplierSQL+` FOR SHARE`, a.Tenant, offer.SupplierID))
		if err != nil {
			return zero, err
		}
		if !supplier.Active {
			return zero, ErrConflict
		}
		plan, err := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL+` FOR SHARE`, a.Tenant, proposal.ProductID))
		if err != nil {
			return zero, err
		}
		if !reflect.DeepEqual(plan, proposal.Snapshot.StockPlan) {
			return zero, ErrConflict
		}
		total, err := purchaseTotal(proposal.QuantityUnits, offer)
		if err != nil {
			return zero, err
		}
		var at time.Time
		if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
			return zero, ErrUnavailable
		}
		at = at.UTC()
		after = PurchaseDraft{ID: id, ProposalID: proposal.ID, Revision: 1, State: "draft", Proposal: proposal, Offer: offer, Supplier: supplier, QuantityUnits: proposal.QuantityUnits, TotalCents: total, Source: "internal_declaration", CreatorID: a.User, LastActorID: a.User, Reason: create.Reason, CreatedAt: at, UpdatedAt: at}
	} else {
		p, err := scanPurchaseDraft(tx.QueryRowContext(ctx, purchaseSQL+` FOR UPDATE`, a.Tenant, id))
		if err != nil {
			return zero, err
		}
		before = &p
		if p.Revision != v.ExpectedRevision || p.Revision >= MaxVersion {
			return zero, ErrConflict
		}
		if kind == "approve" {
			proposal, err := scanProposal(tx.QueryRowContext(ctx, proposalSQL+` FOR SHARE`, a.Tenant, p.ProposalID))
			if err != nil {
				return zero, err
			}
			if !reflect.DeepEqual(proposal, p.Proposal) {
				return zero, ErrConflict
			}
			offer, err := scanSupplierOffer(tx.QueryRowContext(ctx, supplierOfferSQL+` FOR SHARE`, a.Tenant, p.Offer.ID))
			if err != nil {
				return zero, err
			}
			if !reflect.DeepEqual(offer, p.Offer) {
				return zero, ErrConflict
			}
			supplier, err := scanSupplier(tx.QueryRowContext(ctx, supplierSQL+` FOR SHARE`, a.Tenant, p.Supplier.ID))
			if err != nil {
				return zero, err
			}
			if !reflect.DeepEqual(supplier, p.Supplier) {
				return zero, ErrConflict
			}
			plan, err := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL+` FOR SHARE`, a.Tenant, p.Proposal.ProductID))
			if err != nil {
				return zero, err
			}
			if !reflect.DeepEqual(plan, p.Proposal.Snapshot.StockPlan) {
				return zero, ErrConflict
			}
		}
		var at time.Time
		if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
			return zero, ErrUnavailable
		}
		after, e = purchaseTransition(p, kind, v, a.User, at.UTC())
		if e != nil {
			return zero, e
		}
	}

	r := PurchaseDraftReceipt{op, id, a.User, kind, before, after}
	if !validPurchaseReceipt(r) {
		return zero, ErrUnavailable
	}
	payload, _ := json.Marshal(after)
	var result sql.Result
	if create != nil {
		result, e = tx.ExecContext(ctx, `INSERT INTO online_purchase_drafts(tenant_id,purchase_id,proposal_id,offer_id,revision,state,payload) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.Tenant, id, after.ProposalID, after.Offer.ID, after.Revision, after.State, payload)
	} else {
		result, e = tx.ExecContext(ctx, `UPDATE online_purchase_drafts SET revision=$3,state=$4,payload=$5 WHERE tenant_id=$1 AND purchase_id=$2 AND revision=$6`, a.Tenant, id, after.Revision, after.State, payload, before.Revision)
	}
	if e != nil {
		return zero, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	actual, e := scanPurchaseDraft(tx.QueryRowContext(ctx, purchaseSQL, a.Tenant, id))
	if e != nil || !reflect.DeepEqual(actual, after) {
		return zero, ErrUnavailable
	}
	receipt, _ := json.Marshal(r)
	result, e = tx.ExecContext(ctx, `INSERT INTO online_purchase_operations(tenant_id,operation_id,purchase_id,actor_id,kind,payload_hash,receipt) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.Tenant, op, id, a.User, kind, h, receipt)
	if e != nil {
		return zero, conflict(e)
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	saved, sh, e := scanPurchaseReceipt(tx.QueryRowContext(ctx, purchaseReceiptSQL, a.Tenant, op))
	if e != nil || sh != h || !reflect.DeepEqual(saved, r) {
		return zero, ErrUnavailable
	}
	result, e = tx.ExecContext(ctx, `INSERT INTO online_purchase_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,$3,$4)`, a.Tenant, op, "purchase.internal."+kind, receipt)
	if e != nil {
		return zero, ErrUnavailable
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	var out []byte
	var event string
	if tx.QueryRowContext(ctx, `SELECT event_type,payload FROM online_purchase_outbox WHERE tenant_id=$1 AND operation_id=$2`, a.Tenant, op).Scan(&event, &out) != nil {
		return zero, ErrUnavailable
	}
	var outReceipt PurchaseDraftReceipt
	if event != "purchase.internal."+kind || json.Unmarshal(out, &outReceipt) != nil || !reflect.DeepEqual(outReceipt, r) {
		return zero, ErrUnavailable
	}
	if tx.Commit() != nil {
		return zero, ErrUnavailable
	}
	return r, nil
}
