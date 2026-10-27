package purchases

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"sort"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

const MaxBoardOrders = 200
const MaxBoardEvidence = 2000

var ErrBoardLimit = errors.New("receiving board source limit; narrow period/supplier/product")

type BoardInput struct {
	From, To, SupplierID, ProductID, State string
	Limit, Offset                          int
}
type BoardOrder struct {
	OrderID          string   `json:"order_id"`
	SupplierID       string   `json:"supplier_id"`
	ProductID        string   `json:"product_id"`
	Unit             string   `json:"unit"`
	CreatedAt        string   `json:"created_at"`
	CommercialStatus string   `json:"commercial_status"`
	ReceivingStatus  string   `json:"receiving_status"`
	PlannedMilli     int64    `json:"planned_milli"`
	AcceptedMilli    int64    `json:"accepted_milli"`
	RemainingMilli   int64    `json:"remaining_milli"`
	ReviewFlags      []string `json:"review_flags"`
}
type BoardGroup struct {
	SupplierID          string `json:"supplier_id"`
	ProductID           string `json:"product_id"`
	Unit                string `json:"unit"`
	Orders              int    `json:"orders"`
	PendingOrders       int    `json:"pending_orders"`
	ReceivedOrders      int    `json:"received_orders"`
	CancelledOrders     int    `json:"cancelled_orders"`
	PlannedMilliExact   string `json:"planned_milli_exact"`
	AcceptedMilliExact  string `json:"accepted_milli_exact"`
	RemainingMilliExact string `json:"remaining_milli_exact"`
	PendingMilliExact   string `json:"pending_milli_exact"`
}
type ReceivingBoard struct {
	From           string       `json:"from"`
	To             string       `json:"to"`
	MeasuredAt     string       `json:"measured_at"`
	State          string       `json:"state"`
	SupplierID     string       `json:"supplier_id"`
	ProductID      string       `json:"product_id"`
	Basis          string       `json:"basis"`
	CohortOrders   int          `json:"cohort_orders"`
	SourceEvidence int64        `json:"source_evidence"`
	MatchedOrders  int          `json:"matched_orders"`
	Limit          int          `json:"limit"`
	Offset         int          `json:"offset"`
	Groups         []BoardGroup `json:"groups"`
	Items          []BoardOrder `json:"items"`
}

func boardRange(in BoardInput) (time.Time, time.Time, error) {
	from, e := time.Parse(time.RFC3339Nano, in.From)
	to, f := time.Parse(time.RFC3339Nano, in.To)
	from = from.UTC()
	to = to.UTC()
	if e != nil || f != nil || from.Year() < 2000 || to.Year() > 2200 || from.Nanosecond() != 0 || to.Nanosecond() != 0 || !from.Before(to) || to.Sub(from) > 366*24*time.Hour || in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.Offset > 1000000 || (in.SupplierID != "" && !validSearchID(in.SupplierID)) || (in.ProductID != "" && !validSearchID(in.ProductID)) {
		return from, to, ErrInvalid
	}
	switch in.State {
	case "pending", "all", "received", "cancelled", "review":
	default:
		return from, to, ErrInvalid
	}
	return from.UTC(), to.UTC(), nil
}
func boardMatches(r ReceivingOverview, state string) bool {
	switch state {
	case "all":
		return true
	case "cancelled":
		return r.CommercialStatus == "cancelled"
	case "pending":
		return r.CommercialStatus != "cancelled" && r.RemainingMilli > 0
	case "received":
		return r.CommercialStatus != "cancelled" && r.RemainingMilli == 0
	case "review":
		return len(r.ReviewFlags) > 0
	}
	return false
}

type boardAccumulator struct {
	row                                   BoardGroup
	planned, accepted, remaining, pending *big.Int
}

func boardGroupAdd(groups map[[3]string]*boardAccumulator, r ReceivingOverview) {
	key := [3]string{r.SupplierID, r.ProductID, r.Unit}
	g := groups[key]
	if g == nil {
		g = &boardAccumulator{row: BoardGroup{SupplierID: r.SupplierID, ProductID: r.ProductID, Unit: r.Unit}, planned: new(big.Int), accepted: new(big.Int), remaining: new(big.Int), pending: new(big.Int)}
		groups[key] = g
	}
	g.row.Orders++
	g.planned.Add(g.planned, big.NewInt(r.PlannedMilli))
	g.accepted.Add(g.accepted, big.NewInt(r.EffectiveAcceptedMilli))
	g.remaining.Add(g.remaining, big.NewInt(r.RemainingMilli))
	if r.CommercialStatus == "cancelled" {
		g.row.CancelledOrders++
	} else if r.RemainingMilli == 0 {
		g.row.ReceivedOrders++
	} else {
		g.row.PendingOrders++
		g.pending.Add(g.pending, big.NewInt(r.RemainingMilli))
	}
}
func Board(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in BoardInput) (ReceivingBoard, error) {
	var out ReceivingBoard
	from, to, e := boardRange(in)
	if e != nil {
		return out, e
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); e != nil {
		return out, e
	}
	// The persisted order creation timestamp is canonical UTC RFC3339Nano.
	// Prefix bounds include ALL fractions in the starting second and exclude
	// ALL fractions in the ending second; using a trailing Z here would not.
	where := `tenant_id=? AND store_id=? AND created_at>=? AND created_at<?`
	args := []any{a.TenantID, a.StoreID, from.Format("2006-01-02T15:04:05"), to.Format("2006-01-02T15:04:05")}
	if in.SupplierID != "" {
		where += ` AND supplier_id=?`
		args = append(args, in.SupplierID)
	}
	if in.ProductID != "" {
		where += ` AND EXISTS(SELECT 1 FROM purchase_order_items i WHERE i.tenant_id=purchase_orders.tenant_id AND i.store_id=purchase_orders.store_id AND i.order_id=purchase_orders.id AND i.product_id=?)`
		args = append(args, in.ProductID)
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,created_at FROM purchase_orders WHERE `+where+` ORDER BY created_at,id LIMIT 201`, args...)
	if e != nil {
		return out, e
	}
	type candidate struct {
		id, created string
		at          time.Time
	}
	candidates := []candidate{}
	for rows.Next() {
		if len(candidates) >= MaxBoardOrders {
			rows.Close()
			return out, ErrBoardLimit
		}
		var r candidate
		if e = rows.Scan(&r.id, &r.created); e != nil {
			rows.Close()
			return out, e
		}
		stamp, e := time.Parse(time.RFC3339Nano, r.created)
		if e != nil || !validSearchID(r.id) || stamp.UTC().Format(time.RFC3339Nano) != r.created || stamp.Before(from) || !stamp.Before(to) {
			rows.Close()
			return out, ErrConflict
		}
		r.at = stamp
		candidates = append(candidates, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].at.Equal(candidates[j].at) {
			return candidates[i].at.Before(candidates[j].at)
		}
		return candidates[i].id < candidates[j].id
	})
	out = ReceivingBoard{From: from.Format(time.RFC3339), To: to.Format(time.RFC3339), State: in.State, SupplierID: in.SupplierID, ProductID: in.ProductID, Basis: "order_creation_cohort_local_evidence_not_supplier_confirmation_or_payment", CohortOrders: len(candidates), Limit: in.Limit, Offset: in.Offset, Groups: []BoardGroup{}, Items: []BoardOrder{}}
	for _, r := range candidates {
		var n int64
		e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND order_id=?)+(SELECT count(*) FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND order_id=?)+(SELECT count(*) FROM purchase_receipt_voids WHERE tenant_id=? AND store_id=? AND order_id=?)+(SELECT count(*) FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND order_id=?)`, a.TenantID, a.StoreID, r.id, a.TenantID, a.StoreID, r.id, a.TenantID, a.StoreID, r.id, a.TenantID, a.StoreID, r.id).Scan(&n)
		if e != nil {
			return out, e
		}
		if n > MaxOverviewRows || n > MaxBoardEvidence-out.SourceEvidence {
			return out, ErrBoardLimit
		}
		out.SourceEvidence += n
	}
	groups := map[[3]string]*boardAccumulator{}
	items := []BoardOrder{}
	for _, r := range candidates {
		v, e := overviewTx(ctx, tx, a, d, OverviewInput{OrderID: r.id, Limit: 1})
		if e != nil {
			return out, e
		}
		if !boardMatches(v, in.State) {
			continue
		}
		boardGroupAdd(groups, v)
		items = append(items, BoardOrder{v.OrderID, v.SupplierID, v.ProductID, v.Unit, r.created, v.CommercialStatus, v.ReceivingStatus, v.PlannedMilli, v.EffectiveAcceptedMilli, v.RemainingMilli, v.ReviewFlags})
	}
	for _, g := range groups {
		g.row.PlannedMilliExact = g.planned.String()
		g.row.AcceptedMilliExact = g.accepted.String()
		g.row.RemainingMilliExact = g.remaining.String()
		g.row.PendingMilliExact = g.pending.String()
		out.Groups = append(out.Groups, g.row)
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		a, b := out.Groups[i], out.Groups[j]
		if a.SupplierID != b.SupplierID {
			return a.SupplierID < b.SupplierID
		}
		if a.ProductID != b.ProductID {
			return a.ProductID < b.ProductID
		}
		return a.Unit < b.Unit
	})
	out.MatchedOrders = len(items)
	end := in.Offset + in.Limit
	if end > len(items) {
		end = len(items)
	}
	if in.Offset < len(items) {
		out.Items = items[in.Offset:end]
	}
	out.MeasuredAt = time.Now().UTC().Format(time.RFC3339Nano)
	return out, tx.Commit()
}
