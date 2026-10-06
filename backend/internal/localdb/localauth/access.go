package localauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"titansystem-backend/internal/localdb/identity"
)

var ErrAccessDenied = errors.New("administração de acesso negada")
var ErrAccessConflict = errors.New("operação de acesso conflitante")
var ErrAccessInput = errors.New("entrada de acesso inválida")

type AdminInput struct {
	OperationID     string `json:"operation_id"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	Reason          string `json:"reason"`
}
type AccessResult struct {
	OperationID      string `json:"operation_id"`
	TargetID         string `json:"identity_id"`
	Repeated         bool   `json:"repeated"`
	AffectedSessions int64  `json:"affected_sessions"`
}

func validAccessID(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func reauthenticate(ctx context.Context, tx *sql.Tx, session Session, password string) error {
	if password == "" || len(password) > 72 {
		return ErrDenied
	}
	var hash []byte
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM local_passwords WHERE tenant_id=? AND identity_id=?`, session.Actor.TenantID, session.Actor.IdentityID).Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDenied
		}
		return err
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil {
		return ErrDenied
	}
	return nil
}

// Administrators may manage only a different non-owner in the current store.
// Managers are restricted to employee/cashier/stock, mirroring staff creation.
func authorizeTarget(ctx context.Context, tx *sql.Tx, session Session, target string) error {
	if target == session.Actor.IdentityID {
		return ErrAccessDenied
	}
	if err := identity.CanOperateTx(ctx, tx, session.Actor, session.Device, identity.ManageStaff); err != nil {
		if errors.Is(err, identity.ErrDenied) {
			return ErrAccessDenied
		}
		return err
	}
	var actorRole, targetRole string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?`, session.Actor.TenantID, session.Actor.IdentityID).Scan(&actorRole); err != nil {
		return err
	}
	err := tx.QueryRowContext(ctx, `SELECT m.role FROM memberships m JOIN membership_stores s ON s.tenant_id=m.tenant_id AND s.identity_id=m.identity_id
  WHERE m.tenant_id=? AND m.identity_id=? AND m.status='active' AND s.store_id=?`, session.Actor.TenantID, target, session.Actor.StoreID).Scan(&targetRole)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAccessDenied
	}
	if err != nil {
		return err
	}
	if targetRole == "owner" {
		return ErrAccessDenied
	}
	if actorRole == "owner" {
		return nil
	}
	if actorRole == "manager" && (targetRole == "employee" || targetRole == "cashier" || targetRole == "stock") {
		return nil
	}
	return ErrAccessDenied
}

func AdministerAccess(ctx context.Context, db *sql.DB, device identity.DeviceContext, token, target, kind string, input AdminInput) (AccessResult, error) {
	if db == nil || !validAccessID(target) || !validAccessID(input.OperationID) || len(input.Reason) > 255 || strings.TrimSpace(input.Reason) == "" || strings.ContainsRune(input.Reason, 0) {
		return AccessResult{}, ErrAccessInput
	}
	if kind != "password_reset" && kind != "sessions_revoked" {
		return AccessResult{}, ErrAccessInput
	}
	var newHash []byte
	var err error
	if kind == "password_reset" {
		newHash, err = HashPassword(input.NewPassword)
		if err != nil {
			return AccessResult{}, ErrAccessInput
		}
	} else if input.NewPassword != "" {
		return AccessResult{}, ErrAccessInput
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return AccessResult{}, err
	}
	defer tx.Rollback()
	session, err := resolve(ctx, tx, token)
	if err != nil {
		return AccessResult{}, err
	}
	if session.Device != device {
		return AccessResult{}, ErrDenied
	}
	if err := authorizeTarget(ctx, tx, session, target); err != nil {
		return AccessResult{}, err
	}
	if err := reauthenticate(ctx, tx, session, input.CurrentPassword); err != nil {
		return AccessResult{}, err
	}
	metadata, _ := json.Marshal([]string{session.Actor.StoreID, session.Actor.IdentityID, target, kind, input.Reason})
	digest := sha256.Sum256(metadata)
	fingerprint := hex.EncodeToString(digest[:])
	var previousHash string
	var affected int64
	err = tx.QueryRowContext(ctx, `SELECT request_hash,affected_sessions FROM account_access_operations WHERE tenant_id=? AND device_id=? AND operation_id=?`, device.TenantID, device.DeviceID, input.OperationID).Scan(&previousHash, &affected)
	result := AccessResult{OperationID: input.OperationID, TargetID: target}
	if err == nil {
		if previousHash != fingerprint {
			return AccessResult{}, ErrAccessConflict
		}
		if kind == "password_reset" {
			var currentHash []byte
			if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM local_passwords WHERE tenant_id=? AND identity_id=?`, device.TenantID, target).Scan(&currentHash); err != nil {
				return AccessResult{}, err
			}
			// A stale reset must never overwrite a password changed afterward.
			if bcrypt.CompareHashAndPassword(currentHash, []byte(input.NewPassword)) != nil {
				return AccessResult{}, ErrAccessConflict
			}
		}
		result.Repeated = true
		result.AffectedSessions = affected
		return result, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return AccessResult{}, err
	}
	if newHash != nil {
		if err := writePassword(ctx, tx, device.TenantID, target, newHash); err != nil {
			return AccessResult{}, err
		}
	}
	affected, err = revokeIdentitySessions(ctx, tx, device.TenantID, target)
	if err != nil {
		return AccessResult{}, err
	}
	if err := recordAccess(ctx, tx, session.Actor, device, input.OperationID, target, kind, input.Reason, fingerprint, affected); err != nil {
		return AccessResult{}, err
	}
	result.AffectedSessions = affected
	return result, tx.Commit()
}

func writePassword(ctx context.Context, tx *sql.Tx, tenant, target string, hash []byte) error {
	result, err := tx.ExecContext(ctx, `UPDATE local_passwords SET password_hash=?,changed_at=? WHERE tenant_id=? AND identity_id=?`, hash, time.Now().UTC().Format(time.RFC3339Nano), tenant, target)
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
	return nil
}

func revokeIdentitySessions(ctx context.Context, tx *sql.Tx, tenant, target string) (int64, error) {
	var expected int64
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM local_sessions WHERE tenant_id=? AND identity_id=? AND revoked_unix IS NULL`, tenant, target).Scan(&expected); err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE local_sessions SET revoked_unix=? WHERE tenant_id=? AND identity_id=? AND revoked_unix IS NULL`, time.Now().Unix(), tenant, target)
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

func recordAccess(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext, operation, target, kind, reason, fingerprint string, affected int64) error {
	result, err := tx.ExecContext(ctx, `INSERT INTO account_access_operations VALUES (?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, actor.StoreID, device.DeviceID, operation, actor.IdentityID, target, kind, reason, fingerprint, affected, time.Now().Unix())
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
	return nil
}
