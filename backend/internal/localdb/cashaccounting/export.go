package cashaccounting

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"strconv"
	"titansystem-backend/internal/localdb/identity"
	"unicode"
	"unicode/utf8"
)

type csvBuffer struct{ bytes.Buffer }

func (b *csvBuffer) Write(p []byte) (int, error) {
	if len(p) > MaxCSVBytes-b.Len() {
		return 0, ErrRange
	}
	return b.Buffer.Write(p)
}
func exportText(s string) error {
	if s == "" {
		return nil
	}
	first, _ := utf8.DecodeRuneInString(s)
	if !utf8.ValidString(s) || len(s) > 4096 || (!unicode.IsLetter(first) && !unicode.IsDigit(first) && first != '_') {
		return ErrText
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ErrText
		}
	}
	return nil
}
func ExportCSV(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, w Window) ([]byte, error) {
	r, err := Read(ctx, db, a, d, w)
	if err != nil {
		return nil, err
	}
	return encodeCSV(ctx, r)
}
func encodeCSV(ctx context.Context, r Report) ([]byte, error) {
	var buf csvBuffer
	if _, err := buf.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		return nil, err
	}
	writer := csv.NewWriter(&buf)
	writer.Comma = ';'
	writer.UseCRLF = true
	header := []string{"format_version", "row_type", "scope", "from", "to", "generated_at", "session_id", "operator_id", "operation_id", "opened_at", "closed_at", "metric", "metric_value", "method", "parts", "recorded_confirmed_cents", "recorded_pending_cents", "recorded_reversed_cents", "entry_id", "source_kind", "source_id", "category", "actor_id", "occurred_at", "cash_delta_cents", "debit_account", "credit_account", "amount_cents"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	num := func(n int64) string { return strconv.FormatInt(n, 10) }
	write := func(kind string, values map[string]string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		values["format_version"] = "1"
		values["row_type"] = kind
		values["scope"] = r.Scope
		values["from"] = r.Window.From
		values["to"] = r.Window.To
		values["generated_at"] = r.GeneratedAt
		row := make([]string, len(header))
		for i, k := range header {
			row[i] = values[k]
		}
		return writer.Write(row)
	}
	for _, name := range metricNames {
		if err := write("period_metric", map[string]string{"metric": name, "metric_value": num(r.Summary[name])}); err != nil {
			return nil, err
		}
	}
	for _, p := range r.Payments {
		if err := write("period_payment", map[string]string{"method": p.Method, "parts": num(p.Parts), "recorded_confirmed_cents": num(p.ConfirmedCents), "recorded_pending_cents": num(p.PendingCents), "recorded_reversed_cents": num(p.ReversedCents)}); err != nil {
			return nil, err
		}
	}
	for _, s := range r.Sessions {
		c := s.Report
		for _, text := range []string{s.OperatorID, c.SessionID, c.OperationID, c.OpenedAt, c.ClosedAt} {
			if err := exportText(text); err != nil {
				return nil, err
			}
		}
		names := []string{"opening_cents", "expected_cents", "declared_cents", "difference_cents", "supply_cents", "withdrawal_cents", "sale_count", "gross_sales_cents", "cancelled_sales_cents", "returned_sales_cents", "net_sales_cents"}
		values := []int64{c.Cash.OpeningCents, c.Cash.ExpectedCents, c.Cash.DeclaredCents, c.Cash.DifferenceCents, c.Cash.SupplyCents, c.Cash.WithdrawalCents, c.SaleCount, c.GrossSalesCents, c.CancelledSalesCents, c.ReturnedSalesCents, c.NetSalesCents}
		for i, name := range names {
			if err := write("session_metric", map[string]string{"session_id": c.SessionID, "operator_id": s.OperatorID, "operation_id": c.OperationID, "opened_at": c.OpenedAt, "closed_at": c.ClosedAt, "metric": name, "metric_value": num(values[i])}); err != nil {
				return nil, err
			}
		}
		for _, p := range c.Payments {
			if err := write("session_payment", map[string]string{"session_id": c.SessionID, "operator_id": s.OperatorID, "method": p.Method, "parts": num(p.Parts), "recorded_confirmed_cents": num(p.ConfirmedCents), "recorded_pending_cents": num(p.PendingCents), "recorded_reversed_cents": num(p.ReversedCents)}); err != nil {
				return nil, err
			}
		}
	}
	for _, p := range r.Entries {
		e := p.Entry
		if len(e.Legs) != 2 || e.Legs[0].Side != "debit" || e.Legs[1].Side != "credit" || e.Legs[0].AmountCents != e.Legs[1].AmountCents {
			return nil, ErrInconsistent
		}
		values := map[string]string{"session_id": p.SessionID, "operator_id": p.OperatorID, "entry_id": e.ID, "source_kind": e.SourceKind, "source_id": e.SourceID, "category": e.Category, "occurred_at": e.OccurredAt, "debit_account": e.Legs[0].Account, "credit_account": e.Legs[1].Account, "cash_delta_cents": num(e.CashDeltaCents), "amount_cents": num(e.Legs[0].AmountCents)}
		if e.OperationID != nil {
			values["operation_id"] = *e.OperationID
		}
		if e.ActorID != nil {
			values["actor_id"] = *e.ActorID
		}
		for k, v := range values {
			if k != "cash_delta_cents" && k != "amount_cents" {
				if err := exportText(v); err != nil {
					return nil, err
				}
			}
		}
		if err := write("journal", values); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
