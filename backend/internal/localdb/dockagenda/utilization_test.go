package dockagenda

import (
	"errors"
	"testing"
	"time"
)

func utilRow(id, start, end, unload, complete string) Appointment {
	a := Appointment{ID: id, StartsAt: start, EndsAt: end, Status: "completed", ArrivedAt: &unload, UnloadingAt: &unload, CompletedAt: &complete}
	return a
}
func TestDockUtilizationClipsActualIndependentOfPlannedAndExactNanos(t *testing.T) {
	from, to, now := insightTime("2026-01-02T10:00:00Z"), insightTime("2026-01-02T11:00:00Z"), insightTime("2026-01-03T12:00:00Z")
	rows := []Appointment{utilRow("old-slot", "2026-01-01T08:00:00Z", "2026-01-01T09:00:00Z", "2026-01-02T09:00:00Z", "2026-01-02T10:00:00.5Z"), utilRow("b", "2026-01-02T10:00:00Z", "2026-01-02T11:00:00Z", "2026-01-02T10:30:00Z", "2026-01-02T12:00:00Z")}
	out, e := utilizationCompute(rows, from, to, now)
	if e != nil || out.TotalSegments != 2 || out.UnloadingSeconds != (ExactSeconds{"3601", "2"}) || out.UnloadingFraction != (ExactSeconds{"3601", "7200"}) || out.WithoutRegisteredUnloadingSeconds != (ExactSeconds{"3599", "2"}) || out.PlannedAppointments != 1 {
		t.Fatal(out, e)
	}
	if out.Items[0].StartsAt != from.Format(time.RFC3339) || out.Items[1].EndsAt != to.Format(time.RFC3339) {
		t.Fatal(out.Items)
	}
}
func TestDockUtilizationPlannedUnionExcludesCancelledNoShowAndPreservesZero(t *testing.T) {
	from, to, now := insightTime("2026-01-02T10:00:00Z"), insightTime("2026-01-02T12:00:00Z"), insightTime("2026-01-03T12:00:00Z")
	rows := []Appointment{{ID: "a", StartsAt: "2026-01-02T09:00:00Z", EndsAt: "2026-01-02T11:00:00Z", Status: "reserved"}, {ID: "b", StartsAt: "2026-01-02T10:30:00Z", EndsAt: "2026-01-02T11:30:00Z", Status: "arrived"}, {ID: "c", StartsAt: "2026-01-02T11:30:00Z", EndsAt: "2026-01-02T12:00:00Z", Status: "cancelled"}}
	out, e := utilizationCompute(rows, from, to, now)
	if e != nil || out.PlannedSeconds != (ExactSeconds{"5400", "1"}) || out.UnloadingFraction != (ExactSeconds{"0", "1"}) || out.TotalSegments != 0 {
		t.Fatal(out, e)
	}
	rows[2].Status = "no_show"
	out, e = utilizationCompute(rows, from, to, now)
	if e != nil || out.PlannedFraction != (ExactSeconds{"3", "4"}) {
		t.Fatal(out, e)
	}
}
func TestDockUtilizationRejectsOverlappingActualAndContradictoryEvidence(t *testing.T) {
	from, to, now := insightTime("2026-01-02T10:00:00Z"), insightTime("2026-01-02T12:00:00Z"), insightTime("2026-01-03T12:00:00Z")
	a := utilRow("a", "2026-01-02T10:00:00Z", "2026-01-02T11:00:00Z", "2026-01-02T10:00:00Z", "2026-01-02T11:00:00Z")
	b := utilRow("b", a.StartsAt, a.EndsAt, "2026-01-02T10:30:00Z", "2026-01-02T11:30:00Z")
	if _, e := utilizationCompute([]Appointment{a, b}, from, to, now); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	for _, change := range []func(*Appointment){func(v *Appointment) { v.Status = "cancelled" }, func(v *Appointment) { v.ArrivedAt = nil }, func(v *Appointment) { v.CompletedAt = nil }, func(v *Appointment) { s := "2201-01-01T00:00:00Z"; v.CompletedAt = &s }, func(v *Appointment) { v.Status = "invented" }} {
		v := a
		change(&v)
		if _, e := utilizationCompute([]Appointment{v}, from, to, now); !errors.Is(e, ErrConflict) {
			t.Fatal(v, e)
		}
	}
}
func TestDockUtilizationOngoingClippedAndAdjacentDischarges(t *testing.T) {
	from, to, now := insightTime("2026-01-02T10:00:00Z"), insightTime("2026-01-02T12:00:00Z"), insightTime("2026-01-02T12:00:00.5Z")
	a := utilRow("a", "2026-01-02T08:00:00Z", "2026-01-02T09:00:00Z", "2026-01-02T09:00:00Z", "2026-01-02T11:00:00Z")
	b := utilRow("b", a.StartsAt, a.EndsAt, "2026-01-02T11:00:00Z", "")
	b.Status = "unloading"
	b.CompletedAt = nil
	out, e := utilizationCompute([]Appointment{b, a}, from, to, now)
	if e != nil || out.UnloadingFraction != (ExactSeconds{"1", "1"}) || !out.Items[1].StillUnloading {
		t.Fatal(out, e)
	}
}

func TestDockUtilizationDailyBucketsPartialMidnightAndExactConservation(t *testing.T) {
	from, to, now := insightTime("2026-01-02T23:30:00Z"), insightTime("2026-01-03T00:30:00Z"), insightTime("2026-01-04T00:00:00Z")
	a := utilRow("a", "2026-01-02T23:00:00Z", "2026-01-03T01:00:00Z", "2026-01-02T23:45:00Z", "2026-01-03T00:15:00.5Z")
	out, e := utilizationCompute([]Appointment{a}, from, to, now)
	if e != nil || len(out.Days) != 2 || out.Days[0].Day != "2026-01-02" || out.Days[1].Day != "2026-01-03" || out.Days[0].PeriodSeconds != (ExactSeconds{"1800", "1"}) || out.Days[0].UnloadingSeconds != (ExactSeconds{"900", "1"}) || out.Days[1].UnloadingSeconds != (ExactSeconds{"1801", "2"}) || out.UnloadingSeconds != (ExactSeconds{"3601", "2"}) {
		t.Fatal(out, e)
	}
}
func TestDockUtilizationDailyEmptyPeriodFullDays(t *testing.T) {
	from, to, now := insightTime("2026-01-01T00:00:00Z"), insightTime("2026-02-01T00:00:00Z"), insightTime("2026-03-01T00:00:00Z")
	out, e := utilizationCompute(nil, from, to, now)
	if e != nil || len(out.Days) != 31 || out.TotalSegments != 0 {
		t.Fatal(out, e)
	}
	for _, d := range out.Days {
		if d.PeriodSeconds != (ExactSeconds{"86400", "1"}) || d.UnloadingFraction != (ExactSeconds{"0", "1"}) {
			t.Fatal(d)
		}
	}
}
