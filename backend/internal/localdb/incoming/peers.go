package incoming

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

func ownerTx(ctx context.Context, tx *sql.Tx, actor identity.Scope, receiver identity.DeviceContext) error {
	if err := identity.CanOperateTx(ctx, tx, actor, receiver, identity.ManageStaff); err != nil {
		return err
	}
	var role string
	if err := tx.QueryRowContext(ctx, "SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?", actor.TenantID, actor.IdentityID).Scan(&role); err != nil {
		return err
	}
	if role != "owner" {
		return ErrDenied
	}
	return nil
}

func deviceKey(ctx context.Context, tx *sql.Tx, device identity.DeviceContext) (ed25519.PublicKey, error) {
	var key []byte
	err := tx.QueryRowContext(ctx, `SELECT public_key FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=? AND status='approved'`,
		device.TenantID, device.StoreID, device.DeviceID).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDenied
	}
	if err != nil {
		return nil, err
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, ErrDenied
	}
	return ed25519.PublicKey(key), nil
}

func peerScope(receiver, sender identity.DeviceContext, eventType string) bool {
	return validID(receiver.TenantID) && validID(receiver.StoreID) && validID(receiver.DeviceID) && validID(sender.DeviceID) &&
		receiver.TenantID == sender.TenantID && receiver.StoreID == sender.StoreID && receiver.DeviceID != sender.DeviceID && knownType(eventType)
}

// Grant is an explicit owner decision for one source, destination and event
// type. Public keys come from approved pairing, never from the received message.
// The caller must have proved the human session and receiver device.
func Grant(ctx context.Context, db *sql.DB, actor identity.Scope, receiver, sender identity.DeviceContext, eventType string) error {
	if db == nil || !peerScope(receiver, sender, eventType) {
		return ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = ownerTx(ctx, tx, actor, receiver); err != nil {
		return err
	}
	key, err := deviceKey(ctx, tx, sender)
	if err != nil {
		return err
	}
	var previous []byte
	var status, issuer string
	err = tx.QueryRowContext(ctx, `SELECT sender_public_key,status,granted_by FROM sync_incoming_peers
		WHERE tenant_id=? AND store_id=? AND receiver_device_id=? AND sender_device_id=? AND event_type=?`,
		receiver.TenantID, receiver.StoreID, receiver.DeviceID, sender.DeviceID, eventType).Scan(&previous, &status, &issuer)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && status == "active" && issuer == actor.IdentityID && bytes.Equal(previous, key) {
		return tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_incoming_peers VALUES (?,?,?,?,?,?,'active',?)
		ON CONFLICT(tenant_id,store_id,receiver_device_id,sender_device_id,event_type)
		DO UPDATE SET sender_public_key=excluded.sender_public_key,status='active',granted_by=excluded.granted_by`,
		receiver.TenantID, receiver.StoreID, receiver.DeviceID, sender.DeviceID, eventType, []byte(key), actor.IdentityID)
	if err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, receiver, sender, eventType, "grant"); err != nil {
		return err
	}
	return tx.Commit()
}

func Revoke(ctx context.Context, db *sql.DB, actor identity.Scope, receiver, sender identity.DeviceContext, eventType string) error {
	if db == nil || !peerScope(receiver, sender, eventType) {
		return ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = ownerTx(ctx, tx, actor, receiver); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE sync_incoming_peers SET status='revoked'
		WHERE tenant_id=? AND store_id=? AND receiver_device_id=? AND sender_device_id=? AND event_type=? AND status='active'`,
		receiver.TenantID, receiver.StoreID, receiver.DeviceID, sender.DeviceID, eventType)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 0 {
		if err = audit(ctx, tx, actor, receiver, sender, eventType, "revoke"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func audit(ctx context.Context, tx *sql.Tx, actor identity.Scope, receiver, sender identity.DeviceContext, eventType, action string) error {
	id, err := localdb.NewID()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_peer_audit VALUES (?,?,?,?,?,?,?,?,?)`,
		id, receiver.TenantID, receiver.StoreID, receiver.DeviceID, sender.DeviceID, eventType, action, actor.IdentityID, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
