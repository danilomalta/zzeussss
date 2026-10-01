package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/register"
)

const supplyBody = `{"session_id":"turn-one","operation_id":"supply-one","kind":"supply","amount_cents":500,"reason":"Troco inicial"}`
const withdrawalBody = `{"session_id":"turn-one","operation_id":"withdraw-one","kind":"withdrawal","amount_cents":300,"reason":"Recolhimento"}`

func movementSetup(t *testing.T) *httpContractFixture {
	t.Helper()
	f := cashSetup(t)
	cashRequest(t, f, "open", cashOpenBody, 201)
	return f
}

func movementCounts(t *testing.T, f *httpContractFixture, want int) {
	t.Helper()
	for _, q := range []string{`SELECT COUNT(*) FROM cash_adjustments`, `SELECT COUNT(*) FROM cash_movements`, `SELECT COUNT(*) FROM outbox WHERE event_type='cash.movement'`} {
		var n int
		if err := f.db.QueryRow(q).Scan(&n); err != nil || n != want {
			t.Fatalf("%s: %d want %d err %v", q, n, want, err)
		}
	}
}

func TestHTTPCashMovementAffectsBlindClosingAndReplaysAfterClosing(t *testing.T) {
	f := movementSetup(t)
	first := cashRequest(t, f, "movements", supplyBody, 200)
	var a, b register.MovementResult
	if err := json.Unmarshal(first, &a); err != nil || a.Repeated || a.MovementID == "" {
		t.Fatalf("%s %v", first, err)
	}
	replay := cashRequest(t, f, "movements", supplyBody, 200)
	if err := json.Unmarshal(replay, &b); err != nil || !b.Repeated || a.MovementID != b.MovementID || a.CreatedAt != b.CreatedAt {
		t.Fatalf("%s %v", replay, err)
	}
	cashRequest(t, f, "movements", withdrawalBody, 200)
	movementCounts(t, f, 2)
	status, body := request(t, f.app, "GET", "/local/v1/cash/current", "", f.token)
	if status != 200 || bytes.Contains(body, []byte("expected_cents")) || bytes.Contains(first, []byte("balance")) {
		t.Fatalf("consulta nao cega: %d %s", status, body)
	}
	closed := cashRequest(t, f, "close", cashCloseBody, 200)
	if !bytes.Contains(closed, []byte(`"expected_cents":1200`)) || !bytes.Contains(closed, []byte(`"difference_cents":0`)) {
		t.Fatalf("fechamento %s", closed)
	}
	cashRequest(t, f, "movements", supplyBody, 200)
	cashRequest(t, f, "movements", bytesToText(bytes.ReplaceAll([]byte(supplyBody), []byte("supply-one"), []byte("new-supply"))), 409)
	movementCounts(t, f, 2)
}

func bytesToText(b []byte) string { return string(b) }

func TestHTTPCashMovementRejectsConflictAndInsufficientMoney(t *testing.T) {
	f := movementSetup(t)
	cashRequest(t, f, "movements", `{"session_id":"turn-one","operation_id":"empty","kind":"withdrawal","amount_cents":1001,"reason":"Recolhimento"}`, 409)
	movementCounts(t, f, 0)
	cashRequest(t, f, "movements", supplyBody, 200)
	cashRequest(t, f, "movements", string(bytes.ReplaceAll([]byte(supplyBody), []byte("500"), []byte("501"))), 409)
	cashRequest(t, f, "movements", string(bytes.ReplaceAll([]byte(supplyBody), []byte("Troco inicial"), []byte("Outro motivo"))), 409)
	cashRequest(t, f, "movements", string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"kind":"supply"`), []byte(`"kind":"withdrawal"`))), 409)
	movementCounts(t, f, 1)
}

func TestHTTPCashMovementRejectsUnsafeJSON(t *testing.T) {
	f := movementSetup(t)
	for _, body := range []string{
		`{}`, `null`, supplyBody + `{}`,
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"amount_cents":500`), []byte(`"amount_cents":5.5`))),
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"amount_cents":500`), []byte(`"amount_cents":0`))),
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"amount_cents":500`), []byte(`"amount_cents":9223372036854775808`))),
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"reason":"Troco inicial"`), []byte(`"reason":null`))),
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"reason":"Troco inicial"`), []byte(`"reason":" "`))),
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"reason":"Troco inicial"`), []byte(`"reason":"a","reason":"b"`))),
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"reason":"Troco inicial"`), []byte(`"reason":"a","actor_id":"forged"`))),
		string(bytes.ReplaceAll([]byte(supplyBody), []byte(`"kind":"supply"`), []byte(`"kind":"sale"`))),
	} {
		cashRequest(t, f, "movements", body, 400)
	}
	movementCounts(t, f, 0)
}

func TestHTTPCashMovementRequiresSessionRoleDeviceAndContract(t *testing.T) {
	for _, scenario := range []string{"session", "cashier", "revoked", "device", "module", "verifier"} {
		t.Run(scenario, func(t *testing.T) {
			f := movementSetup(t)
			want := 403
			switch scenario {
			case "session":
				f.token = ""
				want = 401
			case "cashier":
				if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE identity_id=?`, f.owner.OwnerID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
				f.token = loginToken(t, f.app, f.owner.OwnerID)
			case "revoked":
				if _, err := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id=?`, f.owner.OwnerID); err != nil {
					t.Fatal(err)
				}
				want = 401
			case "device":
				if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE device_id=?`, f.device.DeviceID); err != nil {
					t.Fatal(err)
				}
				want = 401
			case "module":
				if _, err := f.db.Exec(`DELETE FROM module_contract_state`); err != nil {
					t.Fatal(err)
				}
			case "verifier":
				app, err := New(f.db, f.device)
				if err != nil {
					t.Fatal(err)
				}
				f.app = app
				want = 503
			}
			cashRequest(t, f, "movements", supplyBody, want)
			movementCounts(t, f, 0)
		})
	}
}

func TestHTTPCashMovementManagerCanMoveAnotherOperatorsDrawer(t *testing.T) {
	f := movementSetup(t)
	if _, err := f.db.Exec(`INSERT INTO identities VALUES ('cash-manager','Gerente','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO memberships VALUES (?,'cash-manager','manager','active','now')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,'cash-manager',?)`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	owner := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	if err := localauth.SetPassword(context.Background(), f.db, owner, f.device, "cash-manager", "senha-forte-de-teste-1"); err != nil {
		t.Fatal(err)
	}
	f.token = loginToken(t, f.app, "cash-manager")
	cashRequest(t, f, "movements", supplyBody, 200)
	var actor string
	if err := f.db.QueryRow(`SELECT actor_identity_id FROM cash_adjustments`).Scan(&actor); err != nil || actor != "cash-manager" {
		t.Fatalf("auditoria %s %v", actor, err)
	}
	movementCounts(t, f, 1)
}

func TestHTTPCashMovementFailureRollsBackMoneyAuditAndOutbox(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER fail_move BEFORE INSERT ON cash_movements BEGIN SELECT RAISE(ABORT,'test'); END`,
		`CREATE TRIGGER fail_move BEFORE INSERT ON cash_adjustments BEGIN SELECT RAISE(ABORT,'test'); END`,
		`CREATE TRIGGER fail_move BEFORE INSERT ON outbox WHEN NEW.event_type='cash.movement' BEGIN SELECT RAISE(ABORT,'test'); END`,
		`CREATE TRIGGER fail_move BEFORE INSERT ON cash_movements BEGIN SELECT RAISE(IGNORE); END`,
		`CREATE TRIGGER fail_move BEFORE INSERT ON cash_adjustments BEGIN SELECT RAISE(IGNORE); END`,
		`CREATE TRIGGER fail_move BEFORE INSERT ON outbox WHEN NEW.event_type='cash.movement' BEGIN SELECT RAISE(IGNORE); END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			f := movementSetup(t)
			if _, err := f.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			want := 500
			if bytes.Contains([]byte(trigger), []byte("IGNORE")) {
				want = 409
			}
			cashRequest(t, f, "movements", supplyBody, want)
			movementCounts(t, f, 0)
			if _, err := f.db.Exec(`DROP TRIGGER fail_move`); err != nil {
				t.Fatal(err)
			}
			cashRequest(t, f, "movements", supplyBody, 200)
			movementCounts(t, f, 1)
		})
	}
}

func TestHTTPCashMovementConcurrentRequestsOnlyMoveOnce(t *testing.T) {
	f := movementSetup(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", "/local/v1/cash/movements", supplyBody, f.token)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatalf("concorrencia %d", status)
		}
	}
	movementCounts(t, f, 1)
}

func TestHTTPCashMovementPersistsAfterReopen(t *testing.T) {
	f := movementSetup(t)
	first := cashRequest(t, f, "movements", supplyBody, 200)
	var dbPath string
	var seq int
	var dbName string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &dbName, &dbPath); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := localdb.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	actor := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	// New store must reference the reopened database, as required by RequireTx.
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	contracts, err := entitlementstore.New(reopened, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := register.MoveWithContract(context.Background(), reopened, contracts, actor, f.device, register.MovementInput{SessionID: "turn-one", OperationID: "supply-one", Kind: "supply", AmountCents: 500, Reason: "Troco inicial"})
	if err != nil || !result.Repeated || !bytes.Contains(first, []byte(result.MovementID)) {
		t.Fatalf("reabertura %+v %v", result, err)
	}
}

func TestHTTPCashMovementNilContractExpiryAndForeignDevice(t *testing.T) {
	f := movementSetup(t)
	actor := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	in := register.MovementInput{SessionID: "turn-one", OperationID: "supply-one", Kind: "supply", AmountCents: 500, Reason: "Troco inicial"}
	if _, err := register.MoveWithContract(context.Background(), f.db, nil, actor, f.device, in); !errors.Is(err, entitlementstore.ErrNotInstalled) {
		t.Fatalf("nil %v", err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	store, err := entitlementstore.New(f.db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign := f.device
	foreign.DeviceID = "foreign"
	if _, err := register.MoveWithContract(context.Background(), f.db, store, actor, foreign, in); !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("aparelho %v", err)
	}
	cashRequest(t, f, "movements", supplyBody, 200)
	expired, err := entitlementstore.New(f.db, verifier, func() time.Time { return time.Now().Add(48 * time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := register.MoveWithContract(context.Background(), f.db, expired, actor, f.device, in); err == nil {
		t.Fatal("contrato expirado repetiu")
	}
	movementCounts(t, f, 1)
}

func TestHTTPCashMovementOverflowAndForeignSession(t *testing.T) {
	f := movementSetup(t)
	if _, err := f.db.Exec(`UPDATE cash_sessions SET opening_cents=9223372036854775807`); err != nil {
		t.Fatal(err)
	}
	cashRequest(t, f, "movements", supplyBody, 409)
	cashRequest(t, f, "movements", string(bytes.ReplaceAll([]byte(supplyBody), []byte("turn-one"), []byte("foreign-turn"))), 409)
	movementCounts(t, f, 0)
}

func TestHTTPCashMovementFailedWriteRollsBackContractClock(t *testing.T) {
	f := movementSetup(t)
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(5 * time.Minute)
	store, err := entitlementstore.New(f.db, verifier, func() time.Time { return future })
	if err != nil {
		t.Fatal(err)
	}
	var before, after int64
	if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`CREATE TRIGGER block_audit BEFORE INSERT ON cash_adjustments BEGIN SELECT RAISE(ABORT,'test'); END`); err != nil {
		t.Fatal(err)
	}
	actor := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	_, err = register.MoveWithContract(context.Background(), f.db, store, actor, f.device, register.MovementInput{SessionID: "turn-one", OperationID: "test", Kind: "supply", AmountCents: 1, Reason: "Teste"})
	if err == nil {
		t.Fatal("falha ignorada")
	}
	if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&after); err != nil || before != after {
		t.Fatalf("relogio %d -> %d: %v", before, after, err)
	}
	movementCounts(t, f, 0)
}
