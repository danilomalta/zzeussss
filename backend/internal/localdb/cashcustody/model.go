// Package cashcustody records local handover of existing cash withdrawals.
// It does not move money, approve electronic payments or settle a bank account.
package cashcustody

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrInvalid      = errors.New("dados de malote invalidos")
	ErrNotFound     = errors.New("malote ou origem nao encontrado neste escopo")
	ErrConflict     = errors.New("operacao de malote conflitante")
	ErrInconsistent = errors.New("malote e origem nao conferem")
	ErrRange        = errors.New("custodia excede inteiros exatos do cliente")
	ErrOpen         = errors.New("conciliacao disponivel depois do fechamento")
)

const maxSafe int64 = 9007199254740991

type CreateInput struct {
	OperationID           string `json:"operation_id"`
	BagID                 string `json:"bag_id"`
	WithdrawalOperationID string `json:"withdrawal_operation_id"`
	SealID                string `json:"seal_id"`
}
type ActionInput struct {
	OperationID  string `json:"operation_id"`
	SealID       string `json:"seal_id"`
	CountedCents int64  `json:"counted_cents"`
	Reason       string `json:"reason"`
}
type Event struct {
	OperationID string `json:"operation_id"`
	ActorID     string `json:"actor_id"`
	Action      string `json:"action"`
	FromState   string `json:"from_state"`
	ToState     string `json:"to_state"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
type Bag struct {
	ID                    string  `json:"id"`
	SessionID             string  `json:"session_id"`
	WithdrawalOperationID string  `json:"withdrawal_operation_id"`
	MovementID            string  `json:"movement_id"`
	SenderID              string  `json:"sender_id"`
	OperatorID            string  `json:"operator_id"`
	SealID                string  `json:"seal_id"`
	State                 string  `json:"state"`
	DeclaredCents         *int64  `json:"declared_cents"`
	CountedCents          *int64  `json:"counted_cents"`
	DifferenceCents       *int64  `json:"difference_cents"`
	ReceiverID            *string `json:"receiver_id"`
	ReceivedAt            *string `json:"received_at"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
	History               []Event `json:"history"`
}
type Result struct {
	OperationID string `json:"operation_id"`
	Repeated    bool   `json:"repeated"`
	Bag         Bag    `json:"bag"`
}
type ListInput struct {
	State     string
	SessionID string
	Limit     int
	Offset    int
}
type Page struct {
	Items      []Bag `json:"items"`
	Limit      int   `json:"limit"`
	Offset     int   `json:"offset"`
	TotalCount int64 `json:"total_count"`
	HasMore    bool  `json:"has_more"`
}

func validID(s string) bool {
	return s != "" && len(s) <= 128 && s == strings.TrimSpace(s) && !strings.ContainsRune(s, 0) && utf8.ValidString(s)
}

// New resource/receipt IDs must round-trip as one URL path segment.
func pathID(s string) bool {
	if !validID(s) {
		return false
	}
	for i, c := range []byte(s) {
		alpha := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alpha && (i == 0 || c != '_' && c != '.' && c != ':' && c != '-') {
			return false
		}
	}
	return true
}
func validState(s string) bool {
	return s == "prepared" || s == "in_transit" || s == "received" || s == "rejected" || s == "cancelled"
}
func validReason(s string) bool {
	return utf8.ValidString(s) && strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= 500 && !strings.ContainsRune(s, 0)
}
func checked(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		var coded interface{ Code() int }
		if errors.As(err, &coded) && (coded.Code() == 1555 || coded.Code() == 2067) {
			return ErrConflict
		}
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInconsistent
	}
	return nil
}

type origin struct {
	SessionID, MovementID, ActorID, OperatorID string
	Amount                                     int64
}

func source(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (origin, error) {
	var o origin
	var kind string
	var delta sql.NullInt64
	var category, who sql.NullString
	var posted sql.NullInt64
	var journalActor, journalOp, journalAt, movementAt sql.NullString
	var adjustmentAt string
	err := tx.QueryRowContext(ctx, `SELECT x.session_id,x.movement_id,x.actor_identity_id,c.operator_id,x.kind,x.amount_cents,m.amount_cents,j.category,j.cash_delta_cents,b.identity_id,j.actor_identity_id,j.operation_id,j.occurred_at,m.created_at,x.created_at
 FROM cash_adjustments x JOIN cash_sessions c ON c.tenant_id=x.tenant_id AND c.store_id=x.store_id AND c.device_id=x.device_id AND c.id=x.session_id
 LEFT JOIN cash_session_operators b ON b.tenant_id=c.tenant_id AND b.store_id=c.store_id AND b.session_id=c.id
 LEFT JOIN cash_movements m ON m.tenant_id=x.tenant_id AND m.store_id=x.store_id AND m.cash_session_id=x.session_id AND m.id=x.movement_id
 LEFT JOIN cash_journal_entries j ON j.tenant_id=x.tenant_id AND j.store_id=x.store_id AND j.device_id=x.device_id AND j.session_id=x.session_id AND j.entry_id='movement:'||x.movement_id
 WHERE x.tenant_id=? AND x.store_id=? AND x.device_id=? AND x.operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&o.SessionID, &o.MovementID, &o.ActorID, &o.OperatorID, &kind, &o.Amount, &delta, &category, &posted, &who, &journalActor, &journalOp, &journalAt, &movementAt, &adjustmentAt)
	if errors.Is(err, sql.ErrNoRows) {
		return o, ErrNotFound
	}
	if err != nil {
		return o, err
	}
	if kind != "withdrawal" {
		return o, ErrConflict
	}
	if o.Amount <= 0 || o.Amount > maxSafe {
		return o, ErrRange
	}
	if !delta.Valid || delta.Int64 != -o.Amount || !posted.Valid || posted.Int64 != -o.Amount || !category.Valid || category.String != "withdrawal" || !who.Valid || who.String != o.OperatorID || !journalActor.Valid || journalActor.String != o.ActorID || !journalOp.Valid || journalOp.String != id || !journalAt.Valid || !movementAt.Valid || journalAt.String != movementAt.String || movementAt.String != adjustmentAt {
		return o, ErrInconsistent
	}
	return o, nil
}
func load(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, id string) (Bag, error) {
	b := Bag{History: make([]Event, 0)}
	err := tx.QueryRowContext(ctx, `SELECT id,session_id,withdrawal_operation_id,movement_id,sender_id,operator_id,seal_id,state,declared_cents,counted_cents,difference_cents,receiver_id,received_at,created_at,updated_at FROM cash_bags WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&b.ID, &b.SessionID, &b.WithdrawalOperationID, &b.MovementID, &b.SenderID, &b.OperatorID, &b.SealID, &b.State, &b.DeclaredCents, &b.CountedCents, &b.DifferenceCents, &b.ReceiverID, &b.ReceivedAt, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return b, ErrNotFound
	}
	if err != nil {
		return b, err
	}
	o, err := source(ctx, tx, a, d, b.WithdrawalOperationID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) {
		return b, ErrInconsistent
	}
	if err != nil {
		return b, err
	}
	if b.DeclaredCents == nil || *b.DeclaredCents != o.Amount || b.SessionID != o.SessionID || b.MovementID != o.MovementID || b.SenderID != o.ActorID || b.OperatorID != o.OperatorID {
		return b, ErrInconsistent
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id,actor_id,action,from_state,to_state,reason,created_at FROM cash_bag_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND bag_id=? ORDER BY sequence`, a.TenantID, a.StoreID, d.DeviceID, id)
	if err != nil {
		return b, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Event
		if err = rows.Scan(&e.OperationID, &e.ActorID, &e.Action, &e.FromState, &e.ToState, &e.Reason, &e.CreatedAt); err != nil {
			return b, err
		}
		b.History = append(b.History, e)
	}
	return b, rows.Err()
}
func visible(b Bag, actor string) Bag {
	if actor != b.SenderID && actor != b.OperatorID && b.State != "received" {
		b.DeclaredCents = nil
	}
	return b
}
