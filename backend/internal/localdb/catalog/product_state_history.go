package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"titansystem-backend/internal/localdb/identity"
)

type ProductStateChange struct {
	Revision     int64  `json:"revision"`
	StoreID      string `json:"store_id"`
	OperationID  string `json:"operation_id"`
	DeviceID     string `json:"device_id"`
	ActorID      string `json:"actor_id"`
	BeforeStatus string `json:"before_status"`
	AfterStatus  string `json:"after_status"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
}
type ProductStateHistory struct {
	Current    ProductState         `json:"current"`
	Offset     int64                `json:"offset"`
	Limit      int                  `json:"limit"`
	TotalCount int64                `json:"total_count"`
	HasMore    bool                 `json:"has_more"`
	Items      []ProductStateChange `json:"items"`
}

func historicalProductState(body, id, op string, rev int64) (ProductStateInput, error) {
	var in ProductStateInput
	if json.Unmarshal([]byte(body), &in) != nil || in.ProductID != id || in.OperationID != op || !validEditID(op) || rev < 1 || rev > 2147483647 || in.ExpectedRevision != rev-1 || in.Status != "active" && in.Status != "inactive" || len(in.Reason) < 1 || len(in.Reason) > 255 || strings.TrimSpace(in.Reason) != in.Reason || strings.ContainsAny(in.Reason, "\x00\r\n") {
		return in, ErrEditConflict
	}
	canonical, err := json.Marshal(in)
	if err != nil {
		return in, err
	}
	if string(canonical) != body {
		return in, ErrEditConflict
	}
	return in, nil
}
func ProductStateAudit(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int64) (ProductStateHistory, error) {
	if !validEditID(id) || offset < 0 || offset > 9007199254740991 {
		return ProductStateHistory{}, ErrInvalidCatalog
	}
	tx, err := productStateReadTx(ctx, db, a, d)
	if err != nil {
		return ProductStateHistory{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); err != nil {
		return ProductStateHistory{}, err
	}
	out := ProductStateHistory{Offset: offset, Limit: 50, Items: []ProductStateChange{}}
	out.Current, err = ProductStateTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	var min, max int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(revision),0),COALESCE(max(revision),0) FROM catalog_product_state_events WHERE tenant_id=? AND product_id=?`, a.TenantID, id).Scan(&out.TotalCount, &min, &max); err != nil {
		return out, err
	}
	if out.TotalCount != out.Current.Revision || max != out.Current.Revision || (out.TotalCount > 0 && min != 1) {
		return out, ErrEditConflict
	}
	if max > 0 {
		var body, op, after, reason string
		if err = tx.QueryRowContext(ctx, `SELECT request_json,operation_id,after_status,reason FROM catalog_product_state_events WHERE tenant_id=? AND product_id=? AND revision=?`, a.TenantID, id, max).Scan(&body, &op, &after, &reason); err != nil {
			return out, err
		}
		in, e := historicalProductState(body, id, op, max)
		if e != nil {
			return out, e
		}
		if in.Status != out.Current.Status || after != in.Status || reason != in.Reason {
			return out, ErrEditConflict
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.revision,e.store_id,e.operation_id,e.device_id,e.actor_id,e.before_status,e.after_status,e.reason,e.created_at,e.request_json,p.request_json,p.operation_id FROM catalog_product_state_events e LEFT JOIN catalog_product_state_events p ON p.tenant_id=e.tenant_id AND p.product_id=e.product_id AND p.revision=e.revision-1 WHERE e.tenant_id=? AND e.product_id=? ORDER BY e.revision LIMIT 50 OFFSET ?`, a.TenantID, id, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v ProductStateChange
		var body string
		var prev, prevOp sql.NullString
		if err = rows.Scan(&v.Revision, &v.StoreID, &v.OperationID, &v.DeviceID, &v.ActorID, &v.BeforeStatus, &v.AfterStatus, &v.Reason, &v.CreatedAt, &body, &prev, &prevOp); err != nil {
			return out, err
		}
		in, e := historicalProductState(body, id, v.OperationID, v.Revision)
		if e != nil {
			return out, e
		}
		before := "active"
		if v.Revision > 1 {
			if !prev.Valid || !prevOp.Valid {
				return out, ErrEditConflict
			}
			prior, e := historicalProductState(prev.String, id, prevOp.String, v.Revision-1)
			if e != nil {
				return out, e
			}
			before = prior.Status
		}
		if v.BeforeStatus != before || v.AfterStatus != in.Status || v.Reason != in.Reason || v.BeforeStatus == v.AfterStatus {
			return out, ErrEditConflict
		}
		out.Items = append(out.Items, v)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	out.HasMore = offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-offset
	return out, tx.Commit()
}
