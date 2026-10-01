package incoming

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb/identity"
)

func descriptorFixture(t *testing.T) (PublicDescriptor, ed25519.PrivateKey) {
	t.Helper()
	_, signing, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encryption, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := CreatePublicDescriptor(identity.DeviceContext{TenantID: "company", StoreID: "store", DeviceID: "station"}, 1, encryption, signing)
	if err != nil {
		t.Fatal(err)
	}
	return descriptor, signing
}

func TestPublicDescriptorRoundTripAndFingerprints(t *testing.T) {
	descriptor, _ := descriptorFixture(t)
	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	parsed, prints, err := ParsePublicDescriptor(raw)
	if err != nil || parsed.Binding.Device != descriptor.Binding.Device || len(prints.SigningSHA256) != 64 || len(prints.EncryptionSHA256) != 64 {
		t.Fatalf("invalid roundtrip: %v", err)
	}
	_, again, err := ParsePublicDescriptor(raw)
	if err != nil || again != prints {
		t.Fatal("unstable fingerprints")
	}
	if strings.Contains(string(raw), "private_key") {
		t.Fatal("private field exported")
	}
}

func TestPublicDescriptorRejectsTampering(t *testing.T) {
	for _, field := range []string{"revision", "device", "signing", "encryption", "signature"} {
		t.Run(field, func(t *testing.T) {
			descriptor, _ := descriptorFixture(t)
			switch field {
			case "revision":
				descriptor.Binding.Revision++
			case "device":
				descriptor.Binding.Device.StoreID = "another"
			case "signing":
				descriptor.SigningPublicKey[0] ^= 1
			case "encryption":
				descriptor.Binding.PublicKey[0] ^= 1
			case "signature":
				descriptor.Binding.Signature[0] ^= 1
			}
			raw, _ := json.Marshal(descriptor)
			if _, _, err := ParsePublicDescriptor(raw); err == nil {
				t.Fatal("tampering accepted")
			}
		})
	}
}

func TestPublicDescriptorRejectsAmbiguousAndLowOrderKeys(t *testing.T) {
	descriptor, signing := descriptorFixture(t)
	raw, _ := json.Marshal(descriptor)
	for _, invalid := range []string{
		string(raw) + " {}",
		strings.Replace(string(raw), `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(string(raw), `"version":1`, `"version":1,"extra":true`, 1),
		strings.Replace(string(raw), `"version":1`, `"version":null`, 1),
		strings.Replace(string(raw), `"Revision":1`, `"Revision":1,"Revision":1`, 1),
		strings.Replace(string(raw), `"TenantID":"company"`, `"TenantID":"company","TenantID":"company"`, 1),
		strings.Repeat(" ", 16*1024+1),
	} {
		if _, _, err := ParsePublicDescriptor([]byte(invalid)); err == nil {
			t.Fatal("ambiguous descriptor accepted")
		}
	}
	descriptor.Binding.PublicKey = make([]byte, 32)
	descriptor.Binding.Signature = ed25519.Sign(signing, bindingBytes(descriptor.Binding))
	raw, _ = json.Marshal(descriptor)
	if _, _, err := ParsePublicDescriptor(raw); err == nil {
		t.Fatal("signed low-order key accepted")
	}
}
