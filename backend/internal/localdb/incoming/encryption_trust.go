package incoming

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

type EncryptionBinding struct {
	Version   int
	Device    identity.DeviceContext
	Revision  int64
	PublicKey []byte
	Signature []byte
}

func bindingBytes(binding EncryptionBinding) []byte {
	body, _ := json.Marshal(struct {
		Version   int
		Device    identity.DeviceContext
		Revision  int64
		PublicKey []byte
	}{
		binding.Version, binding.Device, binding.Revision, binding.PublicKey})
	return append([]byte("TitanSystem.encryption-binding/v1\x00"), body...)
}

func SignEncryptionBinding(device identity.DeviceContext, revision int64, encryption *ecdh.PrivateKey, signing ed25519.PrivateKey) (EncryptionBinding, error) {
	if !validID(device.TenantID) || !validID(device.StoreID) || !validID(device.DeviceID) || revision < 1 ||
		encryption == nil || encryption.Curve() != ecdh.X25519() || !validKey(signing) {
		return EncryptionBinding{}, ErrInvalid
	}
	binding := EncryptionBinding{Version: 1, Device: device, Revision: revision, PublicKey: encryption.PublicKey().Bytes()}
	binding.Signature = ed25519.Sign(signing, bindingBytes(binding))
	return binding, nil
}

// TrustEncryptionBinding requires explicit owner approval on a proven local
// device, and a signature by the recipient's already approved Ed25519 key.
func TrustEncryptionBinding(ctx context.Context, db *sql.DB, actor identity.Scope, local identity.DeviceContext, binding EncryptionBinding) (bool, error) {
	if db == nil || local.TenantID != binding.Device.TenantID || local.StoreID != binding.Device.StoreID ||
		!validID(binding.Device.DeviceID) || binding.Version != 1 || binding.Revision < 1 ||
		len(binding.PublicKey) != 32 || len(binding.Signature) != 64 {
		return false, ErrInvalid
	}
	public, err := ecdh.X25519().NewPublicKey(binding.PublicKey)
	if err != nil {
		return false, ErrInvalid
	}
	probe, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return false, err
	}
	if _, err := probe.ECDH(public); err != nil {
		return false, ErrInvalid
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := ownerTx(ctx, tx, actor, local); err != nil {
		return false, err
	}
	signing, err := deviceKey(ctx, tx, binding.Device)
	if err != nil {
		return false, err
	}
	if !ed25519.Verify(signing, bindingBytes(binding), binding.Signature) {
		return false, ErrDenied
	}
	var revision int64
	var prior, priorSigning, priorSignature []byte
	var approvedBy string
	err = tx.QueryRowContext(ctx, `SELECT revision,public_key,signing_public_key,signature,approved_by FROM device_encryption_keys WHERE tenant_id=? AND store_id=? AND device_id=?`,
		binding.Device.TenantID, binding.Device.StoreID, binding.Device.DeviceID).Scan(&revision, &prior, &priorSigning, &priorSignature, &approvedBy)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err == nil {
		if binding.Revision < revision {
			return false, ErrConflict
		}
		if binding.Revision == revision {
			if !bytes.Equal(prior, binding.PublicKey) || !bytes.Equal(priorSigning, signing) {
				return false, ErrConflict
			}
			if !ed25519.Verify(signing, bindingBytes(binding), priorSignature) {
				return false, ErrDenied
			}
			if approvedBy == actor.IdentityID {
				if err := tx.Commit(); err != nil {
					return false, err
				}
				return true, nil
			}
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO device_encryption_keys VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(tenant_id,store_id,device_id) DO UPDATE SET revision=excluded.revision,
		public_key=excluded.public_key,signing_public_key=excluded.signing_public_key,signature=excluded.signature,approved_by=excluded.approved_by`,
		binding.Device.TenantID, binding.Device.StoreID, binding.Device.DeviceID, binding.Revision, binding.PublicKey, []byte(signing), binding.Signature, actor.IdentityID)
	if err != nil {
		return false, err
	}
	auditID, err := localdb.NewID()
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO device_encryption_key_audit VALUES (?,?,?,?,?,?,?,?)`,
		auditID, binding.Device.TenantID, binding.Device.StoreID, binding.Device.DeviceID, binding.Revision,
		encryptionKeyID(public), actor.IdentityID, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return false, nil
}

// TrustedEncryptionPublic rechecks current pairing and persisted signature.
// No public key supplied by a relay or untrusted request is returned.
func TrustedEncryptionPublic(ctx context.Context, db *sql.DB, local, recipient identity.DeviceContext) (*ecdh.PublicKey, error) {
	if db == nil || local.TenantID != recipient.TenantID || local.StoreID != recipient.StoreID {
		return nil, ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := deviceKey(ctx, tx, local); err != nil {
		return nil, err
	}
	signing, err := deviceKey(ctx, tx, recipient)
	if err != nil {
		return nil, err
	}
	binding := EncryptionBinding{Version: 1, Device: recipient}
	var originalSigning []byte
	err = tx.QueryRowContext(ctx, `SELECT k.revision,k.public_key,k.signing_public_key,k.signature FROM device_encryption_keys k
		JOIN memberships m ON m.tenant_id=k.tenant_id AND m.identity_id=k.approved_by
		WHERE k.tenant_id=? AND k.store_id=? AND k.device_id=? AND m.status='active' AND m.role='owner'`,
		recipient.TenantID, recipient.StoreID, recipient.DeviceID).Scan(&binding.Revision, &binding.PublicKey, &originalSigning, &binding.Signature)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDenied
	}
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(originalSigning, signing) || !ed25519.Verify(signing, bindingBytes(binding), binding.Signature) {
		return nil, ErrDenied
	}
	public, err := ecdh.X25519().NewPublicKey(binding.PublicKey)
	if err != nil {
		return nil, ErrInvalid
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return public, nil
}

func exactKeyObject(body []byte, fields []string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, ErrInvalid
	}
	allowed := make(map[string]bool)
	for _, name := range fields {
		allowed[name] = true
	}
	values := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] {
			return nil, ErrInvalid
		}
		if _, duplicate := values[name]; duplicate {
			return nil, ErrInvalid
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrInvalid
		}
		values[name] = value
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(values) != len(fields) {
		return nil, ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return values, nil
}
