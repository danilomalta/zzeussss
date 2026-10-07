package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSessionTokenRejectsAlgorithmExpiryAndMalformedScope(t *testing.T) {
	for _, kind := range []string{"access", "refresh"} {
		for _, bad := range []string{"valid", "HS384", "HS512", "missing-exp", "expired", "future-iat", "numeric-sub", "numeric-tenant", "array-role", "whitespace", "wrong-type", "wrong-secret"} {
			t.Run(kind+"/"+bad, func(t *testing.T) {
				claims := jwt.MapClaims{"sub": "operator", "tenant_id": "company", "role": "owner", "type": kind, "exp": time.Now().Add(time.Minute).Unix()}
				method := jwt.SigningMethodHS256
				switch bad {
				case "HS384":
					method = jwt.SigningMethodHS384
				case "HS512":
					method = jwt.SigningMethodHS512
				case "missing-exp":
					delete(claims, "exp")
				case "expired":
					claims["exp"] = time.Now().Add(-time.Minute).Unix()
				case "future-iat":
					claims["iat"] = time.Now().Add(time.Hour).Unix()
				case "numeric-sub":
					claims["sub"] = 12
				case "numeric-tenant":
					claims["tenant_id"] = 12
				case "array-role":
					claims["role"] = []string{"owner"}
				case "whitespace":
					claims["tenant_id"] = " company "
				case "wrong-type":
					claims["type"] = "other"
				}
				key := "test-only-secret"
				if bad == "wrong-secret" {
					key = "different"
				}
				raw, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
				if err != nil {
					t.Fatal(err)
				}
				_, err = ParseSession(raw, "test-only-secret", kind)
				if (err == nil) != (bad == "valid") {
					t.Fatal("aceite incorreto")
				}
			})
		}
	}
}
