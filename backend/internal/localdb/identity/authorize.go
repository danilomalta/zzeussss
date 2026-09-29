// Package identity aplica permissões locais a identidades vinculadas a empresas.
// O chamador deve obter IdentityID de uma sessão validada, nunca de um campo do pedido.
package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrDenied = errors.New("acesso negado")

type Permission string

const (
	Sell             Permission = "sell"
	ManageStock      Permission = "manage_stock"
	ReviewDiscount   Permission = "review_discount"
	ManageStaff      Permission = "manage_staff"
	ViewAccounting   Permission = "view_accounting"
	ManageProduction Permission = "manage_production"
	ViewOrders       Permission = "view_orders"
)

type Scope struct {
	IdentityID string
	TenantID   string
	StoreID    string
}

// Can exige vínculo ativo na empresa e, exceto para o dono, acesso explícito
// à loja. Consultas e mutações que usam Can devem compartilhar transação quando
// a revogação concorrente importar para a decisão.
func Can(ctx context.Context, db *sql.DB, scope Scope, permission Permission) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(scope.IdentityID) == "" || strings.TrimSpace(scope.TenantID) == "" {
		return ErrDenied
	}
	var role, status string
	err := db.QueryRowContext(ctx,
		"SELECT role, status FROM memberships WHERE tenant_id = ? AND identity_id = ?",
		scope.TenantID, scope.IdentityID).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return fmt.Errorf("consultar vínculo: %w", err)
	}
	if status != "active" || !allowed(role, permission) {
		return ErrDenied
	}
	if scope.StoreID == "" {
		if role == "owner" && permission == ManageStaff {
			return nil
		}
		return ErrDenied
	}
	var exists int
	if role == "owner" {
		err = db.QueryRowContext(ctx,
			"SELECT 1 FROM stores WHERE tenant_id = ? AND id = ?",
			scope.TenantID, scope.StoreID).Scan(&exists)
	} else {
		err = db.QueryRowContext(ctx,
			"SELECT 1 FROM membership_stores WHERE tenant_id = ? AND identity_id = ? AND store_id = ?",
			scope.TenantID, scope.IdentityID, scope.StoreID).Scan(&exists)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return fmt.Errorf("consultar loja autorizada: %w", err)
	}
	return nil
}

// CanReviewDiscount impede aprovação da própria solicitação, mesmo por gerente.
func CanReviewDiscount(ctx context.Context, db *sql.DB, scope Scope, requesterID string) error {
	if strings.TrimSpace(requesterID) == "" || scope.IdentityID == requesterID {
		return ErrDenied
	}
	return Can(ctx, db, scope, ReviewDiscount)
}

func allowed(role string, permission Permission) bool {
	switch role {
	case "owner":
		switch permission {
		case Sell, ManageStock, ReviewDiscount, ManageStaff, ViewAccounting, ManageProduction, ViewOrders:
			return true
		}
	case "manager":
		switch permission {
		case Sell, ManageStock, ReviewDiscount, ManageStaff, ViewAccounting, ViewOrders:
			return true
		}
	case "cashier":
		return permission == Sell
	case "stock":
		return permission == ManageStock || permission == ViewOrders
	case "production":
		return permission == ManageProduction || permission == ViewOrders
	case "supplier":
		return permission == ViewOrders
	case "accountant":
		return permission == ViewAccounting
	case "employee":
		return false
	}
	return false
}
