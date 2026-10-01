package incoming

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"titansystem-backend/internal/localdb/identity"
)

type PairChallenge struct {
	Version      int                    `json:"version"`
	Requester    identity.DeviceContext `json:"requester"`
	RequesterKey []byte                 `json:"requester_key"`
	Target       PublicDescriptor       `json:"target"`
	Challenge    []byte                 `json:"challenge"`
	ExpiresUnix  int64                  `json:"expires_unix"`
	Signature    []byte                 `json:"signature"`
}
type PairAnswer struct {
	Version int           `json:"version"`
	Request PairChallenge `json:"request"`
	Proof   []byte        `json:"proof"`
}

func pairRequestBytes(request PairChallenge) []byte {
	body, _ := json.Marshal(struct {
		Version      int
		Requester    identity.DeviceContext
		RequesterKey []byte
		Target       PublicDescriptor
		Challenge    []byte
		ExpiresUnix  int64
	}{request.Version, request.Requester, request.RequesterKey, request.Target, request.Challenge, request.ExpiresUnix})
	return append([]byte("TitanSystem.public-pair-request/v1\x00"), body...)
}
func SignPairChallenge(requester identity.DeviceContext, key ed25519.PrivateKey, target PublicDescriptor, challenge identity.OwnedPairingChallenge) (PairChallenge, error) {
	if !validKey(key) {
		return PairChallenge{}, ErrInvalid
	}
	request := PairChallenge{Version: 1, Requester: requester, RequesterKey: append([]byte(nil), key.Public().(ed25519.PublicKey)...), Target: target, Challenge: append([]byte(nil), challenge.Bytes...), ExpiresUnix: challenge.ExpiresUnix}
	request.Signature = ed25519.Sign(key, pairRequestBytes(request))
	raw, err := json.Marshal(request)
	if err != nil {
		return PairChallenge{}, err
	}
	if _, err := ParsePairChallenge(raw); err != nil {
		return PairChallenge{}, err
	}
	return request, nil
}
func ParsePairChallenge(raw []byte) (PairChallenge, error) {
	if len(raw) == 0 || len(raw) > 16*1024 {
		return PairChallenge{}, ErrInvalid
	}
	fields, err := exactKeyObject(raw, []string{"version", "requester", "requester_key", "target", "challenge", "expires_unix", "signature"})
	if err != nil {
		return PairChallenge{}, ErrInvalid
	}
	if _, err := exactKeyObject(fields["requester"], []string{"TenantID", "StoreID", "DeviceID"}); err != nil {
		return PairChallenge{}, ErrInvalid
	}
	if _, _, err := ParsePublicDescriptor(fields["target"]); err != nil {
		return PairChallenge{}, ErrInvalid
	}
	var request PairChallenge
	if err := json.Unmarshal(raw, &request); err != nil || request.Version != 1 || len(request.RequesterKey) != 32 || len(request.Challenge) != 32 || len(request.Signature) != 64 || request.ExpiresUnix <= 0 || !peerScope(request.Requester, request.Target.Binding.Device, "sale.committed") {
		return PairChallenge{}, ErrInvalid
	}
	if !ed25519.Verify(ed25519.PublicKey(request.RequesterKey), pairRequestBytes(request), request.Signature) {
		return PairChallenge{}, ErrDenied
	}
	return request, nil
}
func PairRequesterFingerprint(request PairChallenge) string {
	return fmt.Sprintf("%x", sha256.Sum256(request.RequesterKey))
}
func ParsePairAnswer(raw []byte) (PairAnswer, error) {
	if len(raw) == 0 || len(raw) > 16*1024 {
		return PairAnswer{}, ErrInvalid
	}
	fields, err := exactKeyObject(raw, []string{"version", "request", "proof"})
	if err != nil {
		return PairAnswer{}, ErrInvalid
	}
	request, err := ParsePairChallenge(fields["request"])
	if err != nil {
		return PairAnswer{}, err
	}
	var answer PairAnswer
	if err := json.Unmarshal(raw, &answer); err != nil || answer.Version != 1 || len(answer.Proof) != 64 {
		return PairAnswer{}, ErrInvalid
	}
	target := request.Target.Binding.Device
	if !ed25519.Verify(ed25519.PublicKey(request.Target.SigningPublicKey), identity.PairingMessage(target.TenantID, target.StoreID, target.DeviceID, request.Challenge), answer.Proof) {
		return PairAnswer{}, ErrDenied
	}
	return answer, nil
}
