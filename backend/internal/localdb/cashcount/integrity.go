package cashcount

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"titansystem-backend/internal/localdb/cashvault"
	"titansystem-backend/internal/localdb/identity"
)

func valid(c Count) error {
	if c.ExpectedCents == nil || *c.ExpectedCents < 0 || *c.ExpectedCents > maxSafe || c.CreatorID == c.CounterID || len(c.History) < 1 || len(c.History) > 3 {
		return ErrInconsistent
	}
	state := ""
	for i, e := range c.History {
		if e.FromState != state {
			return ErrInconsistent
		}
		ok := false
		switch e.Action {
		case "create":
			ok = i == 0 && state == "" && e.ToState == "counting" && e.ActorID == c.CreatorID && e.Reason == c.Reason && e.CreatedAt == c.CreatedAt
		case "submit":
			ok = state == "counting" && e.ToState == "submitted" && e.ActorID == c.CounterID && e.Reason == ""
		case "cancel":
			ok = state == "counting" && e.ToState == "cancelled" && e.ActorID == c.CreatorID && text(e.Reason, 500)
		case "accept", "reject":
			ok = state == "submitted" && e.ActorID != c.CounterID && c.ReviewerID != nil && e.ActorID == *c.ReviewerID && text(e.Reason, 500) && ((e.Action == "accept" && e.ToState == "accepted") || (e.Action == "reject" && e.ToState == "rejected"))
		}
		if !ok {
			return ErrInconsistent
		}
		state = e.ToState
	}
	last := c.History[len(c.History)-1]
	if state != c.State || last.OperationID != c.LastOperationID || last.CreatedAt != c.UpdatedAt {
		return ErrInconsistent
	}
	if c.State == "counting" || c.State == "cancelled" {
		if c.CountedCents != nil || c.DifferenceCents != nil || c.ReviewerID != nil {
			return ErrInconsistent
		}
	} else {
		if c.CountedCents == nil || *c.CountedCents < 0 || *c.CountedCents > maxSafe || c.DifferenceCents == nil || *c.DifferenceCents != *c.CountedCents-*c.ExpectedCents {
			return ErrInconsistent
		}
		if (c.State == "submitted") != (c.ReviewerID == nil) {
			return ErrInconsistent
		}
	}
	return nil
}
func readOperation(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Result, string, error) {
	var r Result
	var body, actor, countID, action, from, to, reason, at, eventBody, eventType string
	err := tx.QueryRowContext(ctx, `SELECT o.result_json,o.actor_id,o.count_id,o.action,o.from_state,o.to_state,o.reason,o.created_at,e.payload_json,e.event_type FROM cash_vault_count_operations o LEFT JOIN cash_vault_count_events e ON e.tenant_id=o.tenant_id AND e.store_id=o.store_id AND e.device_id=o.device_id AND e.operation_id=o.operation_id WHERE o.tenant_id=? AND o.store_id=? AND o.device_id=? AND o.operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&body, &actor, &countID, &action, &from, &to, &reason, &at, &eventBody, &eventType)
	if errors.Is(err, sql.ErrNoRows) {
		return r, "", ErrNotFound
	}
	if err != nil {
		return r, "", ErrInconsistent
	}
	if json.Unmarshal([]byte(body), &r) != nil || r.OperationID != id || r.Repeated || r.Count.ID != countID || valid(r.Count) != nil || eventBody != body || eventType != "cash.vault.count."+action {
		return r, "", ErrInconsistent
	}
	last := r.Count.History[len(r.Count.History)-1]
	if last != (Event{id, actor, action, from, to, reason, at}) {
		return r, "", ErrInconsistent
	}
	return r, actor, nil
}
func load(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Count, error) {
	var c Count
	err := tx.QueryRowContext(ctx, `SELECT id,vault_id,creator_id,counter_id,reason,state,expected_cents,counted_cents,difference_cents,vault_operation_id,reviewer_id,created_at,updated_at,last_operation_id FROM cash_vault_counts WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&c.ID, &c.VaultID, &c.CreatorID, &c.CounterID, &c.Reason, &c.State, &c.ExpectedCents, &c.CountedCents, &c.DifferenceCents, &c.VaultOperationID, &c.ReviewerID, &c.CreatedAt, &c.UpdatedAt, &c.LastOperationID)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,actor_id,action,from_state,to_state,reason,created_at FROM cash_vault_count_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND count_id=? ORDER BY CASE action WHEN 'create' THEN 0 WHEN 'submit' THEN 1 ELSE 2 END`, a.TenantID, a.StoreID, d.DeviceID, id)
	if err != nil {
		return c, err
	}
	c.History = []Event{}
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.OperationID, &e.ActorID, &e.Action, &e.FromState, &e.ToState, &e.Reason, &e.CreatedAt); err != nil {
			rows.Close()
			return c, err
		}
		c.History = append(c.History, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return c, err
	}
	if err = valid(c); err != nil {
		return c, err
	}
	for i, e := range c.History {
		r, _, err := readOperation(ctx, tx, a, d, e.OperationID)
		if err != nil {
			return c, ErrInconsistent
		}
		expected := c
		expected.State = e.ToState
		expected.UpdatedAt = e.CreatedAt
		expected.LastOperationID = e.OperationID
		expected.History = c.History[:i+1]
		if e.ToState == "counting" || e.ToState == "cancelled" {
			expected.CountedCents = nil
			expected.DifferenceCents = nil
			expected.ReviewerID = nil
		} else if e.ToState == "submitted" {
			expected.ReviewerID = nil
		}
		if !reflect.DeepEqual(r.Count, expected) {
			return c, ErrInconsistent
		}
	}
	// Only an unfinished count is tied to the current frozen vault head.
	if c.State == "counting" || c.State == "submitted" {
		v, err := cashvault.InspectTx(ctx, tx, a, d, c.VaultID)
		if err != nil {
			return c, err
		}
		if v.State != "active" || v.LastOperationID != c.VaultOperationID || v.BalanceCents != *c.ExpectedCents {
			return c, ErrInconsistent
		}
	}
	return c, nil
}
