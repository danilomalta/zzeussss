package cashcount

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

const MaxCSVBytes = 2 << 20

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

type boundedCSV struct{ bytes.Buffer }

func (b *boundedCSV) Write(p []byte) (int, error) {
	if len(p) > MaxCSVBytes-b.Len() {
		return 0, ErrRange
	}
	return b.Buffer.Write(p)
}
func ExportCSV(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) ([]byte, error) {
	r, err := ReportVaults(ctx, db, a, d)
	if err != nil {
		return nil, err
	}
	return encodeCSV(ctx, r)
}
func encodeCSV(ctx context.Context, r Report) ([]byte, error) {
	var buf boundedCSV
	if _, err := buf.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		return nil, err
	}
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	w.UseCRLF = true
	if err := w.Write([]string{"format_version", "row_type", "scope", "generated_at", "vault_id", "vault_name", "vault_state", "balance_cents", "latest_count_id", "count_state", "expected_cents", "counted_cents", "difference_cents", "counter_id", "reviewer_id", "total_vaults", "active_vaults", "archived_vaults", "counting", "awaiting_review", "divergent_latest"}); err != nil {
		return nil, err
	}
	number := func(n int64) string { return strconv.FormatInt(n, 10) }
	row := make([]string, 21)
	row[0] = "1"
	row[1] = "summary"
	row[2] = r.Scope
	row[3] = r.GeneratedAt
	row[7] = number(r.TotalBalanceCents)
	for i, n := range []int{r.VaultCount, r.ActiveCount, r.ArchivedCount, r.CountingCount, r.AwaitingReviewCount, r.DivergentLatestCount} {
		row[15+i] = strconv.Itoa(n)
	}
	if err := w.Write(row); err != nil {
		return nil, err
	}
	for _, item := range r.Items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		v := item.Vault
		row = make([]string, 21)
		row[0] = "1"
		row[1] = "vault"
		row[2] = r.Scope
		row[3] = r.GeneratedAt
		row[4] = v.ID
		row[5] = v.Name
		row[6] = v.State
		row[7] = number(v.BalanceCents)
		if c := item.LatestCount; c != nil {
			row[8] = c.ID
			row[9] = c.State
			row[13] = c.CounterID
			if c.ReviewerID != nil {
				row[14] = *c.ReviewerID
			}
			for i, n := range []*int64{c.ExpectedCents, c.CountedCents, c.DifferenceCents} {
				if n != nil {
					row[10+i] = number(*n)
				}
			}
		}
		for _, i := range []int{4, 5, 6, 8, 9, 13, 14} {
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
