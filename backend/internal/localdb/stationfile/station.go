package stationfile

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

var errConfiguration = errors.New("arquivo ou aparelho local invalido")

type Station struct {
	TenantID   string `json:"tenant_id"`
	StoreID    string `json:"store_id"`
	DeviceID   string `json:"device_id"`
	PrivateKey string `json:"private_key"`
}

// OpenVerified opens an EXISTING private database and proves the approved station.
func OpenVerified(ctx context.Context, dbPath, stationPath string) (*sql.DB, identity.DeviceContext, ed25519.PrivateKey, error) {
	station, key, err := loadStation(stationPath)
	if err != nil {
		return nil, identity.DeviceContext{}, nil, err
	}
	db, device, err := openVerified(ctx, dbPath, station, key)
	if err != nil {
		return nil, identity.DeviceContext{}, nil, err
	}
	return db, device, key, nil
}

func loadStation(path string) (Station, ed25519.PrivateKey, error) {
	body, err := readPrivate(path)
	if err != nil {
		return Station{}, nil, err
	}
	if err := exactObject(body, []string{"tenant_id", "store_id", "device_id", "private_key"}); err != nil {
		return Station{}, nil, err
	}
	var station Station
	if err := json.Unmarshal(body, &station); err != nil || !validID(station.TenantID) || !validID(station.StoreID) || !validID(station.DeviceID) {
		return Station{}, nil, errConfiguration
	}
	key, err := base64.RawURLEncoding.DecodeString(station.PrivateKey)
	if err != nil || len(key) != ed25519.PrivateKeySize || !bytes.Equal(key, ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])) {
		return Station{}, nil, errConfiguration
	}
	return station, ed25519.PrivateKey(key), nil
}

func openVerified(ctx context.Context, path string, station Station, key ed25519.PrivateKey) (*sql.DB, identity.DeviceContext, error) {
	if ctx == nil || len(key) != ed25519.PrivateKeySize {
		return nil, identity.DeviceContext{}, errConfiguration
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, identity.DeviceContext{}, errConfiguration
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		return nil, identity.DeviceContext{}, err
	}
	device := identity.DeviceContext{TenantID: station.TenantID, StoreID: station.StoreID, DeviceID: station.DeviceID}
	challenge, err := identity.IssueDeviceChallenge(ctx, db, device)
	if err == nil {
		device, err = identity.CompleteDeviceChallenge(ctx, db, device, challenge.ID, ed25519.Sign(key, identity.DeviceAuthMessage(device, challenge)))
	}
	if err != nil {
		_ = db.Close()
		return nil, identity.DeviceContext{}, err
	}
	return db, device, nil
}

func validID(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsRune(value, 0)
}

func readPrivate(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 16*1024 {
		return nil, errConfiguration
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errConfiguration
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0600 {
		return nil, errConfiguration
	}
	body, err := io.ReadAll(io.LimitReader(file, 16*1024+1))
	if err != nil || len(body) > 16*1024 {
		return nil, errConfiguration
	}
	return body, nil
}

func exactObject(body []byte, fields []string) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return errConfiguration
	}
	allowed := make(map[string]bool)
	for _, field := range fields {
		allowed[field] = true
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || !allowed[name] || seen[name] {
			return errConfiguration
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errConfiguration
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || len(seen) != len(fields) {
		return errConfiguration
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errConfiguration
	}
	return nil
}
