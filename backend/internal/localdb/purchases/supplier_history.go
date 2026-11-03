package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"titansystem-backend/internal/localdb/identity"
)

type SupplierOrigin struct {
	Supplier
	Creation CreationAudit `json:"creation"`
}
type SupplierChange struct {
	Revision     int64  `json:"revision"`
	OperationID  string `json:"operation_id"`
	DeviceID     string `json:"device_id"`
	ActorID      string `json:"actor_id"`
	BeforeName   string `json:"before_name"`
	BeforeStatus string `json:"before_status"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Reason       string `json:"reason"`
	CreatedAt    string `json:"created_at"`
}
type SupplierHistoryPage struct {
	Current    SupplierState    `json:"current"`
	Origin     SupplierOrigin   `json:"origin"`
	Offset     int64            `json:"offset"`
	Limit      int              `json:"limit"`
	TotalCount int64            `json:"total_count"`
	HasMore    bool             `json:"has_more"`
	Items      []SupplierChange `json:"items"`
}

func historicalSupplierEdit(body, id, op string, revision int64) (SupplierEditInput, error) {
	var in SupplierEditInput
	if json.Unmarshal([]byte(body), &in) != nil || in.SupplierID != id || in.OperationID != op || !validSearchID(op) || in.ExpectedRevision != revision-1 || revision < 1 || revision > 2147483647 || len(in.Name) < 1 || len(in.Name) > 255 || len(in.Reason) < 1 || len(in.Reason) > 255 || strings.TrimSpace(in.Name) != in.Name || strings.TrimSpace(in.Reason) != in.Reason || strings.ContainsAny(in.Name+in.Reason, "\x00\r\n") || (in.Status != "active" && in.Status != "inactive") {
		return in, ErrConflict
	}
	canonical, err := json.Marshal(in)
	if err != nil {
		return in, err
	}
	if string(canonical) != body {
		return in, ErrConflict
	}
	return in, nil
}

// Read current state and paginated audit in one authorized snapshot, without advancing the license clock.
func SupplierHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, offset int64) (SupplierHistoryPage, error) {
	if !validSearchID(id) || offset < 0 || offset > MaxQuantity {
		return SupplierHistoryPage{}, ErrInvalid
	}
	tx, err := ReadTx(ctx, db, a, d)
	if err != nil {
		return SupplierHistoryPage{}, err
	}
	defer tx.Rollback()
	out := SupplierHistoryPage{Offset: offset, Limit: 50, Items: []SupplierChange{}}
	out.Current, err = supplierStateTx(ctx, tx, a, id)
	if err != nil {
		return out, err
	}
	out.Origin.ID = id
	out.Origin.Creation.Kind = "supplier.created"
	c := &out.Origin.Creation
	err = tx.QueryRowContext(ctx, `SELECT name,status,operation_id,device_id,created_by,created_at FROM purchase_supplier_originals WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.Origin.Name, &out.Origin.Status, &c.OperationID, &c.DeviceID, &c.ActorID, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrConflict
	}
	if err != nil {
		return out, err
	}
	var n int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_audit WHERE tenant_id=? AND store_id=? AND kind='supplier.created' AND aggregate_id=?`, a.TenantID, a.StoreID, id).Scan(&n); err != nil {
		return out, err
	}
	if n != 1 {
		return out, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_supplier_originals WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&n); err != nil {
		return out, err
	}
	if n != 1 {
		return out, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_audit WHERE tenant_id=? AND store_id=? AND kind='supplier.created' AND aggregate_id=? AND operation_id=? AND device_id=? AND actor_id=? AND created_at=?`, a.TenantID, a.StoreID, id, c.OperationID, c.DeviceID, c.ActorID, c.CreatedAt).Scan(&n); err != nil {
		return out, err
	}
	if n != 1 {
		return out, ErrConflict
	}
	var min, max int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(revision),0),COALESCE(max(revision),0) FROM purchase_supplier_edits WHERE tenant_id=? AND store_id=? AND supplier_id=?`, a.TenantID, a.StoreID, id).Scan(&out.TotalCount, &min, &max); err != nil {
		return out, err
	}
	if out.TotalCount != out.Current.Revision || max != out.Current.Revision || (out.TotalCount > 0 && min != 1) {
		return out, ErrConflict
	}
	if out.TotalCount == 0 {
		if out.Current.Name != out.Origin.Name || out.Current.Status != out.Origin.Status {
			return out, ErrConflict
		}
	} else {
		var body, op string
		if err = tx.QueryRowContext(ctx, `SELECT request_json,operation_id FROM purchase_supplier_edits WHERE tenant_id=? AND store_id=? AND supplier_id=? AND revision=?`, a.TenantID, a.StoreID, id, max).Scan(&body, &op); err != nil {
			return out, err
		}
		in, e := historicalSupplierEdit(body, id, op, max)
		if e != nil {
			return out, e
		}
		if in.Name != out.Current.Name || in.Status != out.Current.Status {
			return out, ErrConflict
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.revision,e.operation_id,e.device_id,e.actor_id,e.before_name,e.before_status,e.created_at,e.request_json,p.request_json,p.operation_id FROM purchase_supplier_edits e LEFT JOIN purchase_supplier_edits p ON p.tenant_id=e.tenant_id AND p.store_id=e.store_id AND p.supplier_id=e.supplier_id AND p.revision=e.revision-1 WHERE e.tenant_id=? AND e.store_id=? AND e.supplier_id=? ORDER BY e.revision LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, id, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v SupplierChange
		var body string
		var previous, previousOp sql.NullString
		if err = rows.Scan(&v.Revision, &v.OperationID, &v.DeviceID, &v.ActorID, &v.BeforeName, &v.BeforeStatus, &v.CreatedAt, &body, &previous, &previousOp); err != nil {
			return out, err
		}
		in, e := historicalSupplierEdit(body, id, v.OperationID, v.Revision)
		if e != nil {
			return out, e
		}
		v.Name, v.Status, v.Reason = in.Name, in.Status, in.Reason
		beforeName, beforeStatus := out.Origin.Name, out.Origin.Status
		if v.Revision > 1 {
			if !previous.Valid || !previousOp.Valid {
				return out, ErrConflict
			}
			prior, e := historicalSupplierEdit(previous.String, id, previousOp.String, v.Revision-1)
			if e != nil {
				return out, e
			}
			beforeName, beforeStatus = prior.Name, prior.Status
		}
		if beforeName != v.BeforeName || beforeStatus != v.BeforeStatus || (v.Name == v.BeforeName && v.Status == v.BeforeStatus) {
			return out, ErrConflict
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
