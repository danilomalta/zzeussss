package localauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

var errSecurityWrite = errors.New("falha na escrita de segurança da conta")

// No token, hash or other user's identity is returned in session listings.
type SessionInfo struct {
	StoreID     string `json:"store_id"`
	DeviceID    string `json:"device_id"`
	CreatedUnix int64  `json:"created_unix"`
	ExpiresUnix int64  `json:"expires_unix"`
	Current     bool   `json:"current"`
}

func ListSessions(ctx context.Context, db *sql.DB, device identity.DeviceContext, token string) ([]SessionInfo, error) {
	if db == nil {
		return nil, ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := resolve(ctx, tx, token)
	if err != nil {
		return nil, err
	}
	if current.Device != device {
		return nil, ErrDenied
	}
	digest := sha256.Sum256([]byte(token))
	rows, err := tx.QueryContext(ctx, `SELECT s.store_id,s.device_id,s.created_unix,s.expires_unix,s.token_sha256=?
        FROM local_sessions s JOIN device_pairings p ON p.tenant_id=s.tenant_id AND p.store_id=s.store_id AND p.device_id=s.device_id
        WHERE s.tenant_id=? AND s.identity_id=? AND s.revoked_unix IS NULL AND s.expires_unix>? AND p.status='approved'
        ORDER BY s.created_unix DESC,s.rowid DESC LIMIT 100`, digest[:], current.Actor.TenantID, current.Actor.IdentityID, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	result := make([]SessionInfo, 0)
	for rows.Next() {
		var item SessionInfo
		if err := rows.Scan(&item.StoreID, &item.DeviceID, &item.CreatedUnix, &item.ExpiresUnix, &item.Current); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// ChangeOwnPassword needs a current session AND the current password. The new
// hash, revocation of every session, and audit record commit together.
func ChangeOwnPassword(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, currentPassword, newPassword string) error {
	if currentPassword == newPassword {
		return ErrDenied
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return updateSecurity(ctx, db, device, token, currentPassword, hash)
}

func RevokeOtherSessions(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, currentPassword string) error {
	return updateSecurity(ctx, db, device, token, currentPassword, nil)
}

func updateSecurity(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, password string, newHash []byte) error {
	if db == nil || len(password) > 72 || password == "" {
		return ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	session, err := resolve(ctx, tx, token)
	if err != nil {
		return err
	}
	if session.Device != device {
		return ErrDenied
	}
	var oldHash []byte
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM local_passwords WHERE tenant_id=? AND identity_id=?`, session.Actor.TenantID, session.Actor.IdentityID).Scan(&oldHash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword(oldHash, []byte(password)) != nil {
		return ErrDenied
	}
	now := time.Now().Unix()
	kind := "sessions_revoked"
	digest := sha256.Sum256([]byte(token))
	query := `UPDATE local_sessions SET revoked_unix=? WHERE tenant_id=? AND identity_id=? AND revoked_unix IS NULL AND expires_unix>? AND token_sha256<>?`
	args := []any{now, session.Actor.TenantID, session.Actor.IdentityID, now, digest[:]}
	if newHash != nil {
		kind = "password_changed"
		updated, err := tx.ExecContext(ctx, `UPDATE local_passwords SET password_hash=?,changed_at=? WHERE tenant_id=? AND identity_id=?`, newHash, time.Now().UTC().Format(time.RFC3339Nano), session.Actor.TenantID, session.Actor.IdentityID)
		if err != nil {
			return err
		}
		written, err := updated.RowsAffected()
		if err != nil {
			return err
		}
		if written != 1 {
			return errSecurityWrite
		}
		query = `UPDATE local_sessions SET revoked_unix=? WHERE tenant_id=? AND identity_id=? AND revoked_unix IS NULL`
		args = []any{now, session.Actor.TenantID, session.Actor.IdentityID}
	}
	var expected int64
	if newHash != nil {
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM local_sessions WHERE tenant_id=? AND identity_id=? AND revoked_unix IS NULL`, session.Actor.TenantID, session.Actor.IdentityID).Scan(&expected)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT count(*) FROM local_sessions WHERE tenant_id=? AND identity_id=? AND revoked_unix IS NULL AND expires_unix>? AND token_sha256<>?`, session.Actor.TenantID, session.Actor.IdentityID, now, digest[:]).Scan(&expected)
	}
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != expected {
		return errSecurityWrite
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO account_security_events VALUES (?,?,?,?,?,?,?,?)`, id, session.Actor.TenantID, session.Actor.StoreID, session.Device.DeviceID, session.Actor.IdentityID, kind, count, now)
	if err != nil {
		return err
	}
	written, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if written != 1 {
		return errSecurityWrite
	}
	return tx.Commit()
}
