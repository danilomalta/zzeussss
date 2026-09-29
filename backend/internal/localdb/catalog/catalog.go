package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

var ErrInvalidCatalog = errors.New("cadastro local inválido")

type ProductInput struct {
	SKU        string
	Barcode    string
	Name       string
	Unit       string
	PriceCents int64
	CostCents  int64
}

type Product struct {
	ID         string `json:"id"`
	SKU        string `json:"sku"`
	Barcode    string `json:"barcode,omitempty"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	PriceCents int64  `json:"price_cents"`
	CostCents  *int64 `json:"cost_cents,omitempty"`
}

type LocationInput struct {
	Kind string
	Name string
}

type Location struct {
	ID string
	LocationInput
}

// CreateProduct cadastra na empresa da sessão. Preço e custo são centavos.
func CreateProduct(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in ProductInput) (string, error) {
	if db == nil {
		return "", errors.New("banco local indisponível")
	}
	in.SKU, in.Barcode, in.Name = strings.TrimSpace(in.SKU), strings.TrimSpace(in.Barcode), strings.TrimSpace(in.Name)
	if in.Unit == "" {
		in.Unit = "unit"
	}
	if in.SKU == "" || in.Name == "" || in.PriceCents < 0 || in.CostCents < 0 ||
		len(in.SKU) > 100 || len(in.Name) > 255 || !validUnit(in.Unit) {
		return "", ErrInvalidCatalog
	}
	id, err := localdb.NewID()
	if err != nil {
		return "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStock); err != nil {
		return "", err
	}
	var barcode any
	if in.Barcode != "" {
		barcode = in.Barcode
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO products
		(tenant_id, id, sku, name, price_cents, cost_cents, unit, barcode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, actor.TenantID, id, in.SKU, in.Name,
		in.PriceCents, in.CostCents, in.Unit, barcode)
	if err != nil {
		return "", fmt.Errorf("cadastrar produto: %w", err)
	}
	return id, tx.Commit()
}

// CreateLocation separa gôndola, depósito, recebimento e produção por loja.
func CreateLocation(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, in LocationInput) (string, error) {
	if db == nil {
		return "", errors.New("banco local indisponível")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 255 || !validKind(in.Kind) {
		return "", ErrInvalidCatalog
	}
	id, err := localdb.NewID()
	if err != nil {
		return "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStock); err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO stock_locations (tenant_id, store_id, id, kind, name)
		VALUES (?, ?, ?, ?, ?)`, actor.TenantID, actor.StoreID, id, in.Kind, in.Name)
	if err != nil {
		return "", fmt.Errorf("cadastrar local: %w", err)
	}
	return id, tx.Commit()
}

// ListProducts devolve no máximo 100 itens da empresa do operador.
func ListProducts(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, limit, offset int) ([]Product, error) {
	if db == nil {
		return nil, errors.New("banco local indisponível")
	}
	if limit < 1 || limit > 100 || offset < 0 {
		return nil, ErrInvalidCatalog
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ViewCatalog); err != nil {
		return nil, err
	}
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id = ? AND identity_id = ?`,
		actor.TenantID, actor.IdentityID).Scan(&role); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, sku, barcode, name, unit, price_cents, cost_cents
		FROM products WHERE tenant_id = ? ORDER BY sku, id LIMIT ? OFFSET ?`, actor.TenantID, limit, offset)
	if err != nil {
		return nil, err
	}
	items := make([]Product, 0)
	for rows.Next() {
		var item Product
		var barcode sql.NullString
		var cost int64
		if err := rows.Scan(&item.ID, &item.SKU, &barcode, &item.Name, &item.Unit, &item.PriceCents, &cost); err != nil {
			rows.Close()
			return nil, err
		}
		item.Barcode = barcode.String
		if role == "owner" || role == "manager" || role == "stock" {
			item.CostCents = &cost
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func ListLocations(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext) ([]Location, error) {
	if db == nil {
		return nil, errors.New("banco local indisponível")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ViewCatalog); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, kind, name FROM stock_locations
		WHERE tenant_id = ? AND store_id = ? ORDER BY kind, id`, actor.TenantID, actor.StoreID)
	if err != nil {
		return nil, err
	}
	items := make([]Location, 0)
	for rows.Next() {
		var item Location
		if err := rows.Scan(&item.ID, &item.Kind, &item.Name); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func validUnit(unit string) bool {
	switch unit {
	case "unit", "kg", "g", "liter", "ml", "meter":
		return true
	default:
		return false
	}
}

func validKind(kind string) bool {
	switch kind {
	case "shelf", "backroom", "receiving", "production":
		return true
	default:
		return false
	}
}
