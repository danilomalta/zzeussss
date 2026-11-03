package localapi

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/receivables"
	"titansystem-backend/internal/localdb/retail"
)

func retailServiceFor(f *httpContractFixture) retail.Service {
	v, _ := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	store, _ := entitlementstore.New(f.db, v, nil)
	return retail.Service{DB: f.db, Contracts: store, Actor: identity.Scope{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, IdentityID: f.owner.OwnerID}, Device: f.device, Token: f.token}
}
func retailEnable(t *testing.T, f *httpContractFixture, limit int64) {
	t.Helper()
	s := retailServiceFor(f)
	ctx := context.Background()
	if e := s.Configure(ctx, "retail-config", retail.Config{CreditEnabled: true, RefundEnabled: true, CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}}); e != nil {
		t.Fatal(e)
	}
	if e := s.SaveCustomer(ctx, "retail-customer", retail.Customer{ID: "customer-external", Name: "Synthetic Customer", CPF: "", Phone: "synthetic-phone", ConsentReference: "consent-explicit", CreditEnabled: true, CreditLimitCents: limit}); e != nil {
		t.Fatal(e)
	}
}
func TestRetailConcurrentConfiguration(t *testing.T) {
	f := arSetup(t)
	s := retailServiceFor(f)
	ctx := context.Background()
	const n = 6
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- s.Configure(ctx, fmt.Sprintf("cfg-%d", i), retail.Config{CreditRoles: []string{"owner"}, RefundRoles: []string{"owner"}, PointsRoles: []string{"owner"}})
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, retail.ErrConflict) {
			t.Fatalf("not revision conflict: %v", e)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
}
func TestRetailPrivacyKeepsFinancialLedger(t *testing.T) {
	f := arSetup(t)
	retailEnable(t, f, 5000)
	arResult(t, f, arPath, arBody, 201)
	s := retailServiceFor(f)
	ctx := context.Background()
	out, e := s.Privacy(ctx, "export", "customer-external", 1, false)
	if e != nil || out.Name != "Synthetic Customer" {
		t.Fatal(out, e)
	}
	out, e = s.Privacy(ctx, "anonymize", "customer-external", 1, true)
	if e != nil || out.Name != "" || out.Phone != "" || !out.Anonymized {
		t.Fatal(out, e)
	}
	out, e = s.Privacy(ctx, "export", "customer-external", 1, false)
	if e != nil || out.Name != "" {
		t.Fatal("export resurrected personal data", e)
	}
	var n int
	f.db.QueryRow(`SELECT count(*) FROM receivable_documents`).Scan(&n)
	if n != 1 {
		t.Fatal("ledger removed")
	}
	rows, e := f.db.Query(`SELECT request_hash,action,subject_id FROM retail_operations`)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for rows.Next() {
		var a, b, c string
		rows.Scan(&a, &b, &c)
		if strings.Contains(a+b+c, "Synthetic Customer") || strings.Contains(a+b+c, "synthetic-phone") {
			t.Fatal("PII in audit")
		}
	}
}
func TestRetailCreditSaleAtomicWithoutCash(t *testing.T) {
	f := salesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
	retailEnable(t, f.httpContractFixture, 5000)
	input := receivables.CreateInput{OperationID: "credit-op", ID: "credit-doc", CustomerReference: "customer-external", DocumentReference: "credit-1", Description: "Synthetic credit", IssuedOn: "2026-11-03", TotalCents: 3198, Installments: []receivables.Installment{{ID: "one", DueOn: "2026-12-01", AmountCents: 3198}}, CreditSale: &receivables.CreditSaleInput{SaleID: "credit-sale", SessionID: "turn-one", Items: salesInput(f).Items}}
	body, _ := json.Marshal(input)
	body = []byte(strings.ReplaceAll(string(body), `,"received_cents":0`, ``))
	got := arResult(t, f.httpContractFixture, arPath, string(body), 201)
	if code, raw := request(t, f.app, "POST", planPath+"/credit-sale/fiscal/reprocess", "{}", f.token); code != 409 || !strings.Contains(string(raw), "credit_fiscal_payment_not_mapped") {
		t.Fatal("credit fiscal not refused", code)
	}
	if got.Document.CreditSaleID != "credit-sale" {
		t.Fatal(got)
	}
	arResult(t, f.httpContractFixture, arPath, string(body), 200)
	for _, table := range []string{"cash_movements", "sale_payments"} {
		var n int
		if e := f.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal("credit became cash", table, n, e)
		}
	}
	assertStockBalance(t, f, f.shelf, 3000)
	input.ID = "credit-doc-2"
	input.OperationID = "credit-op-2"
	input.CreditSale.SaleID = "credit-sale-2"
	input.DocumentReference = "credit-2"
	body, _ = json.Marshal(input)
	body = []byte(strings.ReplaceAll(string(body), `,"received_cents":0`, ``))
	arResult(t, f.httpContractFixture, arPath, string(body), 409)
	var count int
	f.db.QueryRow(`SELECT count(*) FROM sales`).Scan(&count)
	if count != 1 {
		t.Fatal("failed credit persisted")
	}
}
func TestRetailConcurrentReceiptReversal(t *testing.T) {
	f := arSetup(t)
	retailEnable(t, f, 5000)
	arResult(t, f, arPath, arBody, 201)
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("paid", "one", 600, 0), 200)
	s := retailServiceFor(f)
	ctx := context.Background()
	const n = 6
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := receivables.ReverseCash(ctx, f.db, s.Contracts, s.Actor, f.device, f.token, "bill", receivables.ReversalInput{OperationID: fmt.Sprintf("reverse-%d", i), OriginalOperationID: "paid", OriginalDeviceID: f.device.DeviceID, SessionID: "ar-turn", AmountCents: 300, Reason: "Explicit correction"})
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, receivables.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
	doc, e := receivables.Read(ctx, f.db, s.Actor, f.device, "bill")
	if e != nil || doc.ReceivedCents != 300 || doc.ReversedCents != 300 || doc.OutstandingCents != 700 {
		t.Fatal(doc, e)
	}
}
func TestRetailReversalRollback(t *testing.T) {
	f := arSetup(t)
	retailEnable(t, f, 5000)
	arResult(t, f, arPath, arBody, 201)
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("paid", "one", 600, 0), 200)
	_, e := f.db.Exec(`CREATE TRIGGER stop_reversal BEFORE INSERT ON receivable_cash_reversals BEGIN SELECT RAISE(ABORT,'synthetic rollback'); END`)
	if e != nil {
		t.Fatal(e)
	}
	s := retailServiceFor(f)
	_, e = receivables.ReverseCash(context.Background(), f.db, s.Contracts, s.Actor, f.device, f.token, "bill", receivables.ReversalInput{OperationID: "rollback", OriginalOperationID: "paid", OriginalDeviceID: f.device.DeviceID, SessionID: "ar-turn", AmountCents: 100, Reason: "Explicit correction"})
	if e == nil {
		t.Fatal("expected rollback")
	}
	var n int
	if e = f.db.QueryRow(`SELECT count(*) FROM cash_adjustments WHERE kind='withdrawal'`).Scan(&n); e != nil || n != 0 {
		t.Fatal("partial withdrawal", n, e)
	}
}

// Keep the compiler checking callback signatures used by the public service.
var _ func(context.Context, *sql.Tx, identity.Scope) error = retail.Owner

func TestRetailConcurrentSaleFinancialRefund(t *testing.T) {
	f := salesSetup(t)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance}), 200)
	retailEnable(t, f.httpContractFixture, 5000)
	salesRequest(t, f, salesInput(f), 201)
	s := retailServiceFor(f.httpContractFixture)
	var item string
	if e := f.db.QueryRow(`SELECT id FROM sale_items WHERE sale_id='sale-one'`).Scan(&item); e != nil {
		t.Fatal(e)
	}
	const n = 6
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- s.RefundSale(context.Background(), "sale-one", retail.RefundInput{OperationID: fmt.Sprintf("financial-%d", i), ItemID: item, QuantityMilli: 1000, ExpectedCents: 1599, SessionID: "turn-one", Reason: "Explicit correction"})
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, retail.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
	assertStockBalance(t, f, f.shelf, 3000)
	var status string
	f.db.QueryRow(`SELECT status FROM sales WHERE id='sale-one'`).Scan(&status)
	if status != "committed" {
		t.Fatal("financial refund cancelled sale")
	}
}
func TestRetailAbsentReversalFence(t *testing.T) {
	f := arSetup(t)
	retailEnable(t, f, 5000)
	arResult(t, f, arPath, arBody, 201)
	arResult(t, f, arPath+"/bill/receive-cash", arReceive("paid", "one", 600, 0), 200)
	arResult(t, f, arPath+"/operations/uncertain-refund/reconcile", `{"document_id":"bill","action":"reverse_cash","confirmed":true}`, 200)
	s := retailServiceFor(f)
	_, e := receivables.ReverseCash(context.Background(), f.db, s.Contracts, s.Actor, f.device, f.token, "bill", receivables.ReversalInput{OperationID: "uncertain-refund", OriginalDeviceID: f.device.DeviceID, OriginalOperationID: "paid", SessionID: "ar-turn", AmountCents: 100, Reason: "Correction"})
	if !errors.Is(e, receivables.ErrConflict) {
		t.Fatal("late reversal not fenced", e)
	}
}
func TestRetailDefaultRoleAndSessionIsolation(t *testing.T) {
	f := arSetup(t)
	retailEnable(t, f, 5000)
	s := retailServiceFor(f)
	s.Actor.TenantID = "foreign-tenant"
	if e := s.SaveCustomer(context.Background(), "foreign", retail.Customer{ID: "c", Name: "Synthetic", ConsentReference: "c"}); e == nil {
		t.Fatal("foreign token accepted")
	}
	code, _ := request(t, f.app, "GET", "/local/v1/retail/config", "", "")
	if code != 401 {
		t.Fatal(code)
	}
	if _, e := f.db.Exec(`UPDATE memberships SET role='manager' WHERE tenant_id=? AND identity_id=?`, f.owner.TenantID, f.owner.OwnerID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.db.Exec(`INSERT INTO membership_stores(tenant_id,identity_id,store_id) VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
		t.Fatal(e)
	}
	f.token = loginToken(t, f.app, f.owner.OwnerID)
	code, _ = request(t, f.app, "GET", "/local/v1/retail/config", "", f.token)
	if code != 403 {
		t.Fatal("manager default", code)
	}
}
