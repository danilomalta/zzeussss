package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/identity"
)

func catalogDB(t *testing.T) (*sql.DB, identity.Scope, identity.DeviceContext) {
	t.Helper()
	ctx := context.Background()
	db, err := localdb.Open(ctx, filepath.Join(t.TempDir(), "catalog.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	statements := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"market", "Mercado", "now"}},
		{"INSERT INTO tenants VALUES (?, ?, ?)", []any{"other", "Outro", "now"}},
		{"INSERT INTO stores VALUES (?, ?, ?)", []any{"market", "s1", "Loja"}},
		{"INSERT INTO stores VALUES (?, ?, ?)", []any{"other", "s2", "Outra loja"}},
		{"INSERT INTO devices VALUES (?, ?, ?, ?)", []any{"market", "s1", "d1", "Caixa"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"manager", "Gerente", "now"}},
		{"INSERT INTO identities VALUES (?, ?, ?)", []any{"cashier", "Caixa", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"market", "manager", "manager", "active", "now"}},
		{"INSERT INTO memberships VALUES (?, ?, ?, ?, ?)", []any{"market", "cashier", "cashier", "active", "now"}},
		{"INSERT INTO membership_stores VALUES (?, ?, ?)", []any{"market", "manager", "s1"}},
		{"INSERT INTO membership_stores VALUES (?, ?, ?)", []any{"market", "cashier", "s1"}},
		{`INSERT INTO device_pairings
			(tenant_id, store_id, device_id, public_key, challenge, challenge_expires_unix, status, requested_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, []any{"market", "s1", "d1", make([]byte, 32), make([]byte, 32), 9999999999, "approved", "manager"}},
		{"INSERT INTO products (tenant_id, id, sku, name, price_cents) VALUES (?, ?, ?, ?, ?)", []any{"other", "foreign", "X", "Não visível", 100}},
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatalf("preparar cenário: %v", err)
		}
	}
	return db, identity.Scope{IdentityID: "manager", TenantID: "market", StoreID: "s1"},
		identity.DeviceContext{TenantID: "market", StoreID: "s1", DeviceID: "d1"}
}

func TestCatalogIsolationAndCostVisibility(t *testing.T) {
	db, manager, device := catalogDB(t)
	ctx := context.Background()
	id, err := CreateProduct(ctx, db, manager, device, ProductInput{
		SKU: " A-1 ", Barcode: "7891234567890", Name: "Arroz", Unit: "kg", PriceCents: 1599, CostCents: 1025,
	})
	if err != nil || id == "" {
		t.Fatalf("cadastro falhou: %s %v", id, err)
	}
	if _, err := CreateProduct(ctx, db, manager, device, ProductInput{SKU: "A-1", Name: "Duplicado", PriceCents: 1}); err == nil {
		t.Fatal("SKU repetido na empresa foi aceito")
	}
	if _, err := CreateProduct(ctx, db, manager, device, ProductInput{SKU: "A-2", Barcode: "7891234567890", Name: "Duplicado", PriceCents: 1}); err == nil {
		t.Fatal("código de barras repetido foi aceito")
	}
	products, err := ListProducts(ctx, db, manager, device, 10, 0)
	if err != nil || len(products) != 1 || products[0].CostCents == nil || *products[0].CostCents != 1025 || products[0].PriceCents != 1599 {
		t.Fatalf("catálogo do gerente: %+v erro=%v", products, err)
	}
	cashier := identity.Scope{IdentityID: "cashier", TenantID: "market", StoreID: "s1"}
	products, err = ListProducts(ctx, db, cashier, device, 10, 0)
	if err != nil || len(products) != 1 || products[0].CostCents != nil {
		t.Fatalf("custo exposto ao caixa: %+v erro=%v", products, err)
	}
	encoded, err := json.Marshal(products)
	if err != nil || strings.Contains(string(encoded), "cost_cents") {
		t.Fatalf("JSON expôs custo: %s erro=%v", encoded, err)
	}
	if _, err := CreateProduct(ctx, db, cashier, device, ProductInput{SKU: "N", Name: "Negado", PriceCents: 1}); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("caixa cadastrou produto: %v", err)
	}
	if _, err := ListProducts(ctx, db, manager, device, 101, 0); !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("paginação inválida aceita: %v", err)
	}
}

func TestLocationsByStoreAndRevocation(t *testing.T) {
	db, manager, device := catalogDB(t)
	ctx := context.Background()
	if _, err := CreateLocation(ctx, db, manager, device, LocationInput{Kind: "shelf", Name: "Gôndola A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateLocation(ctx, db, manager, device, LocationInput{Kind: "backroom", Name: "Depósito"}); err != nil {
		t.Fatal(err)
	}
	locations, err := ListLocations(ctx, db, manager, device)
	if err != nil || len(locations) != 2 {
		t.Fatalf("locais=%+v erro=%v", locations, err)
	}
	if _, err := CreateLocation(ctx, db, manager, device, LocationInput{Kind: "unknown", Name: "Incorreto"}); !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("tipo inválido aceito: %v", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE device_pairings SET status = 'revoked' WHERE tenant_id = ? AND device_id = ?", "market", "d1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ListLocations(ctx, db, manager, device); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho revogado consultou locais: %v", err)
	}
}
