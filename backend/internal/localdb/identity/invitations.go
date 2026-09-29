package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
)

var ErrInviteUnavailable = errors.New("convite inválido ou indisponível")

const invitationTTL = 15 * time.Minute

// IssueInvite cria um código de uso único para uma identidade já existente.
// issuer.IdentityID deve vir da sessão validada, nunca do JSON enviado pelo cliente.
func IssueInvite(ctx context.Context, db *sql.DB, issuer Scope, targetID, role, storeID string) (string, error) {
	if db == nil {
		return "", errors.New("banco local indisponível")
	}
	if strings.TrimSpace(issuer.IdentityID) == "" || strings.TrimSpace(issuer.TenantID) == "" ||
		strings.TrimSpace(targetID) == "" || strings.TrimSpace(storeID) == "" || issuer.IdentityID == targetID {
		return "", ErrDenied
	}
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("gerar convite: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(entropy[:])
	hash := sha256.Sum256([]byte(token))
	inviteID, err := localdb.NewID()
	if err != nil {
		return "", err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var issuerRole, status string
	if err := tx.QueryRowContext(ctx,
		"SELECT role, status FROM memberships WHERE tenant_id = ? AND identity_id = ?",
		issuer.TenantID, issuer.IdentityID).Scan(&issuerRole, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrDenied
		}
		return "", err
	}
	if status != "active" || !mayInvite(issuerRole, role) {
		return "", ErrDenied
	}
	var storeAllowed int
	if issuerRole == "owner" {
		err = tx.QueryRowContext(ctx, "SELECT 1 FROM stores WHERE tenant_id = ? AND id = ?",
			issuer.TenantID, storeID).Scan(&storeAllowed)
	} else {
		err = tx.QueryRowContext(ctx,
			"SELECT 1 FROM membership_stores WHERE tenant_id = ? AND identity_id = ? AND store_id = ?",
			issuer.TenantID, issuer.IdentityID, storeID).Scan(&storeAllowed)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDenied
	}
	if err != nil {
		return "", err
	}
	var existing int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM memberships WHERE tenant_id = ? AND identity_id = ?",
		issuer.TenantID, targetID).Scan(&existing)
	if err == nil {
		return "", ErrDenied
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO membership_invites
		(id, tenant_id, store_id, target_identity_id, issued_by, role, token_hash, expires_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		inviteID, issuer.TenantID, storeID, targetID, issuer.IdentityID, role, fmt.Sprintf("%x", hash), now+int64(invitationTTL.Seconds()))
	if err != nil {
		return "", fmt.Errorf("registrar convite: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO membership_invite_events VALUES (?, ?, ?, ?, ?)`,
		eventID, inviteID, issuer.IdentityID, "issued", now); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

// RedeemInvite exige a identidade autenticada do destinatário. Conhecer apenas
// o código não concede papel nem permite escolher a empresa pelo corpo HTTP.
func RedeemInvite(ctx context.Context, db *sql.DB, actorID, token string) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(actorID) == "" || len(token) != 43 {
		return ErrInviteUnavailable
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return ErrInviteUnavailable
	}
	hash := sha256.Sum256([]byte(token))
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var inviteID, tenantID, storeID, role string
	err = tx.QueryRowContext(ctx, `UPDATE membership_invites SET consumed_unix = ?
		WHERE token_hash = ? AND target_identity_id = ? AND consumed_unix IS NULL
		AND revoked_unix IS NULL AND expires_unix > ?
		AND EXISTS (SELECT 1 FROM memberships m WHERE m.tenant_id = membership_invites.tenant_id
		AND m.identity_id = membership_invites.issued_by AND m.status = 'active'
		AND (m.role = 'owner' OR (m.role = 'manager'
		AND membership_invites.role IN ('cashier', 'stock', 'employee')
		AND EXISTS (SELECT 1 FROM membership_stores ms WHERE ms.tenant_id = m.tenant_id
		AND ms.identity_id = m.identity_id AND ms.store_id = membership_invites.store_id))))
		RETURNING id, tenant_id, store_id, role`,
		now, fmt.Sprintf("%x", hash), actorID, now).Scan(&inviteID, &tenantID, &storeID, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInviteUnavailable
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memberships (tenant_id, identity_id, role, status, created_at)
		VALUES (?, ?, ?, 'active', ?)`, tenantID, actorID, role, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return ErrInviteUnavailable
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO membership_stores VALUES (?, ?, ?)`,
		tenantID, actorID, storeID); err != nil {
		return err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO membership_invite_events VALUES (?, ?, ?, ?, ?)`,
		eventID, inviteID, actorID, "consumed", now); err != nil {
		return err
	}
	return tx.Commit()
}

// RevokeInvite cancela convite pendente. O dono pode cancelar convites da sua
// empresa; o gerente somente convites que emitiu para loja ainda autorizada.
func RevokeInvite(ctx context.Context, db *sql.DB, actor Scope, inviteID string) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(actor.IdentityID) == "" || strings.TrimSpace(actor.TenantID) == "" ||
		strings.TrimSpace(inviteID) == "" {
		return ErrDenied
	}
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var returned string
	err = tx.QueryRowContext(ctx, `UPDATE membership_invites SET revoked_unix = ?
		WHERE id = ? AND tenant_id = ? AND consumed_unix IS NULL AND revoked_unix IS NULL
		AND EXISTS (SELECT 1 FROM memberships m WHERE m.tenant_id = membership_invites.tenant_id
		AND m.identity_id = ? AND m.status = 'active'
		AND (m.role = 'owner' OR (m.role = 'manager' AND m.identity_id = membership_invites.issued_by
		AND EXISTS (SELECT 1 FROM membership_stores ms WHERE ms.tenant_id = m.tenant_id
		AND ms.identity_id = m.identity_id AND ms.store_id = membership_invites.store_id))))
		RETURNING id`, now, inviteID, actor.TenantID, actor.IdentityID).Scan(&returned)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInviteUnavailable
	}
	if err != nil {
		return err
	}
	eventID, err := localdb.NewID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO membership_invite_events VALUES (?, ?, ?, ?, ?)`,
		eventID, returned, actor.IdentityID, "revoked", now); err != nil {
		return err
	}
	return tx.Commit()
}

func mayInvite(issuerRole, invitedRole string) bool {
	switch issuerRole {
	case "owner":
		switch invitedRole {
		case "manager", "cashier", "stock", "production", "employee", "supplier", "accountant":
			return true
		}
	case "manager":
		return invitedRole == "cashier" || invitedRole == "stock" || invitedRole == "employee"
	}
	return false
}
