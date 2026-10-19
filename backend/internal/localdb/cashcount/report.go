package cashcount

import (
	"context"
	"database/sql"
	"math/big"
	"time"
	"titansystem-backend/internal/localdb/cashvault"
	"titansystem-backend/internal/localdb/identity"
)

type ReportItem struct {
	Vault       cashvault.Vault `json:"vault"`
	LatestCount *Count          `json:"latest_count"`
}
type Report struct {
	Scope                string       `json:"scope"`
	GeneratedAt          string       `json:"generated_at"`
	TotalBalanceCents    int64        `json:"total_balance_cents"`
	VaultCount           int          `json:"vault_count"`
	ActiveCount          int          `json:"active_vault_count"`
	ArchivedCount        int          `json:"archived_vault_count"`
	CountingCount        int          `json:"counting_count"`
	AwaitingReviewCount  int          `json:"awaiting_review_count"`
	DivergentLatestCount int          `json:"divergent_latest_count"`
	Items                []ReportItem `json:"items"`
}

func ReportVaults(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (Report, error) {
	tx, err := readTx(ctx, db, a, d)
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback()
	r, err := reportTx(ctx, tx, a, d)
	if err != nil {
		return Report{}, err
	}
	if err = tx.Commit(); err != nil {
		return Report{}, err
	}
	return r, nil
}
func reportTx(ctx context.Context, tx *sql.Tx, a identity.Scope, d identity.DeviceContext) (Report, error) {
	r := Report{Scope: "local_physical_vaults", GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Items: []ReportItem{}}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM cash_vaults WHERE tenant_id=? AND store_id=? AND device_id=?`, a.TenantID, a.StoreID, d.DeviceID).Scan(&n); err != nil {
		return r, err
	}
	if n > MaxReportVaults {
		return r, ErrRange
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM cash_vaults WHERE tenant_id=? AND store_id=? AND device_id=? ORDER BY created_at,id`, a.TenantID, a.StoreID, d.DeviceID)
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
	total := new(big.Int)
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return Report{}, err
		}
		v, err := cashvault.InspectTx(ctx, tx, a, d, id)
		if err != nil {
			return Report{}, err
		}
		item := ReportItem{Vault: v}
		counts, err := tx.QueryContext(ctx, `SELECT id FROM cash_vault_counts WHERE tenant_id=? AND store_id=? AND device_id=? AND vault_id=? ORDER BY rowid`, a.TenantID, a.StoreID, d.DeviceID, id)
		if err != nil {
			return Report{}, err
		}
		countIDs := []string{}
		for counts.Next() {
			var countID string
			if err = counts.Scan(&countID); err != nil {
				counts.Close()
				return Report{}, err
			}
			countIDs = append(countIDs, countID)
		}
		err = counts.Err()
		counts.Close()
		if err != nil {
			return Report{}, err
		}
		// Validate even superseded counts; a later count must not conceal broken history.
		for _, countID := range countIDs {
			c, err := load(ctx, tx, a, d, countID)
			if err != nil {
				return Report{}, err
			}
			shown := visible(c)
			item.LatestCount = &shown
		}
		if item.LatestCount != nil {
			c := item.LatestCount
			if c.State == "counting" {
				r.CountingCount++
			}
			if c.State == "submitted" {
				r.AwaitingReviewCount++
			}
			if c.DifferenceCents != nil && *c.DifferenceCents != 0 {
				r.DivergentLatestCount++
			}
		}
		if v.State == "active" {
			r.ActiveCount++
		} else {
			r.ArchivedCount++
		}
		total.Add(total, big.NewInt(v.BalanceCents))
		r.Items = append(r.Items, item)
	}
	if !total.IsInt64() || total.Cmp(big.NewInt(maxSafe)) > 0 {
		return Report{}, ErrRange
	}
	r.VaultCount = len(r.Items)
	r.TotalBalanceCents = total.Int64()
	return r, nil
}
