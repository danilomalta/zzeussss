package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"strings"
	"time"
)

type OwnedPairingChallenge struct {
	Bytes       []byte
	ExpiresUnix int64
}

func pairingOwner(ctx context.Context, tx *sql.Tx, actor Scope, local DeviceContext) error {
	if err := CanOperateTx(ctx, tx, actor, local, ManageStaff); err != nil {
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

// Actor and local station must be authenticated by the caller. This creates
// only a pending request; proof and a second owner decision are still required.
func RequestOwnedPairing(ctx context.Context, db *sql.DB, actor Scope, local DeviceContext, deviceID, name string, public ed25519.PublicKey) (OwnedPairingChallenge, error) {
	if db == nil || deviceID == "" || deviceID == local.DeviceID || len(public) != ed25519.PublicKeySize || strings.TrimSpace(name) == "" {
		return OwnedPairingChallenge{}, ErrDenied
	}
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return OwnedPairingChallenge{}, err
	}
	now := time.Now().Unix()
	expires := now + int64(pairingTTL.Seconds())
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return OwnedPairingChallenge{}, err
	}
	defer tx.Rollback()
	if err := pairingOwner(ctx, tx, actor, local); err != nil {
		return OwnedPairingChallenge{}, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO devices VALUES (?,?,?,?)", local.TenantID, local.StoreID, deviceID, strings.TrimSpace(name)); err != nil {
		return OwnedPairingChallenge{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO device_pairings (tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by) VALUES (?,?,?,?,?,?,'pending',?)`, local.TenantID, local.StoreID, deviceID, []byte(public), challenge, expires, actor.IdentityID); err != nil {
		return OwnedPairingChallenge{}, err
	}
	if err := deviceEvent(ctx, tx, local.TenantID, deviceID, actor.IdentityID, "requested", now); err != nil {
		return OwnedPairingChallenge{}, err
	}
	if err := tx.Commit(); err != nil {
		return OwnedPairingChallenge{}, err
	}
	return OwnedPairingChallenge{Bytes: challenge, ExpiresUnix: expires}, nil
}

// CompleteOwnedPairing records proof and final owner approval atomically.
// It never replaces a key or resurrects revoked pairing.
func CompleteOwnedPairing(ctx context.Context, db *sql.DB, actor Scope, local DeviceContext, target DeviceContext, public ed25519.PublicKey, challenge, proof []byte, expiresUnix int64) error {
	if db == nil || target.TenantID != local.TenantID || target.StoreID != local.StoreID || target.DeviceID == local.DeviceID || len(public) != 32 || len(challenge) != 32 || len(proof) != 64 {
		return ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := pairingOwner(ctx, tx, actor, local); err != nil {
		return err
	}
	var storedKey, storedChallenge []byte
	var expiry int64
	var status, issuer string
	if err := tx.QueryRowContext(ctx, "SELECT public_key,challenge,challenge_expires_unix,status,requested_by FROM device_pairings WHERE tenant_id=? AND store_id=? AND device_id=?", target.TenantID, target.StoreID, target.DeviceID).Scan(&storedKey, &storedChallenge, &expiry, &status, &issuer); err != nil {
		return ErrDenied
	}
	if !bytes.Equal(storedKey, public) || !bytes.Equal(storedChallenge, challenge) || expiry != expiresUnix || !ed25519.Verify(public, PairingMessage(target.TenantID, target.StoreID, target.DeviceID, challenge), proof) {
		return ErrDenied
	}
	if status == "approved" {
		return tx.Commit()
	}
	now := time.Now().Unix()
	if (status != "pending" && status != "verified") || expiry <= now {
		return ErrDenied
	}
	var active int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM memberships WHERE tenant_id=? AND identity_id=? AND role='owner' AND status='active'", target.TenantID, issuer).Scan(&active); err != nil {
		return ErrDenied
	}
	if status == "pending" {
		if err := deviceEvent(ctx, tx, target.TenantID, target.DeviceID, target.DeviceID, "proved", now); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE device_pairings SET status='approved',proof_unix=COALESCE(proof_unix,?),approved_by=?,approved_unix=? WHERE tenant_id=? AND store_id=? AND device_id=?", now, actor.IdentityID, now, target.TenantID, target.StoreID, target.DeviceID); err != nil {
		return err
	}
	if err := deviceEvent(ctx, tx, target.TenantID, target.DeviceID, actor.IdentityID, "approved", now); err != nil {
		return err
	}
	return tx.Commit()
}
