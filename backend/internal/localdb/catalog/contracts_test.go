package catalog_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

type contractFixture struct {
	db      *sql.DB
	store   *entitlementstore.Store
	actor   identity.Scope
	device  identity.DeviceContext
	private ed25519.PrivateKey
	now     time.Time
}

func setupContract(t *testing.T) *contractFixture {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "catalog-contract.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	devicePublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := localsetup.Initialize(context.Background(), db, localsetup.Input{
		TenantName: "Mercado", StoreName: "Loja", OwnerName: "Dono", DeviceName: "Caixa",
		Password: "senha-de-teste-em-memoria", PublicKey: devicePublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"issuer": public})
	if err != nil {
		t.Fatal(err)
	}
	f := &contractFixture{db: db, private: private, now: time.Unix(2000000000, 0).UTC(),
		actor:  identity.Scope{TenantID: owner.TenantID, StoreID: owner.StoreID, IdentityID: owner.OwnerID},
		device: identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}}
	f.store, err = entitlementstore.New(db, verifier, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *contractFixture) install(t *testing.T, ids []modules.ID) {
	t.Helper()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.actor.TenantID, Revision: 1,
		IssuedAt: 1999999900, NotBefore: 1999999900, ExpiresAt: 2000003600, Modules: ids})
	if err != nil {
		t.Fatal(err)
	}
	envelope := entitlements.Envelope{KeyID: "issuer", Payload: payload,
		Signature: ed25519.Sign(f.private, entitlements.SigningMessage("issuer", payload))}
	if _, err := f.store.Install(context.Background(), f.actor, f.device, envelope); err != nil {
		t.Fatal(err)
	}
}

func (f *contractFixture) product(actor identity.Scope, device identity.DeviceContext, sku string) (string, error) {
	return catalog.CreateProductWithContract(context.Background(), f.db, f.store, actor, device,
		catalog.ProductInput{SKU: sku, Name: "Arroz", PriceCents: 1599, CostCents: 1025})
}

func assertCatalogCounts(t *testing.T, f *contractFixture, products, locations int) {
	t.Helper()
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM products WHERE tenant_id=?`, products},
		{`SELECT COUNT(*) FROM stock_locations WHERE tenant_id=?`, locations},
	} {
		var count int
		if err := f.db.QueryRow(check.query, f.actor.TenantID).Scan(&count); err != nil || count != check.want {
			t.Fatalf("contagem=%d esperado=%d erro=%v", count, check.want, err)
		}
	}
}

func TestCatalogContractMissingOrNilCannotWrite(t *testing.T) {
	f := setupContract(t)
	if _, err := f.product(f.actor, f.device, "missing"); !errors.Is(err, entitlementstore.ErrNotInstalled) {
		t.Fatalf("sem contrato: %v", err)
	}
	if _, err := catalog.CreateLocationWithContract(context.Background(), f.db, nil, f.actor, f.device,
		catalog.LocationInput{Kind: "shelf", Name: "Gondola"}); !errors.Is(err, entitlementstore.ErrNotInstalled) {
		t.Fatalf("sem verificador: %v", err)
	}
	assertCatalogCounts(t, f, 0, 0)
}

func TestCatalogContractOwnerCannotEnableUnpaidInventory(t *testing.T) {
	f := setupContract(t)
	f.install(t, []modules.ID{modules.Core, modules.Staff})
	if _, err := f.product(f.actor, f.device, "unpaid"); !errors.Is(err, modules.ErrUnavailable) {
		t.Fatalf("dono sem estoque contratado: %v", err)
	}
	if _, err := catalog.CreateLocationWithContract(context.Background(), f.db, f.store, f.actor, f.device,
		catalog.LocationInput{Kind: "backroom", Name: "Deposito"}); !errors.Is(err, modules.ErrUnavailable) {
		t.Fatalf("local sem estoque contratado: %v", err)
	}
	assertCatalogCounts(t, f, 0, 0)
}

func TestCatalogContractAuthorizedWritesAndExactMoney(t *testing.T) {
	f := setupContract(t)
	f.install(t, []modules.ID{modules.Core, modules.Inventory})
	f.now = f.now.Add(time.Second)
	id, err := f.product(f.actor, f.device, "A1")
	if err != nil || id == "" {
		t.Fatalf("produto: %s %v", id, err)
	}
	if _, err := catalog.CreateLocationWithContract(context.Background(), f.db, f.store, f.actor, f.device,
		catalog.LocationInput{Kind: "shelf", Name: "Gondola"}); err != nil {
		t.Fatal(err)
	}
	items, err := catalog.ListProducts(context.Background(), f.db, f.actor, f.device, 50, 0)
	if err != nil || len(items) != 1 || items[0].ID != id || items[0].PriceCents != 1599 ||
		items[0].CostCents == nil || *items[0].CostCents != 1025 {
		t.Fatalf("valores: %+v %v", items, err)
	}
	var observed int64
	if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, f.actor.TenantID).Scan(&observed); err != nil || observed != f.now.Unix() {
		t.Fatalf("observacao nao confirmada: %d %v", observed, err)
	}
	assertCatalogCounts(t, f, 1, 1)
}

func TestCatalogContractDoesNotOverrideRoleOrRevocation(t *testing.T) {
	f := setupContract(t)
	f.install(t, []modules.ID{modules.Core, modules.Inventory})
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.actor.TenantID, f.actor.IdentityID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.product(f.actor, f.device, "cashier"); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("papel: %v", err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='owner',status='revoked' WHERE tenant_id=? AND identity_id=?`, f.actor.TenantID, f.actor.IdentityID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.product(f.actor, f.device, "revoked"); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("revogacao: %v", err)
	}
	assertCatalogCounts(t, f, 0, 0)
}

func TestCatalogContractExpiryPreservesAuthorizedReadAccess(t *testing.T) {
	f := setupContract(t)
	f.install(t, []modules.ID{modules.Core, modules.Inventory})
	if _, err := f.product(f.actor, f.device, "before"); err != nil {
		t.Fatal(err)
	}
	f.now = time.Unix(2000003600, 0).UTC()
	if _, err := f.product(f.actor, f.device, "after"); !errors.Is(err, entitlements.ErrValidity) {
		t.Fatalf("expiracao: %v", err)
	}
	items, err := catalog.ListProducts(context.Background(), f.db, f.actor, f.device, 50, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("leitura historica: %+v %v", items, err)
	}
	assertCatalogCounts(t, f, 1, 0)
}

func TestCatalogContractInsertFailureRollsBackClockObservation(t *testing.T) {
	f := setupContract(t)
	f.install(t, []modules.ID{modules.Core, modules.Inventory})
	if _, err := f.db.Exec(`CREATE TRIGGER reject_catalog BEFORE INSERT ON products BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Minute)
	if _, err := f.product(f.actor, f.device, "fail"); err == nil {
		t.Fatal("falha de gravacao ignorada")
	}
	var observed int64
	if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, f.actor.TenantID).Scan(&observed); err != nil || observed != 2000000000 {
		t.Fatalf("rollback da observacao: %d %v", observed, err)
	}
	assertCatalogCounts(t, f, 0, 0)
}

func TestCatalogContractForeignOrRevokedDeviceCannotWrite(t *testing.T) {
	f := setupContract(t)
	f.install(t, []modules.ID{modules.Core, modules.Inventory})
	foreign := f.device
	foreign.TenantID = "outra-empresa"
	if _, err := f.product(f.actor, foreign, "foreign"); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("empresa: %v", err)
	}
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, f.actor.TenantID, f.device.DeviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.product(f.actor, f.device, "device"); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho: %v", err)
	}
	assertCatalogCounts(t, f, 0, 0)
}
