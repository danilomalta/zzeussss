package cashcustody

import (
	"context"
	"database/sql"
	"errors"
	"math/big"

	"titansystem-backend/internal/localdb/identity"
)

type Coverage struct {
	SessionID             string `json:"session_id"`
	Scope                 string `json:"scope"`
	Complete              bool   `json:"complete"`
	WithdrawalCount       int64  `json:"withdrawal_count"`
	WithdrawnCents        int64  `json:"withdrawn_cents"`
	UnassignedCents       int64  `json:"unassigned_cents"`
	PreparedCents         int64  `json:"prepared_cents"`
	InTransitCents        int64  `json:"in_transit_cents"`
	ReceivedDeclaredCents int64  `json:"received_declared_cents"`
	ReceivedCountedCents  int64  `json:"received_counted_cents"`
	DifferenceCents       int64  `json:"difference_cents"`
	DivergentCount        int64  `json:"divergent_count"`
	CancelledBags         int64  `json:"cancelled_bags"`
	RejectedBags          int64  `json:"rejected_bags"`
}

// Coverage reconciles audited withdrawals, not all outflows or a bank balance.
// It is restricted to the closed drawer's original operator with ManageCash.
func Reconcile(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string) (Coverage, error) {
	if db == nil || !validID(id) {
		return Coverage{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Coverage{}, err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, a, d, identity.ManageCash); err != nil {
		return Coverage{}, err
	}
	var closed sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT c.closed_at FROM cash_sessions c JOIN cash_session_operators o ON o.tenant_id=c.tenant_id AND o.store_id=c.store_id AND o.session_id=c.id WHERE c.tenant_id=? AND c.store_id=? AND c.device_id=? AND c.id=? AND c.operator_id=? AND o.identity_id=?`, a.TenantID, a.StoreID, d.DeviceID, id, a.IdentityID, a.IdentityID).Scan(&closed)
	if errors.Is(err, sql.ErrNoRows) {
		return Coverage{}, ErrNotFound
	}
	if err != nil {
		return Coverage{}, err
	}
	if !closed.Valid {
		return Coverage{}, ErrOpen
	}
	var officialClosed string
	err = tx.QueryRowContext(ctx, `SELECT closed_at FROM cash_closures WHERE tenant_id=? AND store_id=? AND session_id=? AND identity_id=?`, a.TenantID, a.StoreID, id, a.IdentityID).Scan(&officialClosed)
	if errors.Is(err, sql.ErrNoRows) {
		return Coverage{}, ErrInconsistent
	}
	if err != nil {
		return Coverage{}, err
	}
	if officialClosed != closed.String {
		return Coverage{}, ErrInconsistent
	}
	// A removed/changed audit cannot make a formerly classified withdrawal
	// disappear from the coverage report, even if it never had a bag.
	var orphaned int
	err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_journal_entries j LEFT JOIN cash_adjustments x ON x.tenant_id=j.tenant_id AND x.store_id=j.store_id AND x.device_id=j.device_id AND x.session_id=j.session_id AND x.operation_id=j.operation_id AND x.movement_id=j.movement_id AND x.kind='withdrawal' WHERE j.tenant_id=? AND j.store_id=? AND j.device_id=? AND j.session_id=? AND j.category='withdrawal' AND x.operation_id IS NULL`, a.TenantID, a.StoreID, d.DeviceID, id).Scan(&orphaned)
	if err != nil {
		return Coverage{}, err
	}
	if orphaned != 0 {
		return Coverage{}, ErrInconsistent
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id FROM cash_adjustments WHERE tenant_id=? AND store_id=? AND device_id=? AND session_id=? AND kind='withdrawal' ORDER BY operation_id`, a.TenantID, a.StoreID, d.DeviceID, id)
	if err != nil {
		return Coverage{}, err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var op string
		if err = rows.Scan(&op); err != nil {
			rows.Close()
			return Coverage{}, err
		}
		ids = append(ids, op)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Coverage{}, err
	}
	v := Coverage{SessionID: id, Scope: "audited_local_withdrawals", WithdrawalCount: int64(len(ids))}
	sums := map[string]*big.Int{}
	for _, key := range []string{"total", "unassigned", "prepared", "in_transit", "received", "counted", "difference"} {
		sums[key] = new(big.Int)
	}
	// Validate every bag, including terminal and orphaned records, before
	// allocating sources. Missing audit rows must not silently disappear.
	all, err := tx.QueryContext(ctx, `SELECT id FROM cash_bags WHERE tenant_id=? AND store_id=? AND device_id=? AND session_id=? ORDER BY id`, a.TenantID, a.StoreID, d.DeviceID, id)
	if err != nil {
		return Coverage{}, err
	}
	bagIDs := []string{}
	for all.Next() {
		var bagID string
		if err = all.Scan(&bagID); err != nil {
			all.Close()
			return Coverage{}, err
		}
		bagIDs = append(bagIDs, bagID)
	}
	err = all.Err()
	all.Close()
	if err != nil {
		return Coverage{}, err
	}
	assigned := map[string]Bag{}
	for _, bagID := range bagIDs {
		b, err := load(ctx, tx, a, d, bagID)
		if err != nil {
			return Coverage{}, err
		}
		if err = validateHistory(b); err != nil {
			return Coverage{}, err
		}
		switch b.State {
		case "cancelled":
			v.CancelledBags++
		case "rejected":
			v.RejectedBags++
		default:
			if _, exists := assigned[b.WithdrawalOperationID]; exists {
				return Coverage{}, ErrInconsistent
			}
			assigned[b.WithdrawalOperationID] = b
		}
	}
	for _, op := range ids {
		o, err := source(ctx, tx, a, d, op)
		if err != nil {
			return Coverage{}, err
		}
		sums["total"].Add(sums["total"], big.NewInt(o.Amount))
		b, exists := assigned[op]
		if !exists {
			sums["unassigned"].Add(sums["unassigned"], big.NewInt(o.Amount))
			continue
		}
		sums[b.State].Add(sums[b.State], big.NewInt(o.Amount))
		if b.State == "received" {
			if b.CountedCents == nil || b.DifferenceCents == nil {
				return Coverage{}, ErrInconsistent
			}
			sums["counted"].Add(sums["counted"], big.NewInt(*b.CountedCents))
			sums["difference"].Add(sums["difference"], big.NewInt(*b.DifferenceCents))
			if *b.DifferenceCents != 0 {
				v.DivergentCount++
			}
		}
	}
	for key, target := range map[string]*int64{"total": &v.WithdrawnCents, "unassigned": &v.UnassignedCents, "prepared": &v.PreparedCents, "in_transit": &v.InTransitCents, "received": &v.ReceivedDeclaredCents, "counted": &v.ReceivedCountedCents, "difference": &v.DifferenceCents} {
		n := sums[key]
		if !n.IsInt64() || n.Cmp(big.NewInt(maxSafe)) > 0 || n.Cmp(big.NewInt(-maxSafe)) < 0 {
			return Coverage{}, ErrRange
		}
		*target = n.Int64()
	}
	accounted := new(big.Int).Add(sums["unassigned"], sums["prepared"])
	accounted.Add(accounted, sums["in_transit"])
	accounted.Add(accounted, sums["received"])
	if accounted.Cmp(sums["total"]) != 0 {
		return Coverage{}, ErrInconsistent
	}
	v.Complete = true
	if err = tx.Commit(); err != nil {
		return Coverage{}, err
	}
	return v, nil
}
