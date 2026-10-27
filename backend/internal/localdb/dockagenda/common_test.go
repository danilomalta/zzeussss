package dockagenda

import (
	"testing"
	"time"
)

func TestDockAgendaWindowPrecisionTimezoneAndBounds(t *testing.T) {
	for _, v := range [][2]string{{"2026-10-22T09:00:00-03:00", "2026-10-22T10:00:00-03:00"}, {"2026-10-22T12:00:00Z", "2026-10-22T12:05:00Z"}} {
		a, b, e := window(v[0], v[1])
		if e != nil || a.Location() != time.UTC || !a.Before(b) {
			t.Fatal(a, b, e)
		}
	}
	for _, v := range [][2]string{{"bad", "bad"}, {"2026-10-22T12:00:00.1Z", "2026-10-22T13:00:00Z"}, {"2026-10-22T12:00:00Z", "2026-10-22T12:04:59Z"}, {"2026-10-22T12:00:00Z", "2026-10-22T20:00:01Z"}, {"1999-12-31T12:00:00Z", "1999-12-31T13:00:00Z"}} {
		if _, _, e := window(v[0], v[1]); e == nil {
			t.Fatal(v)
		}
	}
	for _, s := range []string{"", " x", "x\x00", "x\n", "x y", string([]byte{255})} {
		if validID(s) {
			t.Fatal(s)
		}
	}
}
func TestDockAgendaDurationsRejectMissingAndBackwardTimes(t *testing.T) {
	n, e := duration("2026-10-22T12:00:00Z", "2026-10-22T12:02:00Z")
	if e != nil || n != 120 {
		t.Fatal(n, e)
	}
	for _, p := range [][2]string{{"bad", "bad"}, {"2026-10-22T12:02:00Z", "2026-10-22T12:00:00Z"}} {
		if _, e := duration(p[0], p[1]); e == nil {
			t.Fatal(p)
		}
	}
}
