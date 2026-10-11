package onlinecatalog

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"strings"
	"testing"
	"time"
)

func documentFixture51() (PurchaseDocumentInput, PurchaseDocumentReceipt) {
	_, r := purchaseFixture49()
	p := r.After
	p.State = "approved"
	p.Revision = 2
	at := p.UpdatedAt.Add(time.Hour)
	file := PurchaseFile{1, "internal_purchase", testOp, p, fixtureActor.User, at}
	doc := PurchaseDocument{ID: testOp, PurchaseID: p.ID, Revision: 1, State: "prepared", File: file, SHA256: creationHash(purchaseFileBytes(file)), CreatorID: fixtureActor.User, LastActorID: fixtureActor.User, Reason: "Documento interno", CreatedAt: at, UpdatedAt: at}
	return PurchaseDocumentInput{testOp, p.ID, 2, "Documento interno"}, PurchaseDocumentReceipt{testOp, testOp, fixtureActor.User, "prepare", nil, doc}
}
func documentRows51(p PurchaseDocument) *sqlmock.Rows {
	b, _ := json.Marshal(p)
	return sqlmock.NewRows([]string{"id", "purchase", "revision", "state", "payload"}).AddRow(p.ID, p.PurchaseID, p.Revision, p.State, b)
}
func documentReceiptRows51(r PurchaseDocumentReceipt, h string, empty bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"op", "id", "actor", "kind", "hash", "receipt"})
	if !empty {
		b, _ := json.Marshal(r)
		rows.AddRow(r.OperationID, r.DocumentID, r.ActorID, r.Kind, h, b)
	}
	return rows
}
func TestPurchaseDocumentStrictInputBytesIntegrityAndCancellation(t *testing.T) {
	v, r := documentFixture51()
	b, _ := json.Marshal(v)
	if got, e := DecodePurchaseDocument("application/json", b); e != nil || got != v {
		t.Fatal(got, e)
	}
	for _, body := range []string{`{}`, string(b) + `{}`, strings.Replace(string(b), `"expected_purchase_revision":2`, `"expected_purchase_revision":0`, 1), strings.Replace(string(b), `"reason":"Documento interno"`, `"reason":null`, 1), strings.Replace(string(b), `"reason":"Documento interno"`, `"reason":"Documento interno","reason":"extra"`, 1), strings.Replace(string(b), `"reason":"Documento interno"`, `"reason":"Documento interno","state":"sent"`, 1)} {
		if _, e := DecodePurchaseDocument("application/json", []byte(body)); e != ErrInput {
			t.Fatal(body, e)
		}
	}
	data, e := PurchaseDocumentJSON(r.After)
	if e != nil || creationHash([]byte(data)) != r.After.SHA256 || !strings.Contains(data, `"kind":"internal_purchase"`) {
		t.Fatal(data, e)
	}
	d := r.After
	d.File.Purchase.TotalCents++
	if _, e = PurchaseDocumentJSON(d); e != ErrUnavailable {
		t.Fatal("tampered total", e)
	}
	d = r.After
	d.File.Kind = "supplier_confirmation"
	d.SHA256 = creationHash(purchaseFileBytes(d.File))
	if validPurchaseDocument(d) {
		t.Fatal("forged acceptance")
	}
	d = r.After
	after, e := cancelPurchaseDocument(d, ReplenishmentDecisionInput{OperationID: testOp, ExpectedRevision: 1, Reason: "Cancelar"}, fixtureActor.User, d.UpdatedAt.Add(time.Second))
	if e != nil || !reflect.DeepEqual(after.File, d.File) || after.SHA256 != d.SHA256 {
		t.Fatal(after, e)
	}
	if _, e = PurchaseDocumentJSON(after); e != ErrConflict {
		t.Fatal("cancelled download", e)
	}
	if _, e = cancelPurchaseDocument(after, ReplenishmentDecisionInput{ExpectedRevision: 2}, fixtureActor.User, after.UpdatedAt); e != ErrConflict {
		t.Fatal(e)
	}
}
func TestPurchaseDocumentAtomicReadbackAuditAndOutbox(t *testing.T) {
	for _, bad := range []string{"success", "ignored_insert", "forged_document", "audit_failure", "outbox_failure", "forged_outbox"} {
		t.Run(bad, func(t *testing.T) {
			v, r := documentFixture51()
			s, m := qualityFixture(t)
			qualityLive(m)
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery("SELECT operation_id::text").WillReturnRows(documentReceiptRows51(r, "", true))
			p := r.After.File.Purchase
			m.ExpectQuery("SELECT purchase_id::text.*FOR SHARE").WillReturnRows(purchaseRows49(p))
			purchaseParents49(m, p)
			m.ExpectQuery("SELECT clock_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(r.After.CreatedAt))
			rows := int64(1)
			if bad == "ignored_insert" {
				rows = 0
			}
			m.ExpectExec("INSERT INTO online_purchase_documents").WillReturnResult(sqlmock.NewResult(0, rows))
			if bad != "ignored_insert" {
				d := r.After
				if bad == "forged_document" {
					d.Reason = "Other"
				}
				m.ExpectQuery("SELECT document_id::text").WillReturnRows(documentRows51(d))
			}
			if bad != "ignored_insert" && bad != "forged_document" {
				if bad == "audit_failure" {
					m.ExpectExec("INSERT INTO online_purchase_document_operations").WillReturnResult(sqlmock.NewResult(0, 0))
				} else {
					m.ExpectExec("INSERT INTO online_purchase_document_operations").WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectQuery("SELECT operation_id::text").WillReturnRows(documentReceiptRows51(r, proposalHash(v.OperationID, "prepare", v), false))
					n := int64(1)
					if bad == "outbox_failure" {
						n = 0
					}
					m.ExpectExec("INSERT INTO online_purchase_document_outbox").WillReturnResult(sqlmock.NewResult(0, n))
					if bad != "outbox_failure" {
						out := r
						if bad == "forged_outbox" {
							out.After.Reason = "Other"
						}
						b, _ := json.Marshal(out)
						m.ExpectQuery("SELECT event_type,payload").WillReturnRows(sqlmock.NewRows([]string{"event", "payload"}).AddRow("purchase.document.prepare", b))
					}
				}
			}
			if bad == "success" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			got, e := s.PreparePurchaseDocument(context.Background(), fixtureActor, v)
			if bad == "success" {
				if e != nil || !reflect.DeepEqual(got, r) {
					t.Fatal(got, e)
				}
			} else if e != ErrUnavailable {
				t.Fatal(e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestPurchaseDocumentReplayAndDeniedState(t *testing.T) {
	v, r := documentFixture51()
	for _, bad := range []string{"same", "other_actor", "changed_intent"} {
		t.Run(bad, func(t *testing.T) {
			s, m := qualityFixture(t)
			qualityLive(m)
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			h := proposalHash(v.OperationID, "prepare", v)
			out := r
			if bad == "other_actor" {
				out.ActorID = testOp
				out.After.LastActorID = testOp
				out.After.CreatorID = testOp
				out.After.File.PreparedBy = testOp
				out.After.SHA256 = creationHash(purchaseFileBytes(out.After.File))
			}
			if bad == "changed_intent" {
				h = strings.Repeat("a", 64)
			}
			m.ExpectQuery("SELECT operation_id::text").WillReturnRows(documentReceiptRows51(out, h, false))
			if bad == "same" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			_, e := s.PreparePurchaseDocument(context.Background(), fixtureActor, v)
			if bad == "same" {
				if e != nil {
					t.Fatal(e)
				}
			} else if e != ErrConflict {
				t.Fatal(e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(documentReceiptRows51(r, "", true))
	p := r.After.File.Purchase
	p.State = "draft"
	m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(purchaseRows49(p))
	m.ExpectRollback()
	if _, e := s.PreparePurchaseDocument(context.Background(), fixtureActor, v); e != ErrConflict {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	a := fixtureActor
	a.Role = "stock"
	if _, e := (Store{}).PreparePurchaseDocument(context.Background(), a, v); e != ErrDenied {
		t.Fatal(e)
	}
	if MigratePurchaseDocuments(context.Background(), nil) != ErrUnavailable {
		t.Fatal("nil DB")
	}
}
