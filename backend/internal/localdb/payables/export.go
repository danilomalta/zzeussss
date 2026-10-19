package payables

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"sort"
	"strconv"
	"titansystem-backend/internal/localdb/identity"
	"unicode"
	"unicode/utf8"
)

const MaxCSVBytes = 4 << 20

type boundedCSV struct{ bytes.Buffer }

func (b *boundedCSV) Write(p []byte) (int, error) {
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
	r, e := ReadReport(ctx, db, a, d, w)
	if e != nil {
		return nil, e
	}
	return encodeCSV(ctx, r)
}
func encodeCSV(ctx context.Context, r Report) ([]byte, error) {
	var b boundedCSV
	if _, e := b.Write([]byte{0xef, 0xbb, 0xbf}); e != nil {
		return nil, e
	}
	w := csv.NewWriter(&b)
	w.Comma = ';'
	w.UseCRLF = true
	if e := w.Write([]string{"format_version", "row_type", "scope", "from", "to", "as_of", "currency", "metric", "metric_value", "payable_id", "supplier_id", "supplier_name", "document_reference", "state", "installment_id", "due_on", "amount_cents", "timing"}); e != nil {
		return nil, e
	}
	base := func(kind string) []string {
		x := make([]string, 18)
		copy(x, []string{"1", kind, r.Scope, r.Window.From, r.Window.To, r.Window.AsOf, r.Currency})
		return x
	}
	keys := []string{}
	for k := range r.Summary {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		x := base("summary")
		x[7] = k
		x[8] = strconv.FormatInt(r.Summary[k], 10)
		if e := w.Write(x); e != nil {
			return nil, e
		}
	}
	for _, v := range r.Items {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		x := base("installment")
		copy(x[9:], []string{v.PayableID, v.SupplierID, v.SupplierName, v.DocumentReference, v.State, v.Installment.ID, v.Installment.DueOn, strconv.FormatInt(v.Installment.AmountCents, 10), v.Timing})
		for _, i := range []int{9, 10, 11, 12, 13, 14, 15, 17} {
			if e := exportText(x[i]); e != nil {
				return nil, e
			}
		}
		if e := w.Write(x); e != nil {
			return nil, e
		}
	}
	w.Flush()
	if e := w.Error(); e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return b.Bytes(), nil
}
