package fleet

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/big"
	"time"
	"titansystem-backend/internal/localdb/identity"
)

type CostReport struct {
	AssetID                       string  `json:"asset_id"`
	From                          string  `json:"from"`
	ToExclusive                   string  `json:"to_exclusive"`
	MeasuredAt                    string  `json:"measured_at"`
	ExpenseCentsExact             string  `json:"expense_cents_exact"`
	AttributedIncomeCentsExact    string  `json:"attributed_income_cents_exact"`
	EstimatedImpactCentsExact     string  `json:"estimated_impact_cents_exact"`
	PeriodDepreciationCentsExact  string  `json:"period_depreciation_cents_exact"`
	BookValueCentsExact           *string `json:"book_value_cents_exact"`
	PartialContributionCentsExact string  `json:"partial_contribution_cents_exact"`
	DistanceMilliExact            string  `json:"distance_milli_exact"`
	HoursMilliExact               string  `json:"hours_milli_exact"`
	CostPerMeterNumerator         string  `json:"cost_per_meter_numerator_cents"`
	CostPerMeterDenominator       string  `json:"cost_per_meter_denominator_milli"`
	MeterBasis                    string  `json:"meter_basis"`
	Records                       int     `json:"source_records"`
	Voided                        int     `json:"voided_records"`
	Trips                         int     `json:"returned_trips"`
	CashConfirmed                 bool    `json:"cash_confirmed"`
	NetProfit                     bool    `json:"net_profit"`
	ProfilePresent                bool    `json:"profile_present"`
}

func monthsCompleted(start, end time.Time) int64 {
	if end.Before(start) {
		return 0
	}
	m := (end.Year()-start.Year())*12 + int(end.Month()-start.Month())
	target := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, m, 0)
	last := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	day := start.Day()
	if day > last {
		day = last
	}
	anniversary := time.Date(target.Year(), target.Month(), day, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), time.UTC)
	if anniversary.After(end) {
		m--
	}
	if m < 0 {
		return 0
	}
	return int64(m)
}
func depreciation(p Profile, at time.Time) *big.Int {
	start, e := time.Parse("2006-01-02", p.InServiceOn)
	if e != nil || p.UsefulMonths < 1 {
		return big.NewInt(0)
	}
	months := monthsCompleted(start, at.UTC())
	if months > p.UsefulMonths {
		months = p.UsefulMonths
	}
	v := big.NewInt(p.AcquisitionCents - p.ResidualCents)
	v.Mul(v, big.NewInt(months))
	return v.Quo(v, big.NewInt(p.UsefulMonths))
}

// Costs reports current managerial annotations, never actual bank settlement or net profit.
func Costs(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext, id, from, to string) (CostReport, error) {
	out := CostReport{AssetID: id, From: from, ToExclusive: to, MeasuredAt: time.Now().UTC().Format(time.RFC3339Nano)}
	f, e := stamp(from, false)
	if e != nil {
		return out, e
	}
	t, e := stamp(to, false)
	if e != nil {
		return out, e
	}
	start, _ := time.Parse(time.RFC3339, f)
	end, _ := time.Parse(time.RFC3339, t)
	if db == nil || !ID(id) || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return out, ErrInvalid
	}
	out.From = f
	out.ToExclusive = t
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if _, e = authorize(ctx, tx, a, d, true); e != nil {
		return out, e
	}
	_, asset, e := assetTx(ctx, tx, a, id)
	if e != nil {
		return out, e
	}
	out.MeterBasis = asset.MeterBasis
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM fleet_records WHERE tenant_id=? AND store_id=? AND asset_id=? AND kind IN('expense','income','impact','profile','trip')`, a.TenantID, a.StoreID, id).Scan(&count); e != nil {
		return out, e
	}
	if count > MaxSource {
		return out, ErrLimit
	}
	out.Records = count
	rows, e := tx.QueryContext(ctx, `SELECT `+columns+` FROM fleet_records WHERE tenant_id=? AND store_id=? AND asset_id=? AND kind IN('expense','income','impact','profile','trip') ORDER BY kind,id`, a.TenantID, a.StoreID, id)
	if e != nil {
		return out, e
	}
	all := []Record{}
	for rows.Next() {
		r, e := scan(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		all = append(all, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	expense, income, impact, dep, distance, hours := big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0)
	for _, r := range all {
		if e = proof(ctx, tx, a, r); e != nil {
			return out, e
		}
		switch r.Kind {
		case "expense", "income", "impact":
			var v Amount
			if json.Unmarshal(r.Data, &v) != nil || !number(v.AmountCents) {
				return out, ErrConflict
			}
			at, e := time.Parse(time.RFC3339, v.IncurredAt)
			if e != nil {
				return out, ErrConflict
			}
			if at.Before(start) || !at.Before(end) {
				continue
			}
			if r.State == "voided" {
				out.Voided++
				continue
			}
			if r.State != "recorded" {
				return out, ErrConflict
			}
			target := expense
			if r.Kind == "income" {
				target = income
			}
			if r.Kind == "impact" {
				target = impact
			}
			target.Add(target, big.NewInt(v.AmountCents))
		case "profile":
			var p Profile
			if json.Unmarshal(r.Data, &p) != nil {
				return out, ErrConflict
			}
			dep.Sub(depreciation(p, end), depreciation(p, start))
			book := new(big.Int).Sub(big.NewInt(p.AcquisitionCents), depreciation(p, end)).String()
			out.BookValueCentsExact = &book
			out.ProfilePresent = true
		case "trip":
			if r.State != "returned" {
				continue
			}
			var trip Trip
			if json.Unmarshal(r.Data, &trip) != nil || trip.EndMeterMilli == nil {
				return out, ErrConflict
			}
			at, e := time.Parse(time.RFC3339Nano, trip.ReturnedAt)
			if e != nil {
				return out, ErrConflict
			}
			if at.Before(start) || !at.Before(end) {
				continue
			}
			if *trip.EndMeterMilli < trip.StartMeterMilli {
				return out, ErrConflict
			}
			delta := big.NewInt(*trip.EndMeterMilli - trip.StartMeterMilli)
			if asset.MeterBasis == "km" {
				distance.Add(distance, delta)
			}
			if asset.MeterBasis == "hour" {
				hours.Add(hours, delta)
			}
			out.Trips++
		}
	}
	out.ExpenseCentsExact = expense.String()
	out.AttributedIncomeCentsExact = income.String()
	out.EstimatedImpactCentsExact = impact.String()
	out.PeriodDepreciationCentsExact = dep.String()
	out.PartialContributionCentsExact = new(big.Int).Sub(new(big.Int).Sub(income, expense), dep).String()
	out.DistanceMilliExact = distance.String()
	out.HoursMilliExact = hours.String()
	out.CostPerMeterNumerator = new(big.Int).Add(expense, dep).String()
	out.CostPerMeterDenominator = "0"
	if asset.MeterBasis == "km" {
		out.CostPerMeterDenominator = distance.String()
	}
	if asset.MeterBasis == "hour" {
		out.CostPerMeterDenominator = hours.String()
	}
	return out, tx.Commit()
}
