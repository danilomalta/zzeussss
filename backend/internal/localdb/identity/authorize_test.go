package identity

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"titansystem-backend/internal/localdb"
)

func setup(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := localdb.Open(ctx, filepath.Join(t.TempDir(), "device.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"market", "Mercado", "now"}},
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"supplier", "Fornecedor", "now"}},
		{"INSERT INTO stores VALUES (?, ?, ?)", []any{"market", "m1", "Loja 1"}},
		{"INSERT INTO stores VALUES (?, ?, ?)", []any{"market", "m2", "Loja 2"}},
		{"INSERT INTO stores VALUES (?, ?, ?)", []any{"supplier", "f1", "Fábrica"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"dono", "Dono", "now"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"gerente", "Gerente", "now"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"caixa", "Caixa", "now"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"contador", "Contador", "now"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"fornecedor", "Fornecedor", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"market", "dono", "owner", "active", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"market", "gerente", "manager", "active", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"market", "caixa", "cashier", "active", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"market", "contador", "accountant", "active", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"supplier", "fornecedor", "production", "active", "now"}},
		{"INSERT INTO membership_stores VALUES (?, ?, ?)", []any{"market", "gerente", "m1"}},
		{"INSERT INTO membership_stores VALUES (?, ?, ?)", []any{"market", "caixa", "m1"}},
		{"INSERT INTO membership_stores VALUES (?, ?, ?)", []any{"market", "contador", "m1"}},
		{"INSERT INTO membership_stores VALUES (?, ?, ?)", []any{"supplier", "fornecedor", "f1"}},
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatalf("preparar cenário: %v", err)
		}
	}
	return db
}

func TestTenantStoreAndRoleIsolation(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	cases := []struct {
		name       string
		scope      Scope
		permission Permission
		allowed    bool
	}{
		{"dono na própria loja", Scope{"dono", "market", "m2"}, ManageStock, true},
		{"dono em outra empresa", Scope{"dono", "supplier", "f1"}, ManageStock, false},
		{"gerente na loja permitida", Scope{"gerente", "market", "m1"}, ReviewDiscount, true},
		{"gerente na loja não vinculada", Scope{"gerente", "market", "m2"}, ReviewDiscount, false},
		{"caixa vendendo", Scope{"caixa", "market", "m1"}, Sell, true},
		{"caixa aprovando desconto", Scope{"caixa", "market", "m1"}, ReviewDiscount, false},
		{"contador vendo financeiro", Scope{"contador", "market", "m1"}, ViewAccounting, true},
		{"contador vendendo", Scope{"contador", "market", "m1"}, Sell, false},
		{"produção no fornecedor", Scope{"fornecedor", "supplier", "f1"}, ManageProduction, true},
		{"produção no mercado", Scope{"fornecedor", "market", "m1"}, ManageProduction, false},
		{"sem loja", Scope{"gerente", "market", ""}, ManageStock, false},
		{"dono gerenciando vínculos", Scope{"dono", "market", ""}, ManageStaff, true},
		{"permissão desconhecida", Scope{"dono", "market", "m1"}, Permission("all"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Can(ctx, db, tc.scope, tc.permission)
			if tc.allowed && err != nil {
				t.Fatalf("permissão esperada: %v", err)
			}
			if !tc.allowed && !errors.Is(err, ErrDenied) {
				t.Fatalf("negação esperada: %v", err)
			}
		})
	}
}

func TestRevocationAndSelfApproval(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	scope := Scope{"gerente", "market", "m1"}
	if err := CanReviewDiscount(ctx, db, scope, "gerente"); !errors.Is(err, ErrDenied) {
		t.Fatalf("autorrevisão deveria falhar: %v", err)
	}
	if err := CanReviewDiscount(ctx, db, scope, "caixa"); err != nil {
		t.Fatalf("gerente deveria poder revisar pedido alheio: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"UPDATE memberships SET status = 'revoked' WHERE tenant_id = ? AND identity_id = ?",
		"market", "gerente"); err != nil {
		t.Fatal(err)
	}
	if err := CanReviewDiscount(ctx, db, scope, "caixa"); !errors.Is(err, ErrDenied) {
		t.Fatalf("vínculo revogado deveria negar: %v", err)
	}
}

func TestStoreLinkCannotCrossTenant(t *testing.T) {
	db := setup(t)
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO membership_stores VALUES (?, ?, ?)", "market", "caixa", "f1")
	if err == nil {
		t.Fatal("vínculo com loja de outra empresa foi aceito")
	}
}
