package localapi

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/receivables"
)

// Isolated SQLite/API evidence; the contract provider is not Focus acceptance.
func nfe55DeferredFixture(t *testing.T) *stockHTTPFixture {
	t.Helper()
	f := fiscalFixture(t, "55")
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 9, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Finance, modules.Fiscal, modules.Staff}), 200)
	retailEnable(t, f.httpContractFixture, 5000)
	svc := retailServiceFor(f.httpContractFixture)
	p, err := fiscal.ReadProfile(context.Background(), f.db, svc.Actor, svc.Device, "issuer-one", 0)
	if err != nil {
		t.Fatal(err)
	}
	settings := p.Input
	settings.OperationID = "nfe55-credit-timezone"
	settings.ExpectedRevision = p.Revision
	settings.Timezone = "America/Sao_Paulo"
	taxRequest(t, f, "POST", "/local/v1/fiscal/profiles", settings, 201, nil)
	policy := taxPolicyFixture(f)
	policy.ProfileRevision = p.Revision + 1
	policy.Rule.Model = "55"
	policy.Rule.BaseRounding = "before_tax"
	policy.Rule.Goods = &fiscalcore.GoodsTaxClassification{Origin: "0", ICMS: "102", PIS: "07", COFINS: "07", IPIScope: "not_applicable", FCPScope: "not_applicable"}
	policy.Rule.Components = nil
	for _, code := range []string{"ICMS", "PIS", "COFINS"} {
		policy.Rule.Components = append(policy.Rule.Components, fiscalcore.TaxComponent{Code: code, Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"})
	}
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)
	quote := taxQuoteFixture(policy)
	quote.Context.Model = "55"
	quote.Item.UnitPrice = "15.99"
	quote.Context.Date = time.Now().In(time.FixedZone("test", -3*3600)).Format("2006-01-02")
	taxRequest(t, f, "POST", "/local/v1/fiscal/tax-quotes", quote, 201, nil)
	in := receivables.CreateInput{OperationID: "nf55-credit", ID: "nf55-ar", CustomerReference: "customer-external", DocumentReference: "synthetic", Description: "Synthetic", IssuedOn: "2026-11-03", TotalCents: 3198, Installments: []receivables.Installment{{ID: "first", DueOn: "2026-12-01", AmountCents: 3198}}, CreditSale: &receivables.CreditSaleInput{SaleID: "nf55-credit-sale", SessionID: "turn-one", Items: salesInput(f).Items, TaxQuoteOperations: []string{quote.OperationID}}}
	b, _ := json.Marshal(in)
	arResult(t, f.httpContractFixture, arPath, strings.ReplaceAll(string(b), `,"received_cents":0`, ``), 201)
	return f
}

func TestHTTPNFe55DeferredOriginReplayAndRecovery(t *testing.T) {
	for _, method := range []string{"21", "91"} {
		t.Run(method, func(t *testing.T) {
			f := nfe55DeferredFixture(t)
			p := &nfe55ContractProvider{state: "recovery_required"}
			nfe55ContractApp(t, f, p, "homologation")
			base := "/local/v1/fiscal/nfe55/sales/nf55-credit-sale/"
			for _, body := range []string{`{"payment_code":"91"}`, `{"payment_code":"01","confirmed":true}`, `{"confirmed":true}`, `{"payment_code":"21","confirmed":true,"amount_cents":1}`} {
				code, _ := request(t, f.app, "POST", base+"preview", body, f.token)
				if code != 400 {
					t.Fatal("invalid selection", code)
				}
			}
			var preview fiscalNFe55PreviewTest
			for i := 0; i < 2; i++ {
				code, b := request(t, f.app, "POST", base+"preview", `{"payment_code":"`+method+`","confirmed":true}`, f.token)
				if code != 200 || json.Unmarshal(b, &preview) != nil || preview.DeferredPaymentCode != method || preview.TotalCents != 3198 || !preview.MappingSupported {
					t.Fatal("deferred preview", code, string(b))
				}
			}
			// Opening the existing snapshot needs no original request on the client.
			code, b := request(t, f.app, "POST", base+"preview", `{}`, f.token)
			if code != 200 || !strings.Contains(string(b), `"deferred_payment_code":"`+method+`"`) {
				t.Fatal("snapshot recovery", code)
			}
			other := "21"
			if method == "21" {
				other = "91"
			}
			code, _ = request(t, f.app, "POST", base+"preview", `{"payment_code":"`+other+`","confirmed":true}`, f.token)
			if code != 409 {
				t.Fatal("changed immutable choice", code)
			}
			body := `{"nature":"VENDA","purpose":"1","freight_mode":"9","recipient":{"cpf":"12345678909","name":"Synthetic recipient","address":{"street":"Synthetic street","number":"1","district":"Centro","city":"Sao Paulo","uf":"SP"}}}`
			code, b = request(t, f.app, "POST", base+"emit", body, f.token)
			if code != 200 || p.last.Model != "55" || !strings.Contains(string(b), "recovery_required") {
				t.Fatal("deferred emission contract", code)
			}
			code, _ = request(t, f.app, "POST", base+"query", `{}`, f.token)
			if code != 200 {
				t.Fatal("ID-only recovery", code)
			}
			var payments, cash, choices, intents int
			for query, target := range map[string]*int{
				`SELECT count(*) FROM sale_payments WHERE sale_id='nf55-credit-sale'`:            &payments,
				`SELECT count(*) FROM cash_movements WHERE reason='venda:nf55-credit-sale'`:      &cash,
				`SELECT count(*) FROM fiscal_deferred_payments WHERE sale_id='nf55-credit-sale'`: &choices,
				`SELECT count(*) FROM fiscal_intents WHERE sale_id='nf55-credit-sale'`:           &intents,
			} {
				if err := f.db.QueryRow(query).Scan(target); err != nil {
					t.Fatal(err)
				}
			}
			if payments != 0 || cash != 0 || choices != 1 || intents != 1 {
				t.Fatal("duplicate or fictitious payment", payments, cash, choices, intents)
			}
			var snapshot fiscal.Snapshot
			var raw string
			if err := f.db.QueryRow(`SELECT snapshot_json FROM fiscal_intents WHERE sale_id='nf55-credit-sale'`).Scan(&raw); err != nil || json.Unmarshal([]byte(raw), &snapshot) != nil {
				t.Fatal("snapshot", err)
			}
			if len(snapshot.Payments) != 0 || snapshot.DeferredPayment == nil || snapshot.DeferredPayment.Code != method || snapshot.DeferredPayment.DocumentID != "nf55-ar" {
				t.Fatal("AR origin lost")
			}
		})
	}
}

type fiscalNFe55PreviewTest struct {
	DeferredPaymentCode string `json:"deferred_payment_code"`
	TotalCents          int64  `json:"total_cents"`
	MappingSupported    bool   `json:"mapping_supported"`
}

func TestHTTPNFe55DeferredOwnerAndProductionGate(t *testing.T) {
	for _, environment := range []string{"production", "A3"} {
		t.Run(environment, func(t *testing.T) {
			f := nfe55DeferredFixture(t)
			p := &nfe55ContractProvider{}
			nfe55ContractApp(t, f, p, environment)
			code, _ := request(t, f.app, "POST", "/local/v1/fiscal/nfe55/sales/nf55-credit-sale/preview", `{"payment_code":"21","confirmed":true}`, f.token)
			var n int
			f.db.QueryRow(`SELECT count(*) FROM fiscal_deferred_payments`).Scan(&n)
			if code != 409 || n != 0 || p.calls != 0 {
				t.Fatal("gate changed source", code, n, p.calls)
			}
		})
	}
	f := nfe55DeferredFixture(t)
	p := &nfe55ContractProvider{}
	nfe55ContractApp(t, f, p, "homologation")
	for _, role := range []string{"manager", "accountant", "cashier"} {
		t.Run(role, func(t *testing.T) {
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"nf55-credit-`+role+`","identity_id":"nf55-credit-`+role+`","name":"Synthetic","role":"`+role+`","password":"test-password-strong-1"}`, f.token)
			if code != 201 {
				t.Fatal("staff", code)
			}
			code, b := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"nf55-credit-`+role+`","password":"test-password-strong-1"}`, "")
			var auth struct {
				Token string `json:"token"`
			}
			if code != 200 || json.Unmarshal(b, &auth) != nil {
				t.Fatal("login", code)
			}
			code, _ = request(t, f.app, "POST", "/local/v1/fiscal/nfe55/sales/nf55-credit-sale/preview", `{"payment_code":"21","confirmed":true}`, auth.Token)
			if code != 403 {
				t.Fatal("non-owner selected method", code)
			}
		})
	}
	var n int
	f.db.QueryRow(`SELECT count(*) FROM fiscal_deferred_payments`).Scan(&n)
	if n != 0 {
		t.Fatal("unauthorized choice persisted")
	}
}

func TestHTTPNFe55DeferredConcurrentChoice(t *testing.T) {
	f := nfe55DeferredFixture(t)
	p := &nfe55ContractProvider{}
	nfe55ContractApp(t, f, p, "homologation")
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, code := range []string{"21", "91"} {
		wg.Add(1)
		go func(method string) {
			defer wg.Done()
			status, _ := request(t, f.app, "POST", "/local/v1/fiscal/nfe55/sales/nf55-credit-sale/preview", `{"payment_code":"`+method+`","confirmed":true}`, f.token)
			codes <- status
		}(code)
	}
	wg.Wait()
	close(codes)
	wins, conflicts := 0, 0
	for code := range codes {
		switch code {
		case 200:
			wins++
		case 409:
			conflicts++
		default:
			t.Fatal("generic or unexpected result", code)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("choice race", wins, conflicts)
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_deferred_payments WHERE sale_id='nf55-credit-sale'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate choices", count, err)
	}
}
