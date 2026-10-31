package security

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestSessionRejectsTokensSignedWithPublicOrBlankSecret(t *testing.T) {
	for i, key := range []string{"", " \t", "super_secret_key_change_in_prod", "titanpass"} {
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "operator", "tenant_id": "company", "role": "owner", "type": "access", "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(key))
		if err != nil {
			t.Fatal("test token could not be built")
		}
		if _, err = ParseSession(raw, key, "access"); err == nil {
			t.Fatalf("unsafe signing case %d accepted", i)
		}
	}
}
