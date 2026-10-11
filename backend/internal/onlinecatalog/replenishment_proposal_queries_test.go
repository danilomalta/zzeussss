package onlinecatalog

import (
	"context"
	"encoding/csv"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func TestProposalQueriesPageSnapshotAndCSVSafety(t *testing.T) {
	_, r := proposalFixture45()
	r.After.Snapshot.Product.Name = "=Item;seguro"
	r.After.Reason = "=SUM(1)"
	q := ProposalQuery{State: "planned", ProductID: 1, Limit: 1}
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"total", "at"}).AddRow(2, r.After.CreatedAt))
	m.ExpectQuery("SELECT proposal_id::text").WillReturnRows(proposalRows45(r.After))
	m.ExpectCommit()
	p, e := s.ReplenishmentProposals(context.Background(), fixtureActor, q)
	if e != nil || p.Total != 2 || !p.HasMore || p.NextOffset == nil || *p.NextOffset != 1 || len(p.SourceHash) != 64 {
		t.Fatal(p, e)
	}
	data, e := ReplenishmentProposalCSV(p)
	if e != nil {
		t.Fatal(e)
	}
	reader := csv.NewReader(strings.NewReader(data))
	reader.Comma = ';'
	rows, e := reader.ReadAll()
	if e != nil || len(rows) != 2 || rows[1][3] != "'=Item;seguro" || rows[1][11] != "'=SUM(1)" {
		t.Fatal(rows, e)
	}
	p.Items[0].QuantityUnits++
	if _, e := ReplenishmentProposalCSV(p); e != ErrInput {
		t.Fatal("altered page hash accepted", e)
	}
	p.Items[0].Snapshot.SuggestedUnits++
	p.SourceHash = proposalPageHash(p)
	if _, e := ReplenishmentProposalCSV(p); e != ErrUnavailable {
		t.Fatal("forged snapshot accepted", e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestProposalQueriesHistoryAndChecksPreserveSnapshots(t *testing.T) {
	_, r := proposalFixture45()
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT proposal_id::text").WillReturnRows(proposalRows45(r.After))
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	m.ExpectQuery("SELECT operation_id::text").WillReturnRows(proposalReceiptRows45(r, "1234567890123456789012345678901234567890123456789012345678901234", false))
	m.ExpectCommit()
	history, e := s.ReplenishmentProposalHistory(context.Background(), fixtureActor, r.ProposalID, 50, 0)
	if e != nil || history.Total != 1 || history.HasMore || history.Items[0].After.QuantityUnits != 5 {
		t.Fatal(history, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	current := r.After.Snapshot.StockPlan
	got := checkProposalSnapshot(r.After, &current, time.Now())
	if !got.SnapshotMatches || !got.CanApprove || got.Status != "matches" {
		t.Fatal(got)
	}
	current.StockUnits++
	got = checkProposalSnapshot(r.After, &current, time.Now())
	if got.SnapshotMatches || got.CanApprove || got.Status != "changed" || got.Proposal.Snapshot.StockUnits != 4 {
		t.Fatal(got)
	}
	got = checkProposalSnapshot(r.After, nil, time.Now())
	if got.Status != "product_unavailable" || got.Current != nil || got.CanApprove {
		t.Fatal(got)
	}
	current = r.After.Snapshot.StockPlan
	r.After.State = "approved"
	got = checkProposalSnapshot(r.After, &current, time.Now())
	if got.CanApprove || !got.SnapshotMatches {
		t.Fatal(got)
	}
}
func TestProposalQueriesUnavailableAndDeniedDoNotInventRows(t *testing.T) {
	s, m := qualityFixture(t)
	qualityLive(m)
	m.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"total", "at"}).AddRow(1, time.Now()))
	m.ExpectQuery("SELECT proposal_id::text").WillReturnRows(sqlmock.NewRows([]string{"id", "product", "revision", "state", "payload"}))
	m.ExpectRollback()
	if _, e := s.ReplenishmentProposals(context.Background(), fixtureActor, ProposalQuery{State: "all", Limit: 50}); e != ErrUnavailable {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	a := fixtureActor
	a.Role = "stock"
	if _, e := (Store{}).ReplenishmentProposalSummary(context.Background(), a, ProposalQuery{State: "all", Limit: 50}); e != ErrDenied {
		t.Fatal(e)
	}
}
