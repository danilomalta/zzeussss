package purchases

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"time"
	"titansystem-backend/internal/localdb/dockagenda"
	"titansystem-backend/internal/localdb/identity"
)

const MaxOverviewRows = 1000

var ErrOverviewLimit = errors.New("receiving overview exceeds 1000 source records for order")

type OverviewInput struct {
	OrderID       string
	Limit, Offset int
}
type AgendaCounts struct {
	Reserved  int `json:"reserved"`
	Arrived   int `json:"arrived"`
	Unloading int `json:"unloading"`
	Completed int `json:"completed"`
	Cancelled int `json:"cancelled"`
	NoShow    int `json:"no_show"`
}
type ReceivingOverview struct {
	OrderID                  string                   `json:"order_id"`
	SupplierID               string                   `json:"supplier_id"`
	MeasuredAt               string                   `json:"measured_at"`
	CommercialStatus         string                   `json:"commercial_status"`
	ReceivingStatus          string                   `json:"receiving_status"`
	Authorization            *ReceivingAuthorization  `json:"authorization"`
	ProductID                string                   `json:"product_id"`
	Unit                     string                   `json:"unit"`
	PlannedMilli             int64                    `json:"planned_milli"`
	EffectiveDeliveredMilli  int64                    `json:"effective_delivered_milli"`
	EffectiveAcceptedMilli   int64                    `json:"effective_accepted_milli"`
	PartialRejectedMilli     int64                    `json:"partial_rejected_milli"`
	RemainingMilli           int64                    `json:"remaining_milli"`
	FullyRejectedMilliExact  string                   `json:"fully_rejected_milli_exact"`
	VoidedAcceptedMilliExact string                   `json:"voided_accepted_milli_exact"`
	ReceiptCount             int64                    `json:"receipt_count"`
	VoidedReceiptCount       int64                    `json:"voided_receipt_count"`
	FullRejectionCount       int                      `json:"full_rejection_count"`
	Agenda                   AgendaCounts             `json:"agenda_counts"`
	TotalAppointments        int                      `json:"total_appointments"`
	SourceRecords            int64                    `json:"source_records"`
	Limit                    int                      `json:"limit"`
	Offset                   int                      `json:"offset"`
	Appointments             []dockagenda.Appointment `json:"appointments"`
	ReviewFlags              []string                 `json:"review_flags"`
	SupplierConfirmation     string                   `json:"supplier_confirmation"`
	PaymentEvaluation        string                   `json:"payment_evaluation"`
}

func overviewFlags(r ReceivingOverview) []string {
	flags := []string{}
	if r.CommercialStatus == "cancelled" {
		flags = append(flags, "order_cancelled_no_new_receiving")
	}
	if r.Authorization == nil && r.CommercialStatus != "cancelled" {
		flags = append(flags, "local_receiving_authorization_missing")
	}
	if r.RemainingMilli > 0 && r.CommercialStatus != "cancelled" {
		flags = append(flags, "accepted_quantity_pending")
	}
	if r.EffectiveAcceptedMilli > 0 && r.RemainingMilli > 0 {
		flags = append(flags, "partial_receiving")
	}
	if r.PartialRejectedMilli > 0 || r.FullRejectionCount > 0 {
		flags = append(flags, "rejections_recorded_review_agreement")
	}
	if r.VoidedReceiptCount > 0 {
		flags = append(flags, "receipt_void_recorded_not_refund")
	}
	if r.Agenda.Completed > 0 && r.RemainingMilli > 0 {
		flags = append(flags, "completed_dock_with_receiving_pending_review")
	}
	if r.CommercialStatus == "cancelled" && (r.Agenda.Reserved+r.Agenda.Arrived+r.Agenda.Unloading) > 0 {
		flags = append(flags, "cancelled_order_with_active_agenda_review")
	}
	if r.TotalAppointments == 0 {
		flags = append(flags, "no_local_dock_appointment")
	}
	return flags
}
func Overview(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, in OverviewInput) (ReceivingOverview, error) {
	var out ReceivingOverview
	if !validSearchID(in.OrderID) || in.Limit < 1 || in.Limit > 100 || in.Offset < 0 || in.Offset > 1000000 {
		return out, ErrInvalid
	}
	tx, e := ReadTx(ctx, db, a, d)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ManageStock); e != nil {
		return out, e
	}
	out, e = overviewTx(ctx, tx, a, d, in)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}

// Caller owns the already authorized transaction; no commit or writes here.
func overviewTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext, in OverviewInput) (ReceivingOverview, error) {
	var out ReceivingOverview
	var e error
	// Bound all source evidence before expensive receipt validation or pagination.
	var count int64
	e = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM purchase_receipts WHERE tenant_id=? AND store_id=? AND order_id=?)+(SELECT count(*) FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND order_id=?)+(SELECT count(*) FROM purchase_receipt_voids WHERE tenant_id=? AND store_id=? AND order_id=?)+(SELECT count(*) FROM dock_agenda_appointments WHERE tenant_id=? AND store_id=? AND order_id=?)`, a.TenantID, a.StoreID, in.OrderID, a.TenantID, a.StoreID, in.OrderID, a.TenantID, a.StoreID, in.OrderID, a.TenantID, a.StoreID, in.OrderID).Scan(&count)
	if e != nil {
		return out, e
	}
	if count > MaxOverviewRows {
		return out, ErrOverviewLimit
	}
	base, e := receivingTraceTx(ctx, tx, a, in.OrderID, 0)
	if e != nil {
		return out, e
	}
	out = ReceivingOverview{OrderID: in.OrderID, CommercialStatus: base.CommercialStatus, ReceivingStatus: base.ReceivingStatus, Authorization: base.Authorization, ProductID: base.ProductID, Unit: base.Unit, PlannedMilli: base.PlannedMilli, EffectiveDeliveredMilli: base.DeliveredMilli, EffectiveAcceptedMilli: base.AcceptedMilli, PartialRejectedMilli: base.RejectedMilli, RemainingMilli: base.RemainingMilli, VoidedAcceptedMilliExact: base.VoidedAcceptedMilliExact, ReceiptCount: base.TotalCount, VoidedReceiptCount: base.VoidedCount, SourceRecords: count, Limit: in.Limit, Offset: in.Offset, Appointments: []dockagenda.Appointment{}, SupplierConfirmation: "not_proven_by_local_records", PaymentEvaluation: "not_evaluated"}
	if e = tx.QueryRowContext(ctx, `SELECT supplier_id FROM purchase_orders WHERE tenant_id=? AND store_id=? AND id=?`, a.TenantID, a.StoreID, in.OrderID).Scan(&out.SupplierID); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM purchase_delivery_rejections WHERE tenant_id=? AND store_id=? AND order_id=? ORDER BY created_at,id`, a.TenantID, a.StoreID, in.OrderID)
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
	rejected := new(big.Int)
	for _, id := range ids {
		r, e := rejectionTx(ctx, tx, a, id)
		if e != nil {
			return out, e
		}
		if r.OrderID != in.OrderID || r.ProductID != base.ProductID || r.Unit != base.Unit {
			return out, ErrConflict
		}
		rejected.Add(rejected, big.NewInt(r.DeliveredMilli))
	}
	out.FullRejectionCount = len(ids)
	out.FullyRejectedMilliExact = rejected.String()
	appointments, e := dockagenda.OrderSnapshotTx(ctx, tx, a, d, in.OrderID, MaxOverviewRows)
	if e != nil {
		return out, e
	}
	for _, r := range appointments {
		if r.SupplierID != out.SupplierID {
			return out, ErrConflict
		}
		switch r.Status {
		case "reserved":
			out.Agenda.Reserved++
		case "arrived":
			out.Agenda.Arrived++
		case "unloading":
			out.Agenda.Unloading++
		case "completed":
			out.Agenda.Completed++
		case "cancelled":
			out.Agenda.Cancelled++
		case "no_show":
			out.Agenda.NoShow++
		}
	}
	out.TotalAppointments = len(appointments)
	out.ReviewFlags = overviewFlags(out)
	end := in.Offset + in.Limit
	if end > len(appointments) {
		end = len(appointments)
	}
	if in.Offset < len(appointments) {
		out.Appointments = appointments[in.Offset:end]
	}
	out.MeasuredAt = time.Now().UTC().Format(time.RFC3339Nano)
	return out, nil
}
