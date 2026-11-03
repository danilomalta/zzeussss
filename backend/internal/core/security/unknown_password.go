package security

import (
	"crypto/rand"
	"golang.org/x/crypto/bcrypt"
)

// This random verifier belongs to no account and cannot authenticate anyone.
// Unknown identities still incur the same bcrypt cost as current local accounts.
var unknownIdentityVerifier = func() []byte {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		panic("local authentication verifier unavailable")
	}
	hash, err := bcrypt.GenerateFromPassword(random[:], bcrypt.DefaultCost)
	clear(random[:])
	if err != nil {
		panic("local authentication verifier unavailable")
	}
	return hash
}()

// RejectUnknownPassword performs password work but can never authenticate an account.
func RejectUnknownPassword(password string) {
	_ = bcrypt.CompareHashAndPassword(unknownIdentityVerifier, []byte(password))
}
