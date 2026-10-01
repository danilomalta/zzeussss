package stationfile

import (
	"bytes"
	"crypto/ed25519"

	"titansystem-backend/internal/localdb/identity"
)

// SignPairingForStation does not open a database or approve the station.
// The caller must validate the signed request and requester fingerprint first.
func SignPairingForStation(path string, expected identity.DeviceContext, public ed25519.PublicKey, challenge []byte) ([]byte, error) {
	station, key, err := loadStation(path)
	if err != nil {
		return nil, err
	}
	if expected.TenantID != station.TenantID || expected.StoreID != station.StoreID || expected.DeviceID != station.DeviceID || len(challenge) != 32 || !bytes.Equal(public, key.Public().(ed25519.PublicKey)) {
		return nil, errConfiguration
	}
	return ed25519.Sign(key, identity.PairingMessage(expected.TenantID, expected.StoreID, expected.DeviceID, challenge)), nil
}
