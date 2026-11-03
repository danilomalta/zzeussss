package production

import (
	"context"
	"database/sql"
	"errors"

	"titansystem-backend/internal/localdb/identity"
)

var ErrTrace = errors.New("rastreabilidade de producao inconsistente")

type TraceIngredient struct {
	ProductID     string `json:"product_id"`
	Unit          string `json:"unit"`
	PlannedMilli  int64  `json:"planned_milli"`
	ReservedMilli int64  `json:"reserved_milli"`
	ConsumedMilli int64  `json:"consumed_milli"`
}
type TraceMaterials struct {
	ActiveCount   int64                `json:"active_count"`
	ConsumedCount int64                `json:"consumed_count"`
	ReleasedCount int64                `json:"released_count"`
	Current       *MaterialReservation `json:"current"`
}
type TraceLosses struct {
	RecordedMilli     int64 `json:"recorded_milli"`
	UnclassifiedMilli int64 `json:"unclassified_milli"`
}
type TraceLot struct {
	Lot     ProductionLot `json:"lot"`
	Quality LotQuality    `json:"quality"`
}
type TraceLots struct {
	ProducedMilli   int64      `json:"produced_milli"`
	AssignedMilli   int64      `json:"assigned_milli"`
	UnassignedMilli int64      `json:"unassigned_milli"`
	TotalCount      int64      `json:"total_count"`
	Offset          int64      `json:"offset"`
	HasMore         bool       `json:"has_more"`
	Items           []TraceLot `json:"items"`
}
type OrderTrace struct {
	Order       Order             `json:"order"`
	Ingredients []TraceIngredient `json:"ingredients"`
	Materials   TraceMaterials    `json:"materials"`
	Stages      *StagePlan        `json:"stages"`
	Result      *ProductionResult `json:"result"`
	Losses      *TraceLosses      `json:"losses"`
	Lots        *TraceLots        `json:"lots"`
}

func plannedTraceIngredients(order Order) ([]TraceIngredient, error) {
	if _, _, err := normalize(order.Recipe.PublishInput); err != nil {
		return nil, ErrTrace
	}
	if order.VersionID != order.Recipe.VersionID || order.PlannedBatches < 1 || order.PlannedBatches > MaxQuantity/order.Recipe.YieldMilli || order.PlannedOutputMilli != order.PlannedBatches*order.Recipe.YieldMilli {
		return nil, ErrTrace
	}
	items := make([]TraceIngredient, 0, len(order.Recipe.Ingredients))
	for _, v := range order.Recipe.Ingredients {
		if order.PlannedBatches > MaxQuantity/v.QuantityMilli {
			return nil, ErrTrace
		}
		items = append(items, TraceIngredient{ProductID: v.ProductID, Unit: v.Unit, PlannedMilli: v.QuantityMilli * order.PlannedBatches})
	}
	return items, nil
}
func traceMaterialsTx(ctx context.Context, tx *sql.Tx, a identity.Scope, order Order, items []TraceIngredient) (TraceMaterials, error) {
	out := TraceMaterials{}
	rows, err := tx.QueryContext(ctx, `SELECT id,status FROM production_material_reservations WHERE tenant_id=? AND store_id=? AND order_id=?`, a.TenantID, a.StoreID, order.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var currentID string
	for rows.Next() {
		var id, status string
		if err = rows.Scan(&id, &status); err != nil {
			return out, err
		}
		switch status {
		case "active":
			out.ActiveCount++
			currentID = id
		case "consumed":
			out.ConsumedCount++
			currentID = id
		case "released":
			if out.ReleasedCount == MaxQuantity {
				return out, ErrTrace
			}
			out.ReleasedCount++
		default:
			return out, ErrTrace
		}
		if out.ActiveCount+out.ConsumedCount > 1 {
			return out, ErrTrace
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	if currentID == "" {
		return out, nil
	}
	current, err := materialGetTx(ctx, tx, a, currentID)
	if err != nil {
		return out, err
	}
	if current.LocationID != order.LocationID || len(current.Items) != len(items) {
		return out, ErrTrace
	}
	expected := map[string]int{}
	for i, v := range items {
		expected[v.ProductID] = i
	}
	seen := map[string]bool{}
	for _, v := range current.Items {
		i, ok := expected[v.ProductID]
		if !ok || seen[v.ProductID] || v.Unit != items[i].Unit || v.QuantityMilli != items[i].PlannedMilli {
			return out, ErrTrace
		}
		seen[v.ProductID] = true
		if current.Status == "active" {
			items[i].ReservedMilli = v.QuantityMilli
		} else if current.Status == "consumed" {
			items[i].ConsumedMilli = v.QuantityMilli
		} else {
			return out, ErrTrace
		}
	}
	if out.ConsumedCount == 1 {
		if _, err = consumedForOrder(ctx, tx, a, order); err != nil {
			return out, err
		}
	}
	out.Current = &current
	return out, nil
}
func traceResultTx(ctx context.Context, tx *sql.Tx, a identity.Scope, id string) (ProductionResult, error) {
	var out ProductionResult
	var movement sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT id,order_id,reservation_id,product_id,location_id,unit,planned_milli,produced_milli,shortfall_milli,order_revision,status,operation_id,device_id,actor_id,reason,movement_id,created_at FROM production_results WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id).Scan(&out.ResultID, &out.OrderID, &out.ReservationID, &out.ProductID, &out.LocationID, &out.Unit, &out.PlannedMilli, &out.ProducedMilli, &out.ShortfallMilli, &out.Revision, &out.Status, &out.OperationID, &out.DeviceID, &out.ActorID, &out.Reason, &movement, &out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrTrace
	}
	out.MovementID = movement.String
	return out, err
}
func traceQualityTx(ctx context.Context, tx *sql.Tx, a identity.Scope, lot ProductionLot) (LotQuality, error) {
	revision, err := qualityRevisionTx(ctx, tx, a, lot.ID)
	if err != nil {
		return LotQuality{}, err
	}
	out := LotQuality{LotID: lot.ID, LotStatus: lot.Status, Revision: revision, Status: "not_assessed"}
	if revision > 0 {
		v, e := scanQuality(tx.QueryRowContext(ctx, `SELECT `+qualityColumns+` FROM production_quality_reviews WHERE tenant_id=? AND store_id=? AND lot_id=? AND revision=?`, a.TenantID, a.StoreID, lot.ID, revision))
		if e != nil {
			return out, e
		}
		out.Latest = &v
		out.Status = v.Status
	}
	return out, nil
}
func traceLotsTx(ctx context.Context, tx *sql.Tx, a identity.Scope, resultID string, offset int) (TraceLots, error) {
	summary, err := lotSummaryTx(ctx, tx, a, resultID)
	if err != nil {
		return TraceLots{}, err
	}
	out := TraceLots{ProducedMilli: summary.ProducedMilli, AssignedMilli: summary.AssignedMilli, UnassignedMilli: summary.UnassignedMilli, Offset: int64(offset), Items: []TraceLot{}}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM production_lots WHERE tenant_id=? AND store_id=? AND result_id=?`, a.TenantID, a.StoreID, resultID).Scan(&out.TotalCount); err != nil {
		return out, err
	}
	if out.TotalCount < 0 || out.TotalCount > MaxQuantity {
		return out, ErrTrace
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+lotColumns+` FROM production_lots WHERE tenant_id=? AND store_id=? AND result_id=? ORDER BY created_at,id LIMIT 50 OFFSET ?`, a.TenantID, a.StoreID, resultID, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	lots := []ProductionLot{}
	for rows.Next() {
		v, e := scanLot(rows)
		if e != nil {
			return out, e
		}
		lots = append(lots, v)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if err = rows.Close(); err != nil {
		return out, err
	}
	for _, v := range lots {
		quality, e := traceQualityTx(ctx, tx, a, v)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, TraceLot{Lot: v, Quality: quality})
	}
	out.HasMore = out.Offset < out.TotalCount && int64(len(out.Items)) < out.TotalCount-out.Offset
	return out, nil
}

// One authorized read transaction preserves a consistent view across all module
// tables. Never call public DB-reading helpers from here: the local pool has one
// connection, and separate transactions would lose the common snapshot.
func GetOrderTrace(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id string, lotOffset int) (OrderTrace, error) {
	if !validID(id) || lotOffset < 0 || int64(lotOffset) > MaxQuantity {
		return OrderTrace{}, ErrInvalid
	}
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return OrderTrace{}, err
	}
	defer tx.Rollback()
	order, err := scanOrder(tx.QueryRowContext(ctx, `SELECT `+orderColumns+` FROM production_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, id))
	if err != nil {
		return OrderTrace{}, err
	}
	out := OrderTrace{Order: order}
	out.Ingredients, err = plannedTraceIngredients(order)
	if err != nil {
		return OrderTrace{}, err
	}
	out.Materials, err = traceMaterialsTx(ctx, tx, a, order, out.Ingredients)
	if err != nil {
		return OrderTrace{}, err
	}
	stages, err := stagePlanTx(ctx, tx, a, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return OrderTrace{}, err
	}
	if err == nil {
		out.Stages = &stages
	}
	if order.CompletionID != "" {
		result, e := traceResultTx(ctx, tx, a, order.CompletionID)
		if e != nil {
			return OrderTrace{}, e
		}
		if result.OrderID != order.ID || result.LocationID != order.LocationID || result.ProductID != order.Recipe.OutputProductID || result.Unit != order.Recipe.OutputUnit || result.PlannedMilli != order.PlannedOutputMilli || result.ProducedMilli < 0 || result.ProducedMilli > result.PlannedMilli || result.ShortfallMilli != result.PlannedMilli-result.ProducedMilli || result.Status != "completed" || result.Revision != order.Revision || out.Materials.ConsumedCount != 1 || out.Materials.Current.ID != result.ReservationID {
			return OrderTrace{}, ErrTrace
		}
		if err = guardStagesCompleted(ctx, tx, a, id); err != nil {
			return OrderTrace{}, err
		}
		out.Result = &result
		losses, e := lossSummaryTx(ctx, tx, a, result.ResultID)
		if e != nil {
			return OrderTrace{}, e
		}
		out.Losses = &TraceLosses{RecordedMilli: losses.RecordedLossMilli, UnclassifiedMilli: losses.UnclassifiedShortfallMilli}
		lots, e := traceLotsTx(ctx, tx, a, result.ResultID, lotOffset)
		if e != nil {
			return OrderTrace{}, e
		}
		out.Lots = &lots
	}
	return out, tx.Commit()
}
