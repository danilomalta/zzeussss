package dockagenda

import (
	"testing"
	"time"
)

func insightTime(s string) time.Time { v, _ := time.Parse(time.RFC3339Nano, s); return v }
func TestDockInsightExactGapsClippingAndAdjacency(t *testing.T) {
	from, to := insightTime("2026-10-22T08:00:00Z"), insightTime("2026-10-22T14:00:00Z")
	rows := []Appointment{{StartsAt: "2026-10-22T07:00:00Z", EndsAt: "2026-10-22T09:00:00Z"}, {StartsAt: "2026-10-22T10:00:00Z", EndsAt: "2026-10-22T11:00:00Z"}, {StartsAt: "2026-10-22T11:00:00Z", EndsAt: "2026-10-22T12:00:00Z"}}
	gaps, e := freeGaps(from, to, time.Hour, rows)
	if e != nil || len(gaps) != 2 || gaps[0].StartsAt != "2026-10-22T09:00:00Z" || gaps[0].FirstFitEndsAt != "2026-10-22T10:00:00Z" || gaps[1].EndsAt != "2026-10-22T14:00:00Z" {
		t.Fatal(gaps, e)
	}
	rows[2].StartsAt = "2026-10-22T10:30:00Z"
	if _, e = freeGaps(from, to, time.Hour, rows); e == nil {
		t.Fatal("overlapping stored slots produced plausible windows")
	}
}
func TestDockInsightAveragePreservesNanosecondsAndMissingSamples(t *testing.T) {
	m := metricNew("supplier")
	if metricDone(m).AverageWaitingSeconds != nil || metricDone(m).AverageUnloadingSeconds != nil {
		t.Fatal("unknown became zero")
	}
	arrival, unload, complete := "2026-10-22T10:00:01Z", "2026-10-22T10:00:01.5Z", "2026-10-22T10:00:03Z"
	r := Appointment{Status: "completed", StartsAt: "2026-10-22T10:00:00Z", ArrivedAt: &arrival, UnloadingAt: &unload, CompletedAt: &complete}
	if e := metricAdd(m, r); e != nil {
		t.Fatal(e)
	}
	out := metricDone(m)
	if out.AverageWaitingSeconds.Numerator != "1" || out.AverageWaitingSeconds.Denominator != "2" || out.AverageUnloadingSeconds.Numerator != "3" || out.AverageUnloadingSeconds.Denominator != "2" || out.LateArrivals != 1 {
		t.Fatal(out)
	}
	r.Status = "cancelled"
	r.UnloadingAt = nil
	r.CompletedAt = nil
	if e := metricAdd(m, r); e != nil {
		t.Fatal(e)
	}
	out = metricDone(m)
	if out.ArrivalSamples != 1 || out.Cancelled != 1 {
		t.Fatal(out)
	}
	if _, e := metricDuration(complete, arrival); e == nil {
		t.Fatal("negative duration")
	}
}
