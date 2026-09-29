package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
)

const deviceChallengeTTL = time.Minute

type DeviceChallenge struct {
	ID    string
	Nonce []byte
}

type DeviceContext struct {
	TenantID string
	StoreID  string
	DeviceID string
}

// DeviceAuthMessage liga assinatura, operação, IDs e nonce ao mesmo desafio.
func DeviceAuthMessage(ctx DeviceContext, challenge DeviceChallenge) []byte {
	message := []byte("titan-device-auth-v1\x00" + ctx.TenantID + "\x00" + ctx.StoreID + "\x00" + ctx.DeviceID + "\x00" + challenge.ID + "\x00")
	return append(message, challenge.Nonce...)
}

// IssueDeviceChallenge só entrega desafio para chave pública aprovada.
// A rota futura deve limitar tentativas; este pacote não cria sessão de usuário.
func IssueDeviceChallenge(ctx context.Context, db *sql.DB, device DeviceContext) (DeviceChallenge, error) {
	if db == nil {
		return DeviceChallenge{}, errors.New("banco local indisponível")
	}
	if strings.TrimSpace(device.TenantID) == "" || strings.TrimSpace(device.StoreID) == "" || strings.TrimSpace(device.DeviceID) == "" {
		return DeviceChallenge{}, ErrDenied
	}
	var status string
	err := db.QueryRowContext(ctx, `SELECT status FROM device_pairings
		WHERE tenant_id = ? AND store_id = ? AND device_id = ?`,
		device.TenantID, device.StoreID, device.DeviceID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceChallenge{}, ErrDenied
	}
	if err != nil {
		return DeviceChallenge{}, err
	}
	if status != "approved" {
		return DeviceChallenge{}, ErrDenied
	}
	id, err := localdb.NewID()
	if err != nil {
		return DeviceChallenge{}, err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return DeviceChallenge{}, err
	}
	now := time.Now().UTC().Unix()
	_, err = db.ExecContext(ctx, `INSERT INTO device_auth_challenges
		(id, tenant_id, store_id, device_id, nonce, issued_unix, expires_unix)
		SELECT ?, tenant_id, store_id, device_id, ?, ?, ? FROM device_pairings
		WHERE tenant_id = ? AND store_id = ? AND device_id = ? AND status = 'approved'`,
		id, nonce, now, now+int64(deviceChallengeTTL.Seconds()),
		device.TenantID, device.StoreID, device.DeviceID)
	if err != nil {
		return DeviceChallenge{}, err
	}
	var persisted int
	if err := db.QueryRowContext(ctx, "SELECT 1 FROM device_auth_challenges WHERE id = ?", id).Scan(&persisted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeviceChallenge{}, ErrDenied
		}
		return DeviceChallenge{}, err
	}
	return DeviceChallenge{ID: id, Nonce: nonce}, nil
}

// CompleteDeviceChallenge prova posse da chave privada em desafio de uso único.
// O contexto devolvido é só do dispositivo; não autentica o operador humano.
func CompleteDeviceChallenge(ctx context.Context, db *sql.DB, device DeviceContext, challengeID string, signature []byte) (DeviceContext, error) {
	if db == nil {
		return DeviceContext{}, errors.New("banco local indisponível")
	}
	if strings.TrimSpace(challengeID) == "" || len(signature) != ed25519.SignatureSize ||
		strings.TrimSpace(device.TenantID) == "" || strings.TrimSpace(device.StoreID) == "" || strings.TrimSpace(device.DeviceID) == "" {
		return DeviceContext{}, ErrDenied
	}
	now := time.Now().UTC().Unix()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return DeviceContext{}, err
	}
	defer tx.Rollback()
	var nonce, publicKey []byte
	var expires int64
	var status string
	var consumed sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT c.nonce, c.expires_unix, c.consumed_unix, p.public_key, p.status
		FROM device_auth_challenges c JOIN device_pairings p
		ON p.tenant_id = c.tenant_id AND p.store_id = c.store_id AND p.device_id = c.device_id
		WHERE c.id = ? AND c.tenant_id = ? AND c.store_id = ? AND c.device_id = ?`,
		challengeID, device.TenantID, device.StoreID, device.DeviceID).
		Scan(&nonce, &expires, &consumed, &publicKey, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceContext{}, ErrDenied
	}
	if err != nil {
		return DeviceContext{}, err
	}
	challenge := DeviceChallenge{ID: challengeID, Nonce: nonce}
	if status != "approved" || consumed.Valid || expires <= now ||
		!ed25519.Verify(ed25519.PublicKey(publicKey), DeviceAuthMessage(device, challenge), signature) {
		return DeviceContext{}, ErrDenied
	}
	result, err := tx.ExecContext(ctx, `UPDATE device_auth_challenges SET consumed_unix = ?
		WHERE id = ? AND tenant_id = ? AND store_id = ? AND device_id = ?
		AND consumed_unix IS NULL AND expires_unix > ?
		AND EXISTS (SELECT 1 FROM device_pairings p WHERE p.tenant_id = device_auth_challenges.tenant_id
		AND p.store_id = device_auth_challenges.store_id AND p.device_id = device_auth_challenges.device_id
		AND p.status = 'approved')`, now, challengeID,
		device.TenantID, device.StoreID, device.DeviceID, now)
	if err != nil {
		return DeviceContext{}, err
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return DeviceContext{}, ErrDenied
	}
	if err := tx.Commit(); err != nil {
		return DeviceContext{}, err
	}
	return device, nil
}
