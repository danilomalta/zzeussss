package incoming

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"titansystem-backend/internal/localdb/identity"
)

type encryptionFile struct {
	Version    int                    `json:"version"`
	Device     identity.DeviceContext `json:"device"`
	PrivateKey []byte                 `json:"private_key"`
}

// CreateEncryptionKey creates a separate private file once; it never updates
// station.json or replaces an existing key. The caller proves this device.
func CreateEncryptionKey(path string, device identity.DeviceContext) (*ecdh.PrivateKey, error) {
	if path == "" || !validID(device.TenantID) || !validID(device.StoreID) || !validID(device.DeviceID) {
		return nil, ErrInvalid
	}
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := json.Marshal(encryptionFile{1, device, key.Bytes()})
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(body); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

// LoadEncryptionKey checks a regular 0600 file and its device binding.
// Do not regenerate a key when loading fails: investigate/recover the file.
func LoadEncryptionKey(path string, expected identity.DeviceContext) (*ecdh.PrivateKey, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 4096 {
		return nil, ErrInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0600 {
		return nil, ErrInvalid
	}
	body, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(body) > 4096 {
		return nil, ErrInvalid
	}
	// Exact outer fields; device scope is checked against trusted startup.
	fields, err := exactKeyObject(body, []string{"version", "device", "private_key"})
	if err != nil {
		return nil, err
	}
	if _, err := exactKeyObject(fields["device"], []string{"TenantID", "StoreID", "DeviceID"}); err != nil {
		return nil, err
	}
	var record encryptionFile
	if err := json.Unmarshal(body, &record); err != nil {
		return nil, ErrInvalid
	}
	if record.Version != 1 || record.Device != expected || len(record.PrivateKey) != 32 {
		return nil, ErrInvalid
	}
	if !validID(expected.TenantID) || !validID(expected.StoreID) || !validID(expected.DeviceID) {
		return nil, ErrInvalid
	}
	return ecdh.X25519().NewPrivateKey(record.PrivateKey)
}
