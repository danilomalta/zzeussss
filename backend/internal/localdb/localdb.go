// Package localdb contém a fundação SQLite de um dispositivo TitanSystem.
// Não substitui nem inicializa o PostgreSQL legado automaticamente.
package localdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open abre um arquivo SQLite privado do dispositivo e aplica migrações
// incrementais. Passe um caminho local escolhido pela instalação; nunca um
// arquivo compartilhado por rede nem o caminho de um banco existente do usuário.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("caminho SQLite vazio")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("caminho SQLite: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return nil, fmt.Errorf("diretório SQLite: %w", err)
	}
	info, err := os.Lstat(absolute)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(absolute, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if createErr != nil {
			return nil, fmt.Errorf("criar SQLite: %w", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("fechar SQLite novo: %w", closeErr)
		}
	} else if err != nil {
		return nil, fmt.Errorf("inspecionar SQLite: %w", err)
	} else if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("SQLite existente deve ser arquivo regular privado (0600)")
	}

	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	// Uma conexão por processo mantém PRAGMAs por conexão consistentes e
	// explicita o limite de um escritor. Outros processos ainda usam locks SQLite.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	fail := func(cause error) (*sql.DB, error) {
		_ = db.Close()
		return nil, cause
	}
	if err := ValidateSchema(ctx, db); err != nil {
		return fail(err)
	}
	for _, statement := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fail(fmt.Errorf("configurar SQLite (%s): %w", statement, err))
		}
	}
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode = WAL").Scan(&mode); err != nil {
		return fail(fmt.Errorf("ativar WAL: %w", err))
	}
	if !strings.EqualFold(mode, "wal") {
		return fail(fmt.Errorf("WAL não disponível neste arquivo: %s", mode))
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = FULL"); err != nil {
		return fail(fmt.Errorf("configurar durabilidade SQLite: %w", err))
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		return fail(fmt.Errorf("chaves estrangeiras não ativas: valor=%d erro=%v", foreignKeys, err))
	}
	if err := Migrate(ctx, db); err != nil {
		return fail(err)
	}
	return db, nil
}

// NewID gera um UUID v4 local com entropia criptográfica, sem consultar servidor.
func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("gerar ID local: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
