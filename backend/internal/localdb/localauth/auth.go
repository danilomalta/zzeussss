// Package localauth armazena credenciais humanas e sessões do dispositivo
// offline. Senha e token em texto puro não são persistidos no SQLite.
package localauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"titansystem-backend/internal/localdb/identity"
)

var ErrDenied = errors.New("credenciais locais inválidas")

const sessionTTL = 8 * time.Hour

func HashPassword(password string) ([]byte, error) {
	if len(password) < 12 || len(password) > 72 || strings.TrimSpace(password) != password {
		return nil, ErrDenied
	}
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

// SetPassword requer dono/gerente provado pelo serviço local. A instalação
// inicial usa HashPassword antes de existir qualquer conta autenticável.
func SetPassword(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, targetID, password string) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(targetID) == "" {
		return ErrDenied
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStaff); err != nil {
		return err
	}
	var role, status string
	err = tx.QueryRowContext(ctx, `SELECT role,status FROM memberships WHERE tenant_id=? AND identity_id=?`, actor.TenantID, targetID).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return ErrDenied
	}
	if role == "owner" && targetID != actor.IdentityID {
		return ErrDenied
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO local_passwords VALUES (?,?,?,?) ON CONFLICT(tenant_id,identity_id)
		DO UPDATE SET password_hash=excluded.password_hash,changed_at=excluded.changed_at`, actor.TenantID, targetID, hash, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE local_sessions SET revoked_unix=? WHERE tenant_id=? AND identity_id=? AND revoked_unix IS NULL`, time.Now().Unix(), actor.TenantID, targetID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

type Session struct {
	Token       string
	Actor       identity.Scope
	Device      identity.DeviceContext
	ExpiresUnix int64
}

// Login associa senha humana ao aparelho local já autenticado pelo processo.
func Login(ctx context.Context, db *sql.DB, device identity.DeviceContext, identityID, password string) (Session, error) {
	if db == nil {
		return Session{}, errors.New("banco local indisponível")
	}
	if identityID == "" || password == "" || device.TenantID == "" || device.StoreID == "" || device.DeviceID == "" {
		return Session{}, ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback()
	var hash []byte
	var role string
	err = tx.QueryRowContext(ctx, `SELECT p.password_hash,m.role FROM local_passwords p JOIN memberships m
		ON m.tenant_id=p.tenant_id AND m.identity_id=p.identity_id
		WHERE p.tenant_id=? AND p.identity_id=? AND m.status='active'`, device.TenantID, identityID).Scan(&hash, &role)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrDenied
	}
	if err != nil {
		return Session{}, err
	}
	if err = bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return Session{}, ErrDenied
	}
	var allowed int
	if role == "owner" {
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM stores WHERE tenant_id=? AND id=?`, device.TenantID, device.StoreID).Scan(&allowed)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM membership_stores WHERE tenant_id=? AND identity_id=? AND store_id=?`, device.TenantID, identityID, device.StoreID).Scan(&allowed)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrDenied
	}
	if err != nil {
		return Session{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`, device.TenantID, device.StoreID, device.DeviceID).Scan(&allowed)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrDenied
	}
	if err != nil {
		return Session{}, err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return Session{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	now := time.Now().Unix()
	expires := now + int64(sessionTTL.Seconds())
	_, err = tx.ExecContext(ctx, `INSERT INTO local_sessions VALUES (?,?,?,?,?,?,NULL,?)`, digest[:], device.TenantID, device.StoreID, device.DeviceID, identityID, expires, now)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(); err != nil {
		return Session{}, err
	}
	return Session{token, identity.Scope{TenantID: device.TenantID, StoreID: device.StoreID, IdentityID: identityID}, device, expires}, nil
}

// Resolve revalida vínculo e aparelho a cada chamada, inclusive após revogação.
func Resolve(ctx context.Context, db *sql.DB, token string) (Session, error) {
	if db == nil {
		return Session{}, errors.New("banco local indisponível")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return Session{}, ErrDenied
	}
	digest := sha256.Sum256([]byte(token))
	var actor identity.Scope
	var device identity.DeviceContext
	var expires int64
	err = db.QueryRowContext(ctx, `SELECT s.tenant_id,s.store_id,s.device_id,s.identity_id,s.expires_unix
		FROM local_sessions s JOIN memberships m ON m.tenant_id=s.tenant_id AND m.identity_id=s.identity_id
		JOIN device_pairings p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.device_id=s.device_id
		WHERE s.token_sha256=? AND s.revoked_unix IS NULL AND s.expires_unix>?
		AND m.status='active' AND p.status='approved'
		AND (m.role='owner' OR EXISTS (SELECT 1 FROM membership_stores ms
		WHERE ms.tenant_id=s.tenant_id AND ms.identity_id=s.identity_id AND ms.store_id=s.store_id))`, digest[:], time.Now().Unix()).
		Scan(&actor.TenantID, &actor.StoreID, &device.DeviceID, &actor.IdentityID, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrDenied
	}
	if err != nil {
		return Session{}, err
	}
	device.TenantID, device.StoreID = actor.TenantID, actor.StoreID
	return Session{token, actor, device, expires}, nil
}

func Logout(ctx context.Context, db *sql.DB, token string) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if _, err := Resolve(ctx, db, token); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(token))
	_, err := db.ExecContext(ctx, `UPDATE local_sessions SET revoked_unix=? WHERE token_sha256=? AND revoked_unix IS NULL`, time.Now().Unix(), digest[:])
	return err
}
