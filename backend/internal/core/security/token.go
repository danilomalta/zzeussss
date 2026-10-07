package security

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ParseSession accepts only the algorithm emitted by this API and requires expiry.
// It does not replace the live membership lookup or persist individual sessions.
func ParseSession(raw, secret, kind string) (jwt.MapClaims, error) {
	invalid := errors.New("sessão inválida")
	if secret == "" || len(raw) > 8192 || (kind != "access" && kind != "refresh") {
		return nil, invalid
	}
	token, err := jwt.Parse(raw, func(*jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || token == nil || !token.Valid {
		return nil, invalid
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != kind {
		return nil, invalid
	}
	for _, field := range []string{"sub", "tenant_id", "role"} {
		value, ok := claims[field].(string)
		if !ok || value == "" || value != strings.TrimSpace(value) || len(value) > 255 {
			return nil, invalid
		}
	}
	return claims, nil
}
