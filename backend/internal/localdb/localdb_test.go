package localdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func temporaryDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "device.sqlite")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("abrir banco local: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func TestCreateReopenAndRepeatMigration(t *testing.T) {
	ctx := context.Background()
	db, path := temporaryDB(t)
	if _, err := db.ExecContext(ctx, "INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)", "t1", "Mercado", "2026-09-29T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("reaplicar migração: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reabrir banco: %v", err)
	}
	defer reopened.Close()
	var count, migrations int
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants WHERE id = ?", "t1").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := reopened.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if count != 1 || migrations != 37 {
		t.Fatalf("dados/migrações inesperados: tenants=%d migrations=%d", count, migrations)
	}
	var foreignKeys int
	var mode string
	if err := reopened.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := reopened.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || mode != "wal" {
		t.Fatalf("configuração SQLite inesperada: foreign_keys=%d journal=%s", foreignKeys, mode)
	}
}

func TestRollbackPreservesDatabase(t *testing.T) {
	ctx := context.Background()
	db, _ := temporaryDB(t)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)", "temp", "Teste", "now"); err != nil {
		t.Fatal(err)
	}
	// Uma loja não pode apontar para uma empresa inexistente.
	if _, err := tx.ExecContext(ctx, "INSERT INTO stores (tenant_id, id, name) VALUES (?, ?, ?)", "missing", "s1", "Invalida"); err == nil {
		t.Fatal("FK inválida foi aceita")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants WHERE id = ?", "temp").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("transação falha deixou registro parcial")
	}
}

func TestTenantReferencesCannotCross(t *testing.T) {
	ctx := context.Background()
	db, _ := temporaryDB(t)
	setup := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"market", "Mercado", "now"}},
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"supplier", "Fornecedor", "now"}},
		{"INSERT INTO stores VALUES (?, ?, ?)", []any{"market", "s1", "Loja"}},
		{"INSERT INTO devices VALUES (?, ?, ?, ?)", []any{"market", "s1", "d1", "Caixa"}},
		{"INSERT INTO stock_locations VALUES (?, ?, ?, ?, ?)", []any{"market", "s1", "shelf", "shelf", "Gondola"}},
		{"INSERT INTO products (tenant_id, id, sku, name, price_cents) VALUES (?, ?, ?, ?, ?)", []any{"supplier", "p1", "sku1", "Item", 100}},
	}
	for _, item := range setup {
		if _, err := db.ExecContext(ctx, item.query, item.args...); err != nil {
			t.Fatalf("preparar cenário: %v", err)
		}
	}
	_, err := db.ExecContext(ctx, `INSERT INTO stock_movements
		(id, tenant_id, store_id, device_id, product_id, location_id, quantity_milli, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"move1", "market", "s1", "d1", "p1", "shelf", 1000, "compra", "now")
	if err == nil {
		t.Fatal("produto de outro tenant foi aceito no movimento")
	}
}

func TestIDsGeneratedWithoutNetwork(t *testing.T) {
	a, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 36 || a == b || a[14] != '4' {
		t.Fatalf("IDs UUID v4 inesperados: %q e %q", a, b)
	}
}
