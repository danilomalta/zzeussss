package payables

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestScheduleExactPrecisionBoundariesAndCivilDates(t *testing.T) {
	parts := []Installment{{"one", "2028-02-29", MaxSafe - 1}, {"two", "2028-02-29", 1}}
	if !schedule(parts, MaxSafe, "2028-02-28") {
		t.Fatal("exact maximum or same-day installments rejected")
	}
	if schedule(parts, MaxSafe-1, "2028-02-28") {
		t.Fatal("inexact sum accepted")
	}
	parts[1].AmountCents = 2
	if schedule(parts, MaxSafe, "2028-02-28") {
		t.Fatal("overflow accepted")
	}
	parts[1].AmountCents = 1
	parts[1].ID = "one"
	if schedule(parts, MaxSafe, "2028-02-28") {
		t.Fatal("duplicate ID")
	}
	for _, d := range []string{"2026-02-29", "2026-2-01", "2026-10-20T00:00:00Z", "1899-12-31"} {
		if date(d) {
			t.Fatal(d)
		}
	}
	parts = make([]Installment, 120)
	for i := range parts {
		parts[i] = Installment{strings.Repeat("a", i+1), "2026-10-20", 1}
	}
	if !schedule(parts, 120, "2026-10-20") {
		t.Fatal("120 installments rejected")
	}
	parts = append(parts, Installment{"last", "2026-10-20", 1})
	if schedule(parts, 121, "2026-10-20") {
		t.Fatal("121 installments accepted")
	}
}
func TestCSVTextLimitsAndCancelledContext(t *testing.T) {
	r := Report{Scope: "local_supplier_obligations", Currency: "BRL", Window: Window{"2026-10-01", "2026-10-20", "2026-10-20"}, Summary: map[string]int64{}, Items: []DueItem{}}
	for _, s := range []string{"=SUM(1)", "+1", "-1", "@formula", " text", "\tformula", "Supplier\nTwo"} {
		if !errors.Is(exportText(s), ErrText) {
			t.Fatal(s)
		}
	}
	if exportText("Açúcar e Pães") != nil {
		t.Fatal("valid UTF8 rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := encodeCSV(ctx, r); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	b := boundedCSV{}
	if _, e := b.Write(make([]byte, MaxCSVBytes)); e != nil {
		t.Fatal(e)
	}
	if _, e := b.Write([]byte{1}); !errors.Is(e, ErrRange) {
		t.Fatal(e)
	}
}
