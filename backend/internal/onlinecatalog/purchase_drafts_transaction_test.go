package onlinecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"testing"
	"time"
)

func purchaseFixture49() (PurchaseDraftInput, PurchaseDraftReceipt) {
	_, pr := proposalFixture45()
	pr.After.State = "approved"
	pr.After.Revision = 2
	_, or := offerFixture48()
	_, sr := supplierFixture47()
	at := pr.After.CreatedAt.Add(time.Hour)
	v := PurchaseDraftInput{OperationID: testOp, ProposalID: pr.After.ID, OfferID: or.After.ID, ExpectedProposalRevision: 2, ExpectedOfferVersion: 1, Reason: "Compra interna"}
	total, _ := purchaseTotal(pr.After.QuantityUnits, or.After)
	p := PurchaseDraft{ID: testOp, ProposalID: v.ProposalID, Revision: 1, State: "draft", Proposal: pr.After, Offer: or.After, Supplier: sr.After, QuantityUnits: pr.After.QuantityUnits, TotalCents: total, Source: "internal_declaration", CreatorID: fixtureActor.User, LastActorID: fixtureActor.User, Reason: v.Reason, CreatedAt: at, UpdatedAt: at}
	return v, PurchaseDraftReceipt{OperationID: testOp, PurchaseID: testOp, ActorID: fixtureActor.User, Kind: "create", After: p}
}
func purchaseRows49(p PurchaseDraft) *sqlmock.Rows {
	b, _ := json.Marshal(p)
	return sqlmock.NewRows([]string{"id", "proposal", "offer", "revision", "state", "payload"}).AddRow(p.ID, p.ProposalID, p.Offer.ID, p.Revision, p.State, b)
}
func purchaseReceiptRows49(r PurchaseDraftReceipt, h string, empty bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"op", "id", "actor", "kind", "hash", "receipt"})
	if !empty {
		b, _ := json.Marshal(r)
		rows.AddRow(r.OperationID, r.PurchaseID, r.ActorID, r.Kind, h, b)
	}
	return rows
}
func purchaseParents49(m sqlmock.Sqlmock, p PurchaseDraft) {
	m.ExpectQuery("SELECT proposal_id::text.*FOR SHARE").WillReturnRows(proposalRows45(p.Proposal))
	m.ExpectQuery("SELECT offer_id::text.*FOR SHARE").WillReturnRows(offerRows48(p.Offer))
	m.ExpectQuery("SELECT supplier_id::text.*FOR SHARE").WillReturnRows(supplierRows47(p.Supplier))
	m.ExpectQuery("SELECT id,nome.*FOR SHARE").WillReturnRows(planRows43(p.Proposal.Snapshot.StockPlan))
}
func TestPurchaseDraftSaveReadbackAtomicOutbox(t *testing.T) {
	for _, outcome := range []string{"success", "outbox-failure", "ignored-insert", "forged-readback", "ignored-audit", "forged-outbox"} {
		t.Run(outcome, func(t *testing.T) {
			s, m := qualityFixture(t)
			v, r := purchaseFixture49()
			h := proposalHash(v.OperationID, "create", v)
			qualityLive(m)
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT operation_id::text").WillReturnRows(purchaseReceiptRows49(r, h, true))
			purchaseParents49(m, r.After)
			m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.CreatedAt))
			count := int64(1)
			if outcome == "ignored-insert" {
				count = 0
			}
			m.ExpectExec("INSERT INTO online_purchase_drafts").WillReturnResult(sqlmock.NewResult(0, count))
			if outcome != "ignored-insert" {
				actual := r.After
				if outcome == "forged-readback" {
					actual.QuantityUnits++
				}
				m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(purchaseRows49(actual))
				if outcome != "forged-readback" {
					count = 1
					if outcome == "ignored-audit" {
						count = 0
					}
					m.ExpectExec("INSERT INTO online_purchase_operations").WillReturnResult(sqlmock.NewResult(0, count))
					if outcome != "ignored-audit" {
						m.ExpectQuery("SELECT operation_id::text").WillReturnRows(purchaseReceiptRows49(r, h, false))
						if outcome == "outbox-failure" {
							m.ExpectExec("INSERT INTO online_purchase_outbox").WillReturnError(errors.New("private details"))
						} else {
							m.ExpectExec("INSERT INTO online_purchase_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
							out := r
							if outcome == "forged-outbox" {
								out.After.QuantityUnits++
							}
							b, _ := json.Marshal(out)
							m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("purchase.internal.create", b))
						}
					}
				}
			}
			if outcome == "success" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.CreatePurchaseDraft(context.Background(), fixtureActor, v)
			if outcome == "success" && (e != nil || !reflect.DeepEqual(got, r)) || outcome != "success" && e != ErrUnavailable {
				t.Fatal(got, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestPurchaseDraftReplayAndScopedConflict(t *testing.T) {
	for _, outcome := range []string{"replay", "changed-input", "other-actor"} {
		s, m := qualityFixture(t)
		v, r := purchaseFixture49()
		h := proposalHash(v.OperationID, "create", v)
		if outcome == "changed-input" {
			v.Reason += "changed"
		}
		if outcome == "other-actor" {
			r.ActorID = "11111111-1111-4111-8111-111111111115"
			r.After.CreatorID = r.ActorID
			r.After.LastActorID = r.ActorID
		}
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(purchaseReceiptRows49(r, h, false))
		if outcome == "replay" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		_, e := s.CreatePurchaseDraft(context.Background(), fixtureActor, v)
		if outcome == "replay" && e != nil || outcome != "replay" && e != ErrConflict {
			t.Fatal(outcome, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestPurchaseRejectsChangedOfferDuringApproval(t *testing.T) {
	s, m := qualityFixture(t)
	_, r := purchaseFixture49()
	v := ReplenishmentDecisionInput{OperationID: "11111111-1111-4111-8111-111111111115", ExpectedRevision: 1, Reason: "Aprovar"}
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(purchaseReceiptRows49(r, "", true))
	m.ExpectQuery("SELECT purchase_id::text.*FOR UPDATE").WillReturnRows(purchaseRows49(r.After))
	m.ExpectQuery("SELECT proposal_id::text.*FOR SHARE").WillReturnRows(proposalRows45(r.After.Proposal))
	changed := r.After.Offer
	changed.UnitPriceCents++
	m.ExpectQuery("SELECT offer_id::text.*FOR SHARE").WillReturnRows(offerRows48(changed))
	m.ExpectRollback()
	if _, e := s.DecidePurchaseDraft(context.Background(), fixtureActor, r.PurchaseID, "approve", v); e != ErrConflict {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestPurchaseCancellationPublishesImmutableSnapshots(t *testing.T) {
	s, m := qualityFixture(t)
	_, created := purchaseFixture49()
	v := ReplenishmentDecisionInput{OperationID: "11111111-1111-4111-8111-111111111115", ExpectedRevision: 1, Reason: "Cancelamento interno"}
	before := created.After
	at := before.CreatedAt.Add(time.Second)
	after, e := purchaseTransition(before, "cancel", v, fixtureActor.User, at)
	if e != nil {
		t.Fatal(e)
	}
	r := PurchaseDraftReceipt{OperationID: v.OperationID, PurchaseID: before.ID, ActorID: fixtureActor.User, Kind: "cancel", Before: &before, After: after}
	h := proposalHash(before.ID, "cancel", v)
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(purchaseReceiptRows49(r, "", true))
	m.ExpectQuery("SELECT purchase_id::text.*FOR UPDATE").WillReturnRows(purchaseRows49(before))
	m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(at))
	m.ExpectExec("UPDATE online_purchase_drafts").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(purchaseRows49(after))
	m.ExpectExec("INSERT INTO online_purchase_operations").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(purchaseReceiptRows49(r, h, false))
	m.ExpectExec("INSERT INTO online_purchase_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
	b, _ := json.Marshal(r)
	m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("purchase.internal.cancel", b))
	m.ExpectCommit()
	got, e := s.DecidePurchaseDraft(context.Background(), fixtureActor, before.ID, "cancel", v)
	if e != nil || !reflect.DeepEqual(got, r) {
		t.Fatal(got, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestPurchasePersistentSessionAndRoles(t *testing.T) {
	s, m := qualityFixture(t)
	v, _ := purchaseFixture49()
	m.ExpectBegin()
	m.ExpectQuery("SELECT s.id").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	if _, e := s.CreatePurchaseDraft(context.Background(), fixtureActor, v); e != ErrDenied {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"stock", "cashier", "accountant", "employee"} {
		a := fixtureActor
		a.Role = role
		if _, e := s.CreatePurchaseDraft(context.Background(), a, v); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
