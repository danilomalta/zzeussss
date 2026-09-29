package identity

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// CanOperate verifica usuário, papel, empresa, loja e dispositivo aprovado na
// mesma leitura transacional. O chamador deve provar previamente as identidades
// humana e do aparelho; IDs enviados pelo cliente não são prova suficiente.
func CanOperate(ctx context.Context, db *sql.DB, actor Scope, device DeviceContext, permission Permission) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(actor.IdentityID) == "" || strings.TrimSpace(actor.TenantID) == "" ||
		strings.TrimSpace(actor.StoreID) == "" || strings.TrimSpace(device.DeviceID) == "" ||
		actor.TenantID != device.TenantID || actor.StoreID != device.StoreID {
		return ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var role, status string
	err = tx.QueryRowContext(ctx, "SELECT role, status FROM memberships WHERE tenant_id = ? AND identity_id = ?",
		actor.TenantID, actor.IdentityID).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return err
	}
	if status != "active" || !allowed(role, permission) {
		return ErrDenied
	}
	if role != "owner" {
		var linked int
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM membership_stores
			WHERE tenant_id = ? AND identity_id = ? AND store_id = ?`,
			actor.TenantID, actor.IdentityID, actor.StoreID).Scan(&linked)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDenied
		}
		if err != nil {
			return err
		}
	}
	var approved int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM device_pairings
		WHERE tenant_id = ? AND store_id = ? AND device_id = ? AND status = 'approved'`,
		actor.TenantID, actor.StoreID, device.DeviceID).Scan(&approved)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// CanOperateReview acrescenta a regra de não aprovar a própria solicitação.
func CanOperateReview(ctx context.Context, db *sql.DB, actor Scope, device DeviceContext, requesterID string) error {
	if strings.TrimSpace(requesterID) == "" || requesterID == actor.IdentityID {
		return ErrDenied
	}
	return CanOperate(ctx, db, actor, device, ReviewDiscount)
}
