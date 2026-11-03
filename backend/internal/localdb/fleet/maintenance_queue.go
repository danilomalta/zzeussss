package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type MaintenanceQueueItem struct {
	Request        Record             `json:"request"`
	Classification *MaintenanceRecord `json:"classification"`
	SpecialtyName  string             `json:"specialty_name"`
	AssetProfile   *MaintenanceRecord `json:"asset_profile"`
}
type MaintenanceQueue struct {
	Items         []MaintenanceQueueItem `json:"items"`
	Total         int64                  `json:"total"`
	Unassigned    int64                  `json:"unassigned"`
	Emergency     int64                  `json:"emergency"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
	MeasuredAt    string                 `json:"measured_at"`
	PushDelivered bool                   `json:"push_delivered"`
}

func MaintenanceDispatch(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, specialtyID, technicianID, state string, limit, offset int) (MaintenanceQueue, error) {
	out := MaintenanceQueue{Items: []MaintenanceQueueItem{}, Limit: limit, Offset: offset, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if db == nil || specialtyID != "" && !ID(specialtyID) || technicianID != "" && !ID(technicianID) || !oneOf(state, "", "open", "triaged", "approved", "in_service", "waiting_parts", "inspection", "released", "cancelled", "cancelled_blocked") || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	manager, e := authorize(ctx, tx, a, d, false)
	if e != nil {
		return out, e
	}
	skills := map[string]bool{}
	if technicianID != "" {
		if !manager && technicianID != a.IdentityID {
			return out, identity.ErrDenied
		}
		if e = assignee(ctx, tx, a, d, technicianID); e != nil {
			return out, e
		}
		tech, e := maintenanceLoad(ctx, tx, a, "technician", technicianID)
		if e != nil {
			return out, e
		}
		if tech.State != "active" {
			return out, ErrConflict
		}
		var v MaintenanceTechnician
		if json.Unmarshal(tech.Data, &v) != nil {
			return out, ErrConflict
		}
		for _, id := range v.SpecialtyIDs {
			s, e := maintenanceLoad(ctx, tx, a, "specialty", id)
			if e != nil {
				return out, e
			}
			if s.State == "active" {
				skills[id] = true
			}
		}
	}
	if specialtyID != "" {
		if _, e = maintenanceLoad(ctx, tx, a, "specialty", specialtyID); e != nil {
			return out, e
		}
	}
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind='request'`, a.TenantID, a.StoreID).Scan(&count); e != nil {
		return out, e
	}
	if count > MaxSource {
		return out, ErrLimit
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM fleet_records WHERE tenant_id=? AND store_id=? AND kind='request' ORDER BY id`, a.TenantID, a.StoreID)
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
		r, e := load(ctx, tx, a, "request", id)
		if e != nil {
			return out, e
		}
		if e = proof(ctx, tx, a, r); e != nil {
			return out, e
		}
		if state != "" && r.State != state || state == "" && oneOf(r.State, "released", "cancelled") {
			continue
		}
		item := MaintenanceQueueItem{Request: r}
		classification, e := maintenanceLoad(ctx, tx, a, "request", id)
		specialty := ""
		if e == nil && classification.State == "active" {
			var v MaintenanceClassification
			if json.Unmarshal(classification.Data, &v) != nil {
				return out, ErrConflict
			}
			specialty = v.SpecialtyID
			s, e := maintenanceLoad(ctx, tx, a, "specialty", specialty)
			if e != nil {
				return out, e
			}
			var name MaintenanceSpecialty
			if json.Unmarshal(s.Data, &name) != nil {
				return out, ErrConflict
			}
			item.SpecialtyName = name.Name
			item.Classification = &classification
		} else if e != nil && !errors.Is(e, ErrNotFound) {
			return out, e
		}
		if specialtyID != "" && specialtyID != specialty {
			continue
		}
		if technicianID != "" && r.AssignedTo != technicianID && !(r.AssignedTo == "" && skills[specialty]) {
			continue
		}
		profile, e := maintenanceLoad(ctx, tx, a, "asset", r.AssetID)
		if e == nil && profile.State == "active" {
			item.AssetProfile = &profile
		} else if e != nil && !errors.Is(e, ErrNotFound) {
			return out, e
		}
		var request Request
		if json.Unmarshal(r.Data, &request) != nil {
			return out, ErrConflict
		}
		if r.AssignedTo == "" {
			out.Unassigned++
		}
		if request.Priority == "emergency" {
			out.Emergency++
		}
		if out.Total >= int64(offset) && len(out.Items) < limit {
			out.Items = append(out.Items, item)
		}
		out.Total++
	}
	return out, tx.Commit()
}
