package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
)

const fiscalProfileBody = `{"operation_id":"issuer-create","profile_id":"issuer-one","expected_revision":0,"cnpj":"12ABC34501DE35","legal_name":"Synthetic test issuer","state":"SP","municipality_ibge":"3550308","state_registration":"123456789","crt":"1","model":"65","environment":"homologation","series":1}`
const fiscalIntentBody = `{"operation_id":"fiscal-one","sale_id":"sale-one","profile_id":"issuer-one","profile_revision":1}`

func fiscalFixture(t *testing.T, model ...string) *stockHTTPFixture {
	t.Helper()
	f := salesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Fiscal, modules.Staff}), 200)
	salesRequest(t, f, salesInput(f), 201)
	profile := fiscalProfileBody
	if len(model) > 0 {
		profile = strings.Replace(profile, `"model":"65"`, `"model":"`+model[0]+`"`, 1)
	}
	code, body := request(t, f.app, "POST", "/local/v1/fiscal/profiles", profile, f.token)
	if code != 201 {
		t.Fatalf("profile %d %s", code, body)
	}
	return f
}
func fiscalIntent(t *testing.T, f *stockHTTPFixture, body string, want int) fiscal.Intent {
	t.Helper()
	code, b := request(t, f.app, "POST", "/local/v1/fiscal/intents", body, f.token)
	if code != want {
		t.Fatalf("intent %d want %d %s", code, want, b)
	}
	var out fiscal.Intent
	if want == 200 || want == 201 {
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestHTTPFiscalSnapshotReplayRevisionAndNoAuthorization(t *testing.T) {
	f := fiscalFixture(t)
	original := fiscalIntent(t, f, fiscalIntentBody, 201)
	if original.Status != "blocked_configuration" || original.Readiness.OfficialAuthorization || original.Readiness.NumberAllocated || original.Snapshot.TotalCents != 3198 || len(original.Snapshot.Items) != 1 {
		t.Fatalf("incorrect result %+v", original)
	}
	repeated := fiscalIntent(t, f, fiscalIntentBody, 200)
	if !repeated.Repeated || repeated.ID != original.ID || repeated.SnapshotSHA256 != original.SnapshotSHA256 {
		t.Fatal("replay changed original")
	}
	fiscalIntent(t, f, strings.Replace(fiscalIntentBody, "fiscal-one", "fiscal-second", 1), 409)
	fiscalIntent(t, f, strings.Replace(fiscalIntentBody, "sale-one", "missing-sale", 1), 409)
	changed := strings.Replace(fiscalProfileBody, `"issuer-create"`, `"issuer-revise"`, 1)
	changed = strings.Replace(changed, `"expected_revision":0`, `"expected_revision":1`, 1)
	changed = strings.Replace(changed, "Synthetic test issuer", "Revised test issuer", 1)
	code, body := request(t, f.app, "POST", "/local/v1/fiscal/profiles", changed, f.token)
	if code != 201 || !bytes.Contains(body, []byte(`"revision":2`)) {
		t.Fatalf("revision %d %s", code, body)
	}
	// Replay is evaluated against the original, not the new profile or sale.
	fiscalIntent(t, f, fiscalIntentBody, 200)
	code, body = request(t, f.app, "GET", "/local/v1/fiscal/operations/fiscal-one/receipt", "", f.token)
	if code != 200 || !bytes.Contains(body, []byte("Synthetic test issuer")) || bytes.Contains(body, []byte("Revised test issuer")) {
		t.Fatal("snapshot overwritten", code, string(body))
	}
	// Stale compare-and-swap cannot overwrite revision 2; original replay is safe.
	code, _ = request(t, f.app, "POST", "/local/v1/fiscal/profiles", strings.Replace(changed, "issuer-revise", "issuer-stale", 1), f.token)
	if code != 409 {
		t.Fatal(code)
	}
	code, body = request(t, f.app, "POST", "/local/v1/fiscal/profiles", fiscalProfileBody, f.token)
	if code != 200 || !bytes.Contains(body, []byte(`"revision":1`)) {
		t.Fatal("old replay lost")
	}
	for _, q := range []string{`UPDATE fiscal_intents SET status='authorized'`, `DELETE FROM fiscal_intents`, `UPDATE fiscal_profiles SET request_json='{}'`, `DELETE FROM fiscal_profiles`} {
		if _, err := f.db.Exec(q); err == nil {
			t.Fatal("immutable record changed", q)
		}
	}
	code, body = request(t, f.app, "GET", "/local/v1/fiscal/intents?limit=1&offset=0&sale_id=sale-one", "", f.token)
	if code != 200 || !bytes.Contains(body, []byte(original.ID)) || !bytes.Contains(body, []byte(`"has_more":false`)) {
		t.Fatal(code, string(body))
	}
}

func TestHTTPFiscalStrictInputsAndNoSecretAdmission(t *testing.T) {
	f := fiscalFixture(t)
	for _, body := range []string{
		strings.Replace(fiscalIntentBody, `"profile_revision":1`, `"profile_revision":null`, 1),
		strings.Replace(fiscalIntentBody, `"profile_revision":1`, `"profile_revision":1,"profile_revision":2`, 1),
		strings.Replace(fiscalIntentBody, `"profile_revision":1`, `"profile_revision":1,"tenant_id":"foreign"`, 1),
		strings.Replace(fiscalIntentBody, `"profile_revision":1`, `"profile_revision":1,"authorized":true`, 1),
		strings.Replace(fiscalIntentBody, `"profile_revision":1`, `"profile_revision":1,"certificate":"PRIVATE_TEST_ONLY"`, 1),
		fiscalIntentBody + `{}`, `{}`, `null`,
	} {
		code, b := request(t, f.app, "POST", "/local/v1/fiscal/intents", body, f.token)
		if code != 400 || bytes.Contains(b, []byte("PRIVATE_TEST_ONLY")) {
			t.Fatal(code, string(b))
		}
	}
	for _, path := range []string{"/fiscal/intents?limit=101", "/fiscal/intents?offset=-1", "/fiscal/intents?limit=1&limit=2", "/fiscal/intents?tenant_id=foreign", "/fiscal/intents?limit=1e2", "/fiscal/profiles/issuer-one?revision=0", "/fiscal/profiles/issuer-one?revision=1&revision=2", "/fiscal/capabilities?emit=true"} {
		code, _ := request(t, f.app, "GET", "/local/v1"+path, "", f.token)
		if code != 400 {
			t.Fatal(path, code)
		}
	}
	code, _ := request(t, f.app, "POST", "/local/v1/fiscal/intents?store_id=foreign", fiscalIntentBody, f.token)
	if code != 400 {
		t.Fatal(code)
	}
	for _, change := range [][2]string{{`"SP"`, `"XX"`}, {`"3550308"`, `"3304557"`}, {`"12ABC34501DE35"`, `"12ABC34501DE36"`}, {`"series":1`, `"series":900`}, {`"crt":"1"`, `"crt":"9"`}} {
		code, _ := request(t, f.app, "POST", "/local/v1/fiscal/profiles", strings.Replace(fiscalProfileBody, change[0], change[1], 1), f.token)
		if code != 400 {
			t.Fatal(change, code)
		}
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_intents`).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}

func TestHTTPFiscalContractSessionAndRoleCannotPromoteOrErase(t *testing.T) {
	f := fiscalFixture(t)
	original := fiscalIntent(t, f, fiscalIntentBody, 201)
	// Install a trusted contract without fiscal; historical reads still work.
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
	fiscalIntent(t, f, fiscalIntentBody, 403)
	code, b := request(t, f.app, "GET", "/local/v1/fiscal/intents/"+original.ID, "", f.token)
	if code != 200 || !bytes.Contains(b, []byte(original.ID)) {
		t.Fatal(code, string(b))
	}
	old := f.token
	code, _ = request(t, f.app, "POST", "/local/v1/logout", "", old)
	if code != 200 && code != 204 {
		t.Fatal(code)
	}
	code, _ = request(t, f.app, "POST", "/local/v1/fiscal/intents", fiscalIntentBody, old)
	if code != 401 {
		t.Fatal("old session wrote", code)
	}
	f.token = loginToken(t, f.app, f.owner.OwnerID)
	code, b = request(t, f.app, "GET", "/local/v1/fiscal/operations/fiscal-one/receipt", "", f.token)
	if code != 200 || !bytes.Contains(b, []byte(original.SnapshotSHA256)) {
		t.Fatal("original lost after session switch", code)
	}
	// A manager/accountant is not allowed to configure the emitter identity.
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 5, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Fiscal, modules.Staff}), 200)
	for _, role := range []string{"manager", "accountant", "cashier"} {
		code, b = request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"create-`+role+`","identity_id":"`+role+`","name":"Test staff","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
		if code != 201 {
			t.Fatal(code, string(b))
		}
		code, b = request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"`+role+`","password":"test-password-strong-1"}`, "")
		if code != 200 {
			t.Fatal(code, string(b))
		}
		var auth struct {
			Token string `json:"token"`
		}
		json.Unmarshal(b, &auth)
		code, _ = request(t, f.app, "POST", "/local/v1/fiscal/profiles", fiscalProfileBody, auth.Token)
		if code != 403 {
			t.Fatal("role configured identity", role, code)
		}
		code, _ = request(t, f.app, "GET", "/local/v1/fiscal/operations/fiscal-one/receipt", "", auth.Token)
		if code != 404 {
			t.Fatal("recovery changed actor", role, code)
		}
		if role == "cashier" {
			code, _ = request(t, f.app, "GET", "/local/v1/fiscal/intents/"+original.ID, "", auth.Token)
			if code != 404 {
				t.Fatal("cashier read other's attempt", code)
			}
		}
	}
}

func TestHTTPFiscalConcurrentReplayExactlyOneRecord(t *testing.T) {
	f := fiscalFixture(t)
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(t, f.app, "POST", "/local/v1/fiscal/intents", fiscalIntentBody, f.token)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	created := 0
	for code := range codes {
		if code == 201 {
			created++
		} else if code != 200 {
			t.Fatal("concurrent replay", code)
		}
	}
	if created != 1 {
		t.Fatal(created)
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_intents`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}

func TestHTTPFiscalRestartEncryptedBackupRestoreOriginalIDs(t *testing.T) {
	f := fiscalFixture(t)
	original := fiscalIntent(t, f, fiscalIntentBody, 201)
	ctx := context.Background()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := filepath.Join(dir, "fiscal.tytbak")
	if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
		t.Fatal(err)
	}
	if err := backup.Verify(ctx, archive, f.device, key); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "restored.sqlite")
	if err := backup.Restore(ctx, archive, path, f.device, key); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := request(t, app, "GET", "/local/v1/fiscal/intents/"+original.ID, "", f.token)
	if code != 401 {
		t.Fatal("restored session not revoked", code)
	}
	token := loginToken(t, app, f.owner.OwnerID)
	code, body := request(t, app, "GET", "/local/v1/fiscal/operations/fiscal-one/receipt", "", token)
	var restored fiscal.Intent
	if code != 200 || json.Unmarshal(body, &restored) != nil || restored.ID != original.ID || restored.SnapshotSHA256 != original.SnapshotSHA256 {
		t.Fatal("restored fiscal attempt differs", code, string(body))
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err = NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	token = loginToken(t, app, f.owner.OwnerID)
	code, body = request(t, app, "POST", "/local/v1/fiscal/intents", fiscalIntentBody, token)
	if code != 200 || !bytes.Contains(body, []byte(original.ID)) || !bytes.Contains(body, []byte(`"repeated":true`)) {
		t.Fatal("restart created new fiscal attempt", code, string(body))
	}
}

func TestHTTPFiscalCapabilitiesAndUFRequirementsRemainExplicitlyUnavailable(t *testing.T) {
	f := httpContractSetup(t)
	code, body := request(t, f.app, "GET", "/local/v1/fiscal/capabilities", "", f.token)
	if code != 200 || !bytes.Contains(body, []byte(`"transmission_available":false`)) || !bytes.Contains(body, []byte(`"nfse_available":false`)) {
		t.Fatal(code, string(body))
	}
	for _, uf := range []string{"AC", "AL", "AP", "AM", "BA", "CE", "DF", "ES", "GO", "MA", "MT", "MS", "MG", "PA", "PB", "PR", "PE", "PI", "RJ", "RN", "RS", "RO", "RR", "SC", "SP", "SE", "TO"} {
		code, body = request(t, f.app, "GET", "/local/v1/fiscal/requirements/"+uf, "", f.token)
		if code != 200 || !bytes.Contains(body, []byte(`"production_validated":false`)) {
			t.Fatal(uf, code, string(body))
		}
	}
	code, _ = request(t, f.app, "GET", "/local/v1/fiscal/requirements/XX", "", f.token)
	if code != 400 {
		t.Fatal(code)
	}
	code, _ = request(t, f.app, "POST", "/local/v1/fiscal/profiles", fiscalProfileBody, f.token)
	if code != 503 {
		t.Fatal("uninstalled contract was not reported unavailable", code)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 1, []modules.ID{modules.Core}), 200)
	code, _ = request(t, f.app, "POST", "/local/v1/fiscal/profiles", fiscalProfileBody, f.token)
	if code != 403 {
		t.Fatal("missing fiscal module configured identity", code)
	}
}

func TestFiscalMutationRevalidatesSessionInsideTransaction(t *testing.T) {
	f := fiscalFixture(t)
	actor := identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}
	var in fiscal.IntentInput
	if err := json.Unmarshal([]byte(fiscalIntentBody), &in); err != nil {
		t.Fatal(err)
	}
	// Even a valid actor captured before logout must not authorize the operation.
	old := f.token
	if err := localauth.Logout(context.Background(), f.db, old); err != nil {
		t.Fatal(err)
	}
	_, err := fiscal.Prepare(context.Background(), f.db, nil, actor, f.device, in, old)
	if !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("revoked token reached license/write", err)
	}
	fresh := loginToken(t, f.app, f.owner.OwnerID)
	foreign := actor
	foreign.StoreID = "other-store"
	_, err = fiscal.Prepare(context.Background(), f.db, nil, foreign, f.device, in, fresh)
	if !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("context mismatch reached write", err)
	}
	var n int
	if err = f.db.QueryRow(`SELECT count(*) FROM fiscal_intents`).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}

// Actual loopback HTTP and separate service processes. This proves only fiscal
// preparation/recovery, never SEFAZ authorization or a complete fiscal circuit.
func TestFiscalRealHTTPPreparationRestartAndRestore(t *testing.T) {
	f := fiscalFixture(t)
	station := pilotPrepareCLIStation(t, f)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	p := &pilotHTTP{t: t, service: pilotStart(t, path, f, station), client: &http.Client{Transport: transport, Timeout: 10 * time.Second}}
	defer func() { p.service.stop(t) }()
	p.token = p.login(f.owner.OwnerID)
	var in fiscal.IntentInput
	if err := json.Unmarshal([]byte(fiscalIntentBody), &in); err != nil {
		t.Fatal(err)
	}
	var original, after fiscal.Intent
	p.call("POST", "/local/v1/fiscal/intents", in, 201, &original)
	p.service.stop(t)
	p.service = pilotStart(t, path, f, station)
	p.token = p.login(f.owner.OwnerID)
	p.call("GET", "/local/v1/fiscal/operations/fiscal-one/receipt", nil, 200, &after)
	if original.ID != after.ID || original.SnapshotSHA256 != after.SnapshotSHA256 {
		t.Fatal("restart lost fiscal preparation")
	}
	p.call("POST", "/local/v1/fiscal/intents", in, 200, &after)
	if !after.Repeated || after.ID != original.ID {
		t.Fatal("restart duplicated preparation")
	}
	restored := ""
	if *pilotBackupCLI != "" {
		restored = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		db, err := localdb.Open(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		archive := filepath.Join(dir, "real-http.tytbak")
		restored = filepath.Join(dir, "restored.sqlite")
		if err = backup.Create(context.Background(), db, f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
		if err = backup.Verify(context.Background(), archive, f.device, key); err != nil {
			t.Fatal(err)
		}
		if err = backup.Restore(context.Background(), archive, restored, f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	p.service.stop(t)
	p.service = pilotStart(t, restored, f, station)
	p.call("GET", "/local/v1/fiscal/operations/fiscal-one/receipt", nil, 401, nil)
	p.token = p.login(f.owner.OwnerID)
	p.call("GET", "/local/v1/fiscal/operations/fiscal-one/receipt", nil, 200, &after)
	if original.ID != after.ID || original.SnapshotSHA256 != after.SnapshotSHA256 || after.Status != "blocked_configuration" || after.Readiness.OfficialAuthorization {
		t.Fatal("restore changed fiscal preparation or invented authorization")
	}
	t.Logf("FISCAL PREPARATION ONLY: %d real loopback HTTP calls, three service processes, restart and encrypted restore preserve original ID/hash; no SEFAZ request, fiscal number or authorization", p.steps)
}
