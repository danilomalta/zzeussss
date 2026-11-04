package localapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/receivables"
)

func deferredFixture(t *testing.T) *stockHTTPFixture {
	t.Helper()
	f := fiscalFixture(t, "65")
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance, modules.Fiscal, modules.Staff}), 200)
	retailEnable(t, f.httpContractFixture, 5000)
	svc := retailServiceFor(f.httpContractFixture)
	profile, e := fiscal.ReadProfile(context.Background(), f.db, svc.Actor, svc.Device, "issuer-one", 0)
	if e != nil {
		t.Fatal(e)
	}
	settings := profile.Input
	settings.OperationID = "deferred-timezone"
	settings.ExpectedRevision = profile.Revision
	settings.Timezone = "America/Sao_Paulo"
	taxRequest(t, f, "POST", "/local/v1/fiscal/profiles", settings, 201, nil)
	policy := taxPolicyFixture(f)
	policy.ProfileRevision = profile.Revision + 1
	policy.Rule.BaseRounding = "before_tax"
	policy.Rule.Goods = &fiscalcore.GoodsTaxClassification{Origin: "0", ICMS: "102", PIS: "07", COFINS: "07", IPIScope: "not_applicable", FCPScope: "not_applicable"}
	policy.Rule.Components = nil
	for _, code := range []string{"ICMS", "PIS", "COFINS"} {
		policy.Rule.Components = append(policy.Rule.Components, fiscalcore.TaxComponent{Code: code, Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"})
	}
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
	quote := taxQuoteFixture(policy)
	quote.Item.UnitPrice = "15.99"
	quote.Context.Date = time.Now().In(time.FixedZone("test", -3*3600)).Format("2006-01-02")
	var priced fiscal.TaxQuote
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", quote, 201, &priced)

	in := receivables.CreateInput{OperationID: "deferred-credit", ID: "deferred-doc", CustomerReference: "customer-external", DocumentReference: "deferred", Description: "Synthetic", IssuedOn: "2026-11-03", TotalCents: 3198, Installments: []receivables.Installment{{ID: "one", DueOn: "2026-12-01", AmountCents: 3198}}, CreditSale: &receivables.CreditSaleInput{SaleID: "deferred-sale", SessionID: "turn-one", Items: salesInput(f).Items, TaxQuoteOperations: []string{quote.OperationID}}}
	b, _ := json.Marshal(in)
	arResult(t, f.httpContractFixture, arPath, strings.ReplaceAll(string(b), `,"received_cents":0`, ``), 201)
	return f
}
func TestDeferredFiscalConcurrentSelectionAndSnapshot(t *testing.T) {
	f := deferredFixture(t)
	s := retailServiceFor(f.httpContractFixture)
	ctx := context.Background()
	const n = 6
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code := "21"
			if i%2 != 0 {
				code = "91"
			}
			errs <- fiscal.SelectDeferredPayment(ctx, f.db, s.Contracts, s.Actor, s.Device, s.Token, "deferred-sale", fmt.Sprintf("select-%d", i), code)
		}(i)
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, fiscal.ErrConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatalf("winners=%d", wins)
	}
	v, e := fiscal.DeferredPaymentForSale(ctx, f.db, s.Contracts, s.Actor, s.Device, s.Token, "deferred-sale")
	if e != nil || !v.Valid(3198) {
		t.Fatal("selection", e)
	}
	intent, e := fiscal.Prepare(ctx, f.db, s.Contracts, s.Actor, s.Device, fiscal.IntentInput{OperationID: "deferred-intent", SaleID: "deferred-sale", ProfileID: "issuer-one", ProfileRevision: 2}, s.Token)
	if e != nil {
		t.Fatal(e)
	}
	if intent.Snapshot.Items[0].Tax == nil {
		t.Fatal("tax origin lost")
	}
	if intent.Snapshot.DeferredPayment == nil || intent.Snapshot.DeferredPayment.Code != v.Code || len(intent.Snapshot.Payments) != 0 {
		t.Fatal("debt described as cash")
	}
	var payments, cash int
	f.db.QueryRow(`SELECT count(*) FROM sale_payments WHERE sale_id='deferred-sale'`).Scan(&payments)
	f.db.QueryRow(`SELECT count(*) FROM cash_movements WHERE reason='venda:deferred-sale'`).Scan(&cash)
	if cash != 0 || payments != 0 {
		t.Fatal("cash fabricated")
	}
	other := s.Actor
	other.TenantID = "foreign-tenant"
	if _, e = fiscal.DeferredPaymentForSale(ctx, f.db, s.Contracts, other, s.Device, s.Token, "deferred-sale"); e == nil {
		t.Fatal("foreign tenant accepted")
	}
	if e = fiscal.SelectDeferredPayment(ctx, f.db, s.Contracts, s.Actor, s.Device, s.Token, "deferred-sale", "other-choice", "01"); e == nil {
		t.Fatal("unsupported choice accepted")
	}
}
func TestHTTPDeferredFiscalOwnerProductionAndNoFinancialEffect(t *testing.T) {
	for _, env := range []string{"homologation", "production", "A3", "role-manager", "role-accountant", "role-cashier"} {
		t.Run(env, func(t *testing.T) {
			f := deferredFixture(t)
			p := &nfe55ContractProvider{state: "recovery_required"}
			certificate := "A1"
			actualEnv := env
			if strings.HasPrefix(env, "role-") {
				actualEnv = "homologation"
			}
			if env == "A3" {
				certificate = "A3"
				actualEnv = "homologation"
			}
			r, e := fiscalproduction.NewProviderRegistry([]fiscalproduction.ProviderRegistration{{Configuration: fiscalproduction.TenantFiscalConfiguration{DocumentModel: "65", Provider: "external", Environment: actualEnv, CRT: "1", CertificateType: certificate, Binding: fiscalproduction.ProviderBinding{TenantID: f.owner.TenantID, StoreID: f.owner.StoreID, DeviceID: f.owner.DeviceID, ProfileID: "issuer-one", ProfileRevision: 2}}, Provider: p}})
			if e != nil {
				t.Fatal(e)
			}
			verifier, _ := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
			f.app, e = NewWithFiscalProvidersAndGate(f.db, f.device, verifier, nil, nil, nil, r)
			if e != nil {
				t.Fatal(e)
			}
			if strings.HasPrefix(env, "role-") {
				role := strings.TrimPrefix(env, "role-")
				if _, e = f.db.Exec(`UPDATE memberships SET role=? WHERE tenant_id=? AND identity_id=?`, role, f.owner.TenantID, f.owner.OwnerID); e != nil {
					t.Fatal(e)
				}
				if _, e = f.db.Exec(`INSERT INTO membership_stores VALUES(?,?,?)`, f.owner.TenantID, f.owner.OwnerID, f.owner.StoreID); e != nil {
					t.Fatal(e)
				}
				f.token = loginToken(t, f.app, f.owner.OwnerID)
			}
			body := `{"operation_id":"explicit-choice","payment_code":"21","nature":"VENDA","presence":"1","confirmed":true}`
			code, raw := request(t, f.app, "POST", "/local/v1/fiscal/deferred/sales/deferred-sale/emit", body, f.token)
			if strings.HasPrefix(env, "role-") {
				if code != 403 || p.calls != 0 {
					t.Fatal("non-owner accepted", env, code)
				}
				return
			}
			if env != "homologation" {
				if code != 409 || p.calls != 0 {
					t.Fatal("blocked provider reached", code)
				}
				return
			}
			if code != 200 || !strings.Contains(string(raw), "recovery_required") || p.last.Model != "65" || p.last.Environment != 2 {
				t.Fatal("homologation routing", code, string(raw))
			}
			code, _ = request(t, f.app, "POST", "/local/v1/fiscal/deferred/sales/deferred-sale/query", "{}", f.token)
			if code != 200 {
				t.Fatal("query", code)
			}
			var total, received int64
			f.db.QueryRow(`SELECT total_cents FROM receivable_documents WHERE id='deferred-doc'`).Scan(&total)
			f.db.QueryRow(`SELECT COALESCE(sum(amount_cents),0) FROM receivable_cash_receipts WHERE document_id='deferred-doc'`).Scan(&received)
			if total != 3198 || received != 0 {
				t.Fatal("fiscal operation changed debt")
			}
		})
	}
}

func TestDeferredFiscalSelectionEncryptedBackupAndIsolation(t *testing.T) {
	f := deferredFixture(t)
	s := retailServiceFor(f.httpContractFixture)
	ctx := context.Background()
	if e := fiscal.SelectDeferredPayment(ctx, f.db, s.Contracts, s.Actor, s.Device, s.Token, "deferred-sale", "stable-choice", "91"); e != nil {
		t.Fatal(e)
	}
	if e := fiscal.SelectDeferredPayment(ctx, f.db, s.Contracts, s.Actor, s.Device, s.Token, "deferred-sale", "stable-choice", "91"); e != nil {
		t.Fatal("replay", e)
	}
	if e := fiscal.SelectDeferredPayment(ctx, f.db, s.Contracts, s.Actor, s.Device, s.Token, "deferred-sale", "changed-choice", "21"); !errors.Is(e, fiscal.ErrConflict) {
		t.Fatal("choice changed", e)
	}
	if _, e := f.db.Exec(`UPDATE fiscal_deferred_payments SET payment_code='21'`); e == nil {
		t.Fatal("immutable choice changed")
	}
	var sourceRows, queued int
	f.db.QueryRow(`SELECT count(*) FROM fiscal_deferred_payments`).Scan(&sourceRows)
	f.db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.fiscal_deferred_selection'`).Scan(&queued)
	if sourceRows != 1 || queued != 0 {
		t.Fatal("duplicate or peer protocol", sourceRows, queued)
	}
	other := s.Actor
	other.StoreID = "foreign-store"
	if _, e := fiscal.DeferredPaymentForSale(ctx, f.db, s.Contracts, other, s.Device, s.Token, "deferred-sale"); e == nil {
		t.Fatal("cross-store read")
	}
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	archive := filepath.Join(t.TempDir(), "deferred.tytbak")
	if e := backup.Create(ctx, f.db, f.device, key, archive); e != nil {
		t.Fatal(e)
	}
	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	if e := backup.Restore(ctx, archive, restored, f.device, key); e != nil {
		t.Fatal(e)
	}
	db, e := localdb.Open(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var code, document string
	var amount int64
	if e = db.QueryRow(`SELECT payment_code,document_id,total_cents FROM fiscal_deferred_payments`).Scan(&code, &document, &amount); e != nil || code != "91" || document != "deferred-doc" || amount != 3198 {
		t.Fatal("restore lost original", e)
	}
	if _, e = localauth.Resolve(ctx, db, f.token); !errors.Is(e, localauth.ErrDenied) {
		t.Fatal("restored session not revoked")
	}
}
