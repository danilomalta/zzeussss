package purchases

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"unicode/utf8"
)

const MaxSupplierReturns = 1000

var ErrReturnLimit = errors.New("supplier return source exceeds limit")

type SupplierReturn struct {
	ID                    string `json:"id"`
	CaseID                string `json:"case_id"`
	ReceiptID             string `json:"receipt_id"`
	OrderID               string `json:"order_id"`
	SupplierID            string `json:"supplier_id"`
	ProductID             string `json:"product_id"`
	Unit                  string `json:"unit"`
	RequestedMilli        int64  `json:"requested_milli"`
	AuthorizedMilli       int64  `json:"authorized_milli"`
	DispatchedMilli       int64  `json:"dispatched_milli"`
	SupplierReceivedMilli int64  `json:"supplier_received_milli"`
	RecoveredMilli        int64  `json:"recovered_milli"`
	State                 string `json:"state"`
	Revision              int64  `json:"revision"`
	CollectionMode        string `json:"collection_mode"`
	PairedDeliveryID      string `json:"paired_delivery_id"`
	RouteReference        string `json:"route_reference"`
	Deadline              string `json:"deadline"`
	FinancialInstruction  string `json:"financial_instruction"`
	CreatedBy             string `json:"created_by"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
	LastReason            string `json:"last_reason"`
	ExternalVerified      bool   `json:"external_verified"`
}
type ReturnOpenInput struct {
	OperationID   string `json:"operation_id"`
	ReturnID      string `json:"return_id"`
	ReceiptID     string `json:"receipt_id"`
	QuantityMilli int64  `json:"quantity_milli"`
	Reason        string `json:"reason"`
}
type ReturnActionInput struct {
	OperationID          string `json:"operation_id"`
	ExpectedRevision     int64  `json:"expected_revision"`
	Action               string `json:"action"`
	QuantityMilli        int64  `json:"quantity_milli"`
	Contact              string `json:"contact"`
	EvidenceReference    string `json:"evidence_reference"`
	ObservedAt           string `json:"observed_at"`
	CollectionMode       string `json:"collection_mode"`
	PairedDeliveryID     string `json:"paired_delivery_id"`
	RouteReference       string `json:"route_reference"`
	Deadline             string `json:"deadline"`
	FinancialInstruction string `json:"financial_instruction"`
	Reason               string `json:"reason"`
}
type ReturnStockInput struct {
	OperationID       string `json:"operation_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	LocationID        string `json:"location_id"`
	QuantityMilli     int64  `json:"quantity_milli"`
	HandoverReference string `json:"handover_reference"`
	ObservedAt        string `json:"observed_at"`
	Reason            string `json:"reason"`
}
type ReturnResult struct {
	OperationID      string             `json:"operation_id"`
	Repeated         bool               `json:"repeated"`
	Return           SupplierReturn     `json:"return"`
	Action           string             `json:"action"`
	StockOperationID string             `json:"stock_operation_id"`
	ActionEvidence   *ReturnActionInput `json:"action_evidence,omitempty"`
	StockEvidence    *ReturnStockInput  `json:"stock_evidence,omitempty"`
}
type ReturnPage struct {
	Items   []SupplierReturn `json:"items"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}
type ReturnHistory struct {
	Items   []ReturnResult `json:"items"`
	Total   int            `json:"total"`
	Limit   int            `json:"limit"`
	Offset  int            `json:"offset"`
	HasMore bool           `json:"has_more"`
}

func returnText(s string, n int, empty bool) bool {
	if s == "" {
		return empty
	}
	if len(s) > n || !utf8.ValidString(s) || strings.TrimSpace(s) != s {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func returnKey(s string) bool { return returnText(s, 128, false) }
func returnObserved(s string) bool {
	t, e := time.Parse(time.RFC3339Nano, s)
	return e == nil && t.Year() >= 2000 && t.Year() <= 2200 && !t.After(time.Now().Add(5*time.Minute))
}
func returnPermissions(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
	for _, p := range []identity.Permission{identity.ViewOrders, identity.ManageStock} {
		if e := identity.CanOperateTx(ctx, tx, a, d, p); e != nil {
			return e
		}
	}
	return nil
}
func returnBegin(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, manager bool) (*sql.Tx, error) {
	if db == nil || c == nil {
		return nil, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = c.RequireTx(ctx, tx, a, d, identity.ViewOrders, modules.Orders); e == nil {
		e = returnPermissions(ctx, tx, a, d)
	}
	if e == nil && manager {
		var allowed bool
		allowed, e = caseManager(ctx, tx, a)
		if e == nil && !allowed {
			e = identity.ErrDenied
		}
		if e == nil {
			e = identity.CanOperateTx(ctx, tx, a, d, identity.ManageReplenishment)
		}
	}
	if e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func returnRequest(id, action string, in any) string {
	b, _ := json.Marshal(struct {
		ID     string `json:"id"`
		Action string `json:"action"`
		Input  any    `json:"input"`
	}{id, action, in})
	return string(b)
}
func returnPrior(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id, body string) (ReturnResult, bool, error) {
	var who, prior, raw string
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_json,result_json FROM purchase_supplier_return_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&who, &prior, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return ReturnResult{}, false, nil
	}
	if e != nil {
		return ReturnResult{}, false, e
	}
	if who != a.IdentityID {
		return ReturnResult{}, false, ErrNotFound
	}
	if prior != body {
		return ReturnResult{}, false, ErrConflict
	}
	r, e := returnOperationTx(ctx, tx, a, d.DeviceID, id, raw)
	r.Repeated = true
	return r, true, e
}
func returnFinish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, body string, r ReturnResult) (ReturnResult, error) {
	raw, _ := json.Marshal(r)
	now := r.Return.UpdatedAt
	if e := one(ctx, tx, `INSERT INTO purchase_supplier_return_operations VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, r.Return.ID, r.Return.Revision, a.IdentityID, r.Action, body, string(raw), now); e != nil {
		return ReturnResult{}, e
	}
	if e := one(ctx, tx, `INSERT INTO purchase_supplier_return_events VALUES(?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, "purchase.supplier_return."+r.Action, string(raw), now); e != nil {
		return ReturnResult{}, e
	}
	return r, tx.Commit()
}
func returnSave(ctx context.Context, tx *sql.Tx, a identity.Scope, v SupplierReturn, previous int64) error {
	b, _ := json.Marshal(v)
	return one(ctx, tx, `UPDATE purchase_supplier_returns SET authorized_milli=?,dispatched_milli=?,supplier_received_milli=?,recovered_milli=?,state=?,revision=?,payload_json=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`, v.AuthorizedMilli, v.DispatchedMilli, v.SupplierReceivedMilli, v.RecoveredMilli, v.State, v.Revision, string(b), a.TenantID, a.StoreID, v.ID, previous)
}
