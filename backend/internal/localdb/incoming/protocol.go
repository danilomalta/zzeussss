// Package incoming stores authenticated events without applying business data.
// This first protocol is local-only: it has no HTTP route or cloud transport.
package incoming

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/outgoing"
)

var (
	ErrInvalid  = errors.New("mensagem local invalida")
	ErrDenied   = errors.New("comunicacao local nao autorizada")
	ErrConflict = errors.New("evento ou operacao recebido com dados divergentes")
)

type Message struct {
	Event       outgoing.Event
	Destination identity.DeviceContext
	Signature   []byte
}

type metadata struct {
	Version       int
	EventID       string
	TenantID      string
	StoreID       string
	DeviceID      string
	OperationID   string
	AggregateID   string
	EventType     string
	SchemaVersion int64
	PayloadSHA256 string
	Destination   identity.DeviceContext
}

func validID(id string) bool {
	return id != "" && len(id) <= 128 && strings.TrimSpace(id) == id && !strings.ContainsRune(id, 0)
}

func knownType(eventType string) bool {
	if eventType == "sale.cancelled" {
		return true
	}
	switch eventType {
	case "sale.committed", "stock.operation", "cash.open", "cash.close":
		return true
	}
	return false
}

func validKey(key ed25519.PrivateKey) bool {
	return len(key) == ed25519.PrivateKeySize && bytes.Equal(key, ed25519.NewKeyFromSeed(key[:ed25519.SeedSize]))
}

func meta(message Message) (metadata, error) {
	e, d := message.Event, message.Destination
	for _, id := range []string{e.EventID, e.TenantID, e.StoreID, e.DeviceID, e.OperationID, e.AggregateID, d.TenantID, d.StoreID, d.DeviceID} {
		if !validID(id) {
			return metadata{}, ErrInvalid
		}
	}
	if e.TenantID != d.TenantID || e.StoreID != d.StoreID || e.DeviceID == d.DeviceID {
		return metadata{}, ErrDenied
	}
	if !knownType(e.EventType) || e.SchemaVersion != 1 || len(e.Payload) == 0 || len(e.Payload) > 128*1024 || !json.Valid(e.Payload) {
		return metadata{}, ErrInvalid
	}
	return metadata{1, e.EventID, e.TenantID, e.StoreID, e.DeviceID, e.OperationID, e.AggregateID, e.EventType, e.SchemaVersion, e.PayloadSHA256(), d}, nil
}

func signingBytes(m metadata) []byte {
	body, _ := json.Marshal(m)
	return append([]byte("TitanSystem.local-event/v1\x00"), body...)
}

// Sign uses the sender's existing device key, never a licensing issuer key.
// Attempts are retry bookkeeping and are not part of business event identity.
func Sign(event outgoing.Event, destination identity.DeviceContext, key ed25519.PrivateKey) (Message, error) {
	message := Message{Event: event, Destination: destination}
	message.Event.Payload = append(json.RawMessage(nil), event.Payload...)
	if !validKey(key) {
		return Message{}, ErrInvalid
	}
	m, err := meta(message)
	if err != nil {
		return Message{}, err
	}
	message.Signature = ed25519.Sign(key, signingBytes(m))
	return message, nil
}

type receiptMetadata struct {
	ReceiptID     string
	EventID       string
	TenantID      string
	StoreID       string
	DeviceID      string
	PayloadSHA256 string
	Destination   identity.DeviceContext
}

func receiptBytes(receipt outgoing.Receipt, destination identity.DeviceContext) []byte {
	body, _ := json.Marshal(receiptMetadata{receipt.ReceiptID, receipt.EventID, receipt.TenantID, receipt.StoreID, receipt.DeviceID, receipt.PayloadSHA256, destination})
	return append([]byte("TitanSystem.local-receipt/v1\x00"), body...)
}

type ReceiptVerifier struct {
	destination identity.DeviceContext
	key         ed25519.PublicKey
}

// Trust is configured by the caller after authorized device pairing.
func NewReceiptVerifier(destination identity.DeviceContext, key ed25519.PublicKey) (*ReceiptVerifier, error) {
	if !validID(destination.TenantID) || !validID(destination.StoreID) || !validID(destination.DeviceID) || len(key) != ed25519.PublicKeySize {
		return nil, ErrInvalid
	}
	return &ReceiptVerifier{destination: destination, key: append(ed25519.PublicKey(nil), key...)}, nil
}

func (v *ReceiptVerifier) Verify(_ context.Context, event outgoing.Event, receipt outgoing.Receipt) error {
	if v == nil || !validID(receipt.ReceiptID) || len(receipt.Proof) != ed25519.SignatureSize ||
		event.TenantID != v.destination.TenantID || event.StoreID != v.destination.StoreID || event.DeviceID == v.destination.DeviceID ||
		receipt.EventID != event.EventID || receipt.TenantID != event.TenantID || receipt.StoreID != event.StoreID ||
		receipt.DeviceID != event.DeviceID || receipt.PayloadSHA256 != event.PayloadSHA256() {
		return ErrDenied
	}
	if !ed25519.Verify(v.key, receiptBytes(receipt, v.destination), receipt.Proof) {
		return ErrDenied
	}
	return nil
}

func messageHash(m metadata, signature []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(append(signingBytes(m), signature...)))
}
