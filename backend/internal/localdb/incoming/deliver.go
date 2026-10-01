package incoming

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/outgoing"
)

type DeliveryResult struct {
	Empty     bool
	EventID   string
	ReceiptID string
	Repeated  bool
}

type deliveryTrust struct {
	signing    ed25519.PublicKey
	encryption *ecdh.PublicKey
	revision   int64
	approvedBy string
}

// DeliverPendingOnce sends at most one existing committed event to one
// authoritative recipient. It performs no loop, listener or business mutation.
// Device, signing key, destination and endpoint are trusted startup config.
func DeliverPendingOnce(ctx context.Context, db *sql.DB, source identity.DeviceContext, key ed25519.PrivateKey, destination identity.DeviceContext, endpoint string, client *http.Client) (DeliveryResult, error) {
	if ctx == nil || db == nil || !validKey(key) {
		return DeliveryResult{}, ErrInvalid
	}
	// Release the read transaction BEFORE any network call.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return DeliveryResult{}, err
	}
	trust, err := deliveryTrustTx(ctx, tx, source, key, destination)
	if err != nil {
		_ = tx.Rollback()
		return DeliveryResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeliveryResult{}, err
	}
	events, err := outgoing.Pending(ctx, db, source, 1)
	if err != nil {
		return DeliveryResult{}, err
	}
	if len(events) == 0 {
		return DeliveryResult{Empty: true}, nil
	}
	event := events[0]
	message, err := Sign(event, destination, key)
	if err != nil {
		return DeliveryResult{}, err
	}
	envelope, err := Seal(message, trust.encryption)
	if err != nil {
		return DeliveryResult{}, err
	}
	verifier, err := NewReceiptVerifier(destination, trust.signing)
	if err != nil {
		return DeliveryResult{}, err
	}
	result, err := PostSealedHTTP(ctx, client, endpoint, envelope, event, verifier)
	if err != nil {
		// A cancelled network request must not erase the pending event. Persist
		// failure bookkeeping with its own short bounded context when possible.
		record, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if failed := outgoing.FailedAttempt(record, db, source, event.EventID); failed != nil {
			return DeliveryResult{}, errors.Join(err, failed)
		}
		return DeliveryResult{}, err
	}
	if err := confirmDelivery(ctx, db, source, key, destination, trust, result.Receipt); err != nil {
		return DeliveryResult{}, err
	}
	return DeliveryResult{EventID: event.EventID, ReceiptID: result.Receipt.ReceiptID, Repeated: result.Repeated}, nil
}

// All current trust checks and acknowledgement writes share this transaction.
// No call to a helper that starts a second transaction is made inside it.
func confirmDelivery(ctx context.Context, db *sql.DB, source identity.DeviceContext, key ed25519.PrivateKey, destination identity.DeviceContext, expected deliveryTrust, receipt outgoing.Receipt) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := deliveryTrustTx(ctx, tx, source, key, destination)
	if err != nil {
		return err
	}
	if current.revision != expected.revision || current.approvedBy != expected.approvedBy ||
		!bytes.Equal(current.signing, expected.signing) || !bytes.Equal(current.encryption.Bytes(), expected.encryption.Bytes()) {
		return ErrDenied
	}
	var event outgoing.Event
	var payload, status string
	err = tx.QueryRowContext(ctx, `SELECT event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,status
		FROM outbox WHERE event_id=? AND tenant_id=? AND store_id=? AND device_id=?`,
		receipt.EventID, source.TenantID, source.StoreID, source.DeviceID).Scan(
		&event.EventID, &event.TenantID, &event.StoreID, &event.DeviceID, &event.OperationID, &event.AggregateID, &event.EventType, &event.SchemaVersion, &payload, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return outgoing.ErrNotFound
	}
	if err != nil {
		return err
	}
	event.Payload = json.RawMessage(payload)
	verifier, err := NewReceiptVerifier(destination, current.signing)
	if err != nil {
		return err
	}
	if err := verifier.Verify(ctx, event, receipt); err != nil {
		return err
	}
	if status == "acked" {
		var priorID, priorHash string
		if err := tx.QueryRowContext(ctx, "SELECT receipt_id,payload_sha256 FROM outbox_receipts WHERE event_id=?", receipt.EventID).Scan(&priorID, &priorHash); err != nil {
			return err
		}
		if priorID != receipt.ReceiptID || priorHash != receipt.PayloadSHA256 {
			return ErrConflict
		}
		return tx.Commit()
	}
	if status != "pending" {
		return ErrConflict
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox_receipts (event_id,tenant_id,store_id,device_id,receipt_id,payload_sha256,confirmed_at) VALUES (?,?,?,?,?,?,?)`,
		receipt.EventID, source.TenantID, source.StoreID, source.DeviceID, receipt.ReceiptID, receipt.PayloadSHA256, now)
	if err != nil {
		return err
	}
	updated, err := tx.ExecContext(ctx, `UPDATE outbox SET status='acked',acked_at=? WHERE event_id=? AND tenant_id=? AND store_id=? AND device_id=? AND status='pending'`,
		now, receipt.EventID, source.TenantID, source.StoreID, source.DeviceID)
	if err != nil {
		return err
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	return tx.Commit()
}

// The signed binding, approver, both pairings and local private-key match are
// read from one snapshot. Reused before sending and during final commit.
func deliveryTrustTx(ctx context.Context, tx *sql.Tx, source identity.DeviceContext, key ed25519.PrivateKey, destination identity.DeviceContext) (deliveryTrust, error) {
	if source.TenantID != destination.TenantID || source.StoreID != destination.StoreID || source.DeviceID == destination.DeviceID {
		return deliveryTrust{}, ErrDenied
	}
	for _, id := range []string{source.TenantID, source.StoreID, source.DeviceID, destination.DeviceID} {
		if !validID(id) {
			return deliveryTrust{}, ErrInvalid
		}
	}
	public, err := deviceKey(ctx, tx, source)
	if err != nil {
		return deliveryTrust{}, err
	}
	if !validKey(key) || !bytes.Equal(public, key.Public().(ed25519.PublicKey)) {
		return deliveryTrust{}, ErrDenied
	}
	signing, err := deviceKey(ctx, tx, destination)
	if err != nil {
		return deliveryTrust{}, err
	}
	binding := EncryptionBinding{Version: 1, Device: destination}
	var original []byte
	var approvedBy string
	err = tx.QueryRowContext(ctx, `SELECT k.revision,k.public_key,k.signing_public_key,k.signature,k.approved_by FROM device_encryption_keys k
		JOIN memberships m ON m.tenant_id=k.tenant_id AND m.identity_id=k.approved_by
		WHERE k.tenant_id=? AND k.store_id=? AND k.device_id=? AND m.status='active' AND m.role='owner'`,
		destination.TenantID, destination.StoreID, destination.DeviceID).Scan(&binding.Revision, &binding.PublicKey, &original, &binding.Signature, &approvedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return deliveryTrust{}, ErrDenied
	}
	if err != nil {
		return deliveryTrust{}, err
	}
	if binding.Revision < 1 || !bytes.Equal(original, signing) || !ed25519.Verify(signing, bindingBytes(binding), binding.Signature) {
		return deliveryTrust{}, ErrDenied
	}
	encryption, err := ecdh.X25519().NewPublicKey(binding.PublicKey)
	if err != nil {
		return deliveryTrust{}, ErrInvalid
	}
	return deliveryTrust{signing: signing, encryption: encryption, revision: binding.Revision, approvedBy: approvedBy}, nil
}
