package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strings"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
)

var errConfiguration = errors.New("configuração local inválida ou indisponível")

type syncConfig struct {
	Version             int    `json:"version"`
	DestinationDeviceID string `json:"destination_device_id"`
	Endpoint            string `json:"endpoint"`
}

type syncStation struct {
	TenantID   string `json:"tenant_id"`
	StoreID    string `json:"store_id"`
	DeviceID   string `json:"device_id"`
	PrivateKey string `json:"private_key"`
}

func loadSyncConfig(path string) (syncConfig, error) {
	body, err := readSyncPrivate(path)
	if err != nil {
		return syncConfig{}, err
	}
	if err := exactSyncObject(body, []string{"version", "destination_device_id", "endpoint"}); err != nil {
		return syncConfig{}, err
	}
	var config syncConfig
	if err := json.Unmarshal(body, &config); err != nil || config.Version != 1 || !syncID(config.DestinationDeviceID) || len(config.Endpoint) > 2048 {
		return syncConfig{}, errConfiguration
	}
	u, err := url.Parse(config.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Path != incoming.SealedReceiverPath || u.RawPath != "" {
		return syncConfig{}, errConfiguration
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || ip == nil || !ip.IsLoopback() {
			return syncConfig{}, errConfiguration
		}
	}
	return config, nil
}

func loadSyncStation(path string) (syncStation, ed25519.PrivateKey, error) {
	body, err := readSyncPrivate(path)
	if err != nil {
		return syncStation{}, nil, err
	}
	if err := exactSyncObject(body, []string{"tenant_id", "store_id", "device_id", "private_key"}); err != nil {
		return syncStation{}, nil, err
	}
	var station syncStation
	if err := json.Unmarshal(body, &station); err != nil || !syncID(station.TenantID) || !syncID(station.StoreID) || !syncID(station.DeviceID) {
		return syncStation{}, nil, errConfiguration
	}
	key, err := base64.RawURLEncoding.DecodeString(station.PrivateKey)
	if err != nil || len(key) != ed25519.PrivateKeySize || !bytes.Equal(key, ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])) {
		return syncStation{}, nil, errConfiguration
	}
	return station, ed25519.PrivateKey(key), nil
}

func openSyncVerified(ctx context.Context, path string, station syncStation, key ed25519.PrivateKey) (*sql.DB, identity.DeviceContext, error) {
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

func syncID(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsRune(value, 0)
}

func readSyncPrivate(path string) ([]byte, error) {
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

func exactSyncObject(body []byte, fields []string) error {
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
