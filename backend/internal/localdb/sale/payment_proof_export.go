package sale

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

const MaxProofCSVBytes = 2 << 20

type proofCSVBuffer struct{ bytes.Buffer }

func (b *proofCSVBuffer) Write(p []byte) (int, error) {
	if len(p) > MaxProofCSVBytes-b.Len() {
		return 0, ErrProofRange
	}
	return b.Buffer.Write(p)
}
func proofExportText(s string) error {
	if s == "" {
		return nil
	}
	first, _ := utf8.DecodeRuneInString(s)
	if len(s) > 4096 || !utf8.ValidString(s) || (!unicode.IsLetter(first) && !unicode.IsDigit(first) && first != '_') {
		return ErrProofText
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return ErrProofText
		}
	}
	return nil
}
func ExportPaymentProofsCSV(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) ([]byte, error) {
	r, err := ReportPaymentProofs(ctx, db, a, d)
	if err != nil {
		return nil, err
	}
	return encodeProofCSV(ctx, r)
}
func encodeProofCSV(ctx context.Context, r ProofReport) ([]byte, error) {
	var b proofCSVBuffer
	if _, err := b.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		return nil, err
	}
	w := csv.NewWriter(&b)
	w.Comma = ';'
	w.UseCRLF = true
	header := []string{"format_version", "row_type", "scope", "generated_at", "proof_id", "checkout_id", "part_id", "method", "state", "amount_cents", "provider", "terminal", "reference", "occurred_at", "submitter_id", "reviewer_id", "reported_active_cents", "accepted_evidence_cents", "submitted_count", "accepted_count", "historical_count", "settled_cents"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	num := func(n int64) string { return strconv.FormatInt(n, 10) }
	write := func(kind string, values map[string]string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		row := make([]string, len(header))
		values["format_version"] = "1"
		values["row_type"] = kind
		values["scope"] = r.Scope
		values["generated_at"] = r.GeneratedAt
		for i, k := range header {
			row[i] = values[k]
		}
		return w.Write(row)
	}
	summary := map[string]string{"reported_active_cents": num(r.ReportedActiveCents), "accepted_evidence_cents": num(r.AcceptedEvidenceCents)}
	if err := write("summary", summary); err != nil {
		return nil, err
	}
	for _, m := range r.Methods {
		if err := write("method", map[string]string{"method": m.Method, "reported_active_cents": num(m.ReportedActiveCents), "accepted_evidence_cents": num(m.AcceptedEvidenceCents), "submitted_count": strconv.Itoa(m.SubmittedCount), "accepted_count": strconv.Itoa(m.AcceptedCount), "historical_count": strconv.Itoa(m.HistoricalCount)}); err != nil {
			return nil, err
		}
	}
	for _, p := range r.Items {
		values := map[string]string{"proof_id": p.ID, "checkout_id": p.CheckoutID, "part_id": p.PartID, "method": p.Method, "state": p.State, "amount_cents": num(p.AmountCents), "provider": p.Provider, "terminal": p.Terminal, "reference": p.Reference, "occurred_at": p.OccurredAt, "submitter_id": p.SubmitterID}
		if p.ReviewerID != nil {
			values["reviewer_id"] = *p.ReviewerID
		}
		for k, v := range values {
			if k != "amount_cents" {
				if err := proofExportText(v); err != nil {
					return nil, err
				}
			}
		}
		if err := write("proof", values); err != nil {
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
	return b.Bytes(), nil
}
