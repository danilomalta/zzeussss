package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/sale"
)

const cancelBody = `{"operation_id":"cancel-one","reason":"Cliente desistiu; mercadoria devolvida"}`

func cancellationSetup(t *testing.T) *stockHTTPFixture {
	t.Helper()
	f := salesSetup(t)
	salesRequest(t, f, salesInput(f), 201)
	return f
}

func cancelRequest(t *testing.T, f *stockHTTPFixture, body string, want int) sale.CancelResult {
	t.Helper()
	status, response := request(t, f.app, "POST", "/local/v1/sales/sale-one/cancel", body, f.token)
	if status != want {
		t.Fatalf("cancelamento: recebido=%d esperado=%d corpo=%s", status, want, response)
	}
	var result sale.CancelResult
	if want == 200 {
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func cancellationCounts(t *testing.T, f *stockHTTPFixture, cancellations, returns int) {
	t.Helper()
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM sale_cancellations`, cancellations},
		{`SELECT COUNT(*) FROM sale_cancellation_stock`, returns},
		{`SELECT COUNT(*) FROM outbox WHERE event_type='sale.cancelled'`, cancellations},
		{`SELECT COUNT(*) FROM cash_movements WHERE reason LIKE 'cancelamento:%'`, cancellations},
		{`SELECT COUNT(*) FROM stock_movements WHERE reason LIKE 'cancelamento:%'`, returns},
		{`SELECT COUNT(*) FROM sales`, 1},
		{`SELECT COUNT(*) FROM sale_operations`, 1},
		{`SELECT COUNT(*) FROM outbox WHERE event_type='sale.committed'`, 1},
	} {
		var count int
		if err := f.db.QueryRow(check.query).Scan(&count); err != nil || count != check.want {
			t.Fatalf("%s: got=%d want=%d err=%v", check.query, count, check.want, err)
		}
	}
}

func TestHTTPCancelSaleRestoresStockCashAndPreservesHistory(t *testing.T) {
	f := cancellationSetup(t)
	original := readReceipt(t, f, "sale-one")
	first := cancelRequest(t, f, cancelBody, 200)
	if first.Repeated || first.RefundedCents != 3198 || first.CancelledAt == "" || first.SaleID != "sale-one" {
		t.Fatalf("estorno: %+v", first)
	}
	again := cancelRequest(t, f, cancelBody, 200)
	if !again.Repeated || again.CancelledAt != first.CancelledAt {
		t.Fatal("repeticao nao preservou recibo")
	}
	cancellationCounts(t, f, 1, 1)
	assertStockBalance(t, f, f.shelf, 5000)
	after := readReceipt(t, f, "sale-one")
	if after.Status != "cancelled" || after.TotalCents != original.TotalCents || after.CommittedAt != original.CommittedAt ||
		after.Items[0] != original.Items[0] || after.Payments[0].AmountCents != original.Payments[0].AmountCents ||
		after.Payments[0].Status != "reversed" || after.FiscalAuthorized || after.Cancellation == nil ||
		after.Cancellation.ActorID != f.owner.OwnerID || after.Cancellation.RefundedCents != 3198 {
		t.Fatalf("historico: %+v", after)
	}
	closed := cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"close-after-refund","declared_cents":1000}`, 200)
	if !bytes.Contains(closed, []byte(`"expected_cents":1000`)) || !bytes.Contains(closed, []byte(`"difference_cents":0`)) {
		t.Fatalf("caixa: %s", closed)
	}
	cancelRequest(t, f, cancelBody, 200)
	salesRequest(t, f, salesInput(f), 409)
	cancellationCounts(t, f, 1, 1)
}

func TestHTTPCancelSaleManagerCanRefundAnotherOperatorsSale(t *testing.T) {
	f := cancellationSetup(t)
	for _, query := range []string{
		`INSERT INTO identities VALUES ('refund-manager','Gerente','now')`,
	} {
		if _, err := f.db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.db.Exec(`INSERT INTO memberships VALUES (?,'refund-manager','manager','active','now')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,'refund-manager',?)`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	owner := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	if err := localauth.SetPassword(context.Background(), f.db, owner, f.device, "refund-manager", "senha-forte-de-teste-1"); err != nil {
		t.Fatal(err)
	}
	managerToken := loginToken(t, f.app, "refund-manager")
	originalToken := f.token
	f.token = managerToken
	cancelRequest(t, f, cancelBody, 200)
	f.token = originalToken
	receipt := readReceipt(t, f, "sale-one")
	if receipt.Cancellation == nil || receipt.Cancellation.ActorID != "refund-manager" {
		t.Fatal("gerente nao registrado")
	}
	cancellationCounts(t, f, 1, 1)
}

func TestHTTPCancelSaleRejectsCashierAndRevocations(t *testing.T) {
	for _, scenario := range []string{"cashier", "owner-revoked", "device-revoked", "manager-without-store"} {
		t.Run(scenario, func(t *testing.T) {
			f := cancellationSetup(t)
			want := 403
			switch scenario {
			case "cashier":
				if _, err := f.db.Exec(`INSERT INTO membership_stores VALUES (?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); err != nil {
					t.Fatal(err)
				}
				if _, err := f.db.Exec(`UPDATE memberships SET role='cashier' WHERE identity_id=?`, f.owner.OwnerID); err != nil {
					t.Fatal(err)
				}
			case "owner-revoked":
				want = 401
				if _, err := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id=?`, f.owner.OwnerID); err != nil {
					t.Fatal(err)
				}
			case "device-revoked":
				want = 401
				if _, err := f.db.Exec(`UPDATE device_pairings SET status='revoked' WHERE device_id=?`, f.device.DeviceID); err != nil {
					t.Fatal(err)
				}
			case "manager-without-store":
				want = 401
				if _, err := f.db.Exec(`UPDATE memberships SET role='manager' WHERE identity_id=?`, f.owner.OwnerID); err != nil {
					t.Fatal(err)
				}
			}
			cancelRequest(t, f, cancelBody, want)
			cancellationCounts(t, f, 0, 0)
			assertCancellationUnchanged(t, f)
		})
	}
}

func assertCancellationUnchanged(t *testing.T, f *stockHTTPFixture) {
	t.Helper()
	var status string
	if err := f.db.QueryRow(`SELECT status FROM sales WHERE id='sale-one'`).Scan(&status); err != nil || status != "committed" {
		t.Fatalf("venda mudou: %s %v", status, err)
	}
	var paid string
	if err := f.db.QueryRow(`SELECT status FROM sale_payments WHERE sale_id='sale-one'`).Scan(&paid); err != nil || paid != "confirmed" {
		t.Fatalf("pagamento mudou: %s %v", paid, err)
	}
	var balance, cash int64
	if err := f.db.QueryRow(`SELECT SUM(quantity_milli) FROM stock_movements WHERE location_id=?`, f.shelf).Scan(&balance); err != nil || balance != 3000 {
		t.Fatalf("estoque mudou: %d %v", balance, err)
	}
	if err := f.db.QueryRow(`SELECT SUM(amount_cents) FROM cash_movements`).Scan(&cash); err != nil || cash != 3198 {
		t.Fatalf("caixa mudou: %d %v", cash, err)
	}
}

func TestHTTPCancelSaleRequiresContractSessionAndOpenDrawer(t *testing.T) {
	t.Run("session", func(t *testing.T) {
		f := cancellationSetup(t)
		status, _ := request(t, f.app, "POST", "/local/v1/sales/sale-one/cancel", cancelBody, "")
		if status != 401 {
			t.Fatalf("sessao: %d", status)
		}
		cancellationCounts(t, f, 0, 0)
	})
	t.Run("missing-module", func(t *testing.T) {
		f := cancellationSetup(t)
		f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 3, []modules.ID{modules.Core, modules.Inventory}), 200)
		cancelRequest(t, f, cancelBody, 403)
		cancellationCounts(t, f, 0, 0)
	})
	t.Run("nil-verifier", func(t *testing.T) {
		f := cancellationSetup(t)
		app, err := New(f.db, f.device)
		if err != nil {
			t.Fatal(err)
		}
		f.app = app
		cancelRequest(t, f, cancelBody, 503)
		actor := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
		_, err = sale.CancelWithContract(context.Background(), f.db, nil, actor, f.device, sale.CancelInput{OperationID: "cancel-one", SaleID: "sale-one", Reason: "Devolucao"})
		if !errors.Is(err, entitlementstore.ErrNotInstalled) {
			t.Fatalf("contrato nil: %v", err)
		}
		cancellationCounts(t, f, 0, 0)
	})
	t.Run("closed-original-drawer", func(t *testing.T) {
		f := cancellationSetup(t)
		cashRequest(t, f.httpContractFixture, "close", `{"session_id":"turn-one","operation_id":"end-original","declared_cents":4198}`, 200)
		cashRequest(t, f.httpContractFixture, "open", `{"session_id":"turn-two","opening_cents":1000}`, 201)
		cancelRequest(t, f, cancelBody, 409)
		cancellationCounts(t, f, 0, 0)
	})
}

func TestHTTPCancelSaleConflictsDoNotDuplicateRefund(t *testing.T) {
	f := cancellationSetup(t)
	cancelRequest(t, f, `{"operation_id":"sale-operation-one","reason":"Devolucao"}`, 409)
	status, _ := request(t, f.app, "POST", "/local/v1/sales/missing/cancel", cancelBody, f.token)
	if status != 404 {
		t.Fatalf("inexistente: %d", status)
	}
	cancelRequest(t, f, cancelBody, 200)
	cancelRequest(t, f, strings.Replace(cancelBody, "cancel-one", "cancel-two", 1), 409)
	cancelRequest(t, f, strings.Replace(cancelBody, "Cliente desistiu", "Outro motivo", 1), 409)
	input := salesInput(f)
	input.OperationID = "cancel-one"
	input.SaleID = "new-sale"
	salesRequest(t, f, input, 409)
	cancellationCounts(t, f, 1, 1)
}

func TestHTTPCancelSaleRejectsAmbiguousBodyAndClientContext(t *testing.T) {
	f := cancellationSetup(t)
	for _, body := range []string{
		`{}`, `null`, cancelBody + ` {}`, `{"operation_id":"x","operation_id":"y","reason":"a"}`,
		`{"operation_id":null,"reason":"a"}`, `{"operation_id":7,"reason":"a"}`,
		`{"operation_id":" x","reason":"a"}`, `{"operation_id":"x","reason":" "}`,
		`{"operation_id":"x","reason":null}`, `{"operation_id":"x","reason":"a","tenant_id":"foreign"}`,
		`{"operation_id":"x","reason":"a","identity_id":"manager"}`,
		`{"operation_id":"x","reason":"a","amount_cents":1}`,
		`{"operation_id":"x","reason":"` + strings.Repeat("a", 501) + `"}`,
	} {
		cancelRequest(t, f, body, 400)
	}
	cancellationCounts(t, f, 0, 0)
	assertCancellationUnchanged(t, f)
}

func TestHTTPCancelSaleFailureRollsBackAllEffectsAndClock(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER refund_fail BEFORE INSERT ON cash_movements WHEN NEW.amount_cents<0 BEGIN SELECT RAISE(ABORT,'test_failure'); END`,
		`CREATE TRIGGER refund_fail BEFORE INSERT ON sale_cancellations BEGIN SELECT RAISE(ABORT,'test_failure'); END`,
		`CREATE TRIGGER refund_fail BEFORE INSERT ON sale_cancellation_stock BEGIN SELECT RAISE(ABORT,'test_failure'); END`,
		`CREATE TRIGGER refund_fail BEFORE UPDATE ON sale_payments BEGIN SELECT RAISE(ABORT,'test_failure'); END`,
		`CREATE TRIGGER refund_fail BEFORE UPDATE ON sales BEGIN SELECT RAISE(ABORT,'test_failure'); END`,
		`CREATE TRIGGER refund_fail BEFORE INSERT ON outbox WHEN NEW.event_type='sale.cancelled' BEGIN SELECT RAISE(ABORT,'test_failure'); END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			f := cancellationSetup(t)
			observed := time.Now().Unix() - 30
			if _, err := f.db.Exec(`UPDATE module_contract_state SET last_observed_unix=?`, observed); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			cancelRequest(t, f, cancelBody, 500)
			cancellationCounts(t, f, 0, 0)
			assertCancellationUnchanged(t, f)
			var after int64
			if err := f.db.QueryRow(`SELECT last_observed_unix FROM module_contract_state`).Scan(&after); err != nil || after != observed {
				t.Fatalf("relogio: %d %v", after, err)
			}
		})
	}
}

func TestHTTPCancelSaleConcurrentRequestsDoNotDuplicate(t *testing.T) {
	f := cancellationSetup(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", "/local/v1/sales/sale-one/cancel", cancelBody, f.token)
			statuses <- status
		}()
	}
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != 200 {
			t.Fatalf("concorrencia: %d", status)
		}
	}
	cancellationCounts(t, f, 1, 1)
	assertStockBalance(t, f, f.shelf, 5000)
}

func TestHTTPCancelSaleSurvivesReopenAndReplayRequiresAuthorization(t *testing.T) {
	f := cancellationSetup(t)
	first := cancelRequest(t, f, cancelBody, 200)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil || path == "" {
		t.Fatalf("arquivo temporario: %v", err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	f.db, f.app = db, app
	repeated := cancelRequest(t, f, cancelBody, 200)
	if !repeated.Repeated || repeated.CancelledAt != first.CancelledAt {
		t.Fatal("recibo mudou apos reabrir")
	}
	cancellationCounts(t, f, 1, 1)
	if _, err := f.db.Exec(`UPDATE memberships SET status='revoked' WHERE identity_id=?`, f.owner.OwnerID); err != nil {
		t.Fatal(err)
	}
	cancelRequest(t, f, cancelBody, 401)
	cancellationCounts(t, f, 1, 1)
}

func TestHTTPCancelSaleMissingFundsOrStockOverflowCannotRefund(t *testing.T) {
	t.Run("cash", func(t *testing.T) {
		f := cancellationSetup(t)
		if _, err := f.db.Exec(`INSERT INTO cash_movements VALUES ('cash-withdrawal',?,?,'turn-one',-2000,'retirada de teste','now')`, f.owner.TenantID, f.owner.StoreID); err != nil {
			t.Fatal(err)
		}
		cancelRequest(t, f, cancelBody, 409)
		cancellationCounts(t, f, 0, 0)
		assertStockBalance(t, f, f.shelf, 3000)
	})
	t.Run("stock", func(t *testing.T) {
		f := cancellationSetup(t)
		if _, err := f.db.Exec(`INSERT INTO stock_movements VALUES ('extra-stock',?,?,?,?,?,?,'test','now')`, f.owner.TenantID, f.owner.StoreID, f.device.DeviceID, f.product, f.shelf, int64(math.MaxInt64-3000)); err != nil {
			t.Fatal(err)
		}
		cancelRequest(t, f, cancelBody, 409)
		cancellationCounts(t, f, 0, 0)
		var cash int64
		if err := f.db.QueryRow(`SELECT SUM(amount_cents) FROM cash_movements`).Scan(&cash); err != nil || cash != 3198 {
			t.Fatalf("estorno parcial: %d %v", cash, err)
		}
	})
}

func TestHTTPCancelSaleRestoresEachRepeatedProductLineOnce(t *testing.T) {
	f := salesSetup(t)
	in := salesInput(f)
	in.Items = []sale.Item{{ProductID: f.product, LocationID: f.shelf, QuantityMilli: 1000}, {ProductID: f.product, LocationID: f.shelf, QuantityMilli: 1000}}
	salesRequest(t, f, in, 201)
	cancelRequest(t, f, cancelBody, 200)
	cancelRequest(t, f, cancelBody, 200)
	cancellationCounts(t, f, 1, 2)
	assertStockBalance(t, f, f.shelf, 5000)
}

func TestHTTPCancelSaleForeignStoreAndDeviceCannotRefund(t *testing.T) {
	f := cancellationSetup(t)
	actor := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	contracts, err := entitlementstore.New(f.db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign := f.device
	foreign.TenantID = "other-company"
	_, err = sale.CancelWithContract(context.Background(), f.db, contracts, actor, foreign, sale.CancelInput{OperationID: "x", SaleID: "sale-one", Reason: "Devolucao"})
	if !errors.Is(err, identity.ErrDenied) {
		t.Fatalf("outra empresa: %v", err)
	}
	if _, err = f.db.Exec(`INSERT INTO stores VALUES (?,'other-store','Outra loja')`, f.owner.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE sales SET store_id='other-store' WHERE id='sale-one'`); err == nil {
		t.Fatal("FK aceitou loja alheia ao caixa")
	}
	// The approved device is still authenticated, but cannot cancel a sale attributed to another device.
	if _, err = f.db.Exec(`INSERT INTO devices VALUES (?,?,'other-device','Outro aparelho')`, f.owner.TenantID, f.owner.StoreID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`UPDATE sales SET device_id='other-device' WHERE id='sale-one'`); err != nil {
		t.Fatal(err)
	}
	cancelRequest(t, f, cancelBody, 404)
	cancellationCounts(t, f, 0, 0)
}

func TestHTTPCancelSaleSilentIgnoredWriteAlsoRollsBack(t *testing.T) {
	for _, query := range []string{
		`CREATE TRIGGER ignore_refund BEFORE UPDATE ON sales BEGIN SELECT RAISE(IGNORE); END`,
		`CREATE TRIGGER ignore_refund BEFORE INSERT ON outbox WHEN NEW.event_type='sale.cancelled' BEGIN SELECT RAISE(IGNORE); END`,
	} {
		t.Run(query, func(t *testing.T) {
			f := cancellationSetup(t)
			if _, err := f.db.Exec(query); err != nil {
				t.Fatal(err)
			}
			cancelRequest(t, f, cancelBody, 409)
			cancellationCounts(t, f, 0, 0)
			assertCancellationUnchanged(t, f)
		})
	}
}

func TestHTTPCancelSaleExpiredContractCannotReplayButHistoryIsReadable(t *testing.T) {
	f := cancellationSetup(t)
	cancelRequest(t, f, cancelBody, 200)
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	store, err := entitlementstore.New(f.db, verifier, func() time.Time { return time.Now().Add(2 * time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	actor := identity.Scope{IdentityID: f.owner.OwnerID, TenantID: f.owner.TenantID, StoreID: f.owner.StoreID}
	_, err = sale.CancelWithContract(context.Background(), f.db, store, actor, f.device, sale.CancelInput{OperationID: "cancel-one", SaleID: "sale-one", Reason: "Cliente desistiu; mercadoria devolvida"})
	if err == nil {
		t.Fatal("contrato expirado autorizou replay")
	}
	receipt := readReceipt(t, f, "sale-one")
	if receipt.Cancellation == nil || receipt.Status != "cancelled" {
		t.Fatal("historico indisponivel")
	}
	cancellationCounts(t, f, 1, 1)
}
