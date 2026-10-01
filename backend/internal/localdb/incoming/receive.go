package incoming

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/outgoing"
)

type Result struct {
	Receipt  outgoing.Receipt
	Repeated bool
}

// Receive commits event and stable signed receipt together. Receipt proves
// durable reception only: it does not prove application of sale or stock.
// receiver and its private key come from verified startup, never a request.
func Receive(ctx context.Context, db *sql.DB, receiver identity.DeviceContext, key ed25519.PrivateKey, message Message) (Result, error) {
	if db == nil || !validKey(key) {
		return Result{}, ErrInvalid
	}
	message.Event.Payload = append(json.RawMessage(nil), message.Event.Payload...)
	message.Signature = append([]byte(nil), message.Signature...)
	m, err := meta(message)
	if err != nil {
		return Result{}, err
	}
	if message.Destination != receiver || len(message.Signature) != ed25519.SignatureSize {
		return Result{}, ErrDenied
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	receiverKey, err := deviceKey(ctx, tx, receiver)
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(receiverKey, key.Public().(ed25519.PublicKey)) {
		return Result{}, ErrDenied
	}
	source := identity.DeviceContext{TenantID: m.TenantID, StoreID: m.StoreID, DeviceID: m.DeviceID}
	sourceKey, err := deviceKey(ctx, tx, source)
	if err != nil {
		return Result{}, err
	}
	var granted []byte
	err = tx.QueryRowContext(ctx, `SELECT p.sender_public_key FROM sync_incoming_peers p
		JOIN memberships u ON u.tenant_id=p.tenant_id AND u.identity_id=p.granted_by
		WHERE p.tenant_id=? AND p.store_id=? AND p.receiver_device_id=? AND p.sender_device_id=? AND p.event_type=?
		AND p.status='active' AND u.status='active' AND u.role='owner'`,
		receiver.TenantID, receiver.StoreID, receiver.DeviceID, source.DeviceID, m.EventType).Scan(&granted)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrDenied
	}
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(granted, sourceKey) || !ed25519.Verify(sourceKey, signingBytes(m), message.Signature) {
		return Result{}, ErrDenied
	}
	hash := messageHash(m, message.Signature)
	receipt := outgoing.Receipt{EventID: m.EventID, TenantID: m.TenantID, StoreID: m.StoreID, DeviceID: m.DeviceID, PayloadSHA256: m.PayloadSHA256}
	var priorHash string
	err = tx.QueryRowContext(ctx, `SELECT message_sha256,receipt_id,receipt_proof FROM incoming_events
		WHERE tenant_id=? AND store_id=? AND receiver_device_id=? AND sender_device_id=? AND event_id=?`,
		receiver.TenantID, receiver.StoreID, receiver.DeviceID, source.DeviceID, m.EventID).Scan(&priorHash, &receipt.ReceiptID, &receipt.Proof)
	if err == nil {
		if priorHash != hash || !ed25519.Verify(receiverKey, receiptBytes(receipt, receiver), receipt.Proof) {
			return Result{}, ErrConflict
		}
		if err = tx.Commit(); err != nil {
			return Result{}, err
		}
		return Result{Receipt: receipt, Repeated: true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, err
	}
	var used int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM incoming_events
		WHERE tenant_id=? AND store_id=? AND receiver_device_id=? AND sender_device_id=? AND operation_id=? AND event_type=?`,
		receiver.TenantID, receiver.StoreID, receiver.DeviceID, source.DeviceID, m.OperationID, m.EventType).Scan(&used); err != nil {
		return Result{}, err
	}
	if used != 0 {
		return Result{}, ErrConflict
	}
	receipt.ReceiptID, err = localdb.NewID()
	if err != nil {
		return Result{}, err
	}
	receipt.Proof = ed25519.Sign(key, receiptBytes(receipt, receiver))
	metadataJSON, err := json.Marshal(m)
	if err != nil {
		return Result{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO incoming_events VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,'received',?)`,
		m.TenantID, m.StoreID, receiver.DeviceID, source.DeviceID, m.EventID, m.OperationID, m.EventType, string(metadataJSON), string(message.Event.Payload),
		hash, m.PayloadSHA256, message.Signature, receipt.ReceiptID, receipt.Proof, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return Result{}, err
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{Receipt: receipt}, nil
}
