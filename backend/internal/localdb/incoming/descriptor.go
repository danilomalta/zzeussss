package incoming

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"titansystem-backend/internal/localdb/identity"
)

// PublicDescriptor contains no private key. Its signature proves possession
// of the advertised signing key, NOT company membership or owner approval.
type PublicDescriptor struct {
	Version          int               `json:"version"`
	SigningPublicKey []byte            `json:"signing_public_key"`
	Binding          EncryptionBinding `json:"binding"`
}

type PublicFingerprints struct {
	SigningSHA256    string
	EncryptionSHA256 string
}

func CreatePublicDescriptor(device identity.DeviceContext, revision int64, encryption *ecdh.PrivateKey, signing ed25519.PrivateKey) (PublicDescriptor, error) {
	binding, err := SignEncryptionBinding(device, revision, encryption, signing)
	if err != nil {
		return PublicDescriptor{}, err
	}
	return PublicDescriptor{Version: 1, SigningPublicKey: append([]byte(nil), signing.Public().(ed25519.PublicKey)...), Binding: binding}, nil
}

// ParsePublicDescriptor validates structure and self-signature only. Import
// must separately verify fingerprints through an authenticated channel and
// require the local owner before changing any pairing or permissions.
func ParsePublicDescriptor(raw []byte) (PublicDescriptor, PublicFingerprints, error) {
	if len(raw) == 0 || len(raw) > 16*1024 {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	fields, err := exactKeyObject(raw, []string{"version", "signing_public_key", "binding"})
	if err != nil {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	inner, err := exactKeyObject(fields["binding"], []string{"Version", "Device", "Revision", "PublicKey", "Signature"})
	if err != nil {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	if _, err := exactKeyObject(inner["Device"], []string{"TenantID", "StoreID", "DeviceID"}); err != nil {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	var descriptor PublicDescriptor
	if err := json.Unmarshal(raw, &descriptor); err != nil {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	b := descriptor.Binding
	if descriptor.Version != 1 || b.Version != 1 || b.Revision < 1 || len(descriptor.SigningPublicKey) != 32 || len(b.PublicKey) != 32 || len(b.Signature) != 64 ||
		!validID(b.Device.TenantID) || !validID(b.Device.StoreID) || !validID(b.Device.DeviceID) {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	if !ed25519.Verify(ed25519.PublicKey(descriptor.SigningPublicKey), bindingBytes(b), b.Signature) {
		return PublicDescriptor{}, PublicFingerprints{}, ErrDenied
	}
	public, err := ecdh.X25519().NewPublicKey(b.PublicKey)
	if err != nil {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	probe, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return PublicDescriptor{}, PublicFingerprints{}, err
	}
	if _, err := probe.ECDH(public); err != nil {
		return PublicDescriptor{}, PublicFingerprints{}, ErrInvalid
	}
	fingerprints := PublicFingerprints{SigningSHA256: fmt.Sprintf("%x", sha256.Sum256(descriptor.SigningPublicKey)), EncryptionSHA256: encryptionKeyID(public)}
	return descriptor, fingerprints, nil
}
