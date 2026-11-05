package supplycycle

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
	"titansystem-backend/internal/sqlitewrite"
)

type ContactPolicy struct {
	Revision  int64  `json:"revision"`
	Enabled   bool   `json:"enabled"`
	Threshold int64  `json:"threshold"`
	Since     string `json:"since"`
	CountMode string `json:"count_mode"`
}
type ContactPolicyInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Enabled          bool   `json:"enabled"`
	Threshold        int64  `json:"threshold"`
	Since            string `json:"since"`
	CountMode        string `json:"count_mode"`
}
type ContactAlert struct {
	SupplierID                  string        `json:"supplier_id"`
	Policy                      ContactPolicy `json:"policy"`
	ManualDeclinedOrders        int64         `json:"manual_declined_orders"`
	AuthenticatedDeclinedOrders int64         `json:"authenticated_declined_orders"`
	SourceComplete              bool          `json:"source_complete"`
	Recommended                 bool          `json:"recommended"`
}

func contactSupplierTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) error {
	var n int
	if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_suppliers WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&n); e != nil {
		return e
	}
	if n != 1 {
		return purchases.ErrNotFound
	}
	return nil
}
func contactPolicyTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ContactPolicy, error) {
	var p ContactPolicy
	var raw string
	var revision int64
	e := tx.QueryRowContext(ctx, `SELECT revision,payload_json FROM supplier_contact_policies WHERE tenant_id=? AND store_id=? AND supplier_id=?`, a.TenantID, a.StoreID, id).Scan(&revision, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return p, nil
	}
	if e != nil {
		return p, e
	}
	if json.Unmarshal([]byte(raw), &p) != nil || p.Revision != revision || revision < 1 || !validContactPolicy(p) {
		return ContactPolicy{}, purchases.ErrConflict
	}
	return p, nil
}
func validContactPolicy(p ContactPolicy) bool {
	if p.Threshold < 0 || p.Threshold > MaxDeliveries {
		return false
	}
	if p.Since != "" && !stamp(p.Since) {
		return false
	}
	if p.CountMode != "" && p.CountMode != "current_declined_orders" {
		return false
	}
	return !p.Enabled || (p.Threshold > 0 && p.Since != "" && p.CountMode == "current_declined_orders")
}
func ConfigureContactPolicy(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, token, id string, in ContactPolicyInput) (Result, error) {
	var out Result
	p := ContactPolicy{Revision: in.ExpectedRevision + 1, Enabled: in.Enabled, Threshold: in.Threshold, Since: in.Since, CountMode: in.CountMode}
	if db == nil || c == nil || !key(id) || !key(in.OperationID) || in.ExpectedRevision < 0 || in.ExpectedRevision >= 1000000 || !validContactPolicy(p) {
		return out, purchases.ErrInvalid
	}
	e := sqlitewrite.Run(ctx, db, purchases.ErrConflict, func(tx *sql.Tx) error {
		if e := fiscal.ValidateSaleSessionTx(ctx, tx, a, d, token); e != nil {
			return e
		}
		if e := c.RequireTx(ctx, tx, a, d, identity.ManageReplenishment, modules.Orders); e != nil {
			return e
		}
		if e := tradeOwner(ctx, tx, a); e != nil {
			return e
		}
		if e := contactSupplierTx(ctx, tx, a, id); e != nil {
			return e
		}
		body := request(id, "contact_policy", in)
		if r, ok, e := contactPrior(ctx, tx, a, d, in.OperationID, body); ok || e != nil {
			out = r
			return e
		}
		before, e := contactPolicyTx(ctx, tx, a, id)
		if e != nil {
			return e
		}
		if before.Revision != in.ExpectedRevision {
			return purchases.ErrConflict
		}
		raw, e := json.Marshal(p)
		if e != nil {
			return e
		}
		if before.Revision == 0 {
			e = one(ctx, tx, `INSERT INTO supplier_contact_policies VALUES(?,?,?,?,?)`, a.TenantID, a.StoreID, id, p.Revision, string(raw))
		} else {
			e = one(ctx, tx, `UPDATE supplier_contact_policies SET revision=?,payload_json=? WHERE tenant_id=? AND store_id=? AND supplier_id=? AND revision=?`, p.Revision, string(raw), a.TenantID, a.StoreID, id, before.Revision)
		}
		if e != nil {
			return e
		}
		out = Result{OperationID: in.OperationID, ContactPolicy: &p}
		rawResult, e := json.Marshal(out)
		if e != nil {
			return e
		}
		return one(ctx, tx, `INSERT INTO supplier_contact_policy_operations VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, in.OperationID, a.IdentityID, id, p.Revision, body, string(rawResult), time.Now().UTC().Format(time.RFC3339Nano))
	})
	if e != nil {
		return Result{}, e
	}
	return out, nil
}

// Current explicit refusals on DISTINCT orders, not revisions or damaged goods.
// A later confirmation stops that order contributing; historical evidence stays.
func SupplierContactAlert(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, token, id string) (ContactAlert, error) {
	out := ContactAlert{SupplierID: id}
	if !key(id) {
		return out, purchases.ErrInvalid
	}
	tx, e := readTx(ctx, db, a, d, false)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = fiscal.ValidateSaleSessionTx(ctx, tx, a, d, token); e != nil {
		return out, e
	}
	if e = contactSupplierTx(ctx, tx, a, id); e != nil {
		return out, e
	}
	out.Policy, e = contactPolicyTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	if !out.Policy.Enabled {
		return out, tx.Commit()
	}
	since, _ := time.Parse(time.RFC3339Nano, out.Policy.Since)
	rows, e := tx.QueryContext(ctx, `SELECT o.id FROM purchase_orders o JOIN purchase_supplier_decisions v ON v.tenant_id=o.tenant_id AND v.store_id=o.store_id AND v.order_id=o.id WHERE o.tenant_id=? AND o.store_id=? AND o.supplier_id=? ORDER BY o.id LIMIT ?`, a.TenantID, a.StoreID, id, MaxDeliveries+1)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var v string
		if e = rows.Scan(&v); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if len(ids) > MaxDeliveries {
		return out, ErrLimit
	}
	for _, order := range ids {
		v, e := decisionTx(ctx, tx, a, order)
		if e != nil {
			return ContactAlert{}, e
		}
		if v == nil {
			return ContactAlert{}, purchases.ErrConflict
		}
		at, e := time.Parse(time.RFC3339Nano, v.RecordedAt)
		if e != nil {
			return ContactAlert{}, purchases.ErrConflict
		}
		if v.Decision != "declined" || at.Before(since) {
			continue
		}
		if v.ExternalVerified {
			out.AuthenticatedDeclinedOrders++
		} else {
			out.ManualDeclinedOrders++
		}
	}
	out.SourceComplete = true
	out.Recommended = out.ManualDeclinedOrders+out.AuthenticatedDeclinedOrders >= out.Policy.Threshold
	return out, tx.Commit()
}

func contactPrior(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, body string) (Result, bool, error) {
	var actor, request, result string
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_json,result_json FROM supplier_contact_policy_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&actor, &request, &result)
	if errors.Is(e, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if e != nil {
		return Result{}, false, e
	}
	if actor != a.IdentityID {
		return Result{}, false, purchases.ErrNotFound
	}
	if body != request {
		return Result{}, false, purchases.ErrConflict
	}
	var out Result
	if json.Unmarshal([]byte(result), &out) != nil || out.OperationID != op || out.Repeated || out.ContactPolicy == nil || !validContactPolicy(*out.ContactPolicy) {
		return Result{}, false, purchases.ErrConflict
	}
	out.Repeated = true
	return out, true, nil
}
