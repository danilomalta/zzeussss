package localauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

const RecoveryTTL = 365 * 24 * time.Hour

func ownerContext(ctx context.Context, tx *sql.Tx, device identity.DeviceContext, owner string) (Session, error) {
	actor := identity.Scope{TenantID: device.TenantID, StoreID: device.StoreID, IdentityID: owner}
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStaff); err != nil {
		if errors.Is(err, identity.ErrDenied) {
			return Session{}, ErrDenied
		}
		return Session{}, err
	}
	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?`, device.TenantID, owner).Scan(&role); err != nil {
		return Session{}, err
	}
	if role != "owner" {
		return Session{}, ErrDenied
	}
	return Session{Actor: actor, Device: device}, nil
}

// IssueOwnerRecovery needs an authenticated owner and a pre-proven station.
// persist must save the secret in an exclusive private file before commit.
// A DB failure may leave that file, but the old key stays valid after rollback.
func IssueOwnerRecovery(ctx context.Context, db *sql.DB, device identity.DeviceContext, owner, password string, secret []byte, persist func() error) error {
	if db == nil || !validAccessID(owner) || len(secret) != 32 || persist == nil {
		return ErrAccessInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	session, err := ownerContext(ctx, tx, device, owner)
	if err != nil {
		return err
	}
	if err := reauthenticate(ctx, tx, session, password); err != nil {
		return err
	}
	digest := sha256.Sum256(secret)
	now := time.Now().Unix()
	if err := persist(); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO owner_recovery_keys VALUES (?,?,?,?,?,?,?,NULL)
  ON CONFLICT(tenant_id,identity_id,device_id) DO UPDATE SET store_id=excluded.store_id,token_sha256=excluded.token_sha256,issued_unix=excluded.issued_unix,expires_unix=excluded.expires_unix,consumed_unix=NULL`, device.TenantID, owner, device.StoreID, device.DeviceID, digest[:], now, now+int64(RecoveryTTL.Seconds()))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errSecurityWrite
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	if err := recordAccess(ctx, tx, session.Actor, device, id, owner, "recovery_issued", "private emergency key prepared", "", 0); err != nil {
		return err
	}
	return tx.Commit()
}

// RecoverOwner consumes all current recovery keys for this owner in this local
// database, changes the password, revokes sessions and audits atomically.
// The caller must prove the station; no session or password is bypassed by ID.
func RecoverOwner(ctx context.Context, db *sql.DB, device identity.DeviceContext, owner, encoded, newPassword string) error {
	if db == nil || !validAccessID(owner) || len(encoded) != 43 {
		return ErrDenied
	}
	secret, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(secret) != 32 || base64.RawURLEncoding.EncodeToString(secret) != encoded {
		return ErrDenied
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return ErrAccessInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	session, err := ownerContext(ctx, tx, device, owner)
	if err != nil {
		return err
	}
	var stored []byte
	var issued, expires int64
	var consumed sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT token_sha256,issued_unix,expires_unix,consumed_unix FROM owner_recovery_keys WHERE tenant_id=? AND identity_id=? AND store_id=? AND device_id=?`, device.TenantID, owner, device.StoreID, device.DeviceID).Scan(&stored, &issued, &expires, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDenied
	}
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	digest := sha256.Sum256(secret)
	if consumed.Valid || expires <= now || issued > now || subtle.ConstantTimeCompare(stored, digest[:]) != 1 {
		return ErrDenied
	}
	if err := writePassword(ctx, tx, device.TenantID, owner, hash); err != nil {
		return err
	}
	affected, err := revokeIdentitySessions(ctx, tx, device.TenantID, owner)
	if err != nil {
		return err
	}
	n, err := consumeRecoveryKeys(ctx, tx, device.TenantID, owner)
	if err != nil {
		return err
	}
	if n < 1 {
		return errSecurityWrite
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	if err := recordAccess(ctx, tx, session.Actor, device, id, owner, "owner_recovered", "single-use emergency recovery", "", affected); err != nil {
		return err
	}
	return tx.Commit()
}

func consumeRecoveryKeys(ctx context.Context, tx *sql.Tx, tenant, owner string) (int64, error) {
	var expected int64
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM owner_recovery_keys WHERE tenant_id=? AND identity_id=? AND consumed_unix IS NULL`, tenant, owner).Scan(&expected); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE owner_recovery_keys SET consumed_unix=? WHERE tenant_id=? AND identity_id=? AND consumed_unix IS NULL`, time.Now().Unix(), tenant, owner)
	if err != nil {
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != expected {
		return 0, errSecurityWrite
	}
	return n, nil
}

func RevokeOwnRecovery(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, password string) error {
	if db == nil {
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
	if _, err := ownerContext(ctx, tx, device, session.Actor.IdentityID); err != nil {
		return err
	}
	if err := reauthenticate(ctx, tx, session, password); err != nil {
		return err
	}
	if _, err := consumeRecoveryKeys(ctx, tx, device.TenantID, session.Actor.IdentityID); err != nil {
		return err
	}
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	if err := recordAccess(ctx, tx, session.Actor, device, id, session.Actor.IdentityID, "recovery_revoked", "emergency recovery keys revoked", "", 0); err != nil {
		return err
	}
	return tx.Commit()
}
