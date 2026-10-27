package cashjournal

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"strconv"
	"unicode"
	"unicode/utf8"

	"titansystem-backend/internal/localdb/identity"
)

const MaxExportEntries = 10000
const MaxExportBytes = 16 << 20

var (
	ErrExportLimit = errors.New("exportacao de caixa excede limite do arquivo")
	ErrExportText  = errors.New("texto do historico nao pode ser exportado com seguranca em CSV")
)

// ExportCSV returns one complete, bounded, reconciled snapshot. It never writes
// a partial file to an HTTP response, alters IDs or repairs historical records.
func ExportCSV(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, id string) ([]byte, error) {
	page, err := readSnapshot(ctx, db, actor, device, id, MaxExportEntries, 0, MaxExportEntries)
	if err != nil {
		return nil, err
	}
	if page.HasMore || page.TotalCount != int64(len(page.Items)) || !page.SourceComplete {
		return nil, ErrInconsistent
	}
	return encodeCSV(ctx, page)
}

// An export cell must round-trip without spreadsheet escaping changing the
// identifier. Unsupported text is rejected instead of prepending an apostrophe
// or silently dropping controls. The JSON history remains canonical/available.
func exportText(s string) error {
	if s == "" {
		return nil
	}
	if len(s) > 4096 || !utf8.ValidString(s) {
		return ErrExportText
	}
	first, _ := utf8.DecodeRuneInString(s)
	if !unicode.IsLetter(first) && !unicode.IsDigit(first) && first != '_' {
		return ErrExportText
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ErrExportText
		}
	}
	return nil
}

type boundedCSV struct{ bytes.Buffer }

func (b *boundedCSV) Write(p []byte) (int, error) {
	if len(p) > MaxExportBytes-b.Len() {
		return 0, ErrExportLimit
	}
	return b.Buffer.Write(p)
}

func encodeCSV(ctx context.Context, page Page) ([]byte, error) {
	if err := exportText(page.SessionID); err != nil {
		return nil, err
	}
	var buf boundedCSV
	if _, err := buf.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		return nil, err
	}
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	w.UseCRLF = true
	header := []string{"format_version", "row_type", "scope", "session_id", "entry_id", "source_kind", "source_id", "category", "operation_id", "actor_id", "occurred_at", "cash_delta_cents", "debit_account", "credit_account", "amount_cents", "total_entries", "expected_cents", "declared_cents", "difference_cents", "total_debit_cents", "total_credit_cents", "unclassified_count"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	number := func(n int64) string { return strconv.FormatInt(n, 10) }
	row := make([]string, len(header))
	row[0] = "1"
	row[1] = "summary"
	row[2] = page.Scope
	row[3] = page.SessionID
	row[15] = number(page.TotalCount)
	row[16] = number(page.Summary.ExpectedCents)
	row[17] = number(page.Summary.DeclaredCents)
	row[18] = number(page.Summary.DifferenceCents)
	row[19] = number(page.Summary.DebitCents)
	row[20] = number(page.Summary.CreditCents)
	row[21] = number(page.Summary.UnclassifiedCount)
	if err := w.Write(row); err != nil {
		return nil, err
	}
	for _, item := range page.Items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(item.Legs) != 2 || item.Legs[0].Side != "debit" || item.Legs[1].Side != "credit" || item.Legs[0].AmountCents != item.Legs[1].AmountCents {
			return nil, ErrInconsistent
		}
		row = make([]string, len(header))
		row[0] = "1"
		row[1] = "entry"
		row[2] = page.Scope
		row[3] = page.SessionID
		row[4] = item.ID
		row[5] = item.SourceKind
		row[6] = item.SourceID
		row[7] = item.Category
		if item.OperationID != nil {
			row[8] = *item.OperationID
		}
		if item.ActorID != nil {
			row[9] = *item.ActorID
		}
		row[10] = item.OccurredAt
		row[11] = number(item.CashDeltaCents)
		row[12] = item.Legs[0].Account
		row[13] = item.Legs[1].Account
		row[14] = number(item.Legs[0].AmountCents)
		for _, i := range []int{4, 5, 6, 7, 8, 9, 10, 12, 13} {
			if err := exportText(row[i]); err != nil {
				return nil, err
			}
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
