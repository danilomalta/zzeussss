package incoming

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"titansystem-backend/internal/localdb/identity"
)

func publicPairRequest(t *testing.T) (PairChallenge, ed25519.PrivateKey) {
	t.Helper()
	target, targetKey := descriptorFixture(t)
	_, requesterKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	requester := identity.DeviceContext{TenantID: "company", StoreID: "store", DeviceID: "controller"}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	request, err := SignPairChallenge(requester, requesterKey, target, identity.OwnedPairingChallenge{Bytes: nonce, ExpiresUnix: time.Now().Unix() + 300})
	if err != nil {
		t.Fatal(err)
	}
	return request, targetKey
}
func TestPairExchangePreservesSignedChallengeAndProof(t *testing.T) {
	request, key := publicPairRequest(t)
	raw, _ := json.Marshal(request)
	parsed, err := ParsePairChallenge(raw)
	if err != nil || len(PairRequesterFingerprint(parsed)) != 64 {
		t.Fatal(err)
	}
	target := request.Target.Binding.Device
	answer := PairAnswer{Version: 1, Request: request, Proof: ed25519.Sign(key, identity.PairingMessage(target.TenantID, target.StoreID, target.DeviceID, request.Challenge))}
	raw, _ = json.Marshal(answer)
	if _, err := ParsePairAnswer(raw); err != nil {
		t.Fatal(err)
	}
	answer.Proof[0] ^= 1
	raw, _ = json.Marshal(answer)
	if _, err := ParsePairAnswer(raw); err == nil {
		t.Fatal("altered proof accepted")
	}
}
func TestPairExchangeRejectsAmbiguityAndAlteredRequest(t *testing.T) {
	for _, field := range []string{"expiry", "challenge", "requester", "signature", "target"} {
		t.Run(field, func(t *testing.T) {
			request, _ := publicPairRequest(t)
			switch field {
			case "expiry":
				request.ExpiresUnix++
			case "challenge":
				request.Challenge[0] ^= 1
			case "requester":
				request.Requester.DeviceID = "another"
			case "signature":
				request.Signature[0] ^= 1
			case "target":
				request.Target.Binding.Revision++
			}
			raw, _ := json.Marshal(request)
			if _, err := ParsePairChallenge(raw); err == nil {
				t.Fatal("altered request accepted")
			}
		})
	}
	request, _ := publicPairRequest(t)
	raw, _ := json.Marshal(request)
	for _, invalid := range []string{string(raw) + " {}", strings.Replace(string(raw), `"version":1`, `"version":1,"version":1`, 1), strings.Replace(string(raw), `"version":1`, `"version":1,"extra":true`, 1), strings.Repeat(" ", 16*1024+1)} {
		if _, err := ParsePairChallenge([]byte(invalid)); err == nil {
			t.Fatal("ambiguous request accepted")
		}
	}
	if _, err := ParsePairAnswer([]byte(`{"version":1,"request":null,"proof":null}`)); err == nil {
		t.Fatal("null answer accepted")
	}
}
