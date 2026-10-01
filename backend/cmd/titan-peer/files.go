package main

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/incoming"
)

func createOwnerEncryption(ctx context.Context, db *sql.DB, actor identity.Scope, device identity.DeviceContext, key ed25519.PrivateKey, path string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := identity.CanOperateTx(ctx, tx, actor, device, identity.ManageStaff); err != nil {
		return err
	}
	var role string
	if err := tx.QueryRowContext(ctx, "SELECT role FROM memberships WHERE tenant_id=? AND identity_id=?", actor.TenantID, actor.IdentityID).Scan(&role); err != nil {
		return err
	}
	if role != "owner" {
		return identity.ErrDenied
	}
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_encryption_keys WHERE tenant_id=? AND store_id=? AND device_id=?", device.TenantID, device.StoreID, device.DeviceID).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return errPeerConfig
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Filesystem and SQLite cannot share an atomic commit. If later approval
	// fails, KEEP the private file for investigation; never overwrite it.
	encryption, err := incoming.CreateEncryptionKey(path, device)
	if err != nil {
		return err
	}
	binding, err := incoming.SignEncryptionBinding(device, 1, encryption, key)
	if err != nil {
		return err
	}
	_, err = incoming.TrustEncryptionBinding(ctx, db, actor, device, binding)
	return err
}

func writePublicDescriptor(path string, descriptor incoming.PublicDescriptor, output io.Writer) error {
	body, err := json.Marshal(descriptor)
	if err != nil {
		return err
	}
	_, fingerprints, err := incoming.ParsePublicDescriptor(body)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Descritor público exportado. Confira as impressões por canal autenticado antes de aprovar um parceiro.\nEd25519 SHA-256: %s\nX25519 SHA-256: %s\n", fingerprints.SigningSHA256, fingerprints.EncryptionSHA256)
	return err
}

func inspectPeer(args []string, output io.Writer) error {
	if len(args) != 2 || args[0] != "--in" || args[1] == "" {
		return errPeerConfig
	}
	info, err := os.Lstat(args[1])
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16*1024 {
		return errPeerConfig
	}
	file, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 16*1024+1))
	if err != nil {
		return err
	}
	descriptor, fingerprints, err := incoming.ParsePublicDescriptor(body)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Identidade declarada: empresa=%q loja=%q aparelho=%q revisão=%d\nEd25519 SHA-256: %s\nX25519 SHA-256: %s\nAssinatura própria válida; este resultado NÃO aprova o parceiro nem suas permissões.\n",
		descriptor.Binding.Device.TenantID, descriptor.Binding.Device.StoreID, descriptor.Binding.Device.DeviceID, descriptor.Binding.Revision, fingerprints.SigningSHA256, fingerprints.EncryptionSHA256)
	return err
}
