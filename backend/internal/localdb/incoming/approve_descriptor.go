package incoming

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

// The actor must come from a validated human session. Fingerprints must have
// been checked over an authenticated channel independent from the descriptor.
// This function never creates or approves device pairings.
func ApprovePublicPeer(ctx context.Context, db *sql.DB, actor identity.Scope, local identity.DeviceContext, raw []byte, signingSHA256, encryptionSHA256 string, eventTypes []string) error {
	descriptor, prints, events, err := checkedPeerDescriptor(local, raw, signingSHA256, encryptionSHA256, eventTypes)
	if err != nil || db == nil {
		return ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ownerTx(ctx, tx, actor, local); err != nil {
		return err
	}
	peer := descriptor.Binding.Device
	signing, err := deviceKey(ctx, tx, peer)
	if err != nil {
		return err
	}
	if !bytes.Equal(signing, descriptor.SigningPublicKey) {
		return ErrDenied
	}
	b := descriptor.Binding
	var priorRevision int64
	var priorKey, priorSigning, priorSignature []byte
	var priorOwner string
	err = tx.QueryRowContext(ctx, "SELECT revision,public_key,signing_public_key,signature,approved_by FROM device_encryption_keys WHERE tenant_id=? AND store_id=? AND device_id=?", peer.TenantID, peer.StoreID, peer.DeviceID).Scan(&priorRevision, &priorKey, &priorSigning, &priorSignature, &priorOwner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// Rotation and revision upgrades require a separate workflow. A descriptor
	// cannot silently replace previously approved key material.
	missing := errors.Is(err, sql.ErrNoRows)
	if !missing && (priorRevision != b.Revision || !bytes.Equal(priorKey, b.PublicKey) || !bytes.Equal(priorSigning, signing) || !bytes.Equal(priorSignature, b.Signature)) {
		return ErrConflict
	}
	if missing || priorOwner != actor.IdentityID {
		_, err = tx.ExecContext(ctx, `INSERT INTO device_encryption_keys VALUES (?,?,?,?,?,?,?,?)
   ON CONFLICT(tenant_id,store_id,device_id) DO UPDATE SET approved_by=excluded.approved_by`, peer.TenantID, peer.StoreID, peer.DeviceID, b.Revision, b.PublicKey, []byte(signing), b.Signature, actor.IdentityID)
		if err != nil {
			return err
		}
		id, err := localdb.NewID()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO device_encryption_key_audit VALUES (?,?,?,?,?,?,?,?)", id, peer.TenantID, peer.StoreID, peer.DeviceID, b.Revision, prints.EncryptionSHA256, actor.IdentityID, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	for _, event := range events {
		var prior []byte
		var status, issuer string
		err := tx.QueryRowContext(ctx, "SELECT sender_public_key,status,granted_by FROM sync_incoming_peers WHERE tenant_id=? AND store_id=? AND receiver_device_id=? AND sender_device_id=? AND event_type=?", local.TenantID, local.StoreID, local.DeviceID, peer.DeviceID, event).Scan(&prior, &status, &issuer)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && status == "active" && issuer == actor.IdentityID && bytes.Equal(prior, signing) {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sync_incoming_peers VALUES (?,?,?,?,?,?,'active',?)
   ON CONFLICT(tenant_id,store_id,receiver_device_id,sender_device_id,event_type) DO UPDATE SET sender_public_key=excluded.sender_public_key,status='active',granted_by=excluded.granted_by`, local.TenantID, local.StoreID, local.DeviceID, peer.DeviceID, event, []byte(signing), actor.IdentityID); err != nil {
			return err
		}
		if err := audit(ctx, tx, actor, local, peer, event, "grant"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RevokePublicPeerEvents changes only selected incoming event permissions.
// Revoked pairing is allowed here so its stale grants can also be withdrawn.
func RevokePublicPeerEvents(ctx context.Context, db *sql.DB, actor identity.Scope, local identity.DeviceContext, raw []byte, signingSHA256, encryptionSHA256 string, eventTypes []string) error {
	descriptor, _, events, err := checkedPeerDescriptor(local, raw, signingSHA256, encryptionSHA256, eventTypes)
	if err != nil || db == nil || len(events) == 0 {
		return ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ownerTx(ctx, tx, actor, local); err != nil {
		return err
	}
	peer := descriptor.Binding.Device
	var signing []byte
	if err := tx.QueryRowContext(ctx, "SELECT public_key FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=?", peer.TenantID, peer.StoreID, peer.DeviceID).Scan(&signing); err != nil {
		return ErrDenied
	}
	if !bytes.Equal(signing, descriptor.SigningPublicKey) {
		return ErrDenied
	}
	for _, event := range events {
		result, err := tx.ExecContext(ctx, "UPDATE sync_incoming_peers SET status='revoked' WHERE tenant_id=? AND store_id=? AND receiver_device_id=? AND sender_device_id=? AND event_type=? AND status='active'", local.TenantID, local.StoreID, local.DeviceID, peer.DeviceID, event)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count > 0 {
			if err := audit(ctx, tx, actor, local, peer, event, "revoke"); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func checkedPeerDescriptor(local identity.DeviceContext, raw []byte, signingSHA256, encryptionSHA256 string, eventTypes []string) (PublicDescriptor, PublicFingerprints, []string, error) {
	descriptor, prints, err := ParsePublicDescriptor(raw)
	if err != nil || signingSHA256 != prints.SigningSHA256 || encryptionSHA256 != prints.EncryptionSHA256 ||
		!peerScope(local, descriptor.Binding.Device, "sale.committed") || len(eventTypes) > 4 {
		return PublicDescriptor{}, PublicFingerprints{}, nil, ErrDenied
	}
	events := append([]string(nil), eventTypes...)
	sort.Strings(events)
	for i, event := range events {
		if !knownType(event) || (i > 0 && events[i-1] == event) {
			return PublicDescriptor{}, PublicFingerprints{}, nil, ErrDenied
		}
	}
	return descriptor, prints, events, nil
}
