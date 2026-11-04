package retail

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"strings"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/sqlitewrite"
	"unicode/utf8"
)

type TransferInput struct {
	OperationID            string `json:"operation_id"`
	DestinationStoreID     string `json:"destination_store_id"`
	DestinationCustodianID string `json:"destination_custodian_id"`
	ProductID              string `json:"product_id"`
	FromLocationID         string `json:"from_location_id"`
	ToLocationID           string `json:"to_location_id"`
	QuantityMilli          int64  `json:"quantity_milli"`
	ExpectedFromMilli      int64  `json:"expected_from_milli"`
	ExpectedToMilli        int64  `json:"expected_to_milli"`
	Confirmed              bool   `json:"confirmed"`
}

func (s Service) storesAllowed(ctx context.Context, tx *sql.Tx) error {
	if e := s.Contracts.RequireTx(ctx, tx, s.Actor, s.Device, identity.ManageStock, modules.Inventory); e != nil {
		return e
	}
	cfg, e := ConfigTx(ctx, tx, s.Actor.TenantID)
	if e != nil {
		return e
	}
	if !cfg.MultistoreEnabled {
		return ErrDisabled
	}
	return nil
}
func (s Service) Transfer(ctx context.Context, in TransferInput) error {
	for _, id := range []string{in.DestinationStoreID, in.DestinationCustodianID, in.ProductID, in.FromLocationID, in.ToLocationID} {
		if !ID(id) {
			return ErrInvalid
		}
	}
	if !in.Confirmed || in.DestinationStoreID == s.Actor.StoreID || in.QuantityMilli < 1 || in.QuantityMilli > Max || in.ExpectedFromMilli < 0 || in.ExpectedToMilli < 0 || in.ExpectedFromMilli > Max || in.ExpectedToMilli > Max {
		return ErrInvalid
	}
	return s.Run(ctx, in.OperationID, "store_transfer", in.ProductID, in, func(tx *sql.Tx) error {
		if e := s.storesAllowed(ctx, tx); e != nil {
			return e
		}
		target := s.Actor
		target.StoreID = in.DestinationStoreID
		if e := identity.CanAccessStoreTx(ctx, tx, target, identity.ManageStock); e != nil {
			return e
		}
		var paired int
		if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, s.Actor.TenantID, target.StoreID, in.DestinationCustodianID).Scan(&paired); e != nil {
			return e
		}
		if paired != 1 {
			return identity.ErrDenied
		}
		source, e := stock.AvailableBalanceInTx(ctx, tx, s.Actor, in.ProductID, in.FromLocationID)
		if e != nil {
			return e
		}
		destination, e := stock.AvailableBalanceInTx(ctx, tx, target, in.ProductID, in.ToLocationID)
		if e != nil {
			return e
		}
		if source.Unit != destination.Unit || source.PhysicalMilli != in.ExpectedFromMilli || destination.PhysicalMilli != in.ExpectedToMilli || source.FreeMilli < in.QuantityMilli || destination.PhysicalMilli > Max-in.QuantityMilli {
			return ErrConflict
		}
		now := s.now().Format(time.RFC3339Nano)
		outID := "rt-" + Hash([]string{s.Actor.TenantID, in.OperationID, "out"})
		inID := "rt-" + Hash([]string{s.Actor.TenantID, in.OperationID, "in"})
		reason := "retail-transfer:" + in.OperationID
		if _, e = tx.ExecContext(ctx, `INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, outID, s.Actor.TenantID, s.Actor.StoreID, s.Device.DeviceID, in.ProductID, in.FromLocationID, -in.QuantityMilli, reason, now); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO stock_movements VALUES(?,?,?,?,?,?,?,?,?)`, inID, s.Actor.TenantID, target.StoreID, in.DestinationCustodianID, in.ProductID, in.ToLocationID, in.QuantityMilli, reason, now); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO retail_store_transfers VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.Actor.TenantID, in.OperationID, s.Actor.IdentityID, s.Actor.StoreID, s.Device.DeviceID, target.StoreID, in.DestinationCustodianID, in.ProductID, in.FromLocationID, in.ToLocationID, in.QuantityMilli, outID, inID, now)
		return e
	})
}

type StorePosition struct {
	StoreID string `json:"store_id"`
	stock.Availability
}
type StoreTotal struct {
	StoreID         string `json:"store_id"`
	GrossSalesCents int64  `json:"gross_sales_cents"`
}
type StoreReport struct {
	GrossSalesCents int64           `json:"gross_sales_cents"`
	StockAsOfUTC    string          `json:"stock_as_of_utc"`
	FromOn          string          `json:"from_on"`
	ToOn            string          `json:"to_on"`
	Totals          []StoreTotal    `json:"totals"`
	Positions       []StorePosition `json:"positions"`
}

func (s Service) StoreReport(ctx context.Context, store, from, to string, all bool) (StoreReport, error) {
	out := StoreReport{FromOn: from, ToOn: to, Totals: []StoreTotal{}, Positions: []StorePosition{}}
	if s.DB == nil || s.Contracts == nil {
		return out, ErrInvalid
	}
	start, e := time.Parse("2006-01-02", from)
	if e != nil {
		return out, ErrInvalid
	}
	end, e := time.Parse("2006-01-02", to)
	if e != nil || end.Before(start) || (!all && !ID(store)) || (all && store != "") {
		return out, ErrInvalid
	}
	e = sqlitewrite.Run(ctx, s.DB, ErrConflict, func(tx *sql.Tx) error {
		if e := s.AuthorizeInventory(ctx, tx); e != nil {
			return e
		}
		if e := s.storesAllowed(ctx, tx); e != nil {
			return e
		}
		stores := []string{store}
		if all {
			stores = []string{}
			rows, e := tx.QueryContext(ctx, `SELECT id FROM stores WHERE tenant_id=? ORDER BY id`, s.Actor.TenantID)
			if e != nil {
				return e
			}
			for rows.Next() {
				var id string
				if e = rows.Scan(&id); e != nil {
					rows.Close()
					return e
				}
				stores = append(stores, id)
				if len(stores) > 1000 {
					rows.Close()
					return ErrConflict
				}
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
		}
		out.GrossSalesCents = 0
		out.StockAsOfUTC = s.now().Format(time.RFC3339Nano)
		out.Totals = []StoreTotal{}
		out.Positions = []StorePosition{}
		for _, id := range stores {
			target := s.Actor
			target.StoreID = id
			if e := identity.CanAccessStoreTx(ctx, tx, target, identity.ManageStock); e != nil {
				if all && errors.Is(e, identity.ErrDenied) {
					continue
				}
				return e
			}
			if e := identity.CanAccessStoreTx(ctx, tx, target, identity.ViewAccounting); e != nil {
				if all && errors.Is(e, identity.ErrDenied) {
					continue
				}
				return e
			}
			rows, e := tx.QueryContext(ctx, `SELECT total_cents FROM sales WHERE tenant_id=? AND store_id=? AND status='committed' AND substr(committed_at,1,10)>=? AND substr(committed_at,1,10)<=?`, s.Actor.TenantID, id, from, to)
			if e != nil {
				return e
			}
			total := new(big.Int)
			for rows.Next() {
				var v int64
				if e = rows.Scan(&v); e != nil {
					rows.Close()
					return e
				}
				total.Add(total, big.NewInt(v))
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
			if !total.IsInt64() || total.Sign() < 0 || total.Int64() > Max {
				return ErrConflict
			}
			if out.GrossSalesCents > Max-total.Int64() {
				return ErrConflict
			}
			out.GrossSalesCents += total.Int64()
			out.Totals = append(out.Totals, StoreTotal{StoreID: id, GrossSalesCents: total.Int64()})
			positions := [][2]string{}
			rows, e = tx.QueryContext(ctx, `SELECT DISTINCT product_id,location_id FROM stock_movements WHERE tenant_id=? AND store_id=? ORDER BY product_id,location_id`, s.Actor.TenantID, id)
			if e != nil {
				return e
			}
			for rows.Next() {
				var pair [2]string
				if e = rows.Scan(&pair[0], &pair[1]); e != nil {
					rows.Close()
					return e
				}
				positions = append(positions, pair)
				if len(positions)+len(out.Positions) > 5000 {
					rows.Close()
					return ErrConflict
				}
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
			for _, pair := range positions {
				v, e := stock.AvailableBalanceInTx(ctx, tx, target, pair[0], pair[1])
				if e != nil {
					return e
				}
				out.Positions = append(out.Positions, StorePosition{StoreID: id, Availability: v})
			}
		}
		return nil
	})
	return out, e
}

func (s Service) CreateStore(ctx context.Context, op, id, name string) error {
	if !ID(id) || strings.TrimSpace(name) == "" || !utf8.ValidString(name) || strings.ContainsAny(name, "\x00\r\n") || len(name) > 200 {
		return ErrInvalid
	}
	return s.Run(ctx, op, "store_create", id, name, func(tx *sql.Tx) error {
		if e := s.storesAllowed(ctx, tx); e != nil {
			return e
		}
		if e := Owner(ctx, tx, s.Actor); e != nil {
			return e
		}
		var n int
		if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM stores WHERE tenant_id=? AND id=?`, s.Actor.TenantID, id).Scan(&n); e != nil {
			return e
		}
		if n != 0 {
			return ErrConflict
		}
		_, e := tx.ExecContext(ctx, `INSERT INTO stores VALUES(?,?,?)`, s.Actor.TenantID, id, name)
		return e
	})
}
