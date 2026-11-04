package security

import "titansystem-backend/internal/secretconfig"

// Private, disposable key generated per test process; never an installed key.
var jwtFixtureKey = newJWTFixtureKey()

func newJWTFixtureKey() string {
	key, err := secretconfig.GenerateJWTKey()
	if err != nil {
		panic("synthetic JWT fixture generation failed")
	}
	return key
}
