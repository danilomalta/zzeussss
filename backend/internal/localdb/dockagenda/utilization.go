package dockagenda

import (
	"context"
	"database/sql"
	"math/big"
	"sort"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

// Utilization is a local historical observation, never a promise of capacity.
type UtilizationInput struct {
	DockID, From, To string
	Limit, Offset    int
}
type OccupiedSegment struct {
	AppointmentID  string       `json:"appointment_id"`
	OrderID        string       `json:"order_id"`
	SupplierID     string       `json:"supplier_id"`
	Status         string       `json:"status"`
	StartsAt       string       `json:"starts_at"`
	EndsAt         string       `json:"ends_at"`
	Seconds        ExactSeconds `json:"seconds"`
	StillUnloading bool         `json:"still_unloading"`
}
type UtilizationDay struct {
	Day               string       `json:"day_utc"`
	From              string       `json:"from"`
	To                string       `json:"to"`
	PeriodSeconds     ExactSeconds `json:"period_seconds"`
	PlannedSeconds    ExactSeconds `json:"planned_seconds"`
	UnloadingSeconds  ExactSeconds `json:"unloading_seconds"`
	UnloadingFraction ExactSeconds `json:"unloading_fraction"`
}
type UtilizationReport struct {
	Dock                              Dock              `json:"dock"`
	From                              string            `json:"from"`
	To                                string            `json:"to"`
	MeasuredAt                        string            `json:"measured_at"`
	Basis                             string            `json:"basis"`
	PeriodSeconds                     ExactSeconds      `json:"period_seconds"`
	PlannedSeconds                    ExactSeconds      `json:"planned_seconds"`
	UnloadingSeconds                  ExactSeconds      `json:"unloading_seconds"`
	WithoutRegisteredUnloadingSeconds ExactSeconds      `json:"without_registered_unloading_seconds"`
	PlannedFraction                   ExactSeconds      `json:"planned_fraction"`
	UnloadingFraction                 ExactSeconds      `json:"unloading_fraction"`
	PlannedAppointments               int               `json:"planned_appointments"`
	TotalSegments                     int               `json:"total_segments"`
	SourceAppointments                int               `json:"source_appointments"`
	Limit                             int               `json:"limit"`
	Offset                            int               `json:"offset"`
	Items                             []OccupiedSegment `json:"items"`
	Days                              []UtilizationDay  `json:"days"`
}
type utilizationSpan struct {
	from, to time.Time
	row      Appointment
}

func exactRatio(n, d int64) ExactSeconds {
	r := new(big.Rat).SetFrac(big.NewInt(n), big.NewInt(d))
	return ExactSeconds{Numerator: r.Num().String(), Denominator: r.Denom().String()}
}
func utilizationTime(raw string) (time.Time, error) {
	t, e := time.Parse(time.RFC3339Nano, raw)
	if e != nil || t.Year() < 2000 || t.Year() > 2200 {
		return t, ErrConflict
	}
	return t.UTC(), nil
}
func clippedSpan(a, b, from, to time.Time) (time.Time, time.Time, bool) {
	if a.Before(from) {
		a = from
	}
	if b.After(to) {
		b = to
	}
	return a, b, a.Before(b)
}
func utilizationCompute(rows []Appointment, from, to, now time.Time) (UtilizationReport, error) {
	out := UtilizationReport{From: from.Format(time.RFC3339), To: to.Format(time.RFC3339), MeasuredAt: now.Format(time.RFC3339Nano), Basis: "registered_unloading_wall_clock_latest_schedule_not_business_hours_or_free_capacity", Items: []OccupiedSegment{}, SourceAppointments: len(rows)}
	planned, actual := []utilizationSpan{}, []utilizationSpan{}
	for _, r := range rows {
		start, e := utilizationTime(r.StartsAt)
		end, f := utilizationTime(r.EndsAt)
		if e != nil || f != nil || !start.Before(end) {
			return out, ErrConflict
		}
		switch r.Status {
		case "reserved", "arrived", "unloading", "completed", "cancelled", "no_show":
		default:
			return out, ErrConflict
		}
		if r.Status != "cancelled" && r.Status != "no_show" {
			a, b, ok := clippedSpan(start, end, from, to)
			if ok {
				planned = append(planned, utilizationSpan{a, b, r})
				out.PlannedAppointments++
			}
		}
		// Reject contradictory lifecycle evidence; do not manufacture plausible totals.
		if (r.Status == "unloading" || r.Status == "completed") != (r.UnloadingAt != nil) || (r.Status == "completed") != (r.CompletedAt != nil) {
			return out, ErrConflict
		}
		if r.UnloadingAt == nil {
			continue
		}
		if r.ArrivedAt == nil {
			return out, ErrConflict
		}
		arrival, e := utilizationTime(*r.ArrivedAt)
		a, f := utilizationTime(*r.UnloadingAt)
		if e != nil || f != nil || a.Before(arrival) || a.After(now) {
			return out, ErrConflict
		}
		b := now
		if r.CompletedAt != nil {
			b, e = utilizationTime(*r.CompletedAt)
			if e != nil || b.After(now) {
				return out, ErrConflict
			}
		}
		if b.Before(a) {
			return out, ErrConflict
		}
		a, b, ok := clippedSpan(a, b, from, to)
		if ok {
			actual = append(actual, utilizationSpan{a, b, r})
		}
	}
	sortSpans := func(v []utilizationSpan) {
		sort.Slice(v, func(i, j int) bool {
			if !v[i].from.Equal(v[j].from) {
				return v[i].from.Before(v[j].from)
			}
			return v[i].row.ID < v[j].row.ID
		})
	}
	sortSpans(planned)
	sortSpans(actual)
	// Planned intervals may overlap when historical completed slots are reused.
	// Measure their union, never sum the same wall-clock interval twice.
	var planNs, actualNs int64
	var cursor time.Time
	for _, v := range planned {
		a := v.from
		if a.Before(cursor) {
			a = cursor
		}
		if v.to.After(a) {
			planNs += int64(v.to.Sub(a))
		}
		if v.to.After(cursor) {
			cursor = v.to
		}
	}
	cursor = time.Time{}
	for _, v := range actual {
		if !cursor.IsZero() && v.from.Before(cursor) {
			return out, ErrConflict
		}
		cursor = v.to
		n := int64(v.to.Sub(v.from))
		actualNs += n
		out.Items = append(out.Items, OccupiedSegment{v.row.ID, v.row.OrderID, v.row.SupplierID, v.row.Status, v.from.Format(time.RFC3339Nano), v.to.Format(time.RFC3339Nano), exactRatio(n, int64(time.Second)), v.row.Status == "unloading"})
	}
	period := int64(to.Sub(from))
	out.PeriodSeconds = exactRatio(period, int64(time.Second))
	out.PlannedSeconds = exactRatio(planNs, int64(time.Second))
	out.UnloadingSeconds = exactRatio(actualNs, int64(time.Second))
	out.WithoutRegisteredUnloadingSeconds = exactRatio(period-actualNs, int64(time.Second))
	out.PlannedFraction = exactRatio(planNs, period)
	out.UnloadingFraction = exactRatio(actualNs, period)
	out.TotalSegments = len(out.Items)
	// UTC calendar buckets for charts. Partial first/last days use only the
	// requested interval as denominator, never an invented full 24-hour shift.
	for begin := from; begin.Before(to); {
		next := time.Date(begin.Year(), begin.Month(), begin.Day()+1, 0, 0, 0, 0, time.UTC)
		if next.After(to) {
			next = to
		}
		p := spanUnionNanos(planned, begin, next)
		u := spanUnionNanos(actual, begin, next)
		n := int64(next.Sub(begin))
		out.Days = append(out.Days, UtilizationDay{begin.Format("2006-01-02"), begin.Format(time.RFC3339), next.Format(time.RFC3339), exactRatio(n, int64(time.Second)), exactRatio(p, int64(time.Second)), exactRatio(u, int64(time.Second)), exactRatio(u, n)})
		begin = next
	}
	return out, nil
}

// Input sorted by start; union also handles overlapping planned snapshots.
func spanUnionNanos(spans []utilizationSpan, from, to time.Time) int64 {
	var total int64
	cursor := from
	for _, v := range spans {
		a, b, ok := clippedSpan(v.from, v.to, from, to)
		if !ok {
			continue
		}
		if a.Before(cursor) {
			a = cursor
		}
		if b.After(a) {
			total += int64(b.Sub(a))
		}
		if b.After(cursor) {
			cursor = b
		}
	}
	return total
}

func Utilization(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in UtilizationInput) (UtilizationReport, error) {
	var out UtilizationReport
	from, to, e := insightRange(in.From, in.To, 31)
	now := time.Now().UTC()
	if e != nil || to.After(now) || !validID(in.DockID) || !pageOK(in.Limit, in.Offset) {
		return out, ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	dock, e := dockTx(ctx, tx, a, in.DockID)
	if e != nil {
		return out, e
	}
	// Include unloading whose planned slot is outside the requested interval.
	// Bound the entire selected dock history, rather than silently omit it.
	rows, e := insightRows(ctx, tx, "tenant_id=? AND store_id=? AND dock_id=?", []any{a.TenantID, a.StoreID, in.DockID})
	if e != nil {
		return out, e
	}
	// Timestamp after materializing the snapshot avoids treating a concurrent
	// committed discharge just before the read as evidence from the future.
	out, e = utilizationCompute(rows, from, to, time.Now().UTC())
	if e != nil {
		return out, e
	}
	out.Dock = dock
	out.Limit = in.Limit
	out.Offset = in.Offset
	end := in.Offset + in.Limit
	if end > len(out.Items) {
		end = len(out.Items)
	}
	if in.Offset >= len(out.Items) {
		out.Items = []OccupiedSegment{}
	} else {
		out.Items = out.Items[in.Offset:end]
	}
	return out, tx.Commit()
}
