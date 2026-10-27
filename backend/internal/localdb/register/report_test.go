package register

import (
	"context"
	"testing"
)

// Decode legacy rows directly: this tests historical aggregation and is NOT
// evidence that a card/PIX charging connector or mixed-sale completion exists.
func TestClosedReportGroupsLegacyPartsBySaleAndKeepsStatusesSeparate(t *testing.T) {
	db, actor, device := setup(t)
	ctx := context.Background()
	if _, err := Open(ctx, db, actor, device, OpenInput{SessionID: "old-turn", OpeningCents: 1000}); err != nil {
		t.Fatal(err)
	}
	for _, sale := range []struct {
		id, status, method, state string
		amount                    int64
	}{
		{"a", "committed", "cash", "confirmed", 500},
		{"b", "committed", "pix", "pending", 96},
		{"c", "cancelled", "card", "reversed", 119},
	} {
		if _, err := db.Exec(`INSERT INTO sales VALUES('market','s1','d1',?,'old-turn',?,?,'now')`, sale.id, sale.status, sale.amount); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO sale_payments VALUES('market',?,'same-part',?,?,?)`, sale.id, sale.method, sale.state, sale.amount); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO cash_movements VALUES('cash-in','market','s1','old-turn',500,'historical cash sale','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := Close(ctx, db, actor, device, CloseInput{SessionID: "old-turn", OperationID: "close-old", DeclaredCents: 1500}); err != nil {
		t.Fatal(err)
	}
	v, err := ReadClosedReport(ctx, db, actor, device, "old-turn")
	if err != nil {
		t.Fatal(err)
	}
	if v.SaleCount != 3 || v.GrossSalesCents != 715 || v.CancelledSalesCents != 119 || v.NetSalesCents != 596 {
		t.Fatal(v)
	}
	if v.Payments[0].ConfirmedCents != 500 || v.Payments[1].ReversedCents != 119 || v.Payments[2].PendingCents != 96 {
		t.Fatal(v.Payments)
	}
	if v.BankSettlementAvailable || v.FinancialLedgerAvailable || v.Cash.ExpectedCents != 1500 {
		t.Fatal("legacy statuses became bank confirmations")
	}
}
