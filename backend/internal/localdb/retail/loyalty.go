package retail

import (
	"context"
	"database/sql"
	"math/big"
	"time"
	"titansystem-backend/internal/sqlitewrite"
)

type PointsInput struct {
	OperationID     string `json:"operation_id"`
	SaleID          string `json:"sale_id,omitempty"`
	Points          int64  `json:"points,omitempty"`
	ExpectedBalance int64  `json:"expected_balance"`
	Confirmed       bool   `json:"confirmed"`
}
type PointsEntry struct {
	SaleID      string `json:"sale_id"`
	EntryID     string `json:"entry_id"`
	Kind        string `json:"kind"`
	Delta       int64  `json:"delta"`
	ExpiresUnix int64  `json:"expires_unix"`
}
type PointBenefit struct {
	OperationID  string `json:"operation_id"`
	Points       int64  `json:"points"`
	BenefitCents int64  `json:"benefit_cents"`
}
type PointsAccount struct {
	Benefits   []PointBenefit `json:"benefits"`
	CustomerID string         `json:"customer_id"`
	Balance    int64          `json:"balance"`
	Entries    []PointsEntry  `json:"entries"`
}

func ratio(amount, base, points int64) (int64, error) {
	if amount < 0 || base < 1 || points < 1 {
		return 0, ErrInvalid
	}
	v := new(big.Int).Div(big.NewInt(amount), big.NewInt(base))
	v.Mul(v, big.NewInt(points))
	if !v.IsInt64() || v.Int64() > Max {
		return 0, ErrInvalid
	}
	return v.Int64(), nil
}
func (s Service) loyalty(ctx context.Context, tx *sql.Tx, customer string) (Config, error) {
	c, e := ConfigTx(ctx, tx, s.Actor.TenantID)
	if e != nil {
		return c, e
	}
	if !c.LoyaltyEnabled {
		return c, ErrDisabled
	}
	if e = RoleTx(ctx, tx, s.Actor, c.PointsRoles); e != nil {
		return c, e
	}
	p, e := CustomerTx(ctx, tx, s.Actor.TenantID, customer)
	if e != nil {
		return c, e
	}
	if p.Anonymized || !p.LoyaltyConsent {
		return c, ErrDisabled
	}
	return c, nil
}
func saleRefunded(ctx context.Context, tx *sql.Tx, tenant, sale string) (int64, error) {
	var amount int64
	e := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT sum(amount_cents) FROM retail_sale_refunds WHERE tenant_id=? AND sale_id=?),0)+COALESCE((SELECT sum(refund_cents) FROM customer_returns WHERE tenant_id=? AND sale_id=?),0)`, tenant, sale, tenant, sale).Scan(&amount)
	return amount, e
}
func pointsEntry(ctx context.Context, tx *sql.Tx, tenant, customer, sale, entry, kind string, delta, expires int64) error {
	if kind != "reverse" {
		entry = "command-" + entry
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO retail_points VALUES(?,?,?,?,?,?,?,?)`, tenant, customer, sale, entry, kind, delta, expires, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}

// synchronizePoints never trusts the client: cancellations/refunds are read from
// their original immutable financial records, before any account result/redemption.
func (s Service) synchronizePoints(ctx context.Context, tx *sql.Tx, customer string) error {
	type award struct {
		sale                                  string
		points, total, base, quantum, expires int64
		status                                string
	}
	list := []award{}
	rows, e := tx.QueryContext(ctx, `SELECT a.sale_id,a.points,a.total_cents,a.earn_cents,a.earn_points,a.expires_unix,s.status FROM retail_point_awards a JOIN sales s ON s.tenant_id=a.tenant_id AND s.id=a.sale_id WHERE a.tenant_id=? AND a.customer_id=? AND a.store_id=?`, s.Actor.TenantID, customer, s.Actor.StoreID)
	if e != nil {
		return e
	}
	for rows.Next() {
		var a award
		if e = rows.Scan(&a.sale, &a.points, &a.total, &a.base, &a.quantum, &a.expires, &a.status); e != nil {
			rows.Close()
			return e
		}
		list = append(list, a)
		if len(list) > 1000 {
			rows.Close()
			return ErrConflict
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, a := range list {
		refund, e := saleRefunded(ctx, tx, s.Actor.TenantID, a.sale)
		if e != nil {
			return e
		}
		if a.status == "cancelled" {
			refund = a.total
		}
		if refund < 0 || refund > a.total {
			return ErrConflict
		}
		want, e := ratio(a.total-refund, a.base, a.quantum)
		if e != nil {
			return e
		}
		if want > a.points {
			want = a.points
		}
		var reversed int64
		if e = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(delta),0) FROM retail_points WHERE tenant_id=? AND customer_id=? AND sale_id=? AND kind='reverse'`, s.Actor.TenantID, customer, a.sale).Scan(&reversed); e != nil {
			return e
		}
		delta := want - (a.points + reversed)
		if delta > 0 {
			return ErrConflict
		}
		if delta < 0 {
			if e = pointsEntry(ctx, tx, s.Actor.TenantID, customer, a.sale, "sync-"+Hash([]any{a.sale, want}), "reverse", delta, a.expires); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s Service) pointsTx(ctx context.Context, tx *sql.Tx, customer string) (PointsAccount, error) {
	out := PointsAccount{CustomerID: customer, Entries: []PointsEntry{}}
	if e := s.synchronizePoints(ctx, tx, customer); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT p.sale_id,p.entry_id,p.kind,p.delta,p.expires_unix FROM retail_points p JOIN retail_point_awards a ON a.tenant_id=p.tenant_id AND a.sale_id=p.sale_id WHERE p.tenant_id=? AND p.customer_id=? AND a.store_id=? ORDER BY p.expires_unix,p.sale_id,p.entry_id`, s.Actor.TenantID, customer, s.Actor.StoreID)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	balance := new(big.Int)
	now := s.now().Unix()
	for rows.Next() {
		var v PointsEntry
		if e = rows.Scan(&v.SaleID, &v.EntryID, &v.Kind, &v.Delta, &v.ExpiresUnix); e != nil {
			return out, e
		}
		out.Entries = append(out.Entries, v)
		if len(out.Entries) > 10000 {
			return out, ErrConflict
		}
		if v.ExpiresUnix > now {
			balance.Add(balance, big.NewInt(v.Delta))
		}
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if !balance.IsInt64() || balance.Cmp(big.NewInt(Max)) > 0 || balance.Cmp(big.NewInt(-Max)) < 0 {
		return out, ErrConflict
	}
	out.Balance = balance.Int64()
	rows.Close()
	out.Benefits = []PointBenefit{}
	benefits, err := tx.QueryContext(ctx, `SELECT b.operation_id,b.points,b.benefit_cents FROM retail_point_redemptions b JOIN retail_operations o ON o.tenant_id=b.tenant_id AND o.operation_id=b.operation_id WHERE b.tenant_id=? AND b.customer_id=? AND o.store_id=? ORDER BY b.operation_id`, s.Actor.TenantID, customer, s.Actor.StoreID)
	if err != nil {
		return out, err
	}
	defer benefits.Close()
	for benefits.Next() {
		var v PointBenefit
		if err = benefits.Scan(&v.OperationID, &v.Points, &v.BenefitCents); err != nil {
			return out, err
		}
		out.Benefits = append(out.Benefits, v)
		if len(out.Benefits) > 10000 {
			return out, ErrConflict
		}
	}
	return out, benefits.Err()
}
func (s Service) Points(ctx context.Context, customer string) (PointsAccount, error) {
	var out PointsAccount
	if s.DB == nil || s.Contracts == nil {
		return out, ErrInvalid
	}
	if !ID(customer) {
		return out, ErrInvalid
	}
	e := sqlitewrite.Run(ctx, s.DB, ErrConflict, func(tx *sql.Tx) error {
		if e := s.Authorize(ctx, tx); e != nil {
			return e
		}
		if _, e := s.loyalty(ctx, tx, customer); e != nil {
			return e
		}
		var e error
		out, e = s.pointsTx(ctx, tx, customer)
		return e
	})
	return out, e
}
func (s Service) Award(ctx context.Context, customer string, in PointsInput) error {
	if !ID(in.SaleID) || !in.Confirmed {
		return ErrInvalid
	}
	return s.Run(ctx, in.OperationID, "points_earn", customer, in, func(tx *sql.Tx) error {
		c, e := s.loyalty(ctx, tx, customer)
		if e != nil {
			return e
		}
		var total int64
		var status string
		e = tx.QueryRowContext(ctx, `SELECT total_cents,status FROM sales WHERE tenant_id=? AND store_id=? AND id=?`, s.Actor.TenantID, s.Actor.StoreID, in.SaleID).Scan(&total, &status)
		if e != nil {
			return ErrInvalid
		}
		if status != "committed" {
			return ErrConflict
		}
		var exists int
		e = tx.QueryRowContext(ctx, `SELECT count(*) FROM retail_point_awards WHERE tenant_id=? AND sale_id=?`, s.Actor.TenantID, in.SaleID).Scan(&exists)
		if e != nil {
			return e
		}
		if exists > 0 {
			return ErrConflict
		}
		returned, e := saleRefunded(ctx, tx, s.Actor.TenantID, in.SaleID)
		if e != nil {
			return e
		}
		points, e := ratio(total-returned, c.EarnCents, c.EarnPoints)
		if e != nil {
			return e
		}
		if points < 1 {
			return ErrDisabled
		}
		account, e := s.pointsTx(ctx, tx, customer)
		if e != nil {
			return e
		}
		if account.Balance > Max-points {
			return ErrConflict
		}
		expires := s.now().Unix() + c.ValidDays*86400
		_, e = tx.ExecContext(ctx, `INSERT INTO retail_point_awards VALUES(?,?,?,?,?,?,?,?,?)`, s.Actor.TenantID, s.Actor.StoreID, in.SaleID, customer, points, total, c.EarnCents, c.EarnPoints, expires)
		if e != nil {
			return e
		}
		return pointsEntry(ctx, tx, s.Actor.TenantID, customer, in.SaleID, in.OperationID, "earn", points, expires)
	})
}

// Redeem records an explicit benefit entitlement, not a payment, cash movement,
// fiscal issuance or an automatic discount. Checkout application remains separate.
func (s Service) Redeem(ctx context.Context, customer string, in PointsInput) error {
	if in.Points < 1 || in.Points > Max || in.ExpectedBalance < 0 || !in.Confirmed {
		return ErrInvalid
	}
	return s.Run(ctx, in.OperationID, "points_redeem", customer, in, func(tx *sql.Tx) error {
		c, e := s.loyalty(ctx, tx, customer)
		if e != nil {
			return e
		}
		out, e := s.pointsTx(ctx, tx, customer)
		if e != nil {
			return e
		}
		if out.Balance != in.ExpectedBalance || in.Points > out.Balance || in.Points%c.RedeemPoints != 0 {
			return ErrConflict
		}
		benefit, e := ratio(in.Points, c.RedeemPoints, c.RedeemCents)
		if e != nil {
			return e
		}
		if benefit < 1 {
			return ErrInvalid
		}
		balances := map[string]int64{}
		expiry := map[string]int64{}
		order := []string{}
		now := s.now().Unix()
		for _, v := range out.Entries {
			if v.ExpiresUnix <= now {
				continue
			}
			if _, ok := balances[v.SaleID]; !ok {
				order = append(order, v.SaleID)
				expiry[v.SaleID] = v.ExpiresUnix
			}
			balances[v.SaleID] += v.Delta
		}
		remaining := in.Points
		for _, sale := range order {
			available := balances[sale]
			if available <= 0 {
				continue
			}
			take := available
			if take > remaining {
				take = remaining
			}
			if take > 0 {
				if e = pointsEntry(ctx, tx, s.Actor.TenantID, customer, sale, in.OperationID, "redeem", -take, expiry[sale]); e != nil {
					return e
				}
				remaining -= take
			}
			if remaining == 0 {
				break
			}
		}
		if remaining != 0 {
			return ErrConflict
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO retail_point_redemptions VALUES(?,?,?,?,?)`, s.Actor.TenantID, customer, in.OperationID, in.Points, benefit)
		return e
	})
}
