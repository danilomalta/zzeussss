package cashjournal

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
)

func TestCSVExportRoundTripsTextAndLeavesUnknownProvenanceEmpty(t *testing.T) {
	value := `opção; com "aspas", unicode 漢字`
	p := Page{SessionID: "turn-one", Scope: "cash_operational", TotalCount: 1, Summary: Summary{ExpectedCents: 200, DeclaredCents: 200, DebitCents: 200, CreditCents: 200, UnclassifiedCount: 1}, Items: []Entry{{ID: "movement:one", SourceKind: "movement", SourceID: "one", Category: "unclassified", OccurredAt: value, CashDeltaCents: 200, Legs: []Leg{{Account: "cash_drawer", Side: "debit", AmountCents: 200}, {Account: "unclassified_control", Side: "credit", AmountCents: 200}}}}}
	data, err := encodeCSV(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff")))
	r.Comma = ';'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	if rows[2][10] != value || rows[2][8] != "" || rows[2][9] != "" || rows[1][21] != "1" || rows[2][4] != p.Items[0].ID {
		t.Fatal("text or unknown provenance changed", rows)
	}
}

func TestCSVExportRejectsPotentialFormulaCellsWithoutChangingIDs(t *testing.T) {
	for _, s := range []string{"=1+1", "+SUM(1)", "-1+2", "@SUM(1)", "＝1+1", "＋1", "－1", "＠1", " leading", "\tformula", "\nformula", "\rformula", "'escaped", `"quoted-first"`, "a\x00b", "a\tb", "a\nb", "a\rb", string([]byte{0xff}), strings.Repeat("a", 4097)} {
		if err := exportText(s); !errors.Is(err, ErrExportText) {
			t.Fatalf("accepted unsupported CSV text %q: %v", s, err)
		}
	}
	for _, s := range []string{"", "valid-id", "_source", "1234567890123456789", `venda:op;="x"`, "ação 漢字"} {
		if err := exportText(s); err != nil {
			t.Fatal(s, err)
		}
	}
}

func TestCSVExportByteLimitReturnsNoPartialFileAndHonorsCancellation(t *testing.T) {
	text := "a" + strings.Repeat("x", 4095)
	entry := Entry{ID: text, SourceKind: "movement", SourceID: text, Category: "unclassified", OccurredAt: text, CashDeltaCents: 1, Legs: []Leg{{Account: "cash_drawer", Side: "debit", AmountCents: 1}, {Account: "unclassified_control", Side: "credit", AmountCents: 1}}}
	p := Page{SessionID: "turn-one", Scope: "cash_operational", TotalCount: 2000, Items: make([]Entry, 2000)}
	for i := range p.Items {
		p.Items[i] = entry
	}
	if data, err := encodeCSV(context.Background(), p); data != nil || !errors.Is(err, ErrExportLimit) {
		t.Fatalf("partial oversized file: %d %v", len(data), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if data, err := encodeCSV(ctx, p); data != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled export returned data: %d %v", len(data), err)
	}
}
