package sale

import (
	"context"
	"database/sql"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

const MaxProofReport = 1000

type ProofMethodSummary struct {
	Method                string `json:"method"`
	SubmittedCount        int    `json:"submitted_count"`
	AcceptedCount         int    `json:"accepted_count"`
	HistoricalCount       int    `json:"historical_count"`
	ReportedActiveCents   int64  `json:"reported_active_cents"`
	AcceptedEvidenceCents int64  `json:"accepted_evidence_cents"`
}
type ProofReport struct {
	Scope                 string               `json:"scope"`
	GeneratedAt           string               `json:"generated_at"`
	ProviderConfirmed     bool                 `json:"provider_confirmed"`
	SettledCents          *int64               `json:"settled_cents"`
	TotalCount            int                  `json:"total_count"`
	PlansWithEvidence     int                  `json:"plans_with_evidence"`
	ReportedActiveCents   int64                `json:"reported_active_cents"`
	AcceptedEvidenceCents int64                `json:"accepted_evidence_cents"`
	Methods               []ProofMethodSummary `json:"methods"`
	Items                 []PaymentProof       `json:"items"`
}

func proofAdd(dst *int64, n int64) error {
	if n < 0 || n > checkoutMaxSafe-*dst {
		return ErrProofRange
	}
	*dst += n
	return nil
}
func ReportPaymentProofs(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (ProofReport, error) {
	tx, _, err := proofReadTx(ctx, db, a, d, true)
	if err != nil {
		return ProofReport{}, err
	}
	defer tx.Rollback()
	r := ProofReport{Scope: "unverified_checkout_evidence", GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Methods: []ProofMethodSummary{{Method: "debit"}, {Method: "credit"}, {Method: "pix"}}, Items: []PaymentProof{}}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM checkout_payment_proofs WHERE tenant_id=? AND store_id=? AND device_id=?`, a.TenantID, a.StoreID, d.DeviceID).Scan(&r.TotalCount); err != nil {
		return r, err
	}
	if r.TotalCount > MaxProofReport {
		return ProofReport{}, ErrProofRange
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM checkout_payment_proofs WHERE tenant_id=? AND store_id=? AND device_id=? ORDER BY rowid`, a.TenantID, a.StoreID, d.DeviceID)
	if err != nil {
		return r, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return r, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return r, err
	}
	plans := map[string]bool{}
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return ProofReport{}, err
		}
		p, err := loadPaymentProof(ctx, tx, a, d, id)
		if err != nil {
			return ProofReport{}, err
		}
		r.Items = append(r.Items, p)
		plans[p.CheckoutID] = true
		for i := range r.Methods {
			m := &r.Methods[i]
			if m.Method != p.Method {
				continue
			}
			switch p.State {
			case "submitted", "accepted":
				if err = proofAdd(&m.ReportedActiveCents, p.AmountCents); err != nil {
					return ProofReport{}, err
				}
				if err = proofAdd(&r.ReportedActiveCents, p.AmountCents); err != nil {
					return ProofReport{}, err
				}
				if p.State == "submitted" {
					m.SubmittedCount++
				} else {
					m.AcceptedCount++
					if err = proofAdd(&m.AcceptedEvidenceCents, p.AmountCents); err != nil {
						return ProofReport{}, err
					}
					if err = proofAdd(&r.AcceptedEvidenceCents, p.AmountCents); err != nil {
						return ProofReport{}, err
					}
				}
			default:
				m.HistoricalCount++
			}
		}
	}
	r.PlansWithEvidence = len(plans)
	if err = tx.Commit(); err != nil {
		return ProofReport{}, err
	}
	return r, nil
}
