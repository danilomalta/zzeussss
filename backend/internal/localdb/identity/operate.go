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
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := CanOperateTx(ctx, tx, actor, device, permission); err != nil {
		return err
	}
	return tx.Commit()
}

// CanOperateTx permite verificar autorização na mesma transação da escrita.
// O chamador continua responsável por validar a sessão humana e a prova do aparelho.
func CanOperateTx(ctx context.Context, tx *sql.Tx, actor Scope, device DeviceContext, permission Permission) error {
	if tx == nil {
		return errors.New("transação local indisponível")
	}
	if strings.TrimSpace(actor.IdentityID) == "" || strings.TrimSpace(actor.TenantID) == "" ||
		strings.TrimSpace(actor.StoreID) == "" || strings.TrimSpace(device.DeviceID) == "" ||
		actor.TenantID != device.TenantID || actor.StoreID != device.StoreID {
		return ErrDenied
	}
	var role, status string
	err := tx.QueryRowContext(ctx, "SELECT role, status FROM memberships WHERE tenant_id = ? AND identity_id = ?",
		actor.TenantID, actor.IdentityID).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return err
	}
	if status != "active" {
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
	permit, err := effective(ctx, tx, actor, role, permission)
	if err != nil {
		return err
	}
	if !permit {
		return ErrDenied
	}
	return nil
}

// CanOperateReview acrescenta a regra de não aprovar a própria solicitação.
func CanOperateReview(ctx context.Context, db *sql.DB, actor Scope, device DeviceContext, requesterID string) error {
	if strings.TrimSpace(requesterID) == "" || requesterID == actor.IdentityID {
		return ErrDenied
	}
	return CanOperate(ctx, db, actor, device, ReviewDiscount)
}
