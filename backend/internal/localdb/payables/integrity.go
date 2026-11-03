package payables

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

func valid(p Payable) error {
	if !key(p.ID) || !text(p.SupplierID, 128) || !text(p.SupplierName, 255) || !key(p.DocumentReference) || !text(p.Description, 500) || p.Currency != "BRL" || !schedule(p.Installments, p.TotalCents, p.IssuedOn) || !text(p.CreatorID, 128) || p.PaymentExecutionAvailable || p.SettledCents != nil || p.Cash != nil || len(p.History) < 1 || len(p.History) > 3 {
		return ErrInconsistent
	}
	state := ""
	previous := time.Time{}
	ops := map[string]bool{}
	for i, e := range p.History {
		at, err := time.Parse(time.RFC3339Nano, e.CreatedAt)
		if err != nil || at.Before(previous) || !key(e.OperationID) || ops[e.OperationID] || !text(e.ActorID, 128) || e.FromState != state {
			return ErrInconsistent
		}
		previous = at
		ops[e.OperationID] = true
		ok := false
		switch e.Action {
		case "create":
			ok = i == 0 && state == "" && e.ToState == "pending" && e.ActorID == p.CreatorID && e.Reason == p.Description && e.CreatedAt == p.CreatedAt
		case "approve", "reject":
			ok = state == "pending" && e.ActorID != p.CreatorID && text(e.Reason, 500) && ((e.Action == "approve" && e.ToState == "approved") || (e.Action == "reject" && e.ToState == "rejected"))
		case "cancel":
			ok = (state == "pending" && e.ActorID == p.CreatorID || state == "approved" && e.ActorID != p.CreatorID) && e.ToState == "cancelled" && text(e.Reason, 500)
		}
		if !ok {
			return ErrInconsistent
		}
		state = e.ToState
	}
	last := p.History[len(p.History)-1]
	if state != p.State || last.CreatedAt != p.UpdatedAt || last.OperationID != p.LastOperationID {
		return ErrInconsistent
	}
	return nil
}
func operation(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Result, string, error) {
	var r Result
	var body, who, pid, action, from, to, reason, at, payload, event string
	e := tx.QueryRowContext(ctx, `SELECT o.result_json,o.actor_id,o.payable_id,o.action,o.from_state,o.to_state,o.reason,o.created_at,e.payload_json,e.event_type FROM supplier_payable_operations o LEFT JOIN supplier_payable_events e ON e.tenant_id=o.tenant_id AND e.store_id=o.store_id AND e.device_id=o.device_id AND e.operation_id=o.operation_id WHERE o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&body, &who, &pid, &action, &from, &to, &reason, &at, &payload, &event)
	if errors.Is(e, sql.ErrNoRows) {
		return r, "", ErrNotFound
	}
	if e != nil {
		return r, "", ErrInconsistent
	}
	if json.Unmarshal([]byte(body), &r) != nil || r.OperationID != id || r.Repeated || r.Payable.ID != pid || valid(r.Payable) != nil || payload != body || event != "supplier.payable."+action {
		return r, "", ErrInconsistent
	}
	if r.Payable.History[len(r.Payable.History)-1] != (Event{id, who, action, from, to, reason, at}) {
		return r, "", ErrInconsistent
	}
	return r, who, nil
}
func load(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Payable, error) {
	var p Payable
	e := tx.QueryRowContext(ctx, `SELECT id,supplier_id,supplier_name,document_reference,description,issued_on,currency,total_cents,creator_id,state,created_at,updated_at,last_operation_id FROM supplier_payables WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&p.ID, &p.SupplierID, &p.SupplierName, &p.DocumentReference, &p.Description, &p.IssuedOn, &p.Currency, &p.TotalCents, &p.CreatorID, &p.State, &p.CreatedAt, &p.UpdatedAt, &p.LastOperationID)
	if errors.Is(e, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if e != nil {
		return p, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,due_on,amount_cents,position FROM supplier_payable_installments WHERE tenant_id=? AND store_id=? AND device_id=? AND payable_id=? ORDER BY position LIMIT 121`, a.TenantID, a.StoreID, d.DeviceID, id)
	if e != nil {
		return p, e
	}
	p.Installments = []Installment{}
	for rows.Next() {
		var v Installment
		var pos int
		if e = rows.Scan(&v.ID, &v.DueOn, &v.AmountCents, &pos); e != nil {
			rows.Close()
			return p, e
		}
		if pos != len(p.Installments)+1 {
			rows.Close()
			return p, ErrInconsistent
		}
		p.Installments = append(p.Installments, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return p, e
	}
	rows, e = tx.QueryContext(ctx, `SELECT operation_id,actor_id,action,from_state,to_state,reason,created_at FROM supplier_payable_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND payable_id=? ORDER BY CASE action WHEN 'create' THEN 0 WHEN 'approve' THEN 1 WHEN 'reject' THEN 1 ELSE 2 END LIMIT 4`, a.TenantID, a.StoreID, d.DeviceID, id)
	if e != nil {
		return p, e
	}
	p.History = []Event{}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.OperationID, &v.ActorID, &v.Action, &v.FromState, &v.ToState, &v.Reason, &v.CreatedAt); e != nil {
			rows.Close()
			return p, e
		}
		p.History = append(p.History, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return p, e
	}
	if e = valid(p); e != nil {
		return p, e
	}
	for i, v := range p.History {
		r, _, err := operation(ctx, tx, a, d, v.OperationID)
		if err != nil {
			return p, ErrInconsistent
		}
		want := p
		want.State = v.ToState
		want.UpdatedAt = v.CreatedAt
		want.LastOperationID = v.OperationID
		want.History = p.History[:i+1]
		if !reflect.DeepEqual(r.Payable, want) {
			return p, ErrInconsistent
		}
	}
	return p, nil
}
