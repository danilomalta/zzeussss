package stocktaking

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

func writeOne(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	r, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func validID(s string) bool {
	if len(s) < 1 || len(s) > 128 || strings.TrimSpace(s) != s || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 33 || r == 127 {
			return false
		}
	}
	return true
}
func validText(s string, limit int) bool {
	if len(s) < 1 || len(s) > limit || strings.TrimSpace(s) != s || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func unitOK(unit string) bool {
	return unit == "unit" || unit == "kg" || unit == "g" || unit == "liter" || unit == "ml" || unit == "meter"
}
func quantityOK(unit string, n int64) bool {
	return n >= 0 && n <= MaxQuantity && (unit != "unit" || n%1000 == 0)
}
func authorize(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
	return identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStock)
}
func requireContract(ctx context.Context, tx *sql.Tx, store *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext) error {
	return store.RequireTx(ctx, tx, actor, device, identity.ManageStock, modules.Inventory)
}
func requireReviewer(ctx context.Context, tx *sql.Tx, a identity.Scope) error {
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.TenantID, a.IdentityID).Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identity.ErrDenied
		}
		return err
	}
	if role != "owner" && role != "manager" {
		return identity.ErrDenied
	}
	return nil
}
func hashRequest(kind, id string, in any) (string, error) {
	b, err := json.Marshal(struct {
		Kind  string
		ID    string
		Input any
	}{kind, id, in})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func prior(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, op, hash string) (Result, bool, error) {
	var actor, savedHash, body string
	err := tx.QueryRowContext(ctx, `SELECT actor_id,request_hash,response_json FROM reviewed_inventory_receipts WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=?`, a.TenantID, a.StoreID, d.DeviceID, op).Scan(&actor, &savedHash, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	if actor != a.IdentityID || savedHash != hash {
		return Result{}, true, ErrConflict
	}
	var r Result
	if err = json.Unmarshal([]byte(body), &r); err != nil {
		return r, true, err
	}
	r.Repeated = true
	return r, true, nil
}
func load(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (Session, error) {
	var s Session
	err := tx.QueryRowContext(ctx, `SELECT id,location_id,title,status,revision,created_by,created_at,updated_at FROM reviewed_inventory_sessions WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&s.ID, &s.LocationID, &s.Title, &s.Status, &s.Revision, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// A sorted hash detects even two opposite stock movements with net zero. The
// exact sum avoids SQLite SUM overflow and a floating point TOTAL fallback.
func stockSnapshot(ctx context.Context, tx *sql.Tx, a identity.Scope, product, location string) (int64, string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,quantity_milli FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=? ORDER BY id`, a.TenantID, a.StoreID, product, location)
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()
	h := sha256.New()
	total := new(big.Int)
	for rows.Next() {
		var id string
		var n int64
		if err = rows.Scan(&id, &n); err != nil {
			return 0, "", err
		}
		fmt.Fprintf(h, "%d:%s:%d;", len(id), id, n)
		total.Add(total, big.NewInt(n))
	}
	if err = rows.Err(); err != nil {
		return 0, "", err
	}
	if !total.IsInt64() || total.Sign() < 0 || total.Cmp(big.NewInt(MaxQuantity)) > 0 {
		return 0, "", ErrConflict
	}
	return total.Int64(), hex.EncodeToString(h.Sum(nil)), nil
}
func linesTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) ([]Line, error) {
	rows, err := tx.QueryContext(ctx, `SELECT product_id,unit,counted_milli,previous_milli,counted_by,counted_at,movement_id FROM reviewed_inventory_lines WHERE tenant_id=? AND store_id=? AND session_id=? ORDER BY product_id`, a.TenantID, a.StoreID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lines := []Line{}
	for rows.Next() {
		var l Line
		if err = rows.Scan(&l.ProductID, &l.Unit, &l.CountedMilli, &l.PreviousMilli, &l.CountedBy, &l.CountedAt, &l.MovementID); err != nil {
			return nil, err
		}
		if l.CountedMilli != nil && l.PreviousMilli != nil {
			v := *l.CountedMilli - *l.PreviousMilli
			l.DifferenceMilli = &v
		}
		lines = append(lines, l)
	}
	return lines, rows.Err()
}
func finish(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, hash, reason string, r Result, request any) (Result, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return Result{}, err
	}
	body, err := json.Marshal(r)
	if err != nil {
		return Result{}, err
	}
	err = writeOne(ctx, tx, `INSERT INTO reviewed_inventory_receipts(tenant_id,store_id,device_id,operation_id,actor_id,session_id,request_hash,request_json,response_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, a.IdentityID, r.InventoryID, hash, string(requestBody), string(body), r.OccurredAt)
	if err != nil {
		return Result{}, err
	}
	err = writeOne(ctx, tx, `INSERT INTO reviewed_inventory_events VALUES(?,?,?,?,?,?,?,?,?,?)`, a.TenantID, a.StoreID, r.InventoryID, r.Revision, r.OperationID, a.IdentityID, d.DeviceID, r.Action, reason, r.OccurredAt)
	if err != nil {
		return Result{}, err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(struct {
		TenantID string `json:"tenant_id"`
		StoreID  string `json:"store_id"`
		ActorID  string `json:"actor_id"`
		Result   Result `json:"result"`
	}{a.TenantID, a.StoreID, a.IdentityID, r})
	if err != nil {
		return Result{}, err
	}
	err = writeOne(ctx, tx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'inventory.reviewed',1,?,?)`, eventID, a.TenantID, a.StoreID, d.DeviceID, r.OperationID, r.InventoryID, string(payload), r.OccurredAt)
	if err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return r, nil
}
