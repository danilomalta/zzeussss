package sale

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

var ErrCartLimit = errors.New("cart revision limit")

type Cart struct {
	ID              string      `json:"id"`
	Revision        int64       `json:"revision"`
	State           string      `json:"state"`
	Label           string      `json:"label"`
	SaleID          string      `json:"sale_id"`
	PaymentID       string      `json:"payment_id"`
	CashSessionID   string      `json:"cash_session_id"`
	Items           []QuoteItem `json:"items"`
	TotalCents      int64       `json:"total_cents"`
	CreatedAt       string      `json:"created_at"`
	UpdatedAt       string      `json:"updated_at"`
	SaleOperationID string      `json:"sale_operation_id"`
	ReceivedCents   int64       `json:"received_cents"`
	ChangeCents     int64       `json:"change_cents"`
}
type CartSaveInput struct {
	OperationID      string `json:"operation_id"`
	CartID           string `json:"cart_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Label            string `json:"label"`
	Items            []Item `json:"items"`
}
type CartActionInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Action           string `json:"action"`
	CashSessionID    string `json:"cash_session_id"`
	ReceivedCents    int64  `json:"received_cents"`
	Reason           string `json:"reason"`
}
type CartResult struct {
	OperationID string `json:"operation_id"`
	Action      string `json:"action"`
	Reason      string `json:"reason"`
	Repeated    bool   `json:"repeated"`
	Cart        Cart   `json:"cart"`
}
type CartPage struct {
	Items   []Cart `json:"items"`
	Total   int64  `json:"total"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"has_more"`
}
type CartHistory struct {
	Items   []CartResult `json:"items"`
	Total   int64        `json:"total"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
	HasMore bool         `json:"has_more"`
}

func cartText(s string, n int, empty bool) bool {
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
func cartKey(s string) bool { return planID(s) && cartText(s, 128, false) }
func cartBegin(ctx context.Context, db *sql.DB, c *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, write bool) (*sql.Tx, error) {
	if db == nil || write && c == nil {
		return nil, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if write {
		e = c.RequireTx(ctx, tx, a, d, identity.Sell, modules.POS)
	} else {
		e = authorizeLegacy(ctx, tx, a, d)
	}
	if e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
func cartReplay(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, hash string) (CartResult, bool, error) {
	var actor, h, raw string
	e := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash,result_json FROM pos_cart_operations WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&actor, &h, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		return CartResult{}, false, nil
	}
	if e != nil {
		return CartResult{}, false, e
	}
	if actor != a.IdentityID {
		return CartResult{}, false, ErrNotFound
	}
	if h != hash {
		return CartResult{}, false, ErrConflict
	}
	r, e := cartOperationTx(ctx, tx, a, d, op, raw)
	r.Repeated = true
	return r, true, e
}
func cartFinish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, r CartResult, h string) (CartResult, error) {
	raw, _ := json.Marshal(r)
	v := r.Cart
	b, _ := json.Marshal(v)
	if v.Revision == 1 {
		_, e := tx.ExecContext(ctx, `INSERT INTO pos_carts VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, v.ID, a.IdentityID, v.State, v.Revision, v.SaleID, string(b), v.UpdatedAt)
		if e != nil {
			return r, e
		}
	} else {
		res, e := tx.ExecContext(ctx, `UPDATE pos_carts SET state=?,revision=?,payload_json=?,updated_at=? WHERE tenant_id=? AND store_id=? AND device_id=? AND id=? AND actor_id=? AND revision=?`, v.State, v.Revision, string(b), v.UpdatedAt, a.TenantID, a.StoreID, d.DeviceID, v.ID, a.IdentityID, v.Revision-1)
		if e != nil {
			return r, e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return r, e
		}
		if n != 1 {
			return r, ErrConflict
		}
	}
	if _, e := tx.ExecContext(ctx, `INSERT INTO pos_cart_operations VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, v.ID, a.IdentityID, v.Revision, r.Action, h, string(raw), v.UpdatedAt); e != nil {
		return r, e
	}
	if _, e := tx.ExecContext(ctx, `INSERT INTO pos_cart_events VALUES(?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, "pos.cart."+r.Action, string(raw), v.UpdatedAt); e != nil {
		return r, e
	}
	return r, tx.Commit()
}
func cartNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }
