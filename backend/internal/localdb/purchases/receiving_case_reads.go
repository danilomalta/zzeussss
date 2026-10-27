package purchases

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

func caseScanTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ReceivingCase, error) {
	var r ReceivingCase
	var evidence string
	e := tx.QueryRowContext(ctx, `SELECT id,order_id,supplier_id,kind,requested_outcome,reference,status,COALESCE(assigned_to,''),COALESCE(due_at,''),revision,opening_evidence_json,created_by,created_at,updated_at,last_reason FROM purchase_receiving_cases WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&r.ID, &r.OrderID, &r.SupplierID, &r.Kind, &r.RequestedOutcome, &r.Reference, &r.Status, &r.AssignedTo, &r.DueAt, &r.Revision, &evidence, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &r.LastReason)
	if errors.Is(e, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if e != nil {
		return r, e
	}
	if !validSearchID(r.ID) || !validSearchID(r.OrderID) || !validSearchID(r.SupplierID) || !validSearchID(r.CreatedBy) || !caseKind(r.Kind) || !caseOutcome(r.RequestedOutcome) || !caseText(r.Reference, 128, true) || !caseText(r.LastReason, 1000, false) || r.Revision < 1 || r.Revision > MaxQuantity || (r.AssignedTo != "" && !validSearchID(r.AssignedTo)) {
		return r, ErrConflict
	}
	switch r.Status {
	case "open", "in_progress", "resolved", "cancelled":
	default:
		return r, ErrConflict
	}
	due, e := caseDue(r.DueAt)
	if e != nil || due != r.DueAt {
		return r, ErrConflict
	}
	created, e := time.Parse(time.RFC3339Nano, r.CreatedAt)
	updated, f := time.Parse(time.RFC3339Nano, r.UpdatedAt)
	if e != nil || f != nil || updated.Before(created) {
		return r, ErrConflict
	}
	if e = json.Unmarshal([]byte(evidence), &r.OpeningEvidence); e != nil {
		return r, ErrConflict
	}
	v := r.OpeningEvidence
	canonical, _ := json.Marshal(v)
	if string(canonical) != evidence || !validSearchID(v.ProductID) || v.PlannedMilli < 1 || v.PlannedMilli > MaxQuantity || v.AcceptedMilli < 0 || v.AcceptedMilli > v.PlannedMilli || v.RemainingMilli != v.PlannedMilli-v.AcceptedMilli || v.ReviewFlags == nil {
		return r, ErrConflict
	}
	switch v.Unit {
	case "unit", "kg", "g", "liter", "ml", "meter":
	default:
		return r, ErrConflict
	}
	if _, e = time.Parse(time.RFC3339Nano, v.MeasuredAt); e != nil {
		return r, ErrConflict
	}
	return r, nil
}
func caseReceiptEvidenceTx(ctx context.Context, tx *sql.Tx, a identity.Scope, device, op string, out CaseResult) error {
	if out.OperationID != op || out.Repeated || !validSearchID(out.Case.ID) || out.Case.Revision < 1 {
		return ErrConflict
	}
	var request, hash string
	e := tx.QueryRowContext(ctx, `SELECT request_json,request_hash FROM purchase_receiving_case_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, device, op).Scan(&request, &hash)
	if e != nil {
		return e
	}
	h := sha256.Sum256([]byte(request))
	if hash != hex.EncodeToString(h[:]) {
		return ErrConflict
	}
	var envelope struct {
		ID     string `json:"case_id"`
		Action string `json:"action"`
	}
	if json.Unmarshal([]byte(request), &envelope) != nil || envelope.ID != out.Case.ID {
		return ErrConflict
	}
	var n int
	e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receiving_case_events e JOIN purchase_receiving_case_operations o ON o.tenant_id=e.tenant_id AND o.store_id=e.store_id AND o.device_id=e.device_id AND o.operation_id=e.operation_id WHERE e.tenant_id=? AND e.store_id=? AND e.device_id=? AND e.operation_id=? AND e.case_id=? AND e.revision=? AND e.action=? AND e.actor_id=o.actor_id AND e.created_at=o.created_at AND e.created_at=?`, a.TenantID, a.StoreID, device, op, out.Case.ID, out.Case.Revision, envelope.Action, out.Case.UpdatedAt).Scan(&n)
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func caseGetTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ReceivingCase, error) {
	row, e := caseScanTx(ctx, tx, a, id)
	if e != nil {
		return row, e
	}
	var count, min, max int64
	e = tx.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(revision),0),COALESCE(max(revision),0) FROM purchase_receiving_case_events WHERE tenant_id=? AND store_id=? AND case_id=?`, a.TenantID, a.StoreID, id).Scan(&count, &min, &max)
	if e != nil {
		return row, e
	}
	if count != row.Revision || min != 1 || max != row.Revision {
		return row, ErrConflict
	}
	var body, device, op string
	e = tx.QueryRowContext(ctx, `SELECT o.response_json,e.device_id,e.operation_id FROM purchase_receiving_case_events e JOIN purchase_receiving_case_operations o ON o.tenant_id=e.tenant_id AND o.store_id=e.store_id AND o.device_id=e.device_id AND o.operation_id=e.operation_id WHERE e.tenant_id=? AND e.store_id=? AND e.case_id=? AND e.revision=?`, a.TenantID, a.StoreID, id, row.Revision).Scan(&body, &device, &op)
	if errors.Is(e, sql.ErrNoRows) {
		return row, ErrConflict
	}
	if e != nil {
		return row, e
	}
	var saved CaseResult
	if json.Unmarshal([]byte(body), &saved) != nil {
		return row, ErrConflict
	}
	current, _ := json.Marshal(row)
	prior, _ := json.Marshal(saved.Case)
	if string(current) != string(prior) {
		return row, ErrConflict
	}
	return row, caseReceiptEvidenceTx(ctx, tx, a, device, op, saved)
}

type CaseDetail struct {
	Case       ReceivingCase `json:"case"`
	Overdue    bool          `json:"overdue"`
	MeasuredAt string        `json:"measured_at"`
}

func caseDetail(row ReceivingCase, now time.Time) CaseDetail {
	out := CaseDetail{Case: row, MeasuredAt: now.UTC().Format(time.RFC3339Nano)}
	if row.DueAt != "" && (row.Status == "open" || row.Status == "in_progress") {
		due, _ := time.Parse(time.RFC3339, row.DueAt)
		out.Overdue = now.After(due)
	}
	return out
}
func GetReceivingCase(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (CaseDetail, error) {
	var out CaseDetail
	if !validSearchID(id) {
		return out, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = caseAuthorize(ctx, tx, a, d); e != nil {
		return out, e
	}
	row, e := caseGetTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out = caseDetail(row, time.Now().UTC())
	return out, tx.Commit()
}

type CaseListInput struct {
	OrderID, Status, AssignedTo string
	Limit, Offset               int
}
type CaseList struct {
	Items      []CaseDetail `json:"items"`
	Total      int64        `json:"total"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
	MeasuredAt string       `json:"measured_at"`
}

func ListReceivingCases(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in CaseListInput) (CaseList, error) {
	out := CaseList{Items: []CaseDetail{}, Limit: in.Limit, Offset: in.Offset}
	if in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.Offset > 1000000 || (in.OrderID != "" && !validSearchID(in.OrderID)) || (in.AssignedTo != "" && !validSearchID(in.AssignedTo)) {
		return out, ErrInvalid
	}
	switch in.Status {
	case "", "open", "in_progress", "resolved", "cancelled":
	default:
		return out, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = caseAuthorize(ctx, tx, a, d); e != nil {
		return out, e
	}
	where := `tenant_id=? AND store_id=?`
	args := []any{a.TenantID, a.StoreID}
	if in.OrderID != "" {
		where += ` AND order_id=?`
		args = append(args, in.OrderID)
	}
	if in.Status != "" {
		where += ` AND status=?`
		args = append(args, in.Status)
	}
	if in.AssignedTo != "" {
		where += ` AND assigned_to=?`
		args = append(args, in.AssignedTo)
	}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM purchase_receiving_cases WHERE `+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM purchase_receiving_cases WHERE `+where+` ORDER BY id LIMIT ? OFFSET ?`, append(args, in.Limit, in.Offset)...)
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
	now := time.Now().UTC()
	out.MeasuredAt = now.Format(time.RFC3339Nano)
	for _, id := range ids {
		row, e := caseGetTx(ctx, tx, a, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, caseDetail(row, now))
	}
	return out, tx.Commit()
}

type CaseHistoryItem struct {
	Action   string     `json:"action"`
	ActorID  string     `json:"actor_id"`
	DeviceID string     `json:"device_id"`
	Result   CaseResult `json:"result"`
}
type CaseHistory struct {
	Items  []CaseHistoryItem `json:"items"`
	Total  int64             `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

func ReceivingCaseHistory(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, limit, offset int) (CaseHistory, error) {
	out := CaseHistory{Items: []CaseHistoryItem{}, Limit: limit, Offset: offset}
	if !validSearchID(id) || limit < 1 || limit > 100 || offset < 0 || offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = caseAuthorize(ctx, tx, a, d); e != nil {
		return out, e
	}
	row, e := caseGetTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out.Total = row.Revision
	rows, e := tx.QueryContext(ctx, `SELECT e.action,e.actor_id,e.device_id,e.operation_id,o.response_json FROM purchase_receiving_case_events e JOIN purchase_receiving_case_operations o ON o.tenant_id=e.tenant_id AND o.store_id=e.store_id AND o.device_id=e.device_id AND o.operation_id=e.operation_id WHERE e.tenant_id=? AND e.store_id=? AND e.case_id=? ORDER BY e.revision LIMIT ? OFFSET ?`, a.TenantID, a.StoreID, id, limit, offset)
	if e != nil {
		return out, e
	}
	type entry struct {
		item CaseHistoryItem
		op   string
	}
	entries := []entry{}
	for rows.Next() {
		var v entry
		var body string
		if e = rows.Scan(&v.item.Action, &v.item.ActorID, &v.item.DeviceID, &v.op, &body); e != nil {
			rows.Close()
			return out, e
		}
		if json.Unmarshal([]byte(body), &v.item.Result) != nil {
			rows.Close()
			return out, ErrConflict
		}
		entries = append(entries, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for index, v := range entries {
		if v.item.Result.Case.Revision != int64(offset+index+1) {
			return out, ErrConflict
		}
		if e = caseReceiptEvidenceTx(ctx, tx, a, v.item.DeviceID, v.op, v.item.Result); e != nil {
			return out, e
		}
		out.Items = append(out.Items, v.item)
	}
	return out, tx.Commit()
}
func ReceivingCaseOperation(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, op string) (CaseResult, error) {
	var out CaseResult
	if !validSearchID(op) {
		return out, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = caseAuthorize(ctx, tx, a, d); e != nil {
		return out, e
	}
	var body string
	e = tx.QueryRowContext(ctx, `SELECT response_json FROM purchase_receiving_case_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND actor_id=?`, a.TenantID, a.StoreID, d.DeviceID, op, a.IdentityID).Scan(&body)
	if errors.Is(e, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if e != nil {
		return out, e
	}
	if json.Unmarshal([]byte(body), &out) != nil {
		return out, ErrConflict
	}
	if e = caseReceiptEvidenceTx(ctx, tx, a, d.DeviceID, op, out); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
