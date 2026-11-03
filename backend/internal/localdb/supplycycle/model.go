// Package supplycycle connects manual supplier evidence, named deliveries,
// checked stock receiving and separately approved financial obligations.
package supplycycle

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
	"titansystem-backend/internal/localdb/payables"
	"titansystem-backend/internal/localdb/purchases"
	"unicode/utf8"
)

const MaxDeliveries = 1000

var ErrLimit = errors.New("supply cycle source limit exceeded")

type DecisionInput struct {
	OperationID       string `json:"operation_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	Decision          string `json:"decision"`
	AcceptedMilli     int64  `json:"accepted_milli"`
	PromisedAt        string `json:"promised_at"`
	Contact           string `json:"contact"`
	EvidenceReference string `json:"evidence_reference"`
	ObservedAt        string `json:"observed_at"`
	Reason            string `json:"reason"`
}
type Decision struct {
	DecisionInput
	OrderID          string `json:"order_id"`
	Revision         int64  `json:"revision"`
	EvidenceMode     string `json:"evidence_mode"`
	ExternalVerified bool   `json:"external_verified"`
	RecordedBy       string `json:"recorded_by"`
	RecordedAt       string `json:"recorded_at"`
}
type PlanInput struct {
	OperationID             string `json:"operation_id"`
	DeliveryID              string `json:"delivery_id"`
	DecisionRevision        int64  `json:"decision_revision"`
	DeliveryReference       string `json:"delivery_reference"`
	PlannedMilli            int64  `json:"planned_milli"`
	AppointmentID           string `json:"appointment_id"`
	WindowEvidenceReference string `json:"window_evidence_reference"`
	ReceivingCaseID         string `json:"receiving_case_id"`
	Reason                  string `json:"reason"`
}
type Delivery struct {
	ID                      string `json:"id"`
	OrderID                 string `json:"order_id"`
	SupplierID              string `json:"supplier_id"`
	ProductID               string `json:"product_id"`
	Unit                    string `json:"unit"`
	DeliveryReference       string `json:"delivery_reference"`
	PlannedMilli            int64  `json:"planned_milli"`
	DecisionRevision        int64  `json:"decision_revision"`
	AppointmentID           string `json:"appointment_id"`
	WindowEvidenceReference string `json:"window_evidence_reference"`
	ReceivingCaseID         string `json:"receiving_case_id"`
	State                   string `json:"state"`
	Revision                int64  `json:"revision"`
	ReceiptID               string `json:"receipt_id"`
	CreatedBy               string `json:"created_by"`
	CreatedAt               string `json:"created_at"`
	UpdatedAt               string `json:"updated_at"`
	LastReason              string `json:"last_reason"`
}
type ActionInput struct {
	OperationID       string `json:"operation_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	Action            string `json:"action"`
	EvidenceReference string `json:"evidence_reference"`
	ObservedAt        string `json:"observed_at"`
	Reason            string `json:"reason"`
}
type ReceiveInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	LocationID       string `json:"location_id"`
	DeliveredMilli   int64  `json:"delivered_milli"`
	AcceptedMilli    int64  `json:"accepted_milli"`
	Reason           string `json:"reason"`
}
type FullRejectionInput struct {
	OperationID         string `json:"operation_id"`
	ExpectedRevision    int64  `json:"expected_revision"`
	DeliveredMilli      int64  `json:"delivered_milli"`
	ExistingRejectionID string `json:"existing_rejection_id"`
	EvidenceReference   string `json:"evidence_reference"`
	ObservedAt          string `json:"observed_at"`
	Reason              string `json:"reason"`
}
type FullRejectionEvidence struct {
	FullRejectionInput
	RecordedBy       string `json:"recorded_by"`
	RecordedDeviceID string `json:"recorded_device_id"`
	RecordedAt       string `json:"recorded_at"`
}
type FinanceInput struct {
	OperationID       string                 `json:"operation_id"`
	ExpectedRevision  int64                  `json:"expected_revision"`
	PayableID         string                 `json:"payable_id"`
	DocumentReference string                 `json:"document_reference"`
	SourceReference   string                 `json:"source_reference"`
	Description       string                 `json:"description"`
	IssuedOn          string                 `json:"issued_on"`
	Currency          string                 `json:"currency"`
	TotalCents        int64                  `json:"total_cents"`
	Installments      []payables.Installment `json:"installments"`
}
type Origin struct {
	PayableID                   string `json:"payable_id"`
	DeliveryID                  string `json:"delivery_id"`
	ReceiptID                   string `json:"receipt_id"`
	OrderID                     string `json:"order_id"`
	SupplierID                  string `json:"supplier_id"`
	AmountCents                 int64  `json:"amount_cents"`
	SourceReference             string `json:"source_reference"`
	OperationID                 string `json:"operation_id"`
	CreatedAt                   string `json:"created_at"`
	ReviewRequired              bool   `json:"review_required"`
	FiscalAuthorizationVerified bool   `json:"fiscal_authorization_verified"`
}
type Result struct {
	Rejection         *purchases.DeliveryRejection `json:"rejection,omitempty"`
	RejectionEvidence *FullRejectionEvidence       `json:"rejection_evidence,omitempty"`
	Action            *ActionInput                 `json:"action_evidence,omitempty"`
	OperationID       string                       `json:"operation_id"`
	Repeated          bool                         `json:"repeated"`
	Decision          *Decision                    `json:"decision,omitempty"`
	Delivery          *Delivery                    `json:"delivery,omitempty"`
	Receipt           *purchases.Receipt           `json:"receipt,omitempty"`
	Payable           *payables.Payable            `json:"payable,omitempty"`
	Origin            *Origin                      `json:"origin,omitempty"`
}
type Cycle struct {
	OrderID                string     `json:"order_id"`
	Decision               *Decision  `json:"decision"`
	PlannedMilli           int64      `json:"planned_milli"`
	EffectiveAcceptedMilli int64      `json:"effective_accepted_milli"`
	RemainingOrderMilli    int64      `json:"remaining_order_milli"`
	AllocatedMilli         int64      `json:"allocated_milli"`
	Deliveries             []Delivery `json:"deliveries"`
	Total                  int        `json:"total"`
	Limit                  int        `json:"limit"`
	Offset                 int        `json:"offset"`
	HasMore                bool       `json:"has_more"`
}
type History struct {
	Items   []Result `json:"items"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	HasMore bool     `json:"has_more"`
}

func text(s string, n int, empty bool) bool {
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
func key(s string) bool { return text(s, 128, false) }
func stamp(s string) bool {
	t, e := time.Parse(time.RFC3339Nano, s)
	return e == nil && t.Year() >= 2000 && t.Year() <= 2200 && !t.After(time.Now().Add(5*time.Minute))
}
func authorize(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, finance bool) error {
	p := identity.ViewOrders
	if finance {
		p = identity.ViewAccounting
	}
	return identity.CanOperateTx(ctx, tx, a, d, p)
}
func begin(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, finance bool) (*sql.Tx, error) {
	if db == nil || c == nil {
		return nil, purchases.ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	p, m := identity.ManageReplenishment, modules.Orders
	if finance {
		p, m = identity.ManageCash, modules.Finance
	}
	if e = c.RequireTx(ctx, tx, a, d, p, m); e == nil {
		e = authorize(ctx, tx, a, d, finance)
	}
	if e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func one(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, e := tx.ExecContext(ctx, q, args...)
	if e != nil {
		var c interface{ Code() int }
		if errors.As(e, &c) && (c.Code() == 1555 || c.Code() == 2067) {
			return purchases.ErrConflict
		}
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return purchases.ErrConflict
	}
	return nil
}
func request(entity, action string, in any) string {
	b, _ := json.Marshal(struct {
		Entity string `json:"entity"`
		Action string `json:"action"`
		Input  any    `json:"input"`
	}{entity, action, in})
	return string(b)
}
func prior(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, body string) (Result, bool, error) {
	var who, saved, result string
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_json,result_json FROM purchase_cycle_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&who, &saved, &result)
	if errors.Is(e, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if e != nil {
		return Result{}, false, e
	}
	if who != a.IdentityID {
		return Result{}, false, purchases.ErrNotFound
	}
	if saved != body {
		return Result{}, false, purchases.ErrConflict
	}
	r, e := verifiedOperation(ctx, tx, a, d, op, result)
	if e != nil {
		return r, false, e
	}
	r.Repeated = true
	return r, true, nil
}
func saveCycleOperationTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, kind, id, body string, revision int64, r Result) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	if e = one(ctx, tx, `INSERT INTO purchase_cycle_operations VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, a.IdentityID, kind, id, revision, body, string(b), now); e != nil {
		return e
	}
	if e = one(ctx, tx, `INSERT INTO purchase_cycle_events VALUES(?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, "purchase.cycle."+kind, string(b), now); e != nil {
		return e
	}
	return nil
}

func finish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, kind, id, body string, revision int64, r Result) (Result, error) {
	if e := saveCycleOperationTx(ctx, tx, a, d, kind, id, body, revision, r); e != nil {
		return Result{}, e
	}
	return r, tx.Commit()
}
