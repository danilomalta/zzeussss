package retail

import (
	"errors"
	"testing"
	"time"
)

func TestCreditCalendarRequiresExplicitZoneAndPreservesLocalDueDay(t *testing.T) {
	at := time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
	day, e := creditDay(at, "America/Sao_Paulo")
	if e != nil || day != "2026-11-03" {
		t.Fatal("premature overdue day", day, e)
	}
	day, e = creditDay(at, "UTC")
	if e != nil || day != "2026-11-04" {
		t.Fatal(day, e)
	}
	for _, zone := range []string{"", "Local", "not-a-timezone"} {
		if _, e = creditDay(at, zone); !errors.Is(e, ErrInvalid) {
			t.Fatal("implicit calendar accepted")
		}
	}
}
