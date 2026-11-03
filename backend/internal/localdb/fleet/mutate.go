package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func assetTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Record, Asset, error) {
	var v Asset
	if !ID(id) {
		return Record{}, v, ErrInvalid
	}
	r, e := load(ctx, tx, a, "asset", id)
	if e == nil {
		e = proof(ctx, tx, a, r)
	}
	if e == nil && !validate("asset", r.Data) {
		e = ErrConflict
	}
	if e == nil {
		e = json.Unmarshal(r.Data, &v)
	}
	return r, v, e
}
func linked(ctx context.Context, tx *sql.Tx, a identity.Scope, kind, id, asset string) error {
	if id == "" {
		return nil
	}
	if !ID(id) {
		return ErrInvalid
	}
	r, e := load(ctx, tx, a, kind, id)
	if e != nil {
		return e
	}
	if e = proof(ctx, tx, a, r); e != nil {
		return e
	}
	if kind == "provider" && r.State != "active" || kind == "request" && r.AssetID != asset {
		return ErrConflict
	}
	return nil
}
func empty(raw []byte) error { var v struct{}; return decode(raw, &v) }
func total(values ...int64) (int64, error) {
	var n int64
	for _, v := range values {
		if !number(v) || n > MaxValue-v {
			return 0, ErrInvalid
		}
		n += v
	}
	return n, nil
}

// Operational state is derived only from records whose current state matches their receipt.
func operationalTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id, ignoreTrip string) (int64, string, error) {
	var count int
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_records WHERE tenant_id=? AND store_id=? AND asset_id=? AND kind IN('request','trip')`, a.TenantID, a.StoreID, id).Scan(&count); e != nil {
		return 0, "", e
	}
	if count > MaxSource {
		return 0, "", ErrLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM fleet_records WHERE tenant_id=? AND store_id=? AND asset_id=? AND kind IN('request','trip') ORDER BY kind,id`, a.TenantID, a.StoreID, id)
	if e != nil {
		return 0, "", e
	}
	all := []Record{}
	for rows.Next() {
		r, e := scan(rows)
		if e != nil {
			rows.Close()
			return 0, "", e
		}
		all = append(all, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return 0, "", e
	}
	var blocked int64
	trip := ""
	for _, r := range all {
		if e = proof(ctx, tx, a, r); e != nil {
			return 0, "", e
		}
		if r.Kind == "trip" && r.ID != ignoreTrip && oneOf(r.State, "planned", "departed") {
			if trip != "" {
				return 0, "", ErrConflict
			}
			trip = r.ID
		}
		if r.Kind == "request" {
			var p Request
			if json.Unmarshal(r.Data, &p) != nil {
				return 0, "", ErrConflict
			}
			if oneOf(r.State, "in_service", "waiting_parts", "inspection", "cancelled_blocked") || oneOf(r.State, "open", "triaged", "approved") && p.Blocks {
				blocked++
			}
		}
	}
	return blocked, trip, nil
}
func blockedTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (bool, error) {
	n, _, e := operationalTx(ctx, tx, a, id, "")
	return n > 0, e
}
func availableTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id, ignoreTrip string) (bool, error) {
	asset, _, e := assetTx(ctx, tx, a, id)
	if e != nil {
		return false, e
	}
	blocked, trip, e := operationalTx(ctx, tx, a, id, ignoreTrip)
	return asset.State == "available" && blocked == 0 && trip == "", e
}

// Mutate is the typed domain boundary for the finite set of fleet resources.
func Mutate(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, kind string, in Input) (Result, error) {
	money := financial(kind) || kind == "request" && in.Action == "approve"
	managerOnly := kind != "request" && !(kind == "trip" && in.ExpectedRevision > 0)
	return write(ctx, db, c, a, d, kind, in, money, managerOnly, func(tx *sql.Tx, r *Record, manager bool, now string) error {
		switch kind {
		case "asset":
			return assetChange(ctx, tx, a, r, in)
		case "provider":
			if oneOf(in.Action, "create", "update") {
				if !validate(kind, in.Data) {
					return ErrInvalid
				}
				r.Data = in.Data
				if in.Action == "create" {
					r.State = "active"
				}
				return nil
			}
			if !oneOf(in.Action, "enable", "disable") || empty(in.Data) != nil {
				return ErrInvalid
			}
			if in.Action == "enable" {
				r.State = "active"
			} else {
				r.State = "inactive"
			}
			return nil
		case "request":
			return requestChange(ctx, tx, a, d, r, in, manager, now)
		case "trip":
			return tripChange(ctx, tx, a, d, r, in, manager, now)
		case "plan":
			var p Plan
			if oneOf(in.Action, "create", "update") {
				if decode(in.Data, &p, "asset_id", "label", "due_at", "due_meter_milli", "note") != nil || !ID(p.AssetID) || !text(p.Label, 160, false) || !text(p.Note, 1000, true) || p.DueAt == "" && p.DueMeterMilli == nil || p.DueMeterMilli != nil && !number(*p.DueMeterMilli) {
					return ErrInvalid
				}
				due, e := stamp(p.DueAt, true)
				if e != nil {
					return e
				}
				p.DueAt = due
				_, asset, e := assetTx(ctx, tx, a, p.AssetID)
				if e != nil {
					return e
				}
				if p.DueMeterMilli != nil && asset.MeterBasis == "none" {
					return ErrInvalid
				}
				if r.AssetID != "" && r.AssetID != p.AssetID {
					return ErrInvalid
				}
				r.AssetID = p.AssetID
				r.Data = encode(p)
				r.State = "active"
				return nil
			}
			if in.Action != "disable" || empty(in.Data) != nil {
				return ErrInvalid
			}
			r.State = "inactive"
			return nil
		case "quote":
			return quoteChange(ctx, tx, a, r, in)
		case "expense", "income", "impact":
			return amountChange(ctx, tx, a, r, in)
		case "profile":
			return profileChange(ctx, tx, a, r, in)
		}
		return ErrInvalid
	})
}
func assetChange(ctx context.Context, tx *sql.Tx, a identity.Scope, r *Record, in Input) error {
	if oneOf(in.Action, "create", "update", "correct_meter") {
		if !validate("asset", in.Data) {
			return ErrInvalid
		}
		var next, prior Asset
		_ = json.Unmarshal(in.Data, &next)
		if in.Action != "create" {
			_ = json.Unmarshal(r.Data, &prior)
			if prior.MeterBasis != next.MeterBasis {
				return ErrConflict
			}
			if next.MeterMilli < prior.MeterMilli && in.Action != "correct_meter" {
				return ErrConflict
			}
		}
		if in.Action == "create" {
			r.State = "available"
		}
		r.Data = in.Data
		return nil
	}
	if empty(in.Data) != nil {
		return ErrInvalid
	}
	switch in.Action {
	case "block":
		if r.State == "retired" {
			return ErrConflict
		}
		r.State = "blocked"
	case "release":
		if r.State == "retired" {
			return ErrConflict
		}
		blocked, e := blockedTx(ctx, tx, a, r.ID)
		if e != nil {
			return e
		}
		if blocked {
			return ErrConflict
		}
		r.State = "available"
	case "retire":
		var n int
		if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind='trip' AND asset_id=? AND state IN('planned','departed')`, a.TenantID, a.StoreID, r.ID).Scan(&n); e != nil {
			return e
		}
		if n != 0 {
			return ErrConflict
		}
		blocked, e := blockedTx(ctx, tx, a, r.ID)
		if e != nil {
			return e
		}
		if blocked {
			return ErrConflict
		}
		r.State = "retired"
	case "activate":
		if r.State != "retired" {
			return ErrConflict
		}
		r.State = "blocked" // Reactivation needs a separate release after review.
	default:
		return ErrInvalid
	}
	return nil
}
func requestChange(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, r *Record, in Input, manager bool, now string) error {
	if in.Action == "create" {
		var p OpenRequest
		if decode(in.Data, &p, "asset_id", "issue", "priority", "location", "blocks", "impact", "reference") != nil || !ID(p.AssetID) || !text(p.Issue, 1000, false) || !text(p.Location, 240, false) || !text(p.Impact, 500, true) || !text(p.Reference, 128, true) || !oneOf(p.Priority, "normal", "urgent", "emergency") {
			return ErrInvalid
		}
		asset, _, e := assetTx(ctx, tx, a, p.AssetID)
		if e != nil {
			return e
		}
		if asset.State == "retired" {
			return ErrConflict
		}
		r.AssetID = p.AssetID
		r.State = "open"
		r.Data = encode(Request{OpenRequest: p, RequesterID: a.IdentityID})
		return nil
	}
	var p Request
	var action RequestAction
	if json.Unmarshal(r.Data, &p) != nil || decode(in.Data, &action, "assigned_to", "quote_id") != nil {
		return ErrInvalid
	}
	if in.Action != "assign" && action.AssignedTo != "" || in.Action != "approve" && action.QuoteID != "" {
		return ErrInvalid
	}
	assigned := r.AssignedTo == a.IdentityID
	closed := oneOf(r.State, "released", "cancelled")
	switch in.Action {
	case "triage":
		if r.State != "open" {
			return ErrConflict
		}
		if r.AssignedTo != "" && !assigned && !manager {
			return identity.ErrDenied
		}
		r.AssignedTo = a.IdentityID
		r.State = "triaged"
	case "assign":
		if !manager {
			return identity.ErrDenied
		}
		if closed || r.State == "cancelled_blocked" {
			return ErrConflict
		}
		if e := assignee(ctx, tx, a, d, action.AssignedTo); e != nil {
			return e
		}
		r.AssignedTo = action.AssignedTo
	case "approve":
		if !manager {
			return identity.ErrDenied
		}
		if r.State != "triaged" || r.AssignedTo == "" || !ID(action.QuoteID) {
			return ErrConflict
		}
		if e := assignee(ctx, tx, a, d, r.AssignedTo); e != nil {
			return e
		}
		q, e := load(ctx, tx, a, "quote", action.QuoteID)
		if e != nil {
			return e
		}
		if e = proof(ctx, tx, a, q); e != nil {
			return e
		}
		var quote Quote
		if json.Unmarshal(q.Data, &quote) != nil || quote.RequestID != r.ID || q.AssetID != r.AssetID {
			return ErrConflict
		}
		p.QuoteID = q.ID
		r.State = "approved"
	case "start":
		if !manager && !assigned {
			return identity.ErrDenied
		}
		if !oneOf(r.State, "approved", "waiting_parts") {
			return ErrConflict
		}
		if r.AssignedTo == "" {
			r.AssignedTo = a.IdentityID
		}
		r.State = "in_service"
	case "wait":
		if !manager && !assigned {
			return identity.ErrDenied
		}
		if r.State != "in_service" {
			return ErrConflict
		}
		r.State = "waiting_parts"
	case "inspect":
		if !manager && !assigned {
			return identity.ErrDenied
		}
		if r.State != "in_service" {
			return ErrConflict
		}
		if okay, e := inspectionGate(ctx, tx, a, *r); e != nil {
			return e
		} else if !okay {
			return ErrConflict
		}
		r.State = "inspection"
	case "release":
		if !manager {
			return identity.ErrDenied
		}
		if r.State != "inspection" {
			return ErrConflict
		}
		if okay, e := inspectionGate(ctx, tx, a, *r); e != nil {
			return e
		} else if !okay {
			return ErrConflict
		}
		r.State = "released"
	case "cancel":
		if !manager && !(p.RequesterID == a.IdentityID && oneOf(r.State, "open", "triaged")) {
			return identity.ErrDenied
		}
		if closed || r.State == "cancelled_blocked" {
			return ErrConflict
		}
		if oneOf(r.State, "open", "triaged") {
			r.State = "cancelled"
		} else {
			r.State = "cancelled_blocked"
		}
	case "clear_hold":
		if !manager {
			return identity.ErrDenied
		}
		if r.State != "cancelled_blocked" {
			return ErrConflict
		}
		r.State = "cancelled"
	case "reopen":
		if !manager {
			return identity.ErrDenied
		}
		if !closed {
			return ErrConflict
		}
		r.State = "open"
		r.AssignedTo = ""
		p.QuoteID = ""
	case "note":
		if closed {
			return ErrConflict
		}
		if !manager && !assigned && p.RequesterID != a.IdentityID {
			return identity.ErrDenied
		}
	default:
		return ErrInvalid
	}
	r.Data = encode(p)
	_ = now
	return nil
}
func tripChange(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, r *Record, in Input, manager bool, now string) error {
	if in.Action == "create" {
		var p TripOpen
		if decode(in.Data, &p, "asset_id", "driver_id", "reference", "description") != nil || !ID(p.AssetID) || !ID(p.DriverID) || !text(p.Reference, 128, false) || !text(p.Description, 1000, true) {
			return ErrInvalid
		}
		if e := assignee(ctx, tx, a, d, p.DriverID); e != nil {
			return e
		}
		available, e := availableTx(ctx, tx, a, p.AssetID, "")
		if e != nil {
			return e
		}
		if !available {
			return ErrConflict
		}
		r.AssetID = p.AssetID
		r.AssignedTo = p.DriverID
		r.State = "planned"
		r.Data = encode(Trip{TripOpen: p})
		return nil
	}
	var p Trip
	var action TripAction
	if json.Unmarshal(r.Data, &p) != nil || decode(in.Data, &action, "loaded", "end_meter_milli") != nil {
		return ErrInvalid
	}
	if !manager && r.AssignedTo != a.IdentityID {
		return identity.ErrDenied
	}
	switch in.Action {
	case "depart":
		if r.State != "planned" || !action.Loaded || action.EndMeterMilli != nil {
			return ErrConflict
		}
		if e := assignee(ctx, tx, a, d, r.AssignedTo); e != nil {
			return e
		}
		available, e := availableTx(ctx, tx, a, r.AssetID, r.ID)
		if e != nil {
			return e
		}
		if !available {
			return ErrConflict
		}
		_, asset, e := assetTx(ctx, tx, a, r.AssetID)
		if e != nil {
			return e
		}
		meter, e := meterTx(ctx, tx, a, r.AssetID, asset.MeterMilli)
		if e != nil {
			return e
		}
		p.StartMeterMilli = meter
		p.DepartedAt = now
		r.State = "departed"
	case "return":
		if r.State != "departed" || action.Loaded || action.EndMeterMilli == nil || !number(*action.EndMeterMilli) || *action.EndMeterMilli < p.StartMeterMilli {
			return ErrConflict
		}
		_, asset, e := assetTx(ctx, tx, a, r.AssetID)
		if e != nil {
			return e
		}
		if asset.MeterBasis == "none" && *action.EndMeterMilli != 0 {
			return ErrInvalid
		}
		p.EndMeterMilli = action.EndMeterMilli
		p.ReturnedAt = now
		r.State = "returned"
	case "cancel":
		if !manager {
			return identity.ErrDenied
		}
		if r.State != "planned" || action.Loaded || action.EndMeterMilli != nil {
			return ErrConflict
		}
		r.State = "cancelled"
	default:
		return ErrInvalid
	}
	r.Data = encode(p)
	return nil
}
func quoteChange(ctx context.Context, tx *sql.Tx, a identity.Scope, r *Record, in Input) error {
	if in.Action != "create" {
		return ErrInvalid
	}
	var p Quote
	if decode(in.Data, &p, "asset_id", "request_id", "provider_id", "parts_cents", "labor_cents", "other_cents", "note") != nil || !ID(p.AssetID) || !ID(p.RequestID) || !text(p.Note, 1000, false) {
		return ErrInvalid
	}
	if _, e := total(p.PartsCents, p.LaborCents, p.OtherCents); e != nil {
		return e
	}
	if _, _, e := assetTx(ctx, tx, a, p.AssetID); e != nil {
		return e
	}
	request, e := load(ctx, tx, a, "request", p.RequestID)
	if e != nil {
		return e
	}
	if e = proof(ctx, tx, a, request); e != nil {
		return e
	}
	if request.AssetID != p.AssetID || request.State != "triaged" {
		return ErrConflict
	}
	if e = linked(ctx, tx, a, "provider", p.ProviderID, p.AssetID); e != nil {
		return e
	}
	r.AssetID = p.AssetID
	r.State = "proposed"
	r.Data = encode(p)
	return nil
}
func amountChange(ctx context.Context, tx *sql.Tx, a identity.Scope, r *Record, in Input) error {
	if in.Action == "void" {
		if empty(in.Data) != nil {
			return ErrInvalid
		}
		if r.State != "recorded" {
			return ErrConflict
		}
		r.State = "voided"
		return nil
	}
	if in.Action != "create" {
		return ErrInvalid
	}
	var p Amount
	if decode(in.Data, &p, "asset_id", "category", "amount_cents", "incurred_at", "provider_id", "request_id", "reference") != nil || !ID(p.AssetID) || !number(p.AmountCents) || p.AmountCents == 0 || !text(p.Reference, 240, false) {
		return ErrInvalid
	}
	allowed := oneOf(p.Category, "fuel", "parts", "labor", "tires", "insurance", "documents", "toll", "other")
	if r.Kind == "income" {
		allowed = oneOf(p.Category, "freight", "service", "other")
	}
	if r.Kind == "impact" {
		allowed = oneOf(p.Category, "downtime", "delay", "other")
	}
	if !allowed {
		return ErrInvalid
	}
	at, e := stamp(p.IncurredAt, false)
	if e != nil {
		return e
	}
	p.IncurredAt = at
	if _, _, e = assetTx(ctx, tx, a, p.AssetID); e != nil {
		return e
	}
	if e = linked(ctx, tx, a, "provider", p.ProviderID, p.AssetID); e != nil {
		return e
	}
	if e = linked(ctx, tx, a, "request", p.RequestID, p.AssetID); e != nil {
		return e
	}
	r.AssetID = p.AssetID
	r.State = "recorded"
	r.Data = encode(p)
	return nil
}
func profileChange(ctx context.Context, tx *sql.Tx, a identity.Scope, r *Record, in Input) error {
	if !oneOf(in.Action, "create", "update") {
		return ErrInvalid
	}
	var p Profile
	if decode(in.Data, &p, "asset_id", "acquisition_cents", "residual_cents", "useful_months", "in_service_on", "note") != nil || !ID(p.AssetID) || !number(p.AcquisitionCents) || !number(p.ResidualCents) || p.ResidualCents > p.AcquisitionCents || p.UsefulMonths < 1 || p.UsefulMonths > 1200 || !text(p.Note, 1000, false) {
		return ErrInvalid
	}
	date, e := time.Parse("2006-01-02", p.InServiceOn)
	if e != nil || date.Year() < 1900 || date.Year() > 2200 {
		return ErrInvalid
	}
	if r.AssetID != "" && r.AssetID != p.AssetID {
		return ErrInvalid
	}
	if _, _, e = assetTx(ctx, tx, a, p.AssetID); e != nil {
		return e
	}
	r.AssetID = p.AssetID
	r.State = "active"
	r.Data = encode(p)
	return nil
}
