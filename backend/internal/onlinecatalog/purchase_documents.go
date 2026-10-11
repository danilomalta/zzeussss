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

// PurchaseFile is a stable internal document, never supplier acceptance or a signed invoice.
type PurchaseFile struct {
	FormatVersion int           `json:"format_version"`
	Kind          string        `json:"kind"`
	DocumentID    string        `json:"document_id"`
	Purchase      PurchaseDraft `json:"purchase"`
	PreparedBy    string        `json:"prepared_by"`
	PreparedAt    time.Time     `json:"prepared_at"`
}
type PurchaseDocument struct {
	ID          string       `json:"id"`
	PurchaseID  string       `json:"purchase_id"`
	Revision    int64        `json:"revision"`
	State       string       `json:"state"`
	File        PurchaseFile `json:"file"`
	SHA256      string       `json:"sha256"`
	CreatorID   string       `json:"creator_id"`
	LastActorID string       `json:"last_actor_id"`
	Reason      string       `json:"reason"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}
type PurchaseDocumentInput struct {
	OperationID              string `json:"operation_id"`
	PurchaseID               string `json:"purchase_id"`
	ExpectedPurchaseRevision int64  `json:"expected_purchase_revision"`
	Reason                   string `json:"reason"`
}
type PurchaseDocumentReceipt struct {
	OperationID string            `json:"operation_id"`
	DocumentID  string            `json:"document_id"`
	ActorID     string            `json:"actor_id"`
	Kind        string            `json:"kind"`
	Before      *PurchaseDocument `json:"before"`
	After       PurchaseDocument  `json:"after"`
}

func normalizePurchaseDocument(v PurchaseDocumentInput) (PurchaseDocumentInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || !ValidUUID(v.PurchaseID) || v.ExpectedPurchaseRevision < 1 || v.ExpectedPurchaseRevision > MaxVersion || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	return v, nil
}
func DecodePurchaseDocument(content string, body []byte) (PurchaseDocumentInput, error) {
	var v PurchaseDocumentInput
	if proposalObject(content, body, map[string]bool{"operation_id": true, "purchase_id": true, "expected_purchase_revision": true, "reason": true}) != nil || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizePurchaseDocument(v)
}
func purchaseFileBytes(v PurchaseFile) []byte { b, _ := json.Marshal(v); return b }
func validPurchaseDocument(p PurchaseDocument) bool {
	return ValidUUID(p.ID) && p.ID == p.File.DocumentID && p.PurchaseID == p.File.Purchase.ID && validPurchaseDraft(p.File.Purchase) && p.File.Purchase.State == "approved" && p.File.FormatVersion == 1 && p.File.Kind == "internal_purchase" && p.File.PreparedBy == p.CreatorID && p.File.PreparedAt.Equal(p.CreatedAt) && p.SHA256 == creationHash(purchaseFileBytes(p.File)) && ((p.State == "prepared" && p.Revision == 1) || (p.State == "cancelled" && p.Revision == 2)) && ValidUUID(p.CreatorID) && ValidUUID(p.LastActorID) && clean(p.Reason, 500, true) && !p.CreatedAt.IsZero() && !p.UpdatedAt.Before(p.CreatedAt)
}
func cancelPurchaseDocument(p PurchaseDocument, v ReplenishmentDecisionInput, actor string, at time.Time) (PurchaseDocument, error) {
	if p.State != "prepared" || p.Revision != v.ExpectedRevision {
		return PurchaseDocument{}, ErrConflict
	}
	p.State = "cancelled"
	p.Revision++
	p.Reason = v.Reason
	p.LastActorID = actor
	p.UpdatedAt = at
	if !validPurchaseDocument(p) {
		return PurchaseDocument{}, ErrUnavailable
	}
	return p, nil
}
func validPurchaseDocumentReceipt(r PurchaseDocumentReceipt) bool {
	if !ValidUUID(r.OperationID) || !ValidUUID(r.ActorID) || r.DocumentID != r.After.ID || r.ActorID != r.After.LastActorID || !validPurchaseDocument(r.After) {
		return false
	}
	if r.Kind == "prepare" {
		return r.Before == nil && r.After.ID == r.OperationID && r.After.State == "prepared" && r.After.CreatorID == r.ActorID && r.After.CreatedAt.Equal(r.After.UpdatedAt)
	}
	if r.Kind != "cancel" || r.Before == nil || !validPurchaseDocument(*r.Before) {
		return false
	}
	p, e := cancelPurchaseDocument(*r.Before, ReplenishmentDecisionInput{OperationID: r.OperationID, ExpectedRevision: r.Before.Revision, Reason: r.After.Reason}, r.ActorID, r.After.UpdatedAt)
	return e == nil && reflect.DeepEqual(p, r.After)
}

const purchaseDocumentColumns = `document_id::text,purchase_id::text,revision,state,payload`
const purchaseDocumentSQL = `SELECT ` + purchaseDocumentColumns + ` FROM online_purchase_documents WHERE tenant_id=$1 AND document_id=$2`
const purchaseDocumentReceiptSQL = `SELECT operation_id::text,document_id::text,actor_id::text,kind,payload_hash,receipt FROM online_purchase_document_operations WHERE tenant_id=$1 AND operation_id=$2`

func scanPurchaseDocument(row scanner) (PurchaseDocument, error) {
	var p PurchaseDocument
	var id, purchase, state string
	var revision int64
	var b []byte
	e := row.Scan(&id, &purchase, &revision, &state, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &p) != nil || !validPurchaseDocument(p) || id != p.ID || purchase != p.PurchaseID || revision != p.Revision || state != p.State {
		return PurchaseDocument{}, ErrUnavailable
	}
	return p, nil
}
func scanPurchaseDocumentReceipt(row scanner) (PurchaseDocumentReceipt, string, error) {
	var r PurchaseDocumentReceipt
	var op, id, actor, kind, h string
	var b []byte
	e := row.Scan(&op, &id, &actor, &kind, &h, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return r, h, ErrMissing
	}
	if e != nil || json.Unmarshal(b, &r) != nil || !validPurchaseDocumentReceipt(r) || op != r.OperationID || id != r.DocumentID || actor != r.ActorID || kind != r.Kind || len(h) != 64 {
		return r, "", ErrUnavailable
	}
	return r, h, nil
}
func (s Store) PurchaseDocument(ctx context.Context, a Actor, id string) (PurchaseDocument, error) {
	if !ValidUUID(id) {
		return PurchaseDocument{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return PurchaseDocument{}, e
	}
	defer tx.Rollback()
	p, e := scanPurchaseDocument(tx.QueryRowContext(ctx, purchaseDocumentSQL, a.Tenant, id))
	if e != nil {
		return p, e
	}
	if tx.Commit() != nil {
		return PurchaseDocument{}, ErrUnavailable
	}
	return p, nil
}
func (s Store) PurchaseDocumentOperation(ctx context.Context, a Actor, op string) (PurchaseDocumentReceipt, error) {
	if !ValidUUID(op) {
		return PurchaseDocumentReceipt{}, ErrInput
	}
	tx, e := s.discountReadTx(ctx, a)
	if e != nil {
		return PurchaseDocumentReceipt{}, e
	}
	defer tx.Rollback()
	r, _, e := scanPurchaseDocumentReceipt(tx.QueryRowContext(ctx, purchaseDocumentReceiptSQL+` AND actor_id=$3`, a.Tenant, op, a.User))
	if e != nil {
		return r, e
	}
	if tx.Commit() != nil {
		return PurchaseDocumentReceipt{}, ErrUnavailable
	}
	return r, nil
}

// Downloads retain historical bytes after parent changes; cancellation blocks active download.
func PurchaseDocumentJSON(p PurchaseDocument) (string, error) {
	if !validPurchaseDocument(p) {
		return "", ErrUnavailable
	}
	if p.State != "prepared" {
		return "", ErrConflict
	}
	b := purchaseFileBytes(p.File)
	if len(b) > 65536 {
		return "", ErrUnavailable
	}
	return string(b), nil
}
func (s Store) PreparePurchaseDocument(ctx context.Context, a Actor, v PurchaseDocumentInput) (PurchaseDocumentReceipt, error) {
	v, e := normalizePurchaseDocument(v)
	if e != nil {
		return PurchaseDocumentReceipt{}, e
	}
	return s.writePurchaseDocument(ctx, a, v.OperationID, "prepare", v.OperationID, proposalHash(v.OperationID, "prepare", v), &v, ReplenishmentDecisionInput{})
}
func (s Store) CancelPurchaseDocument(ctx context.Context, a Actor, id string, v ReplenishmentDecisionInput) (PurchaseDocumentReceipt, error) {
	v, e := normalizeReplenishmentDecision(v)
	if e != nil || !ValidUUID(id) {
		return PurchaseDocumentReceipt{}, ErrInput
	}
	return s.writePurchaseDocument(ctx, a, id, "cancel", v.OperationID, proposalHash(id, "cancel", v), nil, v)
}
func (s Store) writePurchaseDocument(ctx context.Context, a Actor, id, kind, op, h string, create *PurchaseDocumentInput, v ReplenishmentDecisionInput) (PurchaseDocumentReceipt, error) {
	var zero PurchaseDocumentReceipt
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
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, a.Tenant+":purchase-document:"+op); e != nil {
		return zero, ErrUnavailable
	}
	prior, ph, e := scanPurchaseDocumentReceipt(tx.QueryRowContext(ctx, purchaseDocumentReceiptSQL, a.Tenant, op))
	if e == nil {
		if prior.ActorID != a.User || prior.DocumentID != id || prior.Kind != kind || ph != h {
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
	var before *PurchaseDocument
	var after PurchaseDocument
	if create != nil {
		p, e := scanPurchaseDraft(tx.QueryRowContext(ctx, purchaseSQL+` FOR SHARE`, a.Tenant, create.PurchaseID))
		if e != nil {
			return zero, e
		}
		if p.State != "approved" || p.Revision != create.ExpectedPurchaseRevision {
			return zero, ErrConflict
		}
		proposal, e := scanProposal(tx.QueryRowContext(ctx, proposalSQL+` FOR SHARE`, a.Tenant, p.ProposalID))
		if e != nil {
			return zero, e
		}
		offer, e := scanSupplierOffer(tx.QueryRowContext(ctx, supplierOfferSQL+` FOR SHARE`, a.Tenant, p.Offer.ID))
		if e != nil {
			return zero, e
		}
		supplier, e := scanSupplier(tx.QueryRowContext(ctx, supplierSQL+` FOR SHARE`, a.Tenant, p.Supplier.ID))
		if e != nil {
			return zero, e
		}
		product, e := scanStockPlan(tx.QueryRowContext(ctx, stockPlanSQL+` FOR SHARE`, a.Tenant, p.Proposal.ProductID))
		if e != nil {
			return zero, e
		}
		c := checkPurchaseSnapshot(p, &proposal, &offer, &supplier, &product, time.Time{})
		if !c.SnapshotMatches {
			return zero, ErrConflict
		}
		var at time.Time
		if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
			return zero, ErrUnavailable
		}
		at = at.UTC()
		file := PurchaseFile{1, "internal_purchase", id, p, a.User, at}
		after = PurchaseDocument{ID: id, PurchaseID: p.ID, Revision: 1, State: "prepared", File: file, SHA256: creationHash(purchaseFileBytes(file)), CreatorID: a.User, LastActorID: a.User, Reason: create.Reason, CreatedAt: at, UpdatedAt: at}
	} else {
		p, e := scanPurchaseDocument(tx.QueryRowContext(ctx, purchaseDocumentSQL+` FOR UPDATE`, a.Tenant, id))
		if e != nil {
			return zero, e
		}
		before = &p
		if p.State != "prepared" || p.Revision != v.ExpectedRevision {
			return zero, ErrConflict
		}
		var at time.Time
		if tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at) != nil {
			return zero, ErrUnavailable
		}
		after, e = cancelPurchaseDocument(p, v, a.User, at.UTC())
		if e != nil {
			return zero, e
		}
	}
	r := PurchaseDocumentReceipt{op, id, a.User, kind, before, after}
	if !validPurchaseDocumentReceipt(r) {
		return zero, ErrUnavailable
	}
	payload, _ := json.Marshal(after)
	var result sql.Result
	if create != nil {
		result, e = tx.ExecContext(ctx, `INSERT INTO online_purchase_documents(tenant_id,document_id,purchase_id,revision,state,payload) VALUES($1,$2,$3,$4,$5,$6)`, a.Tenant, id, after.PurchaseID, after.Revision, after.State, payload)
	} else {
		result, e = tx.ExecContext(ctx, `UPDATE online_purchase_documents SET revision=$3,state=$4,payload=$5 WHERE tenant_id=$1 AND document_id=$2 AND revision=$6`, a.Tenant, id, after.Revision, after.State, payload, before.Revision)
	}
	if e != nil {
		return zero, conflict(e)
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	actual, e := scanPurchaseDocument(tx.QueryRowContext(ctx, purchaseDocumentSQL, a.Tenant, id))
	if e != nil || !reflect.DeepEqual(actual, after) {
		return zero, ErrUnavailable
	}
	receipt, _ := json.Marshal(r)
	result, e = tx.ExecContext(ctx, `INSERT INTO online_purchase_document_operations(tenant_id,operation_id,document_id,actor_id,kind,payload_hash,receipt) VALUES($1,$2,$3,$4,$5,$6,$7)`, a.Tenant, op, id, a.User, kind, h, receipt)
	if e != nil {
		return zero, conflict(e)
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	saved, sh, e := scanPurchaseDocumentReceipt(tx.QueryRowContext(ctx, purchaseDocumentReceiptSQL, a.Tenant, op))
	if e != nil || sh != h || !reflect.DeepEqual(saved, r) {
		return zero, ErrUnavailable
	}
	result, e = tx.ExecContext(ctx, `INSERT INTO online_purchase_document_outbox(tenant_id,operation_id,event_type,payload) VALUES($1,$2,$3,$4)`, a.Tenant, op, "purchase.document."+kind, receipt)
	if e != nil {
		return zero, ErrUnavailable
	}
	n, e = result.RowsAffected()
	if e != nil || n != 1 {
		return zero, ErrUnavailable
	}
	var out []byte
	var event string
	if tx.QueryRowContext(ctx, `SELECT event_type,payload FROM online_purchase_document_outbox WHERE tenant_id=$1 AND operation_id=$2`, a.Tenant, op).Scan(&event, &out) != nil {
		return zero, ErrUnavailable
	}
	var got PurchaseDocumentReceipt
	if event != "purchase.document."+kind || json.Unmarshal(out, &got) != nil || !reflect.DeepEqual(got, r) {
		return zero, ErrUnavailable
	}
	if tx.Commit() != nil {
		return zero, ErrUnavailable
	}
	return r, nil
}
