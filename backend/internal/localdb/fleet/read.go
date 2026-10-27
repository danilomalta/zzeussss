package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

func Get(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string) (Record, error) {
	if db == nil || !known(kind) || !ID(id) {
		return Record{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Record{}, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, financial(kind)); e != nil {
		return Record{}, e
	}
	r, e := load(ctx, tx, a, kind, id)
	if e == nil {
		e = proof(ctx, tx, a, r)
	}
	if e != nil {
		return Record{}, e
	}
	return r, tx.Commit()
}
func List(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind string, in Query) (Page, error) {
	out := Page{Items: []Record{}, Limit: in.Limit, Offset: in.Offset, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if db == nil || !known(kind) || in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.Offset > 1000000 || in.AssetID != "" && !ID(in.AssetID) || in.AssignedTo != "" && !ID(in.AssignedTo) || !text(in.State, 40, true) {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, financial(kind)); e != nil {
		return out, e
	}
	where := `tenant_id=? AND store_id=? AND kind=?`
	args := []any{a.TenantID, a.StoreID, kind}
	for _, f := range []struct{ key, value string }{{"asset_id", in.AssetID}, {"state", in.State}, {"assigned_to", in.AssignedTo}} {
		if f.value != "" {
			where += " AND " + f.key + "=?"
			args = append(args, f.value)
		}
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_records WHERE `+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM fleet_records WHERE `+where+` ORDER BY id LIMIT ? OFFSET ?`, append(args, in.Limit, in.Offset)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		r, e := scan(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, r := range out.Items {
		if e = proof(ctx, tx, a, r); e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
func Operation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Result, error) {
	if db == nil || !ID(id) {
		return Result{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return Result{}, e
	}
	out, e := receipt(ctx, tx, a, d.DeviceID, id, true)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func RecordHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string, limit, offset int) (History, error) {
	out := History{Items: []HistoryItem{}, Limit: limit, Offset: offset}
	if db == nil || !known(kind) || !ID(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, financial(kind)); e != nil {
		return out, e
	}
	r, e := load(ctx, tx, a, kind, id)
	if e != nil {
		return out, e
	}
	if e = proof(ctx, tx, a, r); e != nil {
		return out, e
	}
	out.Total = r.Revision
	rows, e := tx.QueryContext(ctx, `SELECT action,actor_id,device_id,operation_id FROM fleet_events WHERE tenant_id=? AND store_id=? AND kind=? AND record_id=? ORDER BY revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, kind, id, limit, offset)
	if e != nil {
		return out, e
	}
	type entry struct{ action, actor, device, op string }
	entries := []entry{}
	for rows.Next() {
		var v entry
		if e = rows.Scan(&v.action, &v.actor, &v.device, &v.op); e != nil {
			rows.Close()
			return out, e
		}
		entries = append(entries, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, v := range entries {
		result, e := receipt(ctx, tx, a, v.device, v.op, false)
		if e != nil {
			return out, e
		}
		var raw string
		if e = tx.QueryRowContext(ctx, `SELECT request_json FROM fleet_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, v.device, v.op).Scan(&raw); e != nil {
			return out, e
		}
		var request struct {
			Input Input `json:"input"`
		}
		if json.Unmarshal([]byte(raw), &request) != nil {
			return out, ErrConflict
		}
		out.Items = append(out.Items, HistoryItem{Action: v.action, ActorID: v.actor, DeviceID: v.device, Reason: request.Input.Reason, Result: result})
	}
	return out, tx.Commit()
}

type Availability struct {
	AssetID              string `json:"asset_id"`
	ManualState          string `json:"manual_state"`
	AvailableForDispatch bool   `json:"available_for_dispatch"`
	BlockingRequests     int64  `json:"blocking_requests"`
	ActiveTripID         string `json:"active_trip_id"`
	LivePosition         bool   `json:"live_position"`
	MeasuredAt           string `json:"measured_at"`
}

func AssetAvailability(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Availability, error) {
	out := Availability{AssetID: id, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if db == nil || !ID(id) {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return out, e
	}
	asset, _, e := assetTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out.ManualState = asset.State
	var trip string
	out.BlockingRequests, trip, e = operationalTx(ctx, tx, a, id, "")
	if e != nil {
		return out, e
	}
	out.ActiveTripID = trip
	out.AvailableForDispatch = asset.State == "available" && out.BlockingRequests == 0 && trip == ""
	return out, tx.Commit()
}

type PreventiveItem struct {
	Plan       Record `json:"plan"`
	DueByDate  bool   `json:"due_by_date"`
	DueByMeter bool   `json:"due_by_meter"`
}
type PreventivePage struct {
	Items            []PreventiveItem `json:"items"`
	Total            int64            `json:"total"`
	Limit            int              `json:"limit"`
	Offset           int              `json:"offset"`
	MeasuredAt       string           `json:"measured_at"`
	AutomaticRequest bool             `json:"automatic_request"`
}

func Preventive(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, assetID string, limit, offset int) (PreventivePage, error) {
	out := PreventivePage{Items: []PreventiveItem{}, Limit: limit, Offset: offset, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if db == nil || !ID(assetID) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return out, e
	}
	_, asset, e := assetTx(ctx, tx, a, assetID)
	if e != nil {
		return out, e
	}
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind='plan' AND asset_id=?`, a.TenantID, a.StoreID, assetID).Scan(&count); e != nil {
		return out, e
	}
	if count > MaxSource {
		return out, ErrLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind='plan' AND asset_id=? ORDER BY id`, a.TenantID, a.StoreID, assetID)
	if e != nil {
		return out, e
	}
	all := []Record{}
	for rows.Next() {
		r, e := scan(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		all = append(all, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, r := range all {
		if e = proof(ctx, tx, a, r); e != nil {
			return out, e
		}
		if r.State != "active" {
			continue
		}
		var p Plan
		if json.Unmarshal(r.Data, &p) != nil {
			return out, ErrConflict
		}
		dateDue := false
		if p.DueAt != "" {
			due, e := time.Parse(time.RFC3339, p.DueAt)
			if e != nil {
				return out, ErrConflict
			}
			dateDue = !due.After(time.Now().UTC())
		}
		meter, e := meterTx(ctx, tx, a, assetID, asset.MeterMilli)
		if e != nil {
			return out, e
		}
		meterDue := p.DueMeterMilli != nil && meter >= *p.DueMeterMilli
		if dateDue || meterDue {
			if out.Total >= int64(offset) && len(out.Items) < limit {
				out.Items = append(out.Items, PreventiveItem{r, dateDue, meterDue})
			}
			out.Total++
		}
	}
	return out, tx.Commit()
}

// A completed trip is also meter evidence. A later trip cannot start at an older manual reading.
func meterTx(ctx context.Context, tx *sql.Tx, a identity.Scope, assetID string, manual int64) (int64, error) {
	var count int
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind='trip' AND asset_id=?`, a.TenantID, a.StoreID, assetID).Scan(&count); e != nil {
		return 0, e
	}
	if count > MaxSource {
		return 0, ErrLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind='trip' AND asset_id=?`, a.TenantID, a.StoreID, assetID)
	if e != nil {
		return 0, e
	}
	all := []Record{}
	for rows.Next() {
		r, e := scan(rows)
		if e != nil {
			rows.Close()
			return 0, e
		}
		all = append(all, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, e
	}
	for _, r := range all {
		if e = proof(ctx, tx, a, r); e != nil {
			return 0, e
		}
		if r.State != "returned" {
			continue
		}
		var t Trip
		if json.Unmarshal(r.Data, &t) != nil || t.EndMeterMilli == nil || !number(*t.EndMeterMilli) {
			return 0, ErrConflict
		}
		if *t.EndMeterMilli > manual {
			manual = *t.EndMeterMilli
		}
	}
	return manual, nil
}
