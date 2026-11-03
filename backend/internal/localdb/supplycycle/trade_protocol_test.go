package supplycycle

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTradeSignatureDomainCanonicalPayloadAndEveryFieldBound(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().UTC()
	m := TradeMessage{Version: 1, Kind: "order", ID: "request", Source: TradeParty{"buyer", "store", "device"}, Target: TradeParty{"supplier", "plant", "station"}, ActorID: "owner", CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339Nano), OrderID: "order", ProductID: "product", SKU: "SKU", ProductName: "Produto", Unit: "un", QuantityMilli: 5000, Reason: "Envio aprovado"}
	env, e := signTradeMessage(m, private)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := ParseTradeEnvelope(env, public); e != nil || got != m {
		t.Fatal(got, e)
	}
	if _, e := ParseTradeEnvelope(env, other); e == nil {
		t.Fatal("different signing identity accepted")
	}
	// Alter each non-signature byte without re-signing. Every covered byte must
	// reject; malformed JSON is also rejection, never a fallback interpretation.
	for i := range env.Payload {
		bad := TradeEnvelope{Payload: append([]byte(nil), env.Payload...), Signature: env.Signature}
		bad.Payload[i] ^= 1
		if _, e := ParseTradeEnvelope(bad, public); e == nil {
			t.Fatalf("unsigned alteration at %d", i)
		}
	}
	for _, raw := range []string{
		" " + string(env.Payload),
		strings.Replace(string(env.Payload), `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(string(env.Payload), `"version":1`, `"version":null`, 1),
		strings.Replace(string(env.Payload), `"version":1`, `"extra":1,"version":1`, 1),
		strings.Replace(string(env.Payload), `"quantity_milli":5000`, `"quantity_milli":5e3`, 1),
		strings.Replace(string(env.Payload), `"quantity_milli":5000`, `"quantity_milli":9007199254740992`, 1),
		string(env.Payload) + "{}",
	} {
		bad := TradeEnvelope{Payload: []byte(raw), Signature: ed25519.Sign(private, append([]byte(tradeDomain), []byte(raw)...))}
		if _, e := ParseTradeEnvelope(bad, public); e == nil {
			t.Fatal("ambiguous/noncanonical signed JSON accepted")
		}
	}
	raw, _ := json.Marshal(m)
	bad := TradeEnvelope{Payload: raw, Signature: ed25519.Sign(private, append([]byte("titan-device-pair-v1\x00"), raw...))}
	if _, e := ParseTradeEnvelope(bad, public); e == nil {
		t.Fatal("cross-protocol signature accepted")
	}
	if _, e := ParseTradeEnvelope(TradeEnvelope{Payload: make([]byte, TradeMaxPayload+1), Signature: env.Signature}, public); e == nil {
		t.Fatal("oversized message accepted")
	}
}
