package stocktaking

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stockreservation"
	"titansystem-backend/internal/sqlitewrite"
)

func Open(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in OpenInput, sessionTokens ...string) (Result, error) {
	if db == nil || !validID(in.OperationID) || !validID(in.ID) || !validID(in.LocationID) || !validText(in.Title, 160) || len(in.Items) < 1 || len(in.Items) > MaxLines {
		return Result{}, ErrInvalid
	}
	in.Items = append([]Item(nil), in.Items...)
	sort.Slice(in.Items, func(i, j int) bool { return in.Items[i].ProductID < in.Items[j].ProductID })
	for i, x := range in.Items {
		if !validID(x.ProductID) || !unitOK(x.Unit) || (i > 0 && in.Items[i-1].ProductID == x.ProductID) {
			return Result{}, ErrInvalid
		}
	}
	hash, err := hashRequest("open", in.ID, in)
	if err != nil {
		return Result{}, err
	}
	return writeSession(ctx, db, a, d, sessionTokens, func(tx *sql.Tx) (Result, error) { return openTx(ctx, tx, contracts, a, d, in, hash) })
}
func openTx(ctx context.Context, tx *sql.Tx, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, in OpenInput, hash string) (Result, error) {
	var err error
	if err = authorize(ctx, tx, a, d); err != nil {
		return Result{}, err
	}
	if r, found, err := prior(ctx, tx, a, d, in.OperationID, hash); err != nil || found {
		return r, err
	}
	if err = requireContract(ctx, tx, contracts, a, d); err != nil {
		return Result{}, err
	}
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM reviewed_inventory_sessions WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.ID).Scan(&exists); err != nil {
		return Result{}, err
	}
	if exists != 0 {
		return Result{}, ErrConflict
	}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM stock_locations WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.LocationID).Scan(&exists); err != nil {
		return Result{}, err
	}
	if exists != 1 {
		return Result{}, ErrInvalid
	}
	now := timestamp()
	err = writeOne(ctx, tx, `INSERT INTO reviewed_inventory_sessions VALUES(?,?,?, ?,?,'open',1,?,?,?)`, a.TenantID, a.StoreID, in.ID, in.LocationID, in.Title, a.IdentityID, now, now)
	if err != nil {
		return Result{}, err
	}
	for _, x := range in.Items {
		var unit string
		if err = tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, x.ProductID).Scan(&unit); errors.Is(err, sql.ErrNoRows) {
			return Result{}, ErrInvalid
		} else if err != nil {
			return Result{}, err
		}
		if unit != x.Unit {
			return Result{}, ErrConflict
		}
		err = writeOne(ctx, tx, `INSERT INTO reviewed_inventory_lines(tenant_id,store_id,session_id,product_id,unit) VALUES(?,?,?,?,?)`, a.TenantID, a.StoreID, in.ID, x.ProductID, x.Unit)
		if err != nil {
			return Result{}, err
		}
	}
	return finish(ctx, tx, a, d, hash, in.Title, Result{OperationID: in.OperationID, InventoryID: in.ID, Action: "open", Revision: 1, Status: "open", OccurredAt: now}, in)
}

func Count(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in CountInput, sessionTokens ...string) (Result, error) {
	if db == nil || !validID(id) || !validID(in.OperationID) || in.ExpectedRevision < 1 || in.ExpectedRevision >= MaxQuantity || len(in.Items) < 1 || len(in.Items) > MaxLines {
		return Result{}, ErrInvalid
	}
	in.Items = append([]CountItem(nil), in.Items...)
	sort.Slice(in.Items, func(i, j int) bool { return in.Items[i].ProductID < in.Items[j].ProductID })
	for i, x := range in.Items {
		if !validID(x.ProductID) || x.CountedMilli < 0 || x.CountedMilli > MaxQuantity || (i > 0 && in.Items[i-1].ProductID == x.ProductID) {
			return Result{}, ErrInvalid
		}
	}
	hash, err := hashRequest("count", id, in)
	if err != nil {
		return Result{}, err
	}
	return writeSession(ctx, db, a, d, sessionTokens, func(tx *sql.Tx) (Result, error) { return countTx(ctx, tx, contracts, a, d, id, in, hash) })
}
func countTx(ctx context.Context, tx *sql.Tx, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in CountInput, hash string) (Result, error) {
	var err error
	if err = authorize(ctx, tx, a, d); err != nil {
		return Result{}, err
	}
	if r, found, err := prior(ctx, tx, a, d, in.OperationID, hash); err != nil || found {
		return r, err
	}
	if err = requireContract(ctx, tx, contracts, a, d); err != nil {
		return Result{}, err
	}
	s, err := load(ctx, tx, a, id)
	if err != nil {
		return Result{}, err
	}
	if s.Status != "open" || s.Revision != in.ExpectedRevision {
		return Result{}, ErrConflict
	}
	now := timestamp()
	for _, x := range in.Items {
		var unit, current string
		err = tx.QueryRowContext(ctx, `SELECT l.unit,p.unit FROM reviewed_inventory_lines l JOIN products p ON p.tenant_id=l.tenant_id AND p.id=l.product_id WHERE l.tenant_id=? AND l.store_id=? AND l.session_id=? AND l.product_id=?`, a.TenantID, a.StoreID, id, x.ProductID).Scan(&unit, &current)
		if errors.Is(err, sql.ErrNoRows) {
			return Result{}, ErrInvalid
		}
		if err != nil {
			return Result{}, err
		}
		if unit != current {
			return Result{}, ErrConflict
		}
		if !quantityOK(unit, x.CountedMilli) {
			return Result{}, ErrInvalid
		}
		previous, fingerprint, err := stockSnapshot(ctx, tx, a, x.ProductID, s.LocationID)
		if err != nil {
			return Result{}, err
		}
		err = writeOne(ctx, tx, `UPDATE reviewed_inventory_lines SET counted_milli=?,previous_milli=?,movement_fingerprint=?,counted_by=?,counted_at=? WHERE tenant_id=? AND store_id=? AND session_id=? AND product_id=?`, x.CountedMilli, previous, fingerprint, a.IdentityID, now, a.TenantID, a.StoreID, id, x.ProductID)
		if err != nil {
			return Result{}, err
		}
	}
	err = writeOne(ctx, tx, `UPDATE reviewed_inventory_sessions SET revision=revision+1,updated_at=? WHERE tenant_id=? AND store_id=? AND id=?`, now, a.TenantID, a.StoreID, id)
	if err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, a, d, hash, "", Result{OperationID: in.OperationID, InventoryID: id, Action: "count", Revision: s.Revision + 1, Status: s.Status, OccurredAt: now}, in)
}

func ChangeState(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in StateInput, sessionTokens ...string) (Result, error) {
	if db == nil || !validID(id) || !validID(in.OperationID) || in.ExpectedRevision < 1 || in.ExpectedRevision >= MaxQuantity || !validText(in.Reason, 255) {
		return Result{}, ErrInvalid
	}
	switch in.Action {
	case "submit", "approve", "reject", "cancel", "reopen":
	default:
		return Result{}, ErrInvalid
	}
	hash, err := hashRequest("state", id, in)
	if err != nil {
		return Result{}, err
	}
	return writeSession(ctx, db, a, d, sessionTokens, func(tx *sql.Tx) (Result, error) { return changeStateTx(ctx, tx, contracts, a, d, id, in, hash) })
}
func changeStateTx(ctx context.Context, tx *sql.Tx, contracts *entitlementstore.Store, a identity.Scope, d identity.DeviceContext, id string, in StateInput, hash string) (Result, error) {
	var err error
	if err = authorize(ctx, tx, a, d); err != nil {
		return Result{}, err
	}
	if r, found, err := prior(ctx, tx, a, d, in.OperationID, hash); err != nil || found {
		return r, err
	}
	if err = requireContract(ctx, tx, contracts, a, d); err != nil {
		return Result{}, err
	}
	s, err := load(ctx, tx, a, id)
	if err != nil {
		return Result{}, err
	}
	if s.Revision != in.ExpectedRevision {
		return Result{}, ErrConflict
	}
	lines, err := linesTx(ctx, tx, a, id)
	if err != nil {
		return Result{}, err
	}
	status := ""
	changed := 0
	now := timestamp()
	switch in.Action {
	case "submit":
		if s.Status != "open" {
			return Result{}, ErrConflict
		}
		for _, l := range lines {
			if l.CountedMilli == nil {
				return Result{}, ErrConflict
			}
		}
		status = "submitted"
	case "reopen":
		if s.Status != "submitted" {
			return Result{}, ErrConflict
		}
		status = "open"
	case "approve", "reject":
		if s.Status != "submitted" {
			return Result{}, ErrConflict
		}
		if err = requireReviewer(ctx, tx, a); err != nil {
			return Result{}, err
		}
		for _, l := range lines {
			if l.CountedBy == nil || *l.CountedBy == a.IdentityID {
				return Result{}, identity.ErrDenied
			}
		}
		if in.Action == "reject" {
			status = "rejected"
			break
		}
		changed, err = applyCounts(ctx, tx, a, d, s, lines, now)
		if err != nil {
			return Result{}, err
		}
		status = "approved"
	case "cancel":
		if s.Status != "open" && s.Status != "submitted" {
			return Result{}, ErrConflict
		}
		if s.CreatedBy != a.IdentityID {
			if err = requireReviewer(ctx, tx, a); err != nil {
				return Result{}, err
			}
		}
		status = "cancelled"
	}
	err = writeOne(ctx, tx, `UPDATE reviewed_inventory_sessions SET status=?,revision=revision+1,updated_at=? WHERE tenant_id=? AND store_id=? AND id=?`, status, now, a.TenantID, a.StoreID, id)
	if err != nil {
		return Result{}, err
	}
	return finish(ctx, tx, a, d, hash, in.Reason, Result{OperationID: in.OperationID, InventoryID: id, Action: in.Action, Revision: s.Revision + 1, Status: status, ChangedLines: changed, OccurredAt: now}, in)
}

func applyCounts(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, s Session, lines []Line, now string) (int, error) {
	changed := 0
	for _, l := range lines {
		if l.CountedMilli == nil || l.PreviousMilli == nil {
			return 0, ErrConflict
		}
		var fingerprint, unit string
		if err := tx.QueryRowContext(ctx, `SELECT movement_fingerprint FROM reviewed_inventory_lines WHERE tenant_id=? AND store_id=? AND session_id=? AND product_id=?`, a.TenantID, a.StoreID, s.ID, l.ProductID).Scan(&fingerprint); err != nil {
			return 0, err
		}
		if err := tx.QueryRowContext(ctx, `SELECT unit FROM products WHERE tenant_id=? AND id=?`, a.TenantID, l.ProductID).Scan(&unit); err != nil {
			return 0, err
		}
		if unit != l.Unit {
			return 0, ErrConflict
		}
		balance, currentHash, err := stockSnapshot(ctx, tx, a, l.ProductID, s.LocationID)
		if err != nil {
			return 0, err
		}
		if fingerprint != currentHash || balance != *l.PreviousMilli {
			return 0, ErrStale
		}
		held, err := stockreservation.HeldTx(ctx, tx, a.TenantID, a.StoreID, l.ProductID, s.LocationID)
		if errors.Is(err, stockreservation.ErrUnavailable) {
			return 0, ErrReserved
		}
		if err != nil {
			return 0, err
		}
		if *l.CountedMilli < held {
			return 0, ErrReserved
		}
		difference := *l.CountedMilli - balance
		if difference == 0 {
			continue
		}
		movementID, err := localdb.NewID()
		if err != nil {
			return 0, err
		}
		err = writeOne(ctx, tx, `INSERT INTO stock_movements(id,tenant_id,store_id,device_id,product_id,location_id,quantity_milli,reason,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, movementID, a.TenantID, a.StoreID, d.DeviceID, l.ProductID, s.LocationID, difference, "inventario-aprovado:"+s.ID, now)
		if err != nil {
			return 0, err
		}
		err = writeOne(ctx, tx, `UPDATE reviewed_inventory_lines SET movement_id=? WHERE tenant_id=? AND store_id=? AND session_id=? AND product_id=?`, movementID, a.TenantID, a.StoreID, s.ID, l.ProductID)
		if err != nil {
			return 0, err
		}
		changed++
	}
	return changed, nil
}

// Trusted in-process callers retain the existing optional-token API.
// HTTP always supplies one token, which is revalidated before any business read.
func writeSession(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, tokens []string, work func(*sql.Tx) (Result, error)) (Result, error) {
	if len(tokens) > 1 {
		return Result{}, ErrInvalid
	}
	var out Result
	err := sqlitewrite.Run(ctx, db, ErrConflict, func(tx *sql.Tx) error {
		if len(tokens) == 1 {
			if err := fiscal.ValidateSaleSessionTx(ctx, tx, a, d, tokens[0]); err != nil {
				return err
			}
		}
		var err error
		out, err = work(tx)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return out, nil
}
