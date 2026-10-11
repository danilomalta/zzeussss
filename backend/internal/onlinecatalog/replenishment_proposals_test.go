package onlinecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"strings"
	"testing"
	"time"
)

func proposalFixture45() (ReplenishmentProposalInput, ReplenishmentProposalReceipt) {
	plan := StockPlan{Product: Product{ID: 1, Name: "Item", SKU: "item", PriceCents: 1000, Active: true, Version: 1}, StockUnits: 4, Settings: StockSettings{MinimumUnits: 5, ReorderPointUnits: 8, LeadTimeDays: 2, DailyDemand: "1.0000"}}
	snapshot, _ := replenish(plan, 7)
	at := time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC)
	v := ReplenishmentProposalInput{testOp, 1, 1, 7, 5, "Reposição conferida"}
	p := ReplenishmentProposal{testOp, 1, 1, "planned", snapshot, 5, fixtureActor.User, fixtureActor.User, v.Reason, at, at}
	return v, ReplenishmentProposalReceipt{testOp, testOp, fixtureActor.User, "create", nil, p}
}
func proposalRows45(p ReplenishmentProposal) *sqlmock.Rows {
	b, _ := json.Marshal(p)
	return sqlmock.NewRows([]string{"id", "product", "revision", "state", "payload"}).AddRow(p.ID, p.ProductID, p.Revision, p.State, b)
}
func proposalReceiptRows45(r ReplenishmentProposalReceipt, h string, empty bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"op", "id", "actor", "kind", "hash", "receipt"})
	if !empty {
		b, _ := json.Marshal(r)
		rows.AddRow(r.OperationID, r.ProposalID, r.ActorID, r.Kind, h, b)
	}
	return rows
}
func TestReplenishmentProposalInputsAndHonestSnapshot(t *testing.T) {
	v, r := proposalFixture45()
	b, _ := json.Marshal(v)
	if got, e := DecodeReplenishmentProposal("application/json; charset=utf-8", b); e != nil || got != v {
		t.Fatal(got, e)
	}
	for _, raw := range []string{"{}", string(b) + "{}", strings.Replace(string(b), `"quantity_units":5`, `"quantity_units":null`, 1), strings.Replace(string(b), `"quantity_units":5`, `"quantity_units":5,"quantity_units":5`, 1), strings.Replace(string(b), `"quantity_units":5`, `"quantity_units":5.0`, 1), strings.Replace(string(b), `"quantity_units":5`, `"quantity_units":0`, 1), strings.Replace(string(b), `"review_days":7`, `"review_days":366`, 1), strings.Replace(string(b), `"reason":"Reposição conferida"`, `"reason":"Reposição conferida","tenant_id":"x"`, 1)} {
		if _, e := DecodeReplenishmentProposal("application/json", []byte(raw)); e != ErrInput {
			t.Fatal("unsafe input", e)
		}
	}
	if _, e := DecodeReplenishmentProposal("text/plain", b); e != ErrInput {
		t.Fatal(e)
	}
	decision := ReplenishmentDecisionInput{testOp, 1, "Aprovação conferida"}
	d, _ := json.Marshal(decision)
	if got, e := DecodeReplenishmentDecision("application/json", d); e != nil || got != decision {
		t.Fatal(got, e)
	}
	for _, raw := range []string{`{}`, string(d) + "{}", strings.Replace(string(d), `"expected_revision":1`, `"expected_revision":1.0`, 1), strings.Replace(string(d), `"expected_revision":1`, `"expected_revision":null`, 1), strings.Replace(string(d), `"expected_revision":1`, `"expected_revision":1,"state":"approved"`, 1)} {
		if _, e := DecodeReplenishmentDecision("application/json", []byte(raw)); e != ErrInput {
			t.Fatal(e)
		}
	}
	if !validProposalReceipt(r) {
		t.Fatal("valid snapshot rejected")
	}
	r.After.Snapshot.SuggestedUnits++
	if validProposalReceipt(r) {
		t.Fatal("forged recommendation accepted")
	}
}
func TestReplenishmentProposalStateMachineAndSnapshotPreservation(t *testing.T) {
	_, r := proposalFixture45()
	v := ReplenishmentDecisionInput{testOp, 1, "Aprovação conferida"}
	at := r.After.UpdatedAt.Add(time.Second)
	approved, e := proposalTransition(r.After, "approve", v, fixtureActor.User, at)
	if e != nil || approved.State != "approved" || approved.Revision != 2 || !reflect.DeepEqual(approved.Snapshot, r.After.Snapshot) {
		t.Fatal(approved, e)
	}
	before := r.After
	decision := ReplenishmentProposalReceipt{"11111111-1111-4111-8111-111111111115", before.ID, fixtureActor.User, "approve", &before, approved}
	if !validProposalReceipt(decision) {
		t.Fatal("decision rejected")
	}
	decision.After.QuantityUnits++
	if validProposalReceipt(decision) {
		t.Fatal("decision silently changed quantity")
	}
	v.ExpectedRevision = 2
	if _, e = proposalTransition(approved, "approve", v, fixtureActor.User, at); e != ErrConflict {
		t.Fatal(e)
	}
	cancelled, e := proposalTransition(approved, "cancel", v, fixtureActor.User, at)
	if e != nil || cancelled.State != "cancelled" || cancelled.Revision != 3 {
		t.Fatal(cancelled, e)
	}
	v.ExpectedRevision = 3
	if _, e = proposalTransition(cancelled, "cancel", v, fixtureActor.User, at); e != ErrConflict {
		t.Fatal(e)
	}
	v.ExpectedRevision = 1
	if _, e = proposalTransition(approved, "cancel", v, fixtureActor.User, at); e != ErrConflict {
		t.Fatal(e)
	}
}
func TestReplenishmentProposalSaveReadbackAtomicOutbox(t *testing.T) {
	for _, outcome := range []string{"success", "outbox-failure", "ignored-insert", "forged-readback", "ignored-audit", "forged-outbox"} {
		t.Run(outcome, func(t *testing.T) {
			s, m := qualityFixture(t)
			v, r := proposalFixture45()
			h := proposalHash(v.OperationID, "create", v)
			qualityLive(m)
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT operation_id::text").WillReturnRows(proposalReceiptRows45(r, h, true))
			m.ExpectQuery("SELECT id,nome.*FOR UPDATE").WillReturnRows(planRows43(r.After.Snapshot.StockPlan))
			m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.CreatedAt))
			count := int64(1)
			if outcome == "ignored-insert" {
				count = 0
			}
			m.ExpectExec("INSERT INTO online_replenishment_proposals").WillReturnResult(sqlmock.NewResult(0, count))
			if outcome != "ignored-insert" {
				actual := r.After
				if outcome == "forged-readback" {
					actual.QuantityUnits++
				}
				m.ExpectQuery("SELECT proposal_id::text").WillReturnRows(proposalRows45(actual))
				if outcome != "forged-readback" {
					count = 1
					if outcome == "ignored-audit" {
						count = 0
					}
					m.ExpectExec("INSERT INTO online_replenishment_operations").WillReturnResult(sqlmock.NewResult(0, count))
					if outcome != "ignored-audit" {
						m.ExpectQuery("SELECT operation_id::text").WillReturnRows(proposalReceiptRows45(r, h, false))
						if outcome == "outbox-failure" {
							m.ExpectExec("INSERT INTO online_replenishment_outbox").WillReturnError(errors.New("private details"))
						} else {
							m.ExpectExec("INSERT INTO online_replenishment_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
							out := r
							if outcome == "forged-outbox" {
								out.After.QuantityUnits++
							}
							b, _ := json.Marshal(out)
							m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("replenishment.proposal.create", b))
						}
					}
				}
			}
			if outcome == "success" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.CreateReplenishmentProposal(context.Background(), fixtureActor, v)
			if outcome == "success" && (e != nil || !reflect.DeepEqual(got, r)) || outcome != "success" && e != ErrUnavailable {
				t.Fatal(got, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestReplenishmentProposalReplayAndScopedConflict(t *testing.T) {
	for _, outcome := range []string{"replay", "changed-input", "other-actor"} {
		s, m := qualityFixture(t)
		v, r := proposalFixture45()
		h := proposalHash(v.OperationID, "create", v)
		if outcome == "changed-input" {
			v.QuantityUnits++
		}
		if outcome == "other-actor" {
			r.ActorID = "11111111-1111-4111-8111-111111111115"
			r.After.CreatorID = r.ActorID
			r.After.LastActorID = r.ActorID
		}
		qualityLive(m)
		m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		m.ExpectQuery("SELECT operation_id::text").WillReturnRows(proposalReceiptRows45(r, h, false))
		if outcome == "replay" {
			m.ExpectCommit()
		} else {
			m.ExpectRollback()
		}
		_, e := s.CreateReplenishmentProposal(context.Background(), fixtureActor, v)
		if outcome == "replay" && e != nil || outcome != "replay" && e != ErrConflict {
			t.Fatal(outcome, e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestReplenishmentProposalRejectsChangedStockAndRevokedSession(t *testing.T) {
	s, m := qualityFixture(t)
	_, r := proposalFixture45()
	v := ReplenishmentDecisionInput{"11111111-1111-4111-8111-111111111115", 1, "Aprovação"}
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(proposalReceiptRows45(r, "", true))
	m.ExpectQuery("SELECT proposal_id::text.*FOR UPDATE").WillReturnRows(proposalRows45(r.After))
	plan := r.After.Snapshot.StockPlan
	plan.StockUnits++
	m.ExpectQuery("SELECT id,nome.*FOR UPDATE").WillReturnRows(planRows43(plan))
	m.ExpectRollback()
	if _, e := s.DecideReplenishmentProposal(context.Background(), fixtureActor, r.ProposalID, "approve", v); e != ErrConflict {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	s, m = qualityFixture(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT s.id").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	if _, e := s.CreateReplenishmentProposal(context.Background(), fixtureActor, ReplenishmentProposalInput{testOp, 1, 1, 7, 5, "Pedido"}); e != ErrDenied {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"stock", "cashier", "accountant", "employee"} {
		a := fixtureActor
		a.Role = role
		v, _ := proposalFixture45()
		if _, e := (Store{}).CreateReplenishmentProposal(context.Background(), a, v); e != ErrDenied {
			t.Fatal(role, e)
		}
	}
}
