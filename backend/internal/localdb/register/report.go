package register

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"strings"
	"unicode/utf8"

	"titansystem-backend/internal/localdb/identity"
)

var (
	ErrReportNotFound     = errors.New("fechamento nao encontrado neste escopo")
	ErrReportOpen         = errors.New("relatorio oficial disponivel somente apos encerramento")
	ErrReportInconsistent = errors.New("historico do caixa inconsistente")
	ErrReportRange        = errors.New("relatorio excede inteiros exatos do cliente")
)

const reportMaxSafe int64 = 9007199254740991

// PaymentSummary describes persisted payment statuses, not acquiring-bank
// confirmations. The legacy schema stores card, without debit/credit subtypes.
type PaymentSummary struct {
	Method         string `json:"method"`
	Parts          int64  `json:"parts"`
	ConfirmedCents int64  `json:"recorded_confirmed_cents"`
	PendingCents   int64  `json:"recorded_pending_cents"`
	ReversedCents  int64  `json:"recorded_reversed_cents"`
}

type CashSummary struct {
	OpeningCents        int64 `json:"opening_cents"`
	MovementNetCents    int64 `json:"movement_net_cents"`
	ExpectedCents       int64 `json:"expected_cents"`
	DeclaredCents       int64 `json:"declared_cents"`
	DifferenceCents     int64 `json:"difference_cents"`
	SupplyCents         int64 `json:"supply_cents"`
	WithdrawalCents     int64 `json:"withdrawal_cents"`
	CustomerRefundCents int64 `json:"customer_refund_cents"`
}

type ClosedReport struct {
	SessionID                string           `json:"session_id"`
	OperationID              string           `json:"operation_id"`
	OpenedAt                 string           `json:"opened_at"`
	ClosedAt                 string           `json:"closed_at"`
	Complete                 bool             `json:"complete"`
	Cash                     CashSummary      `json:"cash"`
	SaleCount                int64            `json:"sale_count"`
	GrossSalesCents          int64            `json:"gross_sales_cents"`
	CancelledSalesCents      int64            `json:"cancelled_sales_cents"`
	ReturnedSalesCents       int64            `json:"returned_sales_cents"`
	NetSalesCents            int64            `json:"net_sales_cents"`
	Payments                 []PaymentSummary `json:"payments"`
	BankSettlementAvailable  bool             `json:"bank_settlement_available"`
	FinancialLedgerAvailable bool             `json:"financial_ledger_available"`
}

func reportAdd(total *int64, amount int64) error {
	if amount < 0 || amount > reportMaxSafe-*total {
		return ErrReportRange
	}
	*total += amount
	return nil
}

// ReadClosedReport reads one consistent snapshot of the original operator's
// closed session. It never borrows the current session or reveals an open
// drawer's expected balance. Historical access requires no current POS license.
// A successful report covers all LOCAL persisted rows, not other devices or
// bank settlement. Checkout plans and local browser evidence are not sales.
func ReadClosedReport(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, id string) (ClosedReport, error) {
	if db == nil || id == "" || id != strings.TrimSpace(id) || len(id) > 128 || strings.ContainsRune(id, 0) || !utf8.ValidString(id) {
		return ClosedReport{}, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return ClosedReport{}, err
	}
	defer tx.Rollback()
	if err = authorizeLegacy(ctx, tx, actor, device); err != nil {
		return ClosedReport{}, err
	}
	v, err := readClosedReportTx(ctx, tx, actor, device, id)
	if err != nil {
		return ClosedReport{}, err
	}
	if err = tx.Commit(); err != nil {
		return ClosedReport{}, err
	}
	return v, nil
}

// ReadClosedReportForAccountingTx permits a store-authorized accounting reader
// to inspect a closed turn. The original operator is resolved by the database,
// never supplied by the HTTP client. The caller owns the read transaction.
func ReadClosedReportForAccountingTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, id string) (ClosedReport, error) {
	if tx == nil || id == "" || id != strings.TrimSpace(id) || len(id) > 128 || strings.ContainsRune(id, 0) || !utf8.ValidString(id) {
		return ClosedReport{}, ErrInvalid
	}
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ViewAccounting); err != nil {
		return ClosedReport{}, err
	}
	var operator string
	err := tx.QueryRowContext(ctx, `SELECT operator_id FROM cash_sessions WHERE tenant_id=? AND store_id=? AND device_id=? AND id=?`, actor.TenantID, actor.StoreID, device.DeviceID, id).Scan(&operator)
	if errors.Is(err, sql.ErrNoRows) {
		return ClosedReport{}, ErrReportNotFound
	}
	if err != nil {
		return ClosedReport{}, err
	}
	original := actor
	original.IdentityID = operator
	return readClosedReportTx(ctx, tx, original, device, id)
}

func readClosedReportTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, id string) (ClosedReport, error) {
	var err error
	v := ClosedReport{SessionID: id, Payments: []PaymentSummary{{Method: "cash"}, {Method: "card"}, {Method: "pix"}, {Method: "other"}}}
	var closed sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT c.opened_at,c.closed_at,c.opening_cents FROM cash_sessions c
		JOIN cash_session_operators o ON o.tenant_id=c.tenant_id AND o.store_id=c.store_id AND o.session_id=c.id
		WHERE c.tenant_id=? AND c.store_id=? AND c.device_id=? AND c.id=? AND c.operator_id=? AND o.identity_id=?`, actor.TenantID, actor.StoreID, device.DeviceID, id, actor.IdentityID, actor.IdentityID).Scan(&v.OpenedAt, &closed, &v.Cash.OpeningCents)
	if errors.Is(err, sql.ErrNoRows) {
		return ClosedReport{}, ErrReportNotFound
	}
	if err != nil {
		return ClosedReport{}, err
	}
	if !closed.Valid {
		return ClosedReport{}, ErrReportOpen
	}
	v.ClosedAt = closed.String
	var closureTime string
	err = tx.QueryRowContext(ctx, `SELECT operation_id,declared_cents,expected_cents,closed_at FROM cash_closures WHERE tenant_id=? AND store_id=? AND session_id=? AND identity_id=?`, actor.TenantID, actor.StoreID, id, actor.IdentityID).Scan(&v.OperationID, &v.Cash.DeclaredCents, &v.Cash.ExpectedCents, &closureTime)
	if errors.Is(err, sql.ErrNoRows) {
		return ClosedReport{}, ErrReportInconsistent
	}
	if err != nil {
		return ClosedReport{}, err
	}
	if closureTime != v.ClosedAt {
		return ClosedReport{}, ErrReportInconsistent
	}
	for _, n := range []int64{v.Cash.OpeningCents, v.Cash.ExpectedCents, v.Cash.DeclaredCents} {
		if n < 0 || n > reportMaxSafe {
			return ClosedReport{}, ErrReportRange
		}
	}
	v.Cash.DifferenceCents = v.Cash.DeclaredCents - v.Cash.ExpectedCents
	if err = reportSales(ctx, tx, actor, device, &v); err != nil {
		return ClosedReport{}, err
	}
	// Big integers prevent SQL SUM overflow and order-dependent overflow when
	// signed movements cancel out. The final response must still be JS-exact.
	rows, err := tx.QueryContext(ctx, `SELECT m.amount_cents,a.kind,a.amount_cents FROM cash_movements m
		LEFT JOIN cash_adjustments a ON a.tenant_id=m.tenant_id AND a.store_id=m.store_id AND a.session_id=m.cash_session_id AND a.movement_id=m.id
		WHERE m.tenant_id=? AND m.store_id=? AND m.cash_session_id=? ORDER BY m.id`, actor.TenantID, actor.StoreID, id)
	if err != nil {
		return ClosedReport{}, err
	}
	net := new(big.Int)
	for rows.Next() {
		var amount int64
		var kind sql.NullString
		var adjustment sql.NullInt64
		if err = rows.Scan(&amount, &kind, &adjustment); err != nil {
			rows.Close()
			return ClosedReport{}, err
		}
		net.Add(net, big.NewInt(amount))
		if !kind.Valid {
			continue
		}
		if !adjustment.Valid || adjustment.Int64 <= 0 {
			rows.Close()
			return ClosedReport{}, ErrReportInconsistent
		}
		switch kind.String {
		case "supply":
			if amount != adjustment.Int64 {
				rows.Close()
				return ClosedReport{}, ErrReportInconsistent
			}
			err = reportAdd(&v.Cash.SupplyCents, adjustment.Int64)
		case "withdrawal":
			if amount != -adjustment.Int64 {
				rows.Close()
				return ClosedReport{}, ErrReportInconsistent
			}
			err = reportAdd(&v.Cash.WithdrawalCents, adjustment.Int64)
		default:
			err = ErrReportInconsistent
		}
		if err != nil {
			rows.Close()
			return ClosedReport{}, err
		}
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return ClosedReport{}, err
	}
	if closeErr != nil {
		return ClosedReport{}, closeErr
	}
	if !net.IsInt64() || net.Cmp(big.NewInt(reportMaxSafe)) > 0 || net.Cmp(big.NewInt(-reportMaxSafe)) < 0 {
		return ClosedReport{}, ErrReportRange
	}
	v.Cash.MovementNetCents = net.Int64()
	computed := new(big.Int).Add(net, big.NewInt(v.Cash.OpeningCents))
	if computed.Cmp(big.NewInt(v.Cash.ExpectedCents)) != 0 {
		return ClosedReport{}, ErrReportInconsistent
	}
	saleRows, e := tx.QueryContext(ctx, `SELECT id FROM sales WHERE tenant_id=? AND store_id=? AND device_id=? AND cash_session_id=? AND EXISTS(SELECT 1 FROM customer_returns r WHERE r.tenant_id=sales.tenant_id AND r.sale_id=sales.id)`, actor.TenantID, actor.StoreID, device.DeviceID, id)
	if e != nil {
		return ClosedReport{}, e
	}
	var returnedIDs []string
	for saleRows.Next() {
		var sid string
		if e = saleRows.Scan(&sid); e != nil {
			saleRows.Close()
			return ClosedReport{}, e
		}
		returnedIDs = append(returnedIDs, sid)
	}
	e = saleRows.Err()
	ce := saleRows.Close()
	if e != nil {
		return ClosedReport{}, e
	}
	if ce != nil {
		return ClosedReport{}, ce
	}
	for _, sid := range returnedIDs {
		refunded, e := customerRefundTotalTx(ctx, tx, actor, device, sid)
		if e != nil {
			return ClosedReport{}, ErrReportInconsistent
		}
		if e = reportAdd(&v.ReturnedSalesCents, refunded); e != nil {
			return ClosedReport{}, e
		}
	}
	if v.ReturnedSalesCents > v.NetSalesCents {
		return ClosedReport{}, ErrReportInconsistent
	}
	v.NetSalesCents -= v.ReturnedSalesCents
	v.Cash.CustomerRefundCents = v.ReturnedSalesCents
	v.Complete = true
	return v, nil
}

func reportSales(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, v *ClosedReport) error {
	rows, err := tx.QueryContext(ctx, `SELECT s.id,s.status,s.total_cents,p.id,p.method,p.status,p.amount_cents
		FROM sales s LEFT JOIN sale_payments p ON p.tenant_id=s.tenant_id AND p.sale_id=s.id
		WHERE s.tenant_id=? AND s.store_id=? AND s.device_id=? AND s.cash_session_id=? ORDER BY s.id,p.id`, actor.TenantID, actor.StoreID, device.DeviceID, v.SessionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	indices := map[string]int{"cash": 0, "card": 1, "pix": 2, "other": 3}
	var current, status string
	var total, paid int64
	finish := func() error {
		if current == "" {
			return nil
		}
		if paid != total || total <= 0 {
			return ErrReportInconsistent
		}
		if err := reportAdd(&v.SaleCount, 1); err != nil {
			return err
		}
		if err := reportAdd(&v.GrossSalesCents, total); err != nil {
			return err
		}
		switch status {
		case "committed":
			return reportAdd(&v.NetSalesCents, total)
		case "cancelled":
			return reportAdd(&v.CancelledSalesCents, total)
		default:
			return ErrReportInconsistent
		}
	}
	for rows.Next() {
		var saleID, saleStatus string
		var saleTotal int64
		var part, method, partStatus sql.NullString
		var amount sql.NullInt64
		if err = rows.Scan(&saleID, &saleStatus, &saleTotal, &part, &method, &partStatus, &amount); err != nil {
			return err
		}
		if saleID != current {
			if err = finish(); err != nil {
				return err
			}
			current, status, total, paid = saleID, saleStatus, saleTotal, 0
		}
		if !part.Valid || !method.Valid || !partStatus.Valid || !amount.Valid || amount.Int64 <= 0 {
			return ErrReportInconsistent
		}
		index, ok := indices[method.String]
		if !ok {
			return ErrReportInconsistent
		}
		if err = reportAdd(&paid, amount.Int64); err != nil {
			return err
		}
		p := &v.Payments[index]
		if err = reportAdd(&p.Parts, 1); err != nil {
			return err
		}
		switch partStatus.String {
		case "confirmed":
			if status != "committed" {
				return ErrReportInconsistent
			}
			err = reportAdd(&p.ConfirmedCents, amount.Int64)
		case "pending":
			if status != "committed" {
				return ErrReportInconsistent
			}
			err = reportAdd(&p.PendingCents, amount.Int64)
		case "reversed":
			if status != "cancelled" {
				return ErrReportInconsistent
			}
			err = reportAdd(&p.ReversedCents, amount.Int64)
		default:
			return ErrReportInconsistent
		}
		if err != nil {
			return err
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return finish()
}
