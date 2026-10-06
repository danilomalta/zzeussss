package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
)

const pairingTTL = 5 * time.Minute

// PairingMessage é a mensagem exata assinada pelo aparelho. Os IDs e o desafio
// são ligados à assinatura, evitando reutilizá-la em outra loja ou aparelho.
func PairingMessage(tenantID, storeID, deviceID string, challenge []byte) []byte {
	message := []byte("titan-device-pair-v1\x00" + tenantID + "\x00" + storeID + "\x00" + deviceID + "\x00")
	return append(message, challenge...)
}

// RequestPairing é iniciada por dono/gerente autenticado e recebe somente a
// chave pública Ed25519. A chave privada permanece exclusivamente no aparelho.
func RequestPairing(ctx context.Context, db *sql.DB, actor Scope, deviceID, name string, publicKey ed25519.PublicKey) ([]byte, error) {
	if db == nil {
		return nil, errors.New("banco local indisponível")
	}
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.StoreID) == "" ||
		strings.TrimSpace(actor.IdentityID) == "" || strings.TrimSpace(deviceID) == "" ||
		strings.TrimSpace(name) == "" || len(publicKey) != ed25519.PublicKeySize {
		return nil, ErrDenied
	}
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := requireDeviceAdmin(ctx, tx, actor, actor.StoreID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO devices VALUES (?, ?, ?, ?)",
		actor.TenantID, actor.StoreID, deviceID, name); err != nil {
		return nil, fmt.Errorf("registrar aparelho: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO device_pairings
		(tenant_id, store_id, device_id, public_key, challenge, challenge_expires_unix, status, requested_by)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', ?)`, actor.TenantID, actor.StoreID,
		deviceID, []byte(publicKey), challenge, now+int64(pairingTTL.Seconds()), actor.IdentityID); err != nil {
		return nil, err
	}
	if err := deviceEvent(ctx, tx, actor.TenantID, deviceID, actor.IdentityID, "requested", now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return challenge, nil
}

// ProvePairing valida a posse da chave privada, sem aprovar o aparelho.
func ProvePairing(ctx context.Context, db *sql.DB, tenantID, deviceID string, signature []byte) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(deviceID) == "" || len(signature) != ed25519.SignatureSize {
		return ErrDenied
	}
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var storeID, status string
	var publicKey, challenge []byte
	var expiry int64
	err = tx.QueryRowContext(ctx, `SELECT store_id, public_key, challenge, challenge_expires_unix, status
		FROM device_pairings WHERE tenant_id = ? AND device_id = ?`, tenantID, deviceID).
		Scan(&storeID, &publicKey, &challenge, &expiry, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return err
	}
	if status != "pending" || expiry <= now ||
		!ed25519.Verify(ed25519.PublicKey(publicKey), PairingMessage(tenantID, storeID, deviceID, challenge), signature) {
		return ErrDenied
	}
	result, err := tx.ExecContext(ctx, `UPDATE device_pairings SET status = 'verified', proof_unix = ?
		WHERE tenant_id = ? AND device_id = ? AND status = 'pending' AND challenge_expires_unix > ?`,
		now, tenantID, deviceID, now)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return ErrDenied
	}
	if err := deviceEvent(ctx, tx, tenantID, deviceID, deviceID, "proved", now); err != nil {
		return err
	}
	return tx.Commit()
}

// ApprovePairing exige uma segunda ação de administrador enquanto o desafio é válido.
func ApprovePairing(ctx context.Context, db *sql.DB, actor Scope, deviceID string) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(actor.IdentityID) == "" || strings.TrimSpace(actor.TenantID) == "" ||
		strings.TrimSpace(actor.StoreID) == "" || strings.TrimSpace(deviceID) == "" {
		return ErrDenied
	}
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireDeviceAdmin(ctx, tx, actor, actor.StoreID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE device_pairings SET status = 'approved', approved_by = ?, approved_unix = ?
		WHERE tenant_id = ? AND store_id = ? AND device_id = ? AND status = 'verified'
		AND proof_unix IS NOT NULL AND challenge_expires_unix > ?`, actor.IdentityID, now,
		actor.TenantID, actor.StoreID, deviceID, now)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return ErrDenied
	}
	if err := deviceEvent(ctx, tx, actor.TenantID, deviceID, actor.IdentityID, "approved", now); err != nil {
		return err
	}
	return tx.Commit()
}

// RevokeDevice retira o estado aprovado. Os consumidores futuros devem consultar
// esse estado ao autenticar, inclusive ao reconectar depois de operar offline.
func RevokeDevice(ctx context.Context, db *sql.DB, actor Scope, deviceID string) error {
	if db == nil {
		return errors.New("banco local indisponível")
	}
	if strings.TrimSpace(actor.IdentityID) == "" || strings.TrimSpace(actor.TenantID) == "" ||
		strings.TrimSpace(actor.StoreID) == "" || strings.TrimSpace(deviceID) == "" {
		return ErrDenied
	}
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireDeviceAdmin(ctx, tx, actor, actor.StoreID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE device_pairings SET status = 'revoked', revoked_by = ?, revoked_unix = ?
		WHERE tenant_id = ? AND store_id = ? AND device_id = ? AND status = 'approved'`,
		actor.IdentityID, now, actor.TenantID, actor.StoreID, deviceID)
	if err != nil {
		return err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return ErrDenied
	}
	if err := deviceEvent(ctx, tx, actor.TenantID, deviceID, actor.IdentityID, "revoked", now); err != nil {
		return err
	}
	return tx.Commit()
}

func requireDeviceAdmin(ctx context.Context, tx *sql.Tx, actor Scope, storeID string) error {
	var role, status string
	err := tx.QueryRowContext(ctx, "SELECT role, status FROM memberships WHERE tenant_id = ? AND identity_id = ?",
		actor.TenantID, actor.IdentityID).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return err
	}
	if status != "active" || (role != "owner" && role != "manager") || actor.StoreID != storeID {
		return ErrDenied
	}
	permit, policyErr := effective(ctx, tx, actor, role, ManageStaff)
	if policyErr != nil {
		return policyErr
	}
	if !permit {
		return ErrDenied
	}
	if role == "owner" {
		err = tx.QueryRowContext(ctx, "SELECT 1 FROM stores WHERE tenant_id = ? AND id = ?",
			actor.TenantID, storeID).Scan(new(int))
	} else {
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM membership_stores
			WHERE tenant_id = ? AND identity_id = ? AND store_id = ?`,
			actor.TenantID, actor.IdentityID, storeID).Scan(new(int))
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	return err
}

func deviceEvent(ctx context.Context, tx *sql.Tx, tenantID, deviceID, actorRef, action string, now int64) error {
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO device_pairing_events VALUES (?, ?, ?, ?, ?, ?)",
		id, tenantID, deviceID, actorRef, action, now)
	return err
}
