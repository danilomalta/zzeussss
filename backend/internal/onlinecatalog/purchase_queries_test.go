package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func TestPurchaseQueriesPageAndCSVPreserveExactSnapshot(t *testing.T) {
	_, r := purchaseFixture49()
	r.After.Supplier.Name = "=Supplier;unsafe"
	r.After.Reason = "+reason"
	r.After.Proposal.Snapshot.Product.Name = "@Item"
	s, m := qualityFixture(t)
	qualityLive(m)
	q := PurchaseQuery{State: "draft", ProductID: r.After.Proposal.ProductID, SupplierID: r.After.Supplier.ID, Limit: 1}
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "at"}).AddRow(2, r.After.CreatedAt))
	m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(purchaseRows49(r.After))
	m.ExpectCommit()
	p, e := s.PurchaseDrafts(context.Background(), fixtureActor, q)
	if e != nil || p.Total != 2 || !p.HasMore || p.NextOffset == nil || *p.NextOffset != 1 || len(p.SourceHash) != 64 {
		t.Fatal(p, e)
	}
	data, e := PurchaseDraftCSV(p)
	if e != nil {
		t.Fatal(e)
	}
	rd := csv.NewReader(strings.NewReader(data))
	rd.Comma = ';'
	rows, e := rd.ReadAll()
	if e != nil || len(rows) != 2 || rows[1][3] != "'=Supplier;unsafe" || rows[1][6] != "'@Item" || rows[1][15] != "'+reason" || rows[1][11] != "1250" || rows[1][12] != "internal_declaration" {
		t.Fatal(rows, e)
	}
	p.Items[0].TotalCents++
	if _, e = PurchaseDraftCSV(p); e != ErrInput {
		t.Fatal("hash tamper", e)
	}
	p.SourceHash = purchasePageHash(p)
	if _, e = PurchaseDraftCSV(p); e != ErrUnavailable {
		t.Fatal("invalid snapshot", e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestPurchaseChecksAllParentsAndDecisionEligibility(t *testing.T) {
	_, r := purchaseFixture49()
	p := r.After
	proposal, offer, supplier, product := p.Proposal, p.Offer, p.Supplier, p.Proposal.Snapshot.StockPlan
	check := func() PurchaseCheck {
		return checkPurchaseSnapshot(p, &proposal, &offer, &supplier, &product, time.Now())
	}
	got := check()
	if !got.CanApprove || !got.SnapshotMatches || got.Status != "matches" {
		t.Fatal(got)
	}
	offer.UnitPriceCents++
	got = check()
	if got.OfferMatches || got.CanApprove || got.Status != "changed" || got.Purchase.TotalCents != 1250 {
		t.Fatal(got)
	}
	offer = p.Offer
	supplier.Active = false
	if check().SupplierMatches || check().CanApprove {
		t.Fatal("inactive supplier")
	}
	supplier = p.Supplier
	proposal.State = "cancelled"
	if check().ProposalMatches || check().CanApprove {
		t.Fatal("cancelled proposal")
	}
	proposal = p.Proposal
	product.StockUnits++
	if check().ProductMatches || check().CanApprove {
		t.Fatal("stock changed")
	}
	product = p.Proposal.Snapshot.StockPlan
	got = checkPurchaseSnapshot(p, &proposal, &offer, &supplier, nil, time.Now())
	if got.Status != "parent_unavailable" || got.CanApprove || got.CurrentProduct != nil {
		t.Fatal(got)
	}
	p.State = "approved"
	if check().CanApprove || !check().SnapshotMatches {
		t.Fatal("approved cannot approve again")
	}
	p.State = "draft"
	p.Revision = MaxVersion
	if check().CanApprove {
		t.Fatal("exhausted revision")
	}
}
func TestPurchaseSummaryHistoryAndMissingParentsReadOnly(t *testing.T) {
	_, r := purchaseFixture49()
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"total", "draft", "approved", "cancelled", "at"}).AddRow(1, 1, 0, 0, r.After.CreatedAt))
	m.ExpectCommit()
	sum, e := s.PurchaseDraftSummary(context.Background(), fixtureActor, PurchaseQuery{State: "all", Limit: 50})
	if e != nil || sum.Draft != 1 || sum.Total != 1 {
		t.Fatal(sum, e)
	}
	qualityLive(m)
	m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(purchaseRows49(r.After))
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(purchaseReceiptRows49(r, strings.Repeat("a", 64), false))
	m.ExpectCommit()
	h, e := s.PurchaseDraftHistory(context.Background(), fixtureActor, r.PurchaseID, 50, 0)
	if e != nil || h.Total != 1 || len(h.Items) != 1 || h.Items[0].After.TotalCents != 1250 {
		t.Fatal(h, e)
	}
	qualityLive(m)
	m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(purchaseRows49(r.After))
	m.ExpectQuery("SELECT proposal_id::text").WillReturnRows(proposalRows45(r.After.Proposal))
	m.ExpectQuery("SELECT offer_id::text").WillReturnRows(offerRows48(r.After.Offer))
	m.ExpectQuery("SELECT supplier_id::text").WillReturnRows(supplierRows47(r.After.Supplier))
	m.ExpectQuery("SELECT id,nome").WillReturnError(sql.ErrNoRows)
	m.ExpectQuery("SELECT transaction_timestamp").WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(time.Now()))
	m.ExpectCommit()
	c, e := s.CheckPurchaseDraft(context.Background(), fixtureActor, r.PurchaseID)
	if e != nil || c.CanApprove || c.Status != "parent_unavailable" || c.CurrentProduct != nil {
		t.Fatal(c, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestPurchaseQueriesRejectBrokenCountsRowsAuditAndPermissions(t *testing.T) {
	_, r := purchaseFixture49()
	for _, bad := range []string{"short_page", "wrong_revision", "wrong_audit"} {
		t.Run(bad, func(t *testing.T) {
			s, m := qualityFixture(t)
			qualityLive(m)
			if bad == "short_page" {
				m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n", "at"}).AddRow(1, time.Now()))
				m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(sqlmock.NewRows([]string{"id", "proposal", "offer", "revision", "state", "payload"}))
				m.ExpectRollback()
				if _, e := s.PurchaseDrafts(context.Background(), fixtureActor, PurchaseQuery{State: "all", Limit: 50}); e != ErrUnavailable {
					t.Fatal(e)
				}
			} else {
				m.ExpectQuery("SELECT purchase_id::text").WillReturnRows(purchaseRows49(r.After))
				n := int64(1)
				if bad == "wrong_revision" {
					n = 2
				}
				m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(n))
				if bad == "wrong_audit" {
					broken := r
					broken.After.Reason = "Forged historical reason"
					b, _ := json.Marshal(broken)
					m.ExpectQuery("SELECT operation_id::text").WillReturnRows(sqlmock.NewRows([]string{"op", "id", "actor", "kind", "hash", "receipt"}).AddRow(r.OperationID, r.PurchaseID, r.ActorID, r.Kind, strings.Repeat("a", 64), b))
				}
				m.ExpectRollback()
				if _, e := s.PurchaseDraftHistory(context.Background(), fixtureActor, r.PurchaseID, 50, 0); e != ErrUnavailable {
					t.Fatal(e)
				}
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
	q := PurchaseQuery{State: "all", Limit: 50}
	for _, bad := range []PurchaseQuery{{State: "confirmed", Limit: 50}, {State: "all", Limit: 101}, {State: "all", Limit: 50, SupplierID: "bad"}, {State: "all", Limit: 50, Offset: -1}} {
		if ValidatePurchaseQuery(bad) != ErrInput {
			t.Fatal(bad)
		}
	}
	a := fixtureActor
	a.Role = "stock"
	if _, e := (Store{}).PurchaseDrafts(context.Background(), a, q); e != ErrDenied {
		t.Fatal(e)
	}
	if _, e := (Store{}).PurchaseDrafts(context.Background(), fixtureActor, q); e != ErrUnavailable {
		t.Fatal(e)
	}
}
