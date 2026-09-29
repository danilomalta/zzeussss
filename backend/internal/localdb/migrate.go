package localdb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Apenas migrations/ deste pacote é incorporado; as migrações PostgreSQL em
// backend/db/migrations/ NÃO são abertas nem executadas por esta rotina.
//
//go:embed migrations/*.sql
var migrations embed.FS

// Migrate reaplica com segurança apenas migrações novas. Uma migração aplicada
// com conteúdo diferente interrompe a abertura, preservando os dados.
func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("SQLite não inicializado")
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("listar migrações locais: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return fmt.Errorf("nome de migração inválido: %s", entry.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil || version < 1 {
			return fmt.Errorf("versão de migração inválida: %s", entry.Name())
		}
		content, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("ler migração %s: %w", entry.Name(), err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(content))
		if err := applyMigration(ctx, db, version, checksum, string(content)); err != nil {
			return fmt.Errorf("migração %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, version int, checksum, script string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL
	) STRICT`); err != nil {
		return err
	}
	var previous string
	err = tx.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = ?", version).Scan(&previous)
	if err == nil {
		if previous != checksum {
			return fmt.Errorf("versão %d alterada após aplicação", version)
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// Os arquivos deste pacote usam instruções simples terminadas por ';'.
	// Não incluir triggers nem ponto e vírgula dentro de strings nesta rotina.
	var lines []string
	for _, line := range strings.Split(script, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, checksum, applied_at) VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))",
		version, checksum); err != nil {
		return err
	}
	return tx.Commit()
}
