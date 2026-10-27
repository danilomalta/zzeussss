package supplycycle

// Trade messages use their own signature domain. They are NOT device-pairing
// grants, financial instructions or stock movements. No network address or
// credential is accepted in a signed business message.
import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/purchases"
)

const TradeMaxPayload = 8192
const tradeDomain = "titan-company-order-v1\x00"

type TradeParty struct {
	TenantID string `json:"tenant_id"`
	StoreID  string `json:"store_id"`
	DeviceID string `json:"device_id"`
}

func tradeParty(d identity.DeviceContext) TradeParty {
	return TradeParty{d.TenantID, d.StoreID, d.DeviceID}
}
func (p TradeParty) valid() bool { return key(p.TenantID) && key(p.StoreID) && key(p.DeviceID) }

// Payload is base64-encoded by JSON, preserving the exact signed bytes.
type TradeEnvelope struct {
	Payload   []byte `json:"payload"`
	Signature []byte `json:"signature"`
}
type TradeMessage struct {
	Version       int        `json:"version"`
	Kind          string     `json:"kind"`
	ID            string     `json:"id"`
	Source        TradeParty `json:"source"`
	Target        TradeParty `json:"target"`
	ActorID       string     `json:"actor_id"`
	CreatedAt     string     `json:"created_at"`
	ExpiresAt     string     `json:"expires_at"`
	OrderID       string     `json:"order_id"`
	OfferSHA256   string     `json:"offer_sha256"`
	ProductID     string     `json:"product_id"`
	SKU           string     `json:"sku"`
	ProductName   string     `json:"product_name"`
	Unit          string     `json:"unit"`
	QuantityMilli int64      `json:"quantity_milli"`
	Decision      string     `json:"decision"`
	AcceptedMilli int64      `json:"accepted_milli"`
	PromisedAt    string     `json:"promised_at"`
	Reason        string     `json:"reason"`
}
type TradePeerInput struct {
	OperationID           string     `json:"operation_id"`
	ID                    string     `json:"id"`
	Remote                TradeParty `json:"remote"`
	PublicKey             []byte     `json:"public_key"`
	FingerprintSHA256     string     `json:"fingerprint_sha256"`
	Role                  string     `json:"role"`
	SupplierID            string     `json:"supplier_id"`
	VerificationReference string     `json:"verification_reference"`
}
type TradePeer struct {
	ID                    string     `json:"id"`
	Remote                TradeParty `json:"remote"`
	PublicKey             []byte     `json:"public_key"`
	FingerprintSHA256     string     `json:"fingerprint_sha256"`
	Role                  string     `json:"role"`
	SupplierID            string     `json:"supplier_id"`
	VerificationReference string     `json:"verification_reference"`
	Status                string     `json:"status"`
	Revision              int64      `json:"revision"`
	ApprovedBy            string     `json:"approved_by"`
}
type TradeExportInput struct {
	OperationID string `json:"operation_id"`
	PeerID      string `json:"peer_id"`
	Reason      string `json:"reason"`
}
type TradeImportInput struct {
	OperationID string        `json:"operation_id"`
	PeerID      string        `json:"peer_id"`
	Envelope    TradeEnvelope `json:"envelope"`
}
type TradeDecisionInput struct {
	OperationID   string `json:"operation_id"`
	Decision      string `json:"decision"`
	AcceptedMilli int64  `json:"accepted_milli"`
	PromisedAt    string `json:"promised_at"`
	Reason        string `json:"reason"`
}
type TradeRevokeInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}
type TradeResult struct {
	Recovery    *TradeRecovery `json:"recovery,omitempty"`
	OperationID string         `json:"operation_id"`
	Repeated    bool           `json:"repeated"`
	Peer        *TradePeer     `json:"peer,omitempty"`
	Message     *TradeMessage  `json:"message,omitempty"`
	Envelope    *TradeEnvelope `json:"envelope,omitempty"`
	Cycle       *Result        `json:"cycle,omitempty"`
}

func TradeFingerprint(public []byte) string {
	h := sha256.Sum256(public)
	return hex.EncodeToString(h[:])
}
func TradeEnvelopeSHA256(e TradeEnvelope) string { b, _ := json.Marshal(e); return TradeFingerprint(b) }
func tradeHash(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}

// Reject noncanonical JSON as well as unknown/duplicate/missing/null fields.
// Canonical form is UTF-8 compact JSON using the field order above, all fields
// present, and Go encoding/json escaping. The signature covers domain+payload.
func ParseTradeEnvelope(e TradeEnvelope, public ed25519.PublicKey) (TradeMessage, error) {
	var m TradeMessage
	if len(e.Payload) == 0 || len(e.Payload) > TradeMaxPayload || len(e.Signature) != ed25519.SignatureSize || len(public) != ed25519.PublicKeySize {
		return m, purchases.ErrInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(e.Payload))
	dec.DisallowUnknownFields()
	if dec.Decode(&m) != nil {
		return m, purchases.ErrInvalid
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return m, purchases.ErrInvalid
	}
	canonical, _ := json.Marshal(m)
	if !bytes.Equal(canonical, e.Payload) {
		return m, purchases.ErrInvalid
	}
	if !ed25519.Verify(public, append([]byte(tradeDomain), e.Payload...), e.Signature) {
		return m, identity.ErrDenied
	}
	if m.Version != 1 || !key(m.ID) || !key(m.OrderID) || !key(m.ActorID) || !m.Source.valid() || !m.Target.valid() || m.Source.TenantID == m.Target.TenantID || !key(m.ProductID) || !text(m.SKU, 128, false) || !text(m.ProductName, 255, false) || !text(m.Unit, 32, false) || !text(m.Reason, 500, false) || m.QuantityMilli < 1 || m.QuantityMilli > purchases.MaxQuantity {
		return m, purchases.ErrInvalid
	}
	created, e1 := time.Parse(time.RFC3339Nano, m.CreatedAt)
	expires, e2 := time.Parse(time.RFC3339Nano, m.ExpiresAt)
	if e1 != nil || e2 != nil || created.Year() < 2000 || created.Year() > 2200 || !expires.After(created) || expires.Sub(created) > 7*24*time.Hour {
		return m, purchases.ErrInvalid
	}
	switch m.Kind {
	case "order":
		if m.OfferSHA256 != "" || m.Decision != "" || m.AcceptedMilli != 0 || m.PromisedAt != "" {
			return m, purchases.ErrInvalid
		}
	case "decision":
		if !tradeHash(m.OfferSHA256) || !validTradeDecision(m.Decision, m.AcceptedMilli, m.PromisedAt) || m.AcceptedMilli > m.QuantityMilli {
			return m, purchases.ErrInvalid
		}
	default:
		return m, purchases.ErrInvalid
	}
	return m, nil
}
func validTradeDecision(decision string, qty int64, promised string) bool {
	if decision == "declined" {
		return qty == 0 && promised == ""
	}
	t, e := time.Parse(time.RFC3339Nano, promised)
	return decision == "confirmed" && qty > 0 && qty <= purchases.MaxQuantity && e == nil && t.Year() >= 2000 && t.Year() <= 2200
}
func signTradeMessage(m TradeMessage, key ed25519.PrivateKey) (TradeEnvelope, error) {
	if len(key) != ed25519.PrivateKeySize {
		return TradeEnvelope{}, identity.ErrDenied
	}
	b, e := json.Marshal(m)
	if e != nil {
		return TradeEnvelope{}, e
	}
	out := TradeEnvelope{Payload: b, Signature: ed25519.Sign(key, append([]byte(tradeDomain), b...))}
	_, e = ParseTradeEnvelope(out, key.Public().(ed25519.PublicKey))
	return out, e
}
func tradeFresh(m TradeMessage) bool {
	created, _ := time.Parse(time.RFC3339Nano, m.CreatedAt)
	expires, _ := time.Parse(time.RFC3339Nano, m.ExpiresAt)
	now := time.Now()
	return !created.After(now.Add(5*time.Minute)) && expires.After(now)
}
