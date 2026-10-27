package dockagenda

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"sort"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

var ErrInsightLimit = errors.New("dock insight source exceeds 5000 appointments; narrow filters")

const MaxInsightRows = 5000

type Gap struct {
	StartsAt       string `json:"starts_at"`
	EndsAt         string `json:"ends_at"`
	FirstFitEndsAt string `json:"first_fit_ends_at"`
}
type AvailabilityInput struct {
	DockID          string
	From            string
	To              string
	DurationMinutes int64
}
type FreeWindows struct {
	Dock               Dock         `json:"dock"`
	MeasuredAt         string       `json:"measured_at"`
	From               string       `json:"from"`
	To                 string       `json:"to"`
	DurationMinutes    int64        `json:"duration_minutes"`
	Basis              string       `json:"basis"`
	ConfirmedAvailable bool         `json:"confirmed_available"`
	ActiveUnloading    *Appointment `json:"active_unloading"`
	WaitingCount       int64        `json:"waiting_count"`
	OccupiedWindows    int          `json:"occupied_windows"`
	Windows            []Gap        `json:"windows"`
}
type InsightFilter struct {
	From       string
	To         string
	DockID     string
	SupplierID string
	Limit      int
	Offset     int
}
type ExactSeconds struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}
type SupplierMetrics struct {
	SupplierID              string        `json:"supplier_id"`
	Appointments            int64         `json:"appointments"`
	Reserved                int64         `json:"reserved"`
	Arrived                 int64         `json:"arrived"`
	Unloading               int64         `json:"unloading"`
	Completed               int64         `json:"completed"`
	Cancelled               int64         `json:"cancelled"`
	NoShow                  int64         `json:"no_show"`
	ArrivalSamples          int64         `json:"arrival_samples"`
	LateArrivals            int64         `json:"late_arrivals"`
	WaitingSamples          int64         `json:"waiting_samples"`
	UnloadingSamples        int64         `json:"unloading_samples"`
	AverageWaitingSeconds   *ExactSeconds `json:"average_waiting_seconds"`
	AverageUnloadingSeconds *ExactSeconds `json:"average_unloading_seconds"`
}
type Performance struct {
	From           string            `json:"from"`
	To             string            `json:"to"`
	MeasuredAt     string            `json:"measured_at"`
	Basis          string            `json:"basis"`
	TotalSuppliers int               `json:"total_suppliers"`
	Limit          int               `json:"limit"`
	Offset         int               `json:"offset"`
	All            SupplierMetrics   `json:"all_suppliers"`
	Items          []SupplierMetrics `json:"items"`
}
type PendingItem struct {
	Appointment    Appointment `json:"appointment"`
	Reason         string      `json:"reason"`
	OrderCancelled bool        `json:"order_cancelled"`
	Priority       int         `json:"priority"`
}
type PendingPage struct {
	MeasuredAt string        `json:"measured_at"`
	Items      []PendingItem `json:"items"`
	Total      int           `json:"total"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
}

func insightRange(from, to string, maxDays int) (time.Time, time.Time, error) {
	a, e := time.Parse(time.RFC3339Nano, from)
	b, f := time.Parse(time.RFC3339Nano, to)
	if e != nil || f != nil || a.Year() < 2000 || b.Year() > 2200 || a.Nanosecond() != 0 || b.Nanosecond() != 0 || !a.Before(b) || b.Sub(a) > time.Duration(maxDays)*24*time.Hour {
		return a, b, ErrInvalid
	}
	return a.UTC(), b.UTC(), nil
}
func freeGaps(from, to time.Time, duration time.Duration, occupied []Appointment) ([]Gap, error) {
	out := []Gap{}
	cursor := from
	var priorEnd time.Time
	add := func(a, b time.Time) {
		if b.Sub(a) >= duration {
			out = append(out, Gap{a.Format(time.RFC3339), b.Format(time.RFC3339), a.Add(duration).Format(time.RFC3339)})
		}
	}
	for _, row := range occupied {
		start, e := time.Parse(time.RFC3339, row.StartsAt)
		end, f := time.Parse(time.RFC3339, row.EndsAt)
		if e != nil || f != nil || !start.Before(end) {
			return nil, ErrConflict
		}
		if !priorEnd.IsZero() && start.Before(priorEnd) {
			return nil, ErrConflict
		}
		priorEnd = end
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		if !start.Before(end) {
			continue
		}
		if start.After(cursor) {
			add(cursor, start)
		}
		if end.After(cursor) {
			cursor = end
		}
	}
	if cursor.Before(to) {
		add(cursor, to)
	}
	return out, nil
}

// FreeWindows are PLANNED gaps; observed occupancy never becomes a guarantee.
func AvailableWindows(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in AvailabilityInput) (FreeWindows, error) {
	out := FreeWindows{MeasuredAt: stamp(), DurationMinutes: in.DurationMinutes, Basis: "planned_gaps_require_new_atomic_reservation", Windows: []Gap{}}
	from, to, e := insightRange(in.From, in.To, 7)
	if e != nil || !validID(in.DockID) || in.DurationMinutes < 5 || in.DurationMinutes > 480 {
		return out, ErrInvalid
	}
	out.From = from.Format(time.RFC3339)
	out.To = to.Format(time.RFC3339)
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.Dock, e = dockTx(ctx, tx, a, in.DockID)
	if e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND dock_id=? AND status IN('reserved','arrived','unloading') AND starts_unix<? AND ends_unix>? ORDER BY starts_unix,id LIMIT 5001`, a.TenantID, a.StoreID, in.DockID, to.Unix(), from.Unix())
	if e != nil {
		return out, e
	}
	occupied := []Appointment{}
	for rows.Next() {
		if len(occupied) >= MaxInsightRows {
			rows.Close()
			return out, ErrInsightLimit
		}
		r, e := scanAppointment(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		occupied = append(occupied, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	out.OccupiedWindows = len(occupied)
	active, e := scanAppointment(tx.QueryRowContext(ctx, `SELECT `+columns+` FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND dock_id=? AND status='unloading'`, a.TenantID, a.StoreID, in.DockID))
	if e == nil {
		out.ActiveUnloading = &active
	} else if !errors.Is(e, ErrNotFound) {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND dock_id=? AND status='arrived'`, a.TenantID, a.StoreID, in.DockID).Scan(&out.WaitingCount); e != nil {
		return out, e
	}
	if out.Dock.Enabled {
		out.Windows, e = freeGaps(from, to, time.Duration(in.DurationMinutes)*time.Minute, occupied)
		if e != nil {
			return out, e
		}
	} else {
		out.Basis = "dock_disabled_no_windows"
	}
	return out, tx.Commit()
}
func insightWhere(a identity.Scope, in InsightFilter) (string, []any, error) {
	if !pageOK(in.Limit, in.Offset) || (in.DockID != "" && !validID(in.DockID)) || (in.SupplierID != "" && !validID(in.SupplierID)) {
		return "", nil, ErrInvalid
	}
	where := `tenant_id=? AND store_id=?`
	args := []any{a.TenantID, a.StoreID}
	if in.DockID != "" {
		where += ` AND dock_id=?`
		args = append(args, in.DockID)
	}
	if in.SupplierID != "" {
		where += ` AND supplier_id=?`
		args = append(args, in.SupplierID)
	}
	return where, args, nil
}
func insightRows(ctx context.Context, tx *sql.Tx, where string, args []any) ([]Appointment, error) {
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM dock_agenda_appointments WHERE `+where+` ORDER BY starts_unix,id LIMIT 5001`, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Appointment{}
	for rows.Next() {
		if len(out) >= MaxInsightRows {
			return nil, ErrInsightLimit
		}
		v, e := scanAppointment(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type metricsAccumulator struct {
	r            SupplierMetrics
	wait, unload *big.Int
}

func metricNew(id string) *metricsAccumulator {
	return &metricsAccumulator{r: SupplierMetrics{SupplierID: id}, wait: new(big.Int), unload: new(big.Int)}
}
func metricAdd(m *metricsAccumulator, r Appointment) error {
	m.r.Appointments++
	switch r.Status {
	case "reserved":
		m.r.Reserved++
	case "arrived":
		m.r.Arrived++
	case "unloading":
		m.r.Unloading++
	case "completed":
		m.r.Completed++
	case "cancelled":
		m.r.Cancelled++
	case "no_show":
		m.r.NoShow++
	}
	if r.ArrivedAt != nil && r.Status != "cancelled" {
		arrived, e := time.Parse(time.RFC3339Nano, *r.ArrivedAt)
		start, f := time.Parse(time.RFC3339, r.StartsAt)
		if e != nil || f != nil {
			return ErrConflict
		}
		m.r.ArrivalSamples++
		if int64(arrived.Sub(start)/time.Second) > 0 {
			m.r.LateArrivals++
		}
	}
	if r.UnloadingAt != nil {
		if r.ArrivedAt == nil {
			return ErrConflict
		}
		n, e := metricDuration(*r.ArrivedAt, *r.UnloadingAt)
		if e != nil {
			return e
		}
		m.r.WaitingSamples++
		m.wait.Add(m.wait, n)
	}
	if r.CompletedAt != nil {
		if r.UnloadingAt == nil {
			return ErrConflict
		}
		n, e := metricDuration(*r.UnloadingAt, *r.CompletedAt)
		if e != nil {
			return e
		}
		m.r.UnloadingSamples++
		m.unload.Add(m.unload, n)
	}
	return nil
}
func metricDone(m *metricsAccumulator) SupplierMetrics {
	r := m.r
	exact := func(total *big.Int, n int64) *ExactSeconds {
		if n == 0 {
			return nil
		}
		v := new(big.Rat).SetFrac(total, new(big.Int).Mul(big.NewInt(n), big.NewInt(int64(time.Second))))
		return &ExactSeconds{v.Num().String(), v.Denom().String()}
	}
	r.AverageWaitingSeconds = exact(m.wait, r.WaitingSamples)
	r.AverageUnloadingSeconds = exact(m.unload, r.UnloadingSamples)
	return r
}
func SupplierPerformance(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in InsightFilter) (Performance, error) {
	out := Performance{MeasuredAt: stamp(), Basis: "same_store_planned_start_cohort_not_supplier_fault_or_ranking", Items: []SupplierMetrics{}, Limit: in.Limit, Offset: in.Offset}
	from, to, e := insightRange(in.From, in.To, 366)
	if e != nil {
		return out, e
	}
	out.From = from.Format(time.RFC3339)
	out.To = to.Format(time.RFC3339)
	where, args, e := insightWhere(a, in)
	if e != nil {
		return out, e
	}
	where += ` AND starts_unix>=? AND starts_unix<?`
	args = append(args, from.Unix(), to.Unix())
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	rows, e := insightRows(ctx, tx, where, args)
	if e != nil {
		return out, e
	}
	all := metricNew("")
	groups := map[string]*metricsAccumulator{}
	for _, r := range rows {
		m := groups[r.SupplierID]
		if m == nil {
			m = metricNew(r.SupplierID)
			groups[r.SupplierID] = m
		}
		if e = metricAdd(m, r); e != nil {
			return out, e
		}
		if e = metricAdd(all, r); e != nil {
			return out, e
		}
	}
	ids := []string{}
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out.TotalSuppliers = len(ids)
	out.All = metricDone(all)
	end := in.Offset + in.Limit
	if end > len(ids) {
		end = len(ids)
	}
	for index := in.Offset; index < end; index++ {
		out.Items = append(out.Items, metricDone(groups[ids[index]]))
	}
	return out, tx.Commit()
}
func Pending(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in InsightFilter) (PendingPage, error) {
	out := PendingPage{MeasuredAt: stamp(), Items: []PendingItem{}, Limit: in.Limit, Offset: in.Offset}
	if in.From != "" || in.To != "" {
		return out, ErrInvalid
	}
	where, args, e := insightWhere(a, in)
	if e != nil {
		return out, e
	}
	where += ` AND status IN('reserved','arrived','unloading')`
	tx, e := readTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	rows, e := insightRows(ctx, tx, where, args)
	if e != nil {
		return out, e
	}
	all := []PendingItem{}
	now, _ := time.Parse(time.RFC3339Nano, out.MeasuredAt)
	for _, r := range rows {
		item := PendingItem{Appointment: r}
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_order_cancellations WHERE tenant_id=? AND store_id=? AND order_id=?)`, a.TenantID, a.StoreID, r.OrderID).Scan(&item.OrderCancelled); e != nil {
			return out, e
		}
		end, _ := time.Parse(time.RFC3339, r.EndsAt)
		switch {
		case item.OrderCancelled:
			item.Reason = "order_cancelled_local_resolution_required"
			item.Priority = 1
		case r.Status == "unloading":
			item.Reason = "unloading_in_progress"
			item.Priority = 2
		case r.Status == "arrived":
			item.Reason = "arrived_waiting"
			item.Priority = 3
		case now.After(end):
			item.Reason = "planned_end_passed_review_required"
			item.Priority = 4
		default:
			continue
		}
		all = append(all, item)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Priority != all[j].Priority {
			return all[i].Priority < all[j].Priority
		}
		if all[i].Appointment.StartsAt != all[j].Appointment.StartsAt {
			return all[i].Appointment.StartsAt < all[j].Appointment.StartsAt
		}
		return all[i].Appointment.ID < all[j].Appointment.ID
	})
	out.Total = len(all)
	if in.Offset < len(all) {
		end := in.Offset + in.Limit
		if end > len(all) {
			end = len(all)
		}
		out.Items = all[in.Offset:end]
	}
	return out, tx.Commit()
}

func metricDuration(from, to string) (*big.Int, error) {
	a, e := time.Parse(time.RFC3339Nano, from)
	b, f := time.Parse(time.RFC3339Nano, to)
	if e != nil || f != nil || a.Year() < 2000 || a.Year() > 2200 || b.Year() < 2000 || b.Year() > 2200 || b.Before(a) {
		return nil, ErrConflict
	}
	return big.NewInt(int64(b.Sub(a))), nil
}
