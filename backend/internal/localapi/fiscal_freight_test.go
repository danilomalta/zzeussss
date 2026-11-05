package localapi

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"errors"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/retail"
	"titansystem-backend/internal/localdb/sale"
	"titansystem-backend/internal/localdb/stock"
)

// Explicit synthetic customer choices, not tax advice or Focus acceptance.
func freightSaleFixture(t *testing.T) (*stockHTTPFixture, fiscal.TaxQuote, sale.Input) {
	t.Helper()
	f := fiscalFixture(t, "55")
	var profile fiscal.Profile
	taxRequest(t, f, "GET", "/local/v1/fiscal/profiles/issuer-one", nil, 200, &profile)
	in := profile.Input
	in.OperationID, in.ExpectedRevision, in.Timezone = "freight-profile", profile.Revision, "America/Sao_Paulo"
	taxRequest(t, f, "POST", "/local/v1/fiscal/profiles", in, 201, nil)
	zone, _ := time.LoadLocation(in.Timezone)
	now := time.Now().In(zone)
	policy := taxPolicyFixture(f)
	policy.Rule.ValidFrom = now.AddDate(0, 0, -1).Format("2006-01-02")
	policy.Rule.ValidUntil = now.AddDate(0, 0, 1).Format("2006-01-02")
	policy.ProfileRevision = 2
	policy.Rule.Model, policy.Rule.BaseRounding = "55", "before_tax"
	policy.Rule.Goods = &fiscalcore.GoodsTaxClassification{Origin: "0", ICMS: "102", PIS: "07", COFINS: "07", IPIScope: "not_applicable", FCPScope: "not_applicable"}
	policy.Rule.Components = nil
	for _, code := range []string{"ICMS", "PIS", "COFINS"} {
		policy.Rule.Components = append(policy.Rule.Components, fiscalcore.TaxComponent{Code: code, Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"})
	}
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
	q := taxQuoteFixture(policy)
	q.OperationID, q.Context.Model, q.Item.UnitPrice, q.Item.Freight = "freight-quote", "55", "15.99", "1.01"
	q.Context.Date = now.Format("2006-01-02")
	q.Freight = &fiscal.SaleFreightContract{SaleID: "freight-sale",
		Allocation:  fiscalcore.FreightAllocationInput{Method: fiscalcore.FreightLargestRemainderV1, TotalCents: 101, Items: []fiscalcore.FreightWeight{{ItemID: q.OperationID, Weight: 1}}},
		FreightMode: "0", RefundPolicy: "proportional_with_item", SourceReference: "synthetic-commercial-origin", ReviewReference: "synthetic-review-only",
	}
	var quote fiscal.TaxQuote
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, &quote)
	input := salesInput(f)
	input.Freight = true
	input.SaleID, input.OperationID = "freight-sale", "freight-sale-command"
	input.TaxQuoteOperations = []string{q.OperationID}
	input.Payments[0].AmountCents = 3299
	return f, quote, input
}

func TestHTTPFreightPlanPaymentFrozenRefundAndRestart(t *testing.T) {
	f, quote, input := freightSaleFixture(t)
	previewBody := planJSON(t, sale.QuoteInput{Freight: true, CashSessionID: input.CashSessionID, Items: input.Items, TaxQuoteOperations: input.TaxQuoteOperations})
	code, previewRaw := request(t, f.app, "POST", "/local/v1/sales/quote", previewBody, f.token)
	var preview sale.Quote
	if code != 200 || json.Unmarshal(previewRaw, &preview) != nil || preview.TotalCents != 3299 {
		t.Fatal("freight preview did not use original quote", code)
	}
	planInput := sale.PlanInput{Freight: true, OperationID: "freight-plan", CheckoutID: input.SaleID, CashSessionID: input.CashSessionID, ExpectedTotalCents: 3299, Items: input.Items, TaxQuoteOperations: input.TaxQuoteOperations, Parts: []sale.PlanPartInput{{PartID: "cash", Method: "cash", AmountCents: 3299}}}
	plan := planRequest(t, f, "POST", planPath, planJSON(t, planInput), 201)
	if plan.TotalCents != 3299 || plan.Items[0].Tax == nil || plan.Items[0].Tax.AdditionalCents != 101 {
		t.Fatal("freight not priced before payment")
	}
	final := `{"operation_id":"freight-final","received_cents":4000,"freight":true}`
	code, _ = request(t, f.app, "POST", planPath+"/"+input.SaleID+"/finalize-cash", `{"operation_id":"freight-final","received_cents":4000}`, f.token)
	if code != 409 {
		t.Fatal("omitted freight selection bypassed frozen plan", code)
	}
	planRequest(t, f, "POST", planPath+"/"+input.SaleID+"/finalize-cash", final, 201)
	planRequest(t, f, "POST", planPath+"/"+input.SaleID+"/finalize-cash", final, 200)
	receipt := readReceipt(t, f, input.SaleID)
	if receipt.TotalCents != 3299 || receipt.Items[0].Tax.Freight.SourceReference != quote.Input.Freight.SourceReference {
		t.Fatal("frozen source lost")
	}
	intent := fiscalIntent(t, f, `{"operation_id":"freight-intent","sale_id":"freight-sale","profile_id":"issuer-one","profile_revision":2}`, 201)
	if intent.Snapshot.TotalCents != 3299 || intent.Snapshot.Items[0].Tax == nil || intent.Snapshot.Items[0].Tax.Freight.SaleID != input.SaleID {
		t.Fatal("invoice snapshot lost commercial source")
	}
	changed := quote.Input
	changed.Freight.FreightMode = "1"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", changed, 409, nil)
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	f.db = db
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = NewWithFiscalProvidersAndGate(db, f.device, verifier, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.token = loginToken(t, f.app, f.owner.OwnerID)
	var restoredIntent fiscal.Intent
	taxRequest(t, f, "GET", "/local/v1/fiscal/intents/"+intent.ID, nil, 200, &restoredIntent)
	if restoredIntent.SnapshotSHA256 != intent.SnapshotSHA256 {
		t.Fatal("restart changed original fiscal snapshot")
	}
	recovered := planRequest(t, f, "POST", planPath+"/"+input.SaleID+"/finalize-cash", final, 200)
	if recovered.Completion == nil || recovered.Completion.ChangeCents != 701 || recovered.TotalCents != 3299 {
		t.Fatal("restart changed result")
	}
	// Explicit policy includes the allocated freight in cumulative item refunds.
	for i, cents := range []int64{1649, 1650} {
		body := map[string]any{"operation_id": "freight-return-" + strconv.Itoa(i), "reason": "Synthetic explicit return", "expected_refund_cents": cents, "items": []map[string]any{{"sale_item_id": receipt.Items[0].ItemID, "quantity_milli": 1000, "disposition": "restock"}}}
		raw, _ := json.Marshal(body)
		status, response := request(t, f.app, "POST", "/local/v1/sales/"+input.SaleID+"/returns", string(raw), f.token)
		if status != 201 {
			t.Fatalf("return %d: %d %s", i, status, response)
		}
	}
}

func TestHTTPFreightWrongPaymentRollbackAndQuoteWrite(t *testing.T) {
	f, quote, input := freightSaleFixture(t)
	input.Payments[0].AmountCents--
	salesRequest(t, f, input, 400)
	input.Payments[0].AmountCents++
	if _, err := f.db.Exec(`CREATE TRIGGER fail_freight BEFORE INSERT ON sale_tax_bindings BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
		t.Fatal(err)
	}
	status, _ := request(t, f.app, "POST", "/local/v1/sales", salesJSON(t, input), f.token)
	if status < 400 {
		t.Fatal("write failure accepted")
	}
	assertStockBalance(t, f, f.shelf, 3000)
	if _, err := f.db.Exec(`DROP TRIGGER fail_freight`); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 201)
	// An ignored quote insert must not return a successful phantom quote.
	q := quote.Input
	q.OperationID = "ignored-freight"
	q.Freight.Allocation.Items[0].ItemID = q.OperationID
	if _, err := f.db.Exec(`CREATE TRIGGER ignore_freight BEFORE INSERT ON fiscal_tax_quotes BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", q, 409, nil)
}

func TestHTTPFreightIgnoredBindingRollsBack(t *testing.T) {
	f, _, input := freightSaleFixture(t)
	if _, err := f.db.Exec(`CREATE TRIGGER ignore_freight_binding BEFORE INSERT ON sale_tax_bindings BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	code, _ := request(t, f.app, "POST", "/local/v1/sales", salesJSON(t, input), f.token)
	if code != 409 {
		t.Fatalf("ignored evidence write accepted: status=%d", code)
	}
	for _, query := range []string{`SELECT count(*) FROM sales WHERE id=?`, `SELECT count(*) FROM sale_tax_bindings WHERE sale_id=?`, `SELECT count(*) FROM sale_operations WHERE sale_id=?`} {
		var count int
		if err := f.db.QueryRow(query, input.SaleID).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial sale committed", count, err)
		}
	}
	assertStockBalance(t, f, f.shelf, 3000)
	if _, err := f.db.Exec(`DROP TRIGGER ignore_freight_binding`); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 201)
	salesRequest(t, f, input, 200)
}

func TestHTTPFreightConcurrentQuoteConsumption(t *testing.T) {
	f, _, input := freightSaleFixture(t)
	otherDB := freightOtherDB(t, f)
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	otherApp, err := NewWithFiscalProvidersAndGate(otherDB, f.device, verifier, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stockRequest(t, f, stock.Input{OperationID: "freight-stock", Kind: "transfer", ProductID: f.product, FromLocationID: f.back, ToLocationID: f.shelf, QuantityMilli: 5000, Reason: "Synthetic concurrency supply"}, 201)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, suffix := range []string{"a", "b"} {
		app := f.app
		if suffix == "b" {
			app = otherApp
		}
		in := input
		in.OperationID += suffix
		body := salesJSON(t, in)
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := request(t, app, "POST", "/local/v1/sales", body, f.token)
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	wins, conflicts := 0, 0
	for code := range codes {
		if code == 201 {
			wins++
		} else if code == 409 {
			conflicts++
		} else {
			t.Fatalf("unexpected contention status %d", code)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("quote group consumed more than once", wins, conflicts)
	}
	assertStockBalance(t, f, f.shelf, 6000)
}

func freightOtherDB(t *testing.T, f *stockHTTPFixture) *sql.DB {
	t.Helper()
	var seq int
	var name, path string
	if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestFreightSaleWaitsForIndependentWriterBeforeReading(t *testing.T) {
	f, _, input := freightSaleFixture(t)
	other := freightOtherDB(t, f)
	ctx := context.Background()
	conn, err := other.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(ctx, "ROLLBACK")
	released := make(chan error, 1)
	time.AfterFunc(150*time.Millisecond, func() { _, e := conn.ExecContext(ctx, "ROLLBACK"); released <- e })
	s := retailServiceFor(f.httpContractFixture)
	out, commandErr := sale.CompleteWithContract(ctx, f.db, s.Contracts, s.Actor, s.Device, input, s.Token)
	if e := <-released; e != nil {
		t.Fatal(e)
	}
	if commandErr != nil || out.SaleID != input.SaleID {
		t.Fatal("freight command failed against an independent writer", commandErr)
	}
}

func TestHTTPFreightFinancialRefundRemainsSeparate(t *testing.T) {
	f, _, input := freightSaleFixture(t)
	salesRequest(t, f, input, 201)
	receipt := readReceipt(t, f, input.SaleID)
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Fiscal, modules.Finance}), 200)
	service := retailServiceFor(f.httpContractFixture)
	if err := service.Configure(context.Background(), "freight-refund-config", retail.Config{RefundEnabled: true, RefundRoles: []string{"owner"}, CreditRoles: []string{"owner"}, PointsRoles: []string{"owner"}}); err != nil {
		t.Fatal(err)
	}
	err := service.RefundSale(context.Background(), input.SaleID, retail.RefundInput{OperationID: "freight-financial", ItemID: receipt.Items[0].ItemID, QuantityMilli: 1000, ExpectedCents: 1649, SessionID: input.CashSessionID, Reason: "Synthetic explicit compensation"})
	if !errors.Is(err, retail.ErrDisabled) {
		t.Fatal("legacy financial-only refund bypassed tax-backed sale guard", err)
	}
}

func TestHTTPFreightMultiItemResidualAndIncompleteGroup(t *testing.T) {
	f, original, input := freightSaleFixture(t)
	var q fiscal.TaxQuoteInput
	raw, _ := json.Marshal(original.Input)
	if json.Unmarshal(raw, &q) != nil {
		t.Fatal("fixture")
	}
	q.Freight.SaleID = "freight-split-sale"
	q.Freight.Allocation.Items = []fiscalcore.FreightWeight{{ItemID: "split-one", Weight: 1}, {ItemID: "split-two", Weight: 1}}
	q.Item.Quantity = "1"
	q.OperationID, q.Item.Freight = "split-one", "0.51"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, nil)
	input.SaleID, input.OperationID = q.Freight.SaleID, "split-command"
	input.Items = []sale.Item{{ProductID: f.product, LocationID: f.shelf, QuantityMilli: 1000}, {ProductID: f.product, LocationID: f.shelf, QuantityMilli: 1000}}
	input.TaxQuoteOperations = []string{"split-one", "split-two"}
	salesRequest(t, f, input, 409)
	q.OperationID, q.Item.Freight = "split-two", "0.50"
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", q, 201, nil)
	salesRequest(t, f, input, 201)
	salesRequest(t, f, input, 200)
	r := readReceipt(t, f, input.SaleID)
	if r.TotalCents != 3299 || len(r.Items) != 2 {
		t.Fatal("split total")
	}
	var extra int64
	for _, item := range r.Items {
		if item.Tax == nil {
			t.Fatal("missing source")
		}
		extra += item.Tax.AdditionalCents
	}
	if extra != 101 {
		t.Fatal("residual lost or duplicated")
	}
	input.OperationID, input.SaleID = "split-other-command", "another-sale"
	salesRequest(t, f, input, 409)
}

func TestHTTPFreightScopeMissingTermsAndOldSession(t *testing.T) {
	f, original, input := freightSaleFixture(t)
	other := fiscalFixture(t, "55")
	taxRequest(t, other, "GET", "/local/v1/fiscal/tax-quotes/"+original.Input.OperationID, nil, 404, nil)
	for _, field := range []string{"origin", "source", "review", "refund", "mode", "allocation"} {
		t.Run(field, func(t *testing.T) {
			var q fiscal.TaxQuoteInput
			raw, _ := json.Marshal(original.Input)
			_ = json.Unmarshal(raw, &q)
			q.OperationID = "invalid-freight"
			q.Freight.Allocation.Items[0].ItemID = q.OperationID
			switch field {
			case "origin":
				q.Freight.SaleID = ""
			case "source":
				q.Freight.SourceReference = ""
			case "review":
				q.Freight.ReviewReference = ""
			case "refund":
				q.Freight.RefundPolicy = ""
			case "mode":
				q.Freight.FreightMode = "9"
			case "allocation":
				q.Item.Freight = "1.02"
			}
			taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", q, 400, nil)
		})
	}
	input.Freight = false
	salesRequest(t, f, input, 400)
	input.Freight = true
	if err := localauth.Logout(context.Background(), f.db, f.token); err != nil {
		t.Fatal(err)
	}
	salesRequest(t, f, input, 401)
	var balance int64
	if err := f.db.QueryRow(`SELECT COALESCE(sum(quantity_milli),0) FROM stock_movements WHERE tenant_id=? AND store_id=? AND product_id=? AND location_id=?`, f.owner.TenantID, f.owner.StoreID, f.product, f.shelf).Scan(&balance); err != nil || balance != 3000 {
		t.Fatal("revoked session changed stock", balance, err)
	}
}

func TestHTTPFreightQuoteOwnerAndHomologationOnly(t *testing.T) {
	f, original, _ := freightSaleFixture(t)
	for _, role := range []string{"accountant", "manager", "cashier"} {
		t.Run(role, func(t *testing.T) {
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"freight-staff-`+role+`","identity_id":"freight-`+role+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
			if code != 201 {
				t.Fatal("staff fixture", code)
			}
			code, raw := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"freight-`+role+`","password":"test-password-strong-1"}`, "")
			var session struct {
				Token string `json:"token"`
			}
			if code != 200 || json.Unmarshal(raw, &session) != nil || session.Token == "" {
				t.Fatal("staff login fixture", code)
			}
			var q fiscal.TaxQuoteInput
			body, _ := json.Marshal(original.Input)
			_ = json.Unmarshal(body, &q)
			q.OperationID = "freight-role-" + role
			q.Freight.Allocation.Items[0].ItemID = q.OperationID
			body, _ = json.Marshal(q)
			code, _ = request(t, f.app, "POST", "/local/v1/fiscal/tax-quotes", string(body), session.Token)
			if code != 403 {
				t.Fatal("non-owner quote admitted", code)
			}
		})
	}
	var profile fiscal.Profile
	taxRequest(t, f, "GET", "/local/v1/fiscal/profiles/issuer-one", nil, 200, &profile)
	pi := profile.Input
	pi.OperationID, pi.ExpectedRevision, pi.Environment = "freight-production-profile", profile.Revision, "production"
	taxRequest(t, f, "POST", "/local/v1/fiscal/profiles", pi, 201, nil)
	var policy fiscal.TaxPolicy
	taxRequest(t, f, "GET", "/local/v1/fiscal/tax-policies/"+original.Input.PolicyID, nil, 200, &policy)
	policy.Input.OperationID, policy.Input.ExpectedRevision, policy.Input.ProfileRevision = "freight-production-policy", policy.Revision, profile.Revision+1
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", policy.Input, 201, nil)
	productionQuote := original.Input
	productionQuote.OperationID, productionQuote.PolicyRevision = "freight-production", policy.Revision+1
	productionQuote.Freight.Allocation.Items[0].ItemID = productionQuote.OperationID
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", productionQuote, 400, nil)
}
