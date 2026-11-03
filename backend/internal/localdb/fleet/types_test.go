package fleet

import (
	"testing"
	"time"
)

func TestFleetStrictScalarValidation(t *testing.T) {
	for _, v := range []string{"", " x", "x/xx", "x\x01", "x\x7f", string([]byte{255})} {
		if ID(v) {
			t.Fatal(v)
		}
	}
	for _, raw := range []string{`{"x":1,"x":2}`, `{"x":{}}`, `[]`, `{"x":[]}`} {
		if _, e := canonical([]byte(raw)); e == nil {
			t.Fatal(raw)
		}
	}
	if _, e := total(MaxValue, 1); e == nil {
		t.Fatal("overflow")
	}
	if _, e := total(-1); e == nil {
		t.Fatal("negative")
	}
}
func TestFleetDepreciationPeriodAndResidual(t *testing.T) {
	p := Profile{AcquisitionCents: 120001, ResidualCents: 12001, UsefulMonths: 12, InServiceOn: "2026-01-01"}
	for _, v := range []struct{ date, want string }{{"2025-12-31", "0"}, {"2026-01-31", "0"}, {"2026-02-01", "9000"}, {"2027-02-01", "108000"}} {
		at, _ := time.Parse("2006-01-02", v.date)
		if got := depreciation(p, at).String(); got != v.want {
			t.Fatal(v, got)
		}
	}
}

func TestFleetCalendarMonthClampsToLastDay(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2026-01-31")
	end, _ := time.Parse("2006-01-02", "2026-02-28")
	if got := monthsCompleted(start, end); got != 1 {
		t.Fatal(got)
	}
}
