package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localsetup"
)

func testIssuer(t *testing.T) (*entitlements.Verifier, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": public})
	if err != nil {
		t.Fatal(err)
	}
	return verifier, private
}

func signedTestContract(t *testing.T, private ed25519.PrivateKey, tenant string, revision int64, selected []modules.ID) entitlements.Envelope {
	t.Helper()
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: tenant, Revision: revision,
		IssuedAt: now - 60, NotBefore: now - 60, ExpiresAt: now + 3600, Modules: selected})
	if err != nil {
		t.Fatal(err)
	}
	return entitlements.Envelope{KeyID: "test-issuer", Payload: payload,
		Signature: ed25519.Sign(private, entitlements.SigningMessage("test-issuer", payload))}
}

// A complete signed contract exists only in this test fixture, never startup.
func newLicensedTestApp(t *testing.T, db *sql.DB, device identity.DeviceContext, ownerID string) *fiber.App {
	t.Helper()
	verifier, private := testIssuer(t)
	store, err := entitlementstore.New(db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	actor := identity.Scope{TenantID: device.TenantID, StoreID: device.StoreID, IdentityID: ownerID}
	if _, err := store.Install(context.Background(), actor, device,
		signedTestContract(t, private, device.TenantID, 1, []modules.ID{modules.Core, modules.Inventory})); err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

type httpContractFixture struct {
	db      *sql.DB
	app     *fiber.App
	owner   localsetup.Result
	device  identity.DeviceContext
	private ed25519.PrivateKey
	token   string
}

func httpContractSetup(t *testing.T) *httpContractFixture {
	t.Helper()
	db, err := localdb.Open(context.Background(), filepath.Join(t.TempDir(), "http-contract.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := localsetup.Initialize(context.Background(), db, localsetup.Input{
		TenantName: "Mercado", StoreName: "Loja", OwnerName: "Dono", DeviceName: "Caixa",
		Password: "senha-forte-de-teste-1", PublicKey: public,
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier, private := testIssuer(t)
	device := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
	app, err := NewWithVerifier(db, device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	return &httpContractFixture{db: db, app: app, owner: owner, device: device, private: private, token: loginToken(t, app, owner.OwnerID)}
}

func (f *httpContractFixture) install(t *testing.T, envelope entitlements.Envelope, want int) []byte {
	t.Helper()
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	status, body := request(t, f.app, "POST", "/local/v1/module-contracts", string(encoded), f.token)
	if status != want {
		t.Fatalf("instalacao: %d esperado %d corpo=%s", status, want, body)
	}
	return body
}

func assertHTTPCounts(t *testing.T, f *httpContractFixture, products, history int) {
	t.Helper()
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM products`, products}, {`SELECT COUNT(*) FROM module_contract_history`, history},
	} {
		var count int
		if err := f.db.QueryRow(check.query).Scan(&count); err != nil || count != check.want {
			t.Fatalf("contagem=%d esperado=%d erro=%v", count, check.want, err)
		}
	}
}

const testProductBody = `{"sku":"A1","name":"Arroz","price_cents":1599,"cost_cents":1025}`

func TestHTTPContractInstallationAndCatalogGate(t *testing.T) {
	f := httpContractSetup(t)
	status, _ := request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 403 {
		t.Fatalf("sem contrato: %d", status)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/module-contracts", `{}`, "")
	if status != 401 {
		t.Fatalf("sem sessao: %d", status)
	}
	envelope := signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory})
	f.install(t, envelope, 200)
	if body := f.install(t, envelope, 200); !bytes.Contains(body, []byte(`"repeated":true`)) {
		t.Fatalf("repeticao: %s", body)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 201 {
		t.Fatalf("cadastro autorizado: %d", status)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/locations", `{"kind":"shelf","name":"Gondola"}`, f.token)
	if status != 201 {
		t.Fatalf("local autorizado: %d", status)
	}
	assertHTTPCounts(t, f, 1, 1)
}

func TestHTTPWithoutIssuerKeysPreservesLoginAndReadButCannotWrite(t *testing.T) {
	db, licensed, owner := fixture(t)
	token := loginToken(t, licensed, owner.OwnerID)
	status, _ := request(t, licensed, "POST", "/local/v1/products", testProductBody, token)
	if status != 201 {
		t.Fatalf("preparacao: %d", status)
	}
	app, err := New(db, identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID})
	if err != nil {
		t.Fatal(err)
	}
	token = loginToken(t, app, owner.OwnerID)
	status, body := request(t, app, "GET", "/local/v1/products", "", token)
	if status != 200 || !bytes.Contains(body, []byte(`"price_cents":1599`)) {
		t.Fatalf("consulta: %d %s", status, body)
	}
	for _, path := range []string{"/local/v1/products", "/local/v1/locations", "/local/v1/module-contracts"} {
		status, _ := request(t, app, "POST", path, `{}`, token)
		if status != 503 {
			t.Fatalf("sem emissor %s: %d", path, status)
		}
	}
}

func TestHTTPModuleAndRoleAreBothRequired(t *testing.T) {
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Staff}), 200)
	status, _ := request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 403 {
		t.Fatalf("modulo ausente: %d", status)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Inventory}), 200)
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 403 {
		t.Fatalf("papel caixa: %d", status)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 403)
	if _, err := f.db.Exec(`UPDATE memberships SET role='manager' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 403)
	assertHTTPCounts(t, f, 0, 2)
}

func TestHTTPBadSignatureForeignTenantAndUntrustedIssuerAreRejected(t *testing.T) {
	f := httpContractSetup(t)
	valid := signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory})
	bad := valid
	bad.Signature = make([]byte, ed25519.SignatureSize)
	f.install(t, bad, 400)
	foreign := signedTestContract(t, f.private, "other-company", 1, []modules.ID{modules.Core, modules.Inventory})
	f.install(t, foreign, 400)
	bad = valid
	bad.KeyID = "unknown"
	f.install(t, bad, 400)
	assertHTTPCounts(t, f, 0, 0)
}

func TestHTTPRejectsForgedOrAmbiguousEnvelopeFields(t *testing.T) {
	f := httpContractSetup(t)
	for _, body := range []string{
		`{"key_id":"x","payload":"","signature":"","tenant_id":"other"}`,
		`{"key_id":"x","payload":"","signature":"","public_key":"self"}`,
		`{"key_id":"x","key_id":"y","payload":"","signature":""}`,
		`{"key_id":"x","payload":"","signature":""} {}`, `null`, `{}`,
	} {
		status, _ := request(t, f.app, "POST", "/local/v1/module-contracts", body, f.token)
		if status != 400 {
			t.Fatalf("envelope ambiguo: %d", status)
		}
	}
	assertHTTPCounts(t, f, 0, 0)
}

func TestHTTPOldRevisionCannotReactivateInventory(t *testing.T) {
	f := httpContractSetup(t)
	first := signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory})
	f.install(t, first, 200)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 2, []modules.ID{modules.Core, modules.Staff}), 200)
	f.install(t, first, 409)
	status, _ := request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 403 {
		t.Fatalf("contrato antigo habilitou cadastro: %d", status)
	}
	assertHTTPCounts(t, f, 0, 2)
}

func TestHTTPExpiredStoredContractStillAllowsAuthorizedRead(t *testing.T) {
	f := httpContractSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory}), 200)
	status, _ := request(t, f.app, "POST", "/local/v1/products", testProductBody, f.token)
	if status != 201 {
		t.Fatalf("preparacao: %d", status)
	}
	now := time.Now().Unix()
	payload, err := json.Marshal(entitlements.Claims{Version: 1, TenantID: f.owner.TenantID, Revision: 1,
		IssuedAt: now - 120, NotBefore: now - 120, ExpiresAt: now - 60, Modules: []modules.ID{modules.Core, modules.Inventory}})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a previously valid signed contract now expired, without sleeping.
	if _, err := f.db.Exec(`UPDATE module_contract_history SET payload=?,signature=? WHERE tenant_id=? AND revision=1`, payload,
		ed25519.Sign(f.private, entitlements.SigningMessage("test-issuer", payload)), f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	status, _ = request(t, f.app, "POST", "/local/v1/products", `{"sku":"B1","name":"Outro","price_cents":1}`, f.token)
	if status != 403 {
		t.Fatalf("expirado: %d", status)
	}
	status, body := request(t, f.app, "GET", "/local/v1/products", "", f.token)
	if status != 200 || !bytes.Contains(body, []byte(`"price_cents":1599`)) {
		t.Fatalf("consulta: %d %s", status, body)
	}
	assertHTTPCounts(t, f, 1, 1)
}

func TestHTTPContractCannotBeInstalledAfterSessionRevocation(t *testing.T) {
	f := httpContractSetup(t)
	if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE tenant_id=? AND device_id=?`, f.owner.TenantID, f.owner.DeviceID); err != nil {
		t.Fatal(err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core, modules.Inventory}), 401)
	assertHTTPCounts(t, f, 0, 0)
}
