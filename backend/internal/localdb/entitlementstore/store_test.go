package entitlementstore_test

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
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

type fixture struct {
	db       *sql.DB
	path     string
	store    *entitlementstore.Store
	verifier *entitlements.Verifier
	private  ed25519.PrivateKey
	actor    identity.Scope
	device   identity.DeviceContext
	now      time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "contracts.sqlite")
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	devicePublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := localsetup.Initialize(ctx, db, localsetup.Input{
		TenantName: "Mercado", StoreName: "Matriz", OwnerName: "Dono", DeviceName: "Caixa",
		Password: "senha-de-teste-em-memoria", PublicKey: devicePublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"issuer": public})
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: db, path: path, verifier: verifier, private: private,
		actor:  identity.Scope{TenantID: owner.TenantID, StoreID: owner.StoreID, IdentityID: owner.OwnerID},
		device: identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID},
		now:    time.Unix(2000000000, 0).UTC()}
	f.store, err = entitlementstore.New(db, verifier, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func contract(t *testing.T, f *fixture, revision int64, ids []modules.ID) entitlements.Envelope {
	t.Helper()
	claims := entitlements.Claims{Version: 1, TenantID: f.actor.TenantID, Revision: revision,
		IssuedAt: 1999999900, NotBefore: 1999999900, ExpiresAt: 2000003600, Modules: ids}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return entitlements.Envelope{KeyID: "issuer", Payload: payload,
		Signature: ed25519.Sign(f.private, entitlements.SigningMessage("issuer", payload))}
}

func install(t *testing.T, f *fixture, envelope entitlements.Envelope) {
	t.Helper()
	if _, err := f.store.Install(context.Background(), f.actor, f.device, envelope); err != nil {
		t.Fatal(err)
	}
}

func require(f *fixture, permission identity.Permission, module modules.ID) error {
	ctx := context.Background()
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := f.store.RequireTx(ctx, tx, f.actor, f.device, permission, module); err != nil {
		return err
	}
	return tx.Commit()
}

func TestInstallIsIdempotentAndDoesNotGrantMissingModules(t *testing.T) {
	f := setup(t)
	if err := require(f, identity.Sell, modules.POS); !errors.Is(err, entitlementstore.ErrNotInstalled) {
		t.Fatalf("modulo sem contrato: %v", err)
	}
	envelope := contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS})
	install(t, f, envelope)
	result, err := f.store.Install(context.Background(), f.actor, f.device, envelope)
	if err != nil || !result.Repeated {
		t.Fatalf("repeticao: %+v %v", result, err)
	}
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM module_contract_history`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("historico duplicado: %d %v", count, err)
	}
	if err := require(f, identity.Sell, modules.POS); err != nil {
		t.Fatal(err)
	}
	if err := require(f, identity.ManageProduction, modules.Production); !errors.Is(err, modules.ErrUnavailable) {
		t.Fatalf("producao nao contratada: %v", err)
	}
}

func TestNewRevisionCannotBeReplacedByOldOrConflictingContract(t *testing.T) {
	f := setup(t)
	first := contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS})
	install(t, f, first)
	second := contract(t, f, 2, []modules.ID{modules.Core, modules.Inventory})
	install(t, f, second)
	if _, err := f.store.Install(context.Background(), f.actor, f.device, first); !errors.Is(err, entitlementstore.ErrStale) {
		t.Fatalf("revisao antiga reinstalada: %v", err)
	}
	conflict := contract(t, f, 2, []modules.ID{modules.Core, modules.Inventory, modules.POS})
	if _, err := f.store.Install(context.Background(), f.actor, f.device, conflict); !errors.Is(err, entitlementstore.ErrConflict) {
		t.Fatalf("revisao reutilizada: %v", err)
	}
	if err := require(f, identity.Sell, modules.POS); !errors.Is(err, modules.ErrUnavailable) {
		t.Fatalf("PDV removido continua disponivel: %v", err)
	}
}

func TestManagerCannotInstallAndRevokedOwnerCannotUseContract(t *testing.T) {
	f := setup(t)
	envelope := contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS})
	install(t, f, envelope)
	if _, err := f.db.Exec(`UPDATE memberships SET role='manager' WHERE tenant_id=? AND identity_id=?`, f.actor.TenantID, f.actor.IdentityID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.actor.TenantID, f.actor.IdentityID, f.actor.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Install(context.Background(), f.actor, f.device, envelope); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("gerente instalou contrato: %v", err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='owner',status='revoked' WHERE tenant_id=? AND identity_id=?`, f.actor.TenantID, f.actor.IdentityID); err != nil {
		t.Fatal(err)
	}
	if err := require(f, identity.Sell, modules.POS); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("dono revogado usou contrato: %v", err)
	}
}

func TestTamperedStoredSignatureCannotGrantAccess(t *testing.T) {
	f := setup(t)
	envelope := contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS})
	install(t, f, envelope)
	if _, err := f.db.Exec(`UPDATE module_contract_history SET signature=zeroblob(64) WHERE tenant_id=?`, f.actor.TenantID); err != nil {
		t.Fatal(err)
	}
	if err := require(f, identity.Sell, modules.POS); !errors.Is(err, entitlements.ErrSignature) {
		t.Fatalf("contrato adulterado aceito: %v", err)
	}
}

func TestObservedClockRollbackAndExpirationAreRejected(t *testing.T) {
	f := setup(t)
	install(t, f, contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS}))
	f.now = time.Unix(2000000010, 0)
	if err := require(f, identity.Sell, modules.POS); err != nil {
		t.Fatal(err)
	}
	f.now = time.Unix(2000000005, 0)
	if err := require(f, identity.Sell, modules.POS); !errors.Is(err, entitlementstore.ErrClockRollback) {
		t.Fatalf("retrocesso aceito: %v", err)
	}
	f.now = time.Unix(2000003600, 0)
	if err := require(f, identity.Sell, modules.POS); !errors.Is(err, entitlements.ErrValidity) {
		t.Fatalf("contrato vencido aceito: %v", err)
	}
}

func TestStateFailureRollsBackHistoryAndActiveRevision(t *testing.T) {
	f := setup(t)
	install(t, f, contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS}))
	if _, err := f.db.Exec(`CREATE TRIGGER reject_contract BEFORE UPDATE ON module_contract_state
		WHEN NEW.revision=2 BEGIN SELECT RAISE(ABORT,'falha de teste'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Install(context.Background(), f.actor, f.device, contract(t, f, 2, []modules.ID{modules.Core})); err == nil {
		t.Fatal("falha do ponteiro vigente ignorada")
	}
	var revision, count int
	if err := f.db.QueryRow(`SELECT revision FROM module_contract_state WHERE tenant_id=?`, f.actor.TenantID).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("ponteiro mudou apos falha: %d %v", revision, err)
	}
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM module_contract_history`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("historico parcial: %d %v", count, err)
	}
	if err := require(f, identity.Sell, modules.POS); err != nil {
		t.Fatal(err)
	}
}

func TestContractForOtherCompanyCannotBeInstalled(t *testing.T) {
	f := setup(t)
	claims := entitlements.Claims{Version: 1, TenantID: "foreign-company", Revision: 1,
		IssuedAt: 1999999900, NotBefore: 1999999900, ExpiresAt: 2000003600,
		Modules: []modules.ID{modules.Core}}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	envelope := entitlements.Envelope{KeyID: "issuer", Payload: payload,
		Signature: ed25519.Sign(f.private, entitlements.SigningMessage("issuer", payload))}
	if _, err := f.store.Install(context.Background(), f.actor, f.device, envelope); !errors.Is(err, entitlements.ErrTenant) {
		t.Fatalf("contrato de outra empresa instalado: %v", err)
	}
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM module_contract_history`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("contrato rejeitado deixou historico: %d %v", count, err)
	}
}

func TestContractSurvivesReopenAndRemainsVerified(t *testing.T) {
	f := setup(t)
	install(t, f, contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS}))
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f.db = db
	f.store, err = entitlementstore.New(db, f.verifier, func() time.Time { return f.now })
	if err != nil {
		t.Fatal(err)
	}
	if err := require(f, identity.Sell, modules.POS); err != nil {
		t.Fatal(err)
	}
}

func TestCallerRollbackAlsoRollsBackObservation(t *testing.T) {
	f := setup(t)
	install(t, f, contract(t, f, 1, []modules.ID{modules.Core, modules.Inventory, modules.POS}))
	f.now = time.Unix(2000000010, 0)
	ctx := context.Background()
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := f.store.RequireTx(ctx, tx, f.actor, f.device, identity.Sell, modules.POS); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tenants SET name='Alterado' WHERE id=?`, f.actor.TenantID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var name string
	var observed int64
	if err := f.db.QueryRow(`SELECT name FROM tenants WHERE id=?`, f.actor.TenantID).Scan(&name); err != nil || name != "Mercado" {
		t.Fatalf("mutacao escapou do rollback: %s %v", name, err)
	}
	if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state WHERE tenant_id=?`, f.actor.TenantID).Scan(&observed); err != nil || observed != 2000000000 {
		t.Fatalf("observacao escapou do rollback: %d %v", observed, err)
	}
}
