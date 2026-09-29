// Package localsetup cria a primeira empresa, loja, dono e aparelho em um
// arquivo SQLite novo. Não é uma rota pública e só deve ser chamada no setup.
package localsetup

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"strings"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/localauth"
)

var ErrAlreadyInitialized = errors.New("instalação local já inicializada")

type Input struct {
	TenantName string
	StoreName  string
	OwnerName  string
	DeviceName string
	Password   string
	PublicKey  ed25519.PublicKey
}

type Result struct {
	TenantID string `json:"tenant_id"`
	StoreID  string `json:"store_id"`
	OwnerID  string `json:"owner_id"`
	DeviceID string `json:"device_id"`
}

// Initialize exige base sem empresas. A primeira aprovação de aparelho é um
// evento de raiz auditável da instalação, não um pareamento remoto normal.
func Initialize(ctx context.Context, db *sql.DB, in Input) (Result, error) {
	if db == nil {
		return Result{}, errors.New("banco local indisponível")
	}
	in.TenantName = strings.TrimSpace(in.TenantName)
	in.StoreName = strings.TrimSpace(in.StoreName)
	in.OwnerName = strings.TrimSpace(in.OwnerName)
	in.DeviceName = strings.TrimSpace(in.DeviceName)
	if in.TenantName == "" || in.StoreName == "" || in.OwnerName == "" || in.DeviceName == "" || len(in.PublicKey) != ed25519.PublicKeySize {
		return Result{}, errors.New("dados de instalação inválidos")
	}
	hash, err := localauth.HashPassword(in.Password)
	if err != nil {
		return Result{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenants`).Scan(&count); err != nil {
		return Result{}, err
	}
	if count != 0 {
		return Result{}, ErrAlreadyInitialized
	}
	ids := make([]string, 4)
	for i := range ids {
		if ids[i], err = localdb.NewID(); err != nil {
			return Result{}, err
		}
	}
	result := Result{ids[0], ids[1], ids[2], ids[3]}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	unix := time.Now().Unix()
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO tenants VALUES (?,?,?)`, []any{result.TenantID, in.TenantName, now}},
		{`INSERT INTO stores VALUES (?,?,?)`, []any{result.TenantID, result.StoreID, in.StoreName}},
		{`INSERT INTO identities VALUES (?,?,?)`, []any{result.OwnerID, in.OwnerName, now}},
		{`INSERT INTO memberships VALUES (?,?,'owner','active',?)`, []any{result.TenantID, result.OwnerID, now}},
		{`INSERT INTO devices VALUES (?,?,?,?)`, []any{result.TenantID, result.StoreID, result.DeviceID, in.DeviceName}},
		{`INSERT INTO device_pairings(tenant_id,store_id,device_id,public_key,challenge,challenge_expires_unix,status,requested_by,proof_unix,approved_by,approved_unix)
			VALUES (?,?,?,?,zeroblob(32),?,'approved',?,?,?,?)`, []any{result.TenantID, result.StoreID, result.DeviceID, []byte(in.PublicKey), unix + 300, result.OwnerID, unix, result.OwnerID, unix}},
		{`INSERT INTO local_passwords VALUES (?,?,?,?)`, []any{result.TenantID, result.OwnerID, hash, now}},
	}
	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			return Result{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Result{}, err
	}
	return result, nil
}
