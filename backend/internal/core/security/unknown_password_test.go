package security

import (
	"golang.org/x/crypto/bcrypt"
	"testing"
)

func TestUnknownIdentityVerifierUsesNormalBcryptCost(t *testing.T) {
	cost, err := bcrypt.Cost(unknownIdentityVerifier)
	if err != nil || cost != bcrypt.DefaultCost {
		t.Fatal("unknown identity bypasses normal password cost")
	}
	if err = bcrypt.CompareHashAndPassword(unknownIdentityVerifier, []byte("incorrect-password")); err == nil {
		t.Fatal("dummy verifier accepted a credential")
	}
}
