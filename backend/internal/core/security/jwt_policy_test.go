package security

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSessionRejectsInadequateKeyAndOldSigningKeyWithoutDisclosure(t *testing.T) {
	claims := jwt.MapClaims{"sub": "operator", "tenant_id": "company", "role": "owner", "type": "access", "exp": time.Now().Add(time.Minute).Unix()}
	for i, key := range []string{"short", strings.Repeat("x", 64), strings.Repeat("A", 43), jwtFixtureKey + "="} {
		raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
		if err != nil {
			t.Fatal("fixture signing failed")
		}
		if _, err = ParseSession(raw, key, "access"); err == nil || strings.Contains(err.Error(), key) {
			t.Fatalf("inadequate key case %d accepted or disclosed", i)
		}
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtFixtureKey))
	if err != nil {
		t.Fatal("fixture signing failed")
	}
	if _, err = ParseSession(raw, jwtFixtureKey, "access"); err != nil {
		t.Fatal("synthetic strong key refused")
	}
	other := newJWTFixtureKey()
	if _, err = ParseSession(raw, other, "access"); err == nil || strings.Contains(err.Error(), jwtFixtureKey) || strings.Contains(err.Error(), other) {
		t.Fatal("old signature accepted or key disclosed after explicit key replacement")
	}
}
