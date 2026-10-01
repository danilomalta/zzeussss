package incoming

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/outgoing"
)

const sealedLimit = 256 * 1024
const sealedProtocol = "TitanSystem.local-sealed/X25519-HKDF-SHA256-AES256GCM/v1"

// SealedEnvelope hides event and payload; destination and key fingerprint
// remain visible for routing. It does not configure trust in recipient keys.
type SealedEnvelope struct {
	Version            int                    `json:"version"`
	Destination        identity.DeviceContext `json:"destination"`
	RecipientKeySHA256 string                 `json:"recipient_key_sha256"`
	EphemeralPublic    []byte                 `json:"ephemeral_public"`
	Salt               []byte                 `json:"salt"`
	Nonce              []byte                 `json:"nonce"`
	Ciphertext         []byte                 `json:"ciphertext"`
}

type sealedHeader struct {
	Protocol           string
	Version            int
	Destination        identity.DeviceContext
	RecipientKeySHA256 string
	EphemeralPublic    []byte
	Salt               []byte
	Nonce              []byte
}

func sealedAAD(envelope SealedEnvelope) []byte {
	body, _ := json.Marshal(sealedHeader{sealedProtocol, envelope.Version, envelope.Destination,
		envelope.RecipientKeySHA256, envelope.EphemeralPublic, envelope.Salt, envelope.Nonce})
	return body
}

// Payload is []byte on the inner wire, preserving the exact signed JSON
// bytes. Marshaling json.RawMessage directly would compact whitespace.
type sealedContent struct {
	EventID       string `json:"event_id"`
	TenantID      string `json:"tenant_id"`
	StoreID       string `json:"store_id"`
	DeviceID      string `json:"device_id"`
	OperationID   string `json:"operation_id"`
	AggregateID   string `json:"aggregate_id"`
	EventType     string `json:"event_type"`
	SchemaVersion int64  `json:"schema_version"`
	Payload       []byte `json:"payload"`
	Signature     []byte `json:"signature"`
}

func encryptionKeyID(public *ecdh.PublicKey) string {
	return fmt.Sprintf("%x", sha256.Sum256(public.Bytes()))
}

func sealedAEAD(secret []byte, envelope SealedEnvelope) (cipher.AEAD, error) {
	key := make([]byte, 32)
	info := append([]byte(sealedProtocol+"\x00"), sealedAAD(envelope)...)
	if _, err := io.ReadFull(hkdf.New(sha256.New, secret, envelope.Salt, info), key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Seal requires a recipient X25519 public key obtained through a trusted,
// owner-authorized configuration, never one supplied by an untrusted relay.
// Signing with the existing Ed25519 device key occurs before this function.
func Seal(message Message, recipient *ecdh.PublicKey) (SealedEnvelope, error) {
	if recipient == nil || recipient.Curve() != ecdh.X25519() || len(message.Signature) != ed25519.SignatureSize {
		return SealedEnvelope{}, ErrInvalid
	}
	if _, err := meta(message); err != nil {
		return SealedEnvelope{}, err
	}
	e := message.Event
	plain, err := json.Marshal(sealedContent{e.EventID, e.TenantID, e.StoreID, e.DeviceID, e.OperationID, e.AggregateID, e.EventType, e.SchemaVersion, []byte(e.Payload), message.Signature})
	if err != nil || len(plain) > sealedLimit {
		return SealedEnvelope{}, ErrInvalid
	}
	ephemeral, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return SealedEnvelope{}, err
	}
	secret, err := ephemeral.ECDH(recipient)
	if err != nil {
		return SealedEnvelope{}, ErrInvalid
	}
	envelope := SealedEnvelope{Version: 1, Destination: message.Destination, RecipientKeySHA256: encryptionKeyID(recipient),
		EphemeralPublic: ephemeral.PublicKey().Bytes(), Salt: make([]byte, 32), Nonce: make([]byte, 12)}
	if _, err := io.ReadFull(rand.Reader, envelope.Salt); err != nil {
		return SealedEnvelope{}, err
	}
	if _, err := io.ReadFull(rand.Reader, envelope.Nonce); err != nil {
		return SealedEnvelope{}, err
	}
	aead, err := sealedAEAD(secret, envelope)
	if err != nil {
		return SealedEnvelope{}, err
	}
	envelope.Ciphertext = aead.Seal(nil, envelope.Nonce, plain, sealedAAD(envelope))
	return envelope, nil
}

// OpenSealed authenticates encryption, destination and exact signed bytes.
// Sender identity/authorization is checked by Receive, not by decryption alone.
func OpenSealed(envelope SealedEnvelope, expected identity.DeviceContext, recipient *ecdh.PrivateKey) (Message, error) {
	if recipient == nil || recipient.Curve() != ecdh.X25519() || envelope.Version != 1 || envelope.Destination != expected ||
		!validID(expected.TenantID) || !validID(expected.StoreID) || !validID(expected.DeviceID) ||
		len(envelope.EphemeralPublic) != 32 || len(envelope.Salt) != 32 || len(envelope.Nonce) != 12 ||
		len(envelope.Ciphertext) < 16 || len(envelope.Ciphertext) > sealedLimit+16 ||
		envelope.RecipientKeySHA256 != encryptionKeyID(recipient.PublicKey()) {
		return Message{}, ErrInvalid
	}
	public, err := ecdh.X25519().NewPublicKey(envelope.EphemeralPublic)
	if err != nil {
		return Message{}, ErrInvalid
	}
	secret, err := recipient.ECDH(public)
	if err != nil {
		return Message{}, ErrInvalid
	}
	aead, err := sealedAEAD(secret, envelope)
	if err != nil {
		return Message{}, err
	}
	plain, err := aead.Open(nil, envelope.Nonce, envelope.Ciphertext, sealedAAD(envelope))
	if err != nil {
		return Message{}, ErrDenied
	}
	content, err := decodeSealedContent(plain)
	if err != nil {
		return Message{}, err
	}
	message := Message{Destination: expected, Signature: content.Signature, Event: outgoing.Event{
		EventID: content.EventID, TenantID: content.TenantID, StoreID: content.StoreID, DeviceID: content.DeviceID,
		OperationID: content.OperationID, AggregateID: content.AggregateID, EventType: content.EventType,
		SchemaVersion: content.SchemaVersion, Payload: json.RawMessage(content.Payload)}}
	if _, err := meta(message); err != nil {
		return Message{}, err
	}
	if len(message.Signature) != ed25519.SignatureSize {
		return Message{}, ErrInvalid
	}
	return message, nil
}

// ReceiveSealed never falls back to a plaintext message on decryption failure.
// Both private keys are local trusted process configuration, not HTTP fields.
func ReceiveSealed(ctx context.Context, db *sql.DB, receiver identity.DeviceContext, signingKey ed25519.PrivateKey, encryptionKey *ecdh.PrivateKey, envelope SealedEnvelope) (Result, error) {
	message, err := OpenSealed(envelope, receiver, encryptionKey)
	if err != nil {
		return Result{}, err
	}
	return Receive(ctx, db, receiver, signingKey, message)
}

func decodeSealedContent(body []byte) (sealedContent, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return sealedContent{}, ErrInvalid
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || seen[name] {
			return sealedContent{}, ErrInvalid
		}
		switch name {
		case "event_id", "tenant_id", "store_id", "device_id", "operation_id", "aggregate_id", "event_type", "schema_version", "payload", "signature":
		default:
			return sealedContent{}, ErrInvalid
		}
		seen[name] = true
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return sealedContent{}, ErrInvalid
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(seen) != 10 {
		return sealedContent{}, ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return sealedContent{}, ErrInvalid
	}
	var result sealedContent
	if err := json.Unmarshal(body, &result); err != nil {
		return sealedContent{}, ErrInvalid
	}
	return result, nil
}
