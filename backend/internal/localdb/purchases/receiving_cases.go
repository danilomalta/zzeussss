package purchases

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"unicode/utf8"
)

type CaseEvidence struct {
	ProductID      string   `json:"product_id"`
	Unit           string   `json:"unit"`
	PlannedMilli   int64    `json:"planned_milli"`
	AcceptedMilli  int64    `json:"accepted_milli"`
	RemainingMilli int64    `json:"remaining_milli"`
	ReviewFlags    []string `json:"review_flags"`
	MeasuredAt     string   `json:"measured_at"`
}
type ReceivingCase struct {
	ID               string       `json:"case_id"`
	OrderID          string       `json:"order_id"`
	SupplierID       string       `json:"supplier_id"`
	Kind             string       `json:"kind"`
	RequestedOutcome string       `json:"requested_outcome"`
	Reference        string       `json:"reference"`
	Status           string       `json:"status"`
	AssignedTo       string       `json:"assigned_to"`
	DueAt            string       `json:"due_at"`
	Revision         int64        `json:"revision"`
	OpeningEvidence  CaseEvidence `json:"opening_evidence"`
	CreatedBy        string       `json:"created_by"`
	CreatedAt        string       `json:"created_at"`
	UpdatedAt        string       `json:"updated_at"`
	LastReason       string       `json:"last_reason"`
}
type CaseOpenInput struct {
	OperationID      string `json:"operation_id"`
	CaseID           string `json:"case_id"`
	Kind             string `json:"kind"`
	RequestedOutcome string `json:"requested_outcome"`
	Reference        string `json:"reference"`
	AssignedTo       string `json:"assigned_to"`
	DueAt            string `json:"due_at"`
	Reason           string `json:"reason"`
}
type CaseActionInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	AssignedTo       string `json:"assigned_to"`
	DueAt            string `json:"due_at"`
}
type CaseResult struct {
	OperationID string        `json:"operation_id"`
	Case        ReceivingCase `json:"case"`
	Repeated    bool          `json:"repeated"`
}

func caseText(s string, max int, empty bool) bool {
	if (s == "" && !empty) || len(s) > max || !utf8.ValidString(s) || strings.TrimSpace(s) != s {
		return false
	}
	for _, r := range s {
		if r < 32 && r != '\n' && r != '\t' || r == 127 {
			return false
		}
	}
	return true
}
func caseDue(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	t, e := time.Parse(time.RFC3339Nano, raw)
	t = t.UTC()
	if e != nil || t.Year() < 2000 || t.Year() > 2200 || t.Nanosecond() != 0 {
		return "", ErrInvalid
	}
	return t.Format(time.RFC3339), nil
}
func caseKind(s string) bool {
	switch s {
	case "shortage", "damage", "wrong_product", "other":
		return true
	}
	return false
}
func caseOutcome(s string) bool {
	switch s {
	case "review", "replacement", "credit", "refund", "collection", "other":
		return true
	}
	return false
}
func caseManager(ctx context.Context, tx *sql.Tx, a identity.Scope) (bool, error) {
	var role string
	e := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role)
	if errors.Is(e, sql.ErrNoRows) {
		return false, identity.ErrDenied
	}
	return role == "owner" || role == "manager", e
}
func caseAuthorize(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) error {
	for _, p := range []identity.Permission{identity.ViewOrders, identity.ManageStock} {
		if e := identity.CanOperateTx(ctx, tx, a, d, p); e != nil {
			return e
		}
	}
	return nil
}
func caseAssignee(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) error {
	if id == "" {
		return nil
	}
	if !validSearchID(id) {
		return ErrInvalid
	}
	target := a
	target.IdentityID = id
	return caseAuthorize(ctx, tx, target, d)
}
func caseNullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func caseWrite(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, op, id, action string, input any, managerOnly bool, fn func(*sql.Tx, string, bool) (ReceivingCase, error)) (CaseResult, error) {
	var out CaseResult
	if db == nil || c == nil || !validSearchID(op) || !validSearchID(id) {
		return out, ErrInvalid
	}
	raw, e := json.Marshal(struct {
		ID     string `json:"case_id"`
		Action string `json:"action"`
		Input  any    `json:"input"`
	}{id, action, input})
	if e != nil {
		return out, e
	}
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = caseAuthorize(ctx, tx, a, d); e != nil {
		return out, e
	}
	manager, e := caseManager(ctx, tx, a)
	if e != nil {
		return out, e
	}
	if managerOnly && !manager {
		return out, identity.ErrDenied
	}
	// RequireTx also acquires the SQLite writer before state/idempotency checks.
	if e = c.RequireTx(ctx, tx, a, d, identity.ViewOrders, modules.Orders); e != nil {
		return out, e
	}
	var actor, priorHash, body string
	e = tx.QueryRowContext(ctx, `SELECT actor_id,request_hash,response_json FROM purchase_receiving_case_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&actor, &priorHash, &body)
	if e == nil {
		if actor != a.IdentityID || priorHash != digest {
			return out, ErrConflict
		}
		if e = json.Unmarshal([]byte(body), &out); e != nil {
			return out, ErrConflict
		}
		if e = caseReceiptEvidenceTx(ctx, tx, a, d.DeviceID, op, out); e != nil {
			return out, e
		}
		out.Repeated = true
		return out, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return out, e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row, e := fn(tx, now, manager)
	if e != nil {
		return out, e
	}
	out = CaseResult{OperationID: op, Case: row}
	response, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	if e = one(ctx, tx, `INSERT INTO purchase_receiving_case_operations VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID, digest, string(raw), string(response), now); e != nil {
		return out, e
	}
	if e = one(ctx, tx, `INSERT INTO purchase_receiving_case_events VALUES(?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, id, row.Revision, action, a.IdentityID, d.DeviceID, op, now); e != nil {
		return out, e
	}
	event, e := localdb.NewID()
	if e != nil {
		return out, e
	}
	payload, e := json.Marshal(struct {
		TenantID string     `json:"tenant_id"`
		StoreID  string     `json:"store_id"`
		DeviceID string     `json:"device_id"`
		ActorID  string     `json:"actor_id"`
		Result   CaseResult `json:"result"`
	}{a.TenantID, a.StoreID, d.DeviceID, a.IdentityID, out})
	if e != nil {
		return out, e
	}
	if e = one(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'purchases.receiving_case',1,?,?)`, event, a.TenantID, a.StoreID, d.DeviceID, op, id, string(payload), now); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func OpenReceivingCase(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, order string, in CaseOpenInput) (CaseResult, error) {
	due, e := caseDue(in.DueAt)
	if e != nil || !validSearchID(order) || !caseKind(in.Kind) || !caseOutcome(in.RequestedOutcome) || !caseText(in.Reference, 128, true) || !caseText(in.Reason, 1000, false) || (in.AssignedTo != "" && !validSearchID(in.AssignedTo)) {
		return CaseResult{}, ErrInvalid
	}
	in.DueAt = due
	return caseWrite(ctx, db, c, a, d, in.OperationID, in.CaseID, "open", struct {
		OrderID string        `json:"order_id"`
		Input   CaseOpenInput `json:"input"`
	}{order, in}, false, func(tx *sql.Tx, now string, manager bool) (ReceivingCase, error) {
		if !manager && in.AssignedTo != "" && in.AssignedTo != a.IdentityID {
			return ReceivingCase{}, identity.ErrDenied
		}
		if e := caseAssignee(ctx, tx, a, d, in.AssignedTo); e != nil {
			return ReceivingCase{}, e
		}
		var exists bool
		if e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_receiving_cases WHERE tenant_id=? AND store_id=? AND id=?)`, a.TenantID, a.StoreID, in.CaseID).Scan(&exists); e != nil {
			return ReceivingCase{}, e
		}
		if exists {
			return ReceivingCase{}, ErrConflict
		}
		source, e := overviewTx(ctx, tx, a, d, OverviewInput{OrderID: order, Limit: 1})
		if e != nil {
			return ReceivingCase{}, e
		}
		evidence := CaseEvidence{source.ProductID, source.Unit, source.PlannedMilli, source.EffectiveAcceptedMilli, source.RemainingMilli, source.ReviewFlags, now}
		raw, e := json.Marshal(evidence)
		if e != nil {
			return ReceivingCase{}, e
		}
		e = one(ctx, tx, `INSERT INTO purchase_receiving_cases VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, in.CaseID, order, source.SupplierID, in.Kind, in.RequestedOutcome, in.Reference, "open", caseNullable(in.AssignedTo), caseNullable(due), 1, string(raw), a.IdentityID, now, now, in.Reason)
		if e != nil {
			return ReceivingCase{}, e
		}
		return caseScanTx(ctx, tx, a, in.CaseID)
	})
}
func ActReceivingCase(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in CaseActionInput) (CaseResult, error) {
	if in.ExpectedRevision < 1 || in.ExpectedRevision >= MaxQuantity || !caseText(in.Reason, 1000, false) {
		return CaseResult{}, ErrInvalid
	}
	managerOnly := false
	switch in.Action {
	case "resolve", "cancel", "reopen", "assign", "set_due":
		managerOnly = true
	case "start", "note":
	default:
		return CaseResult{}, ErrInvalid
	}
	if in.Action != "assign" && in.AssignedTo != "" || in.Action != "set_due" && in.DueAt != "" {
		return CaseResult{}, ErrInvalid
	}
	if in.Action == "assign" && in.AssignedTo != "" && !validSearchID(in.AssignedTo) {
		return CaseResult{}, ErrInvalid
	}
	if in.Action == "set_due" {
		due, e := caseDue(in.DueAt)
		if e != nil {
			return CaseResult{}, e
		}
		in.DueAt = due
	}
	return caseWrite(ctx, db, c, a, d, in.OperationID, id, in.Action, in, managerOnly, func(tx *sql.Tx, now string, manager bool) (ReceivingCase, error) {
		row, e := caseGetTx(ctx, tx, a, id)
		if e != nil {
			return row, e
		}
		if row.Revision != in.ExpectedRevision {
			return row, ErrConflict
		}
		active := row.Status == "open" || row.Status == "in_progress"
		canWork := manager || row.AssignedTo == a.IdentityID || row.AssignedTo == "" && row.CreatedBy == a.IdentityID
		switch in.Action {
		case "start":
			if row.Status != "open" {
				return row, ErrConflict
			}
			if !canWork {
				return row, identity.ErrDenied
			}
			row.Status = "in_progress"
			if row.AssignedTo == "" {
				row.AssignedTo = a.IdentityID
			}
		case "note":
			if !active {
				return row, ErrConflict
			}
			if !canWork {
				return row, identity.ErrDenied
			}
		case "resolve":
			if !active {
				return row, ErrConflict
			}
			row.Status = "resolved"
		case "cancel":
			if !active {
				return row, ErrConflict
			}
			row.Status = "cancelled"
		case "reopen":
			if active {
				return row, ErrConflict
			}
			row.Status = "open"
		case "assign":
			if !active {
				return row, ErrConflict
			}
			if e = caseAssignee(ctx, tx, a, d, in.AssignedTo); e != nil {
				return row, e
			}
			row.AssignedTo = in.AssignedTo
		case "set_due":
			if !active {
				return row, ErrConflict
			}
			row.DueAt = in.DueAt
		}
		e = one(ctx, tx, `UPDATE purchase_receiving_cases SET status=?,assigned_to=?,due_at=?,revision=revision+1,updated_at=?,last_reason=? WHERE tenant_id=? AND store_id=? AND id=? AND revision=?`, row.Status, caseNullable(row.AssignedTo), caseNullable(row.DueAt), now, in.Reason, a.TenantID, a.StoreID, id, in.ExpectedRevision)
		if e != nil {
			return row, e
		}
		return caseScanTx(ctx, tx, a, id)
	})
}
