package payables

import (
	"context"
	"database/sql"
	"sort"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type Window struct {
	From string `json:"from"`
	To   string `json:"to"`
	AsOf string `json:"as_of"`
}
type DueItem struct {
	CashAppliedCents  int64       `json:"cash_applied_cents"`
	OutstandingCents  int64       `json:"outstanding_cents"`
	PayableID         string      `json:"payable_id"`
	SupplierID        string      `json:"supplier_id"`
	SupplierName      string      `json:"supplier_name"`
	DocumentReference string      `json:"document_reference"`
	State             string      `json:"state"`
	Installment       Installment `json:"installment"`
	Timing            string      `json:"timing"`
}
type Report struct {
	Scope                     string           `json:"scope"`
	Window                    Window           `json:"window"`
	Currency                  string           `json:"currency"`
	GeneratedAt               string           `json:"generated_at"`
	Complete                  bool             `json:"complete"`
	PaymentExecutionAvailable bool             `json:"payment_execution_available"`
	SettledCents              *int64           `json:"settled_cents"`
	Summary                   map[string]int64 `json:"summary"`
	Items                     []DueItem        `json:"items"`
}

func ReadReport(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, w Window) (Report, error) {
	if !date(w.From) || !date(w.To) || !date(w.AsOf) || w.From > w.To {
		return Report{}, ErrInvalid
	}
	start, _ := time.Parse(time.DateOnly, w.From)
	end, _ := time.Parse(time.DateOnly, w.To)
	if end.Sub(start) >= 366*24*time.Hour {
		return Report{}, ErrInvalid
	}
	tx, e := begin(ctx, db, a, d)
	if e != nil {
		return Report{}, e
	}
	defer tx.Rollback()
	items, e := all(ctx, tx, a, d)
	if e != nil {
		return Report{}, e
	}
	r := Report{Scope: "local_supplier_obligations", Window: w, Currency: "BRL", GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Complete: true, Summary: map[string]int64{}, Items: []DueItem{}}
	for _, k := range []string{"installment_count", "pending_cents", "approved_cents", "rejected_cents", "cancelled_cents", "approved_overdue_cents", "approved_today_cents", "approved_future_cents", "cash_applied_cents", "approved_outstanding_cents", "approved_outstanding_overdue_cents", "approved_outstanding_today_cents", "approved_outstanding_future_cents"} {
		r.Summary[k] = 0
	}
	add := func(k string, n int64) error {
		if n > MaxSafe-r.Summary[k] {
			return ErrRange
		}
		r.Summary[k] += n
		return nil
	}
	for _, p := range items {
		_, _, paid, e := cashForDocument(ctx, tx, a, d, p)
		if e != nil {
			return Report{}, e
		}
		for _, part := range p.Installments {
			if part.DueOn < w.From || part.DueOn > w.To {
				continue
			}
			if len(r.Items) >= 10000 {
				return Report{}, ErrRange
			}
			timing := "future"
			if part.DueOn < w.AsOf {
				timing = "overdue"
			} else if part.DueOn == w.AsOf {
				timing = "today"
			}
			if e = add(p.State+"_cents", part.AmountCents); e != nil {
				return Report{}, e
			}
			if p.State == "approved" {
				if e = add("approved_"+timing+"_cents", part.AmountCents); e != nil {
					return Report{}, e
				}
			}
			if e = add("cash_applied_cents", paid[part.ID]); e != nil {
				return Report{}, e
			}
			remaining := part.AmountCents - paid[part.ID]
			if p.State != "approved" {
				remaining = 0
			}
			if p.State == "approved" {
				if e = add("approved_outstanding_cents", remaining); e != nil {
					return Report{}, e
				}
				if e = add("approved_outstanding_"+timing+"_cents", remaining); e != nil {
					return Report{}, e
				}
			}
			r.Summary["installment_count"]++
			r.Items = append(r.Items, DueItem{paid[part.ID], remaining, p.ID, p.SupplierID, p.SupplierName, p.DocumentReference, p.State, part, timing})
		}
	}
	sort.Slice(r.Items, func(i, j int) bool {
		x, y := r.Items[i], r.Items[j]
		if x.Installment.DueOn != y.Installment.DueOn {
			return x.Installment.DueOn < y.Installment.DueOn
		}
		if x.PayableID != y.PayableID {
			return x.PayableID < y.PayableID
		}
		return x.Installment.ID < y.Installment.ID
	})
	return r, tx.Commit()
}
