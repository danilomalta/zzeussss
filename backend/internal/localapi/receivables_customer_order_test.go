package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/receivables"
	"titansystem-backend/internal/localdb/supplycycle"
)

func billingOrderSetup(t *testing.T) (*tradeHTTPFixture, *tradeHTTPFixture, string) {
	t.Helper()
	buyer, seller := newTradeHTTP(t, true), newTradeHTTP(t, false)
	decision := tradeDecide(t, buyer, seller)
	seller.f.install(t, signedTestContract(t, seller.f.private, seller.f.owner.TenantID, 101, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.POS, modules.Finance}), 200)
	seller.p.call("POST", "/local/v1/cash/open", map[string]any{"session_id": "ar-turn", "opening_cents": 5000}, 201, nil)
	var message string
	if e := seller.f.db.QueryRow(`SELECT id FROM company_trade_messages WHERE direction='received' AND kind='order' AND order_id=?`, decision.Message.OrderID).Scan(&message); e != nil {
		t.Fatal(e)
	}
	return buyer, seller, message
}
func billingOrderInput(t *testing.T, message, id string) receivables.CreateInput {
	t.Helper()
	var in receivables.CreateInput
	if e := json.Unmarshal([]byte(arBody), &in); e != nil {
		t.Fatal(e)
	}
	in.ID = id
	in.OperationID = id + "-create"
	in.DocumentReference = id + "-commercial"
	in.CustomerReference = "customer"
	in.CustomerOrder = &receivables.CustomerOrderInput{MessageID: message, TermsReference: "Documented negotiated terms 42", Confirmed: true}
	return in
}
func billingOrderJSON(t *testing.T, in receivables.CreateInput) string {
	t.Helper()
	b, e := json.Marshal(in)
	if e != nil {
		t.Fatal(e)
	}
	var body map[string]any
	if e = json.Unmarshal(b, &body); e != nil {
		t.Fatal(e)
	}
	installments := body["installments"].([]any)
	for _, v := range installments {
		delete(v.(map[string]any), "received_cents")
	}
	b, e = json.Marshal(body)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func TestHTTPReceivablesCustomerOrderRealFlowAndRestore(t *testing.T) {
	buyer, seller, message := billingOrderSetup(t)
	var origin supplycycle.CustomerBillingOrder
	preview := seller.p.call("GET", arPath+"/customer-orders/"+message, nil, 200, &origin)
	domainSchemaFile(t, "receivables.openapi.json", "CustomerBillingOrder", preview)
	if origin.CustomerReference != "customer" || origin.AcceptedMilli != 3000 || origin.CustomerTenantID != buyer.f.owner.TenantID {
		t.Fatal("wrong authentic origin", origin)
	}
	in := billingOrderInput(t, message, "invoice-one")
	var first, recovered receivables.Result
	domainSchemaFile(t, "receivables.openapi.json", "CreateInput", []byte(billingOrderJSON(t, in)))
	created := seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 201, &first)
	domainSchemaFile(t, "receivables.openapi.json", "Result", created)
	seller.p.call("GET", arPath+"/operations/"+in.OperationID, nil, 200, &recovered)
	if !recovered.Repeated || !reflect.DeepEqual(first.Document, recovered.Document) || first.Document.CustomerOrder.Source.MessageID != message {
		t.Fatal("lost response did not preserve original invoice")
	}
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 200, &recovered)
	// Billing does not sell again, reserve stock or collect cash.
	for _, table := range []string{"sales", "stock_movements", "cash_adjustments"} {
		var n int
		if e := seller.f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); e != nil || n != 0 {
			t.Fatal("billing fabricated effect", table, n, e)
		}
	}
	var partial, final receivables.Result
	seller.p.call("POST", arPath+"/invoice-one/receive-cash", json.RawMessage(arReceive("order-cash-1", "one", 300, 0)), 200, &partial)
	if partial.Document.ReceivedCents != 300 || partial.Document.OutstandingCents != 700 {
		t.Fatal("partial", partial)
	}
	seller.p.call("POST", arPath+"/invoice-one/receive-cash", json.RawMessage(arReceive("order-cash-1", "one", 300, 0)), 200, &recovered)
	seller.p.call("POST", arPath+"/invoice-one/receive-cash", json.RawMessage(arReceive("order-cash-2", "one", 300, 300)), 200, nil)
	seller.p.call("POST", arPath+"/invoice-one/receive-cash", json.RawMessage(arReceive("order-cash-3", "two", 400, 0)), 200, &final)
	if final.Document.State != "settled" || final.Document.OutstandingCents != 0 || len(final.Document.Receipts) != 3 {
		t.Fatal("not settled", final)
	}
	var cash, count int64
	if e := seller.f.db.QueryRow(`SELECT sum(amount_cents),count(*) FROM cash_adjustments`).Scan(&cash, &count); e != nil || cash != 1000 || count != 3 {
		t.Fatal("duplicate collection", cash, count, e)
	}
	seller.p.call("POST", arPath+"/invoice-one/cancel", receivables.CancelInput{OperationID: "cancel-paid", Reason: "Must not implicitly refund"}, 409, nil)
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "order.tytbak")
	ctx := context.Background()
	if e := backup.Create(ctx, seller.f.db, seller.f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Restore(ctx, archive, restored, seller.f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	actor := identity.Scope{TenantID: seller.f.owner.TenantID, StoreID: seller.f.owner.StoreID, IdentityID: seller.f.owner.OwnerID}
	doc, e := receivables.Read(ctx, db, actor, seller.f.device, in.ID)
	if e != nil || !reflect.DeepEqual(doc, final.Document) {
		t.Fatal("origin/settlement lost on restore", e)
	}
}
func TestHTTPReceivablesCustomerOrderConcurrentBilling(t *testing.T) {
	_, seller, message := billingOrderSetup(t)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			in := billingOrderInput(t, message, fmt.Sprintf("concurrent-%d", i))
			code, _ := request(t, seller.f.app, "POST", arPath, billingOrderJSON(t, in), seller.f.token)
			codes <- code
		}(i)
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for code := range codes {
		if code == 201 {
			success++
		} else if code == 409 {
			conflict++
		} else {
			t.Fatal("unexpected status", code)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	for _, table := range []string{"receivable_documents", "receivable_customer_orders", "receivable_operations"} {
		var n int
		if e := seller.f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); e != nil || n != 1 {
			t.Fatal("duplicate invoice/origin", table, n, e)
		}
	}
}
func TestHTTPReceivablesCustomerOrderCancelRebillAndHistory(t *testing.T) {
	_, seller, message := billingOrderSetup(t)
	first := billingOrderInput(t, message, "first")
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, first)), 201, nil)
	seller.p.call("POST", arPath+"/first/cancel", receivables.CancelInput{OperationID: "cancel-first", Reason: "Wrong commercial terms; no cash received"}, 200, nil)
	second := billingOrderInput(t, message, "second")
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, second)), 201, nil)
	var old receivables.Document
	seller.p.call("GET", arPath+"/first", nil, 200, &old)
	if old.State != "cancelled" || old.CustomerOrder == nil || old.CustomerOrder.Source.MessageID != message {
		t.Fatal("cancel lost origin/audit")
	}
	var active, total int
	if e := seller.f.db.QueryRow(`SELECT sum(active),count(*) FROM receivable_customer_orders`).Scan(&active, &total); e != nil || active != 1 || total != 2 {
		t.Fatal(active, total, e)
	}
	if _, e := seller.f.db.Exec(`UPDATE receivable_customer_orders SET active=1 WHERE document_id='first'`); e == nil {
		t.Fatal("cancelled origin reactivated")
	}
}
func TestHTTPReceivablesCustomerOrderRollback(t *testing.T) {
	for _, stage := range []string{"origin", "origin-ignore", "audit", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			_, seller, message := billingOrderSetup(t)
			in := billingOrderInput(t, message, "rollback")
			if stage == "cancel" {
				seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 201, nil)
			}
			table := "receivable_events"
			if stage == "origin" || stage == "origin-ignore" {
				table = "receivable_customer_orders"
			}
			trigger := `CREATE TRIGGER fail_billing BEFORE INSERT ON ` + table + ` BEGIN SELECT RAISE(ABORT,'synthetic billing failure');END;`
			if stage == "origin-ignore" {
				trigger = `CREATE TRIGGER fail_billing BEFORE INSERT ON ` + table + ` BEGIN SELECT RAISE(IGNORE);END;`
			}
			if _, e := seller.f.db.Exec(trigger); e != nil {
				t.Fatal(e)
			}
			if stage == "cancel" {
				seller.p.call("POST", arPath+"/rollback/cancel", receivables.CancelInput{OperationID: "cancel-rollback", Reason: "Synthetic cancellation"}, 500, nil)
				var active int
				if e := seller.f.db.QueryRow(`SELECT active FROM receivable_customer_orders`).Scan(&active); e != nil || active != 1 {
					t.Fatal("cancel released origin before commit", active, e)
				}
				var doc receivables.Document
				seller.p.call("GET", arPath+"/rollback", nil, 200, &doc)
				if doc.State != "open" {
					t.Fatal("partial cancellation")
				}
			} else {
				want := 500
				if stage == "origin-ignore" {
					want = 409
				}
				seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), want, nil)
				for _, tab := range []string{"receivable_documents", "receivable_customer_orders", "receivable_installments", "receivable_operations"} {
					var n int
					if e := seller.f.db.QueryRow(`SELECT count(*) FROM ` + tab).Scan(&n); e != nil || n != 0 {
						t.Fatal("partial billing commit", tab, n, e)
					}
				}
				seller.p.call("GET", arPath+"/operations/"+in.OperationID, nil, 404, nil)
			}
		})
	}
}
func TestHTTPReceivablesCustomerOrderRejectsContextAndInventedTerms(t *testing.T) {
	buyer, seller, message := billingOrderSetup(t)
	in := billingOrderInput(t, message, "invalid")
	in.CustomerOrder.Confirmed = false
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 400, nil)
	in.CustomerOrder.Confirmed = true
	in.CustomerReference = "someone-else"
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 409, nil)
	in.CustomerReference = "customer"
	// The same signed message is outgoing at the buyer: not a customer receivable there.
	code, _ := request(t, buyer.f.app, "GET", arPath+"/customer-orders/"+message, "", buyer.f.token)
	if code != 409 {
		t.Fatal("purchase became receivable", code)
	}
	other := arSetup(t)
	code, _ = request(t, other.app, "GET", arPath+"/customer-orders/"+message, "", other.token)
	if code != 404 {
		t.Fatal("cross-tenant origin", code)
	}
	body := billingOrderJSON(t, in)
	body = strings.Replace(body, `"confirmed":true`, `"confirmed":true,"total_cents":1`, 1)
	code, _ = request(t, seller.f.app, "POST", arPath, body, seller.f.token)
	if code != 400 {
		t.Fatal("forged origin fields", code)
	}
	policyPerson(t, seller.f.db, seller.f.owner, "billing-cashier", "cashier")
	cashier := loginToken(t, seller.f.app, "billing-cashier")
	code, _ = request(t, seller.f.app, "POST", arPath, billingOrderJSON(t, in), cashier)
	if code != 403 {
		t.Fatal("cashier billed order", code)
	}
	seller.f.install(t, signedTestContract(t, seller.f.private, seller.f.owner.TenantID, 102, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 403, nil)
}
func TestHTTPReceivablesCustomerOrderRequiresSignedConfirmation(t *testing.T) {
	buyer, seller := newTradeHTTP(t, true), newTradeHTTP(t, false)
	offer := tradeOffer(t, buyer, seller)
	seller.f.install(t, signedTestContract(t, seller.f.private, seller.f.owner.TenantID, 101, []modules.ID{modules.Core, modules.Inventory, modules.Orders, modules.POS, modules.Finance}), 200)
	in := billingOrderInput(t, offer.Message.ID, "unconfirmed")
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 409, nil)
	// Existing immutable-message protection must reject tampering before billing.
	if _, e := seller.f.db.Exec(`UPDATE company_trade_messages SET envelope_json='{}' WHERE direction='received'`); e == nil {
		t.Fatal("signed history accepted tampering")
	}
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 409, nil)
	seller.p.call("POST", "/local/v1/company-trade/orders/"+offer.Message.ID+"/decision", supplycycle.TradeDecisionInput{OperationID: "decline-no-billing", Decision: "declined", Reason: "No confirmed commercial order"}, 200, nil)
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 409, nil)
}

func TestReceivablesCustomerOrderFrontendBridge(t *testing.T) {
	script := os.Getenv("TITAN_RECEIVABLES_BRIDGE")
	if script == "" {
		t.Skip("opt in: TITAN_RECEIVABLES_BRIDGE=frontend-web/tests/receivables.bridge.mjs")
	}
	_, seller, message := billingOrderSetup(t)
	node, e := exec.LookPath("node")
	if e != nil {
		t.Fatal(e)
	}
	session, _ := json.Marshal(map[string]string{"tenant_id": seller.f.owner.TenantID, "store_id": seller.f.owner.StoreID, "identity_id": seller.f.owner.OwnerID, "device_id": seller.f.device.DeviceID})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, script)
	command.Env = append(os.Environ(), "TITAN_BRIDGE_URL="+seller.p.service.url, "TITAN_BRIDGE_TOKEN="+seller.f.token, "TITAN_BRIDGE_SESSION="+string(session), "TITAN_BRIDGE_ORDER="+message)
	output, e := command.CombinedOutput()
	if e != nil {
		t.Fatalf("customer order real frontend bridge: %v\n%s", e, output)
	}
	t.Log(string(output))
	var amount, count int64
	if e = seller.f.db.QueryRow(`SELECT sum(amount_cents),count(*) FROM cash_adjustments`).Scan(&amount, &count); e != nil || amount != 1000 || count != 2 {
		t.Fatal("source billing duplicated cash", amount, count, e)
	}
	var n int
	if e = seller.f.db.QueryRow(`SELECT count(*) FROM receivable_customer_orders WHERE active=1`).Scan(&n); e != nil || n != 1 {
		t.Fatal("source not linked", n, e)
	}
}

func TestHTTPReceivablesCustomerOrderAuthorizedSecondStoreCannotAdoptOrigin(t *testing.T) {
	_, seller, message := billingOrderSetup(t)
	in := billingOrderInput(t, message, "original-store-invoice")
	seller.p.call("POST", arPath, json.RawMessage(billingOrderJSON(t, in)), 201, nil)
	db := seller.f.db
	tenant := seller.f.owner.TenantID
	if _, e := db.Exec(`INSERT INTO stores VALUES(?, 'second-store','Synthetic second store')`, tenant); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`INSERT INTO devices VALUES(?,'second-store','second-device','Synthetic second device')`, tenant); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`INSERT INTO device_pairings SELECT tenant_id,'second-store','second-device',public_key,challenge,challenge_expires_unix,status,requested_by,proof_unix,approved_by,approved_unix,revoked_by,revoked_unix FROM device_pairings WHERE tenant_id=? AND device_id=?`, tenant, seller.f.device.DeviceID); e != nil {
		t.Fatal(e)
	}
	device := identity.DeviceContext{TenantID: tenant, StoreID: "second-store", DeviceID: "second-device"}
	verifier, e := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": seller.f.private.Public().(ed25519.PublicKey)})
	if e != nil {
		t.Fatal(e)
	}
	app, e := NewWithVerifier(db, device, verifier)
	if e != nil {
		t.Fatal(e)
	}
	token := loginToken(t, app, seller.f.owner.OwnerID)
	for _, path := range []string{arPath + "/original-store-invoice", arPath + "/operations/" + in.OperationID, arPath + "/customer-orders/" + message} {
		code, _ := request(t, app, "GET", path, "", token)
		if code != 404 {
			t.Fatal("authorized other store read original data", path, code)
		}
	}
	code, raw := request(t, app, "GET", arPath, "", token)
	var page receivables.Page
	if code != 200 || json.Unmarshal(raw, &page) != nil || page.Total != 0 {
		t.Fatal("cross-store report", code, page.Total)
	}
	code, _ = request(t, app, "POST", arPath, billingOrderJSON(t, billingOrderInput(t, message, "foreign-invoice")), token)
	if code != 404 {
		t.Fatal("other store adopted source", code)
	}
	code, _ = request(t, app, "POST", arPath+"/original-store-invoice/receive-cash", arReceive("foreign-cash", "one", 100, 0), token)
	if code != 404 {
		t.Fatal("cross-store cash write", code)
	}
	code, _ = request(t, app, "GET", arPath+"/original-store-invoice", "", seller.f.token)
	if code != 401 {
		t.Fatal("old store token valid on new device", code)
	}
	var amount int
	if e = db.QueryRow(`SELECT count(*) FROM cash_adjustments`).Scan(&amount); e != nil || amount != 0 {
		t.Fatal("foreign command collected money", amount, e)
	}
}
