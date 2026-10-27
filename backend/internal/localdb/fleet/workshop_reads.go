package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

func WorkshopGet(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string) (WorkshopResult, error) {
	if db == nil || !workshopKind(kind) || !ID(id) {
		return WorkshopResult{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return WorkshopResult{}, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return WorkshopResult{}, e
	}
	out, e := workshopLoad(ctx, tx, a, kind, id)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func WorkshopOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (WorkshopResult, error) {
	if db == nil || !ID(id) {
		return WorkshopResult{}, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return WorkshopResult{}, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, false); e != nil {
		return WorkshopResult{}, e
	}
	out, e := workshopReceipt(ctx, tx, a, d.DeviceID, id, true)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func WorkshopList(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, requestID, state string, limit, offset int) (WorkshopPage, error) {
	out := WorkshopPage{Items: []WorkshopResult{}, Limit: limit, Offset: offset}
	if db == nil || !workshopKind(kind) || requestID != "" && !ID(requestID) || !text(state, 40, true) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
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
	where := `tenant_id=? AND store_id=? AND kind=?`
	args := []any{a.TenantID, a.StoreID, kind}
	if requestID != "" {
		where += ` AND request_id=?`
		args = append(args, requestID)
	}
	if state != "" {
		where += ` AND state=?`
		args = append(args, state)
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_workshop_records WHERE `+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM fleet_workshop_records WHERE `+where+` ORDER BY id LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		r, e := workshopLoad(ctx, tx, a, kind, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, r)
	}
	return out, tx.Commit()
}
func WorkshopRecordHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, kind, id string, limit, offset int) (WorkshopHistory, error) {
	out := WorkshopHistory{Items: []WorkshopHistoryItem{}, Limit: limit, Offset: offset}
	if db == nil || !workshopKind(kind) || !ID(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
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
	current, e := workshopLoad(ctx, tx, a, kind, id)
	if e != nil {
		return out, e
	}
	out.Total = current.Revision
	rows, e := tx.QueryContext(ctx, `SELECT device_id,operation_id,request_json FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND kind=? AND id=? ORDER BY revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, kind, id, limit, offset)
	if e != nil {
		return out, e
	}
	type entry struct{ device, op, raw string }
	entries := []entry{}
	for rows.Next() {
		var v entry
		if e = rows.Scan(&v.device, &v.op, &v.raw); e != nil {
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
		r, e := workshopReceipt(ctx, tx, a, v.device, v.op, false)
		if e != nil {
			return out, e
		}
		var input struct {
			Input Input `json:"input"`
		}
		if json.Unmarshal([]byte(v.raw), &input) != nil {
			return out, ErrConflict
		}
		out.Items = append(out.Items, WorkshopHistoryItem{r, input.Input.Action, input.Input.Reason})
	}
	return out, tx.Commit()
}
func WorkshopReport(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, requestID string) (WorkshopSummary, error) {
	out := WorkshopSummary{RequestID: requestID, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if db == nil || !ID(requestID) {
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
	r, e := load(ctx, tx, a, "request", requestID)
	if e != nil {
		return out, e
	}
	if e = proof(ctx, tx, a, r); e != nil {
		return out, e
	}
	out.RequestRevision = r.Revision
	all, e := workshopRows(ctx, tx, a, `request_id=?`, []any{requestID}, MaxWorkshopSource)
	if e != nil {
		return out, e
	}
	var firstCount int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND revision=1 AND json_extract(response_json,'$.request_id')=?`, a.TenantID, a.StoreID, requestID).Scan(&firstCount); e != nil {
		return out, e
	}
	if firstCount != len(all) {
		return out, ErrConflict
	}
	out.SourceRecords = len(all)
	totals := map[string]int64{}
	var seq int64
	for _, v := range all {
		switch v.Kind {
		case "binding":
			out.ChecklistRequired = true
			out.BindingRevision = v.Revision
		case "labor":
			var log Labor
			if json.Unmarshal(v.Data, &log) != nil || log.Seconds < 1 || log.Seconds > 57600 {
				return out, ErrConflict
			}
			totals[v.State] += log.Seconds
		case "inspection":
			var n int64
			if e = tx.QueryRowContext(ctx, `SELECT seq FROM fleet_workshop_operations WHERE tenant_id=? AND store_id=? AND kind='inspection' AND id=? AND revision=1`, a.TenantID, a.StoreID, v.ID).Scan(&n); e != nil {
				return out, e
			}
			if n > seq {
				seq = n
				out.LatestInspectionID = v.ID
			}
		}
	}
	out.InspectionSatisfied, e = inspectionGate(ctx, tx, a, r)
	if e != nil {
		return out, e
	}
	out.DeclaredSecondsExact = strconv.FormatInt(totals["declared"], 10)
	out.ConfirmedSecondsExact = strconv.FormatInt(totals["confirmed"], 10)
	out.RejectedSecondsExact = strconv.FormatInt(totals["rejected"], 10)
	out.VoidedSecondsExact = strconv.FormatInt(totals["voided"], 10)
	return out, tx.Commit()
}
