package fiscalproduction

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// Local API-contract evidence only; no Focus/SEFAZ acceptance is claimed.
func TestFocusFreightFrozenQuoteMappingAndRecovery(t *testing.T) {
	a := focusContext(t)
	ctx := context.Background()
	profile, err := fiscal.Configure(ctx, a.DB, a.License, a.Actor, a.Device, fiscal.ProfileInput{OperationID: "freight-profile", ProfileID: "freight-profile", CNPJ: "11222333000181", LegalName: "Synthetic issuer", State: "SP", Municipality: "3550308", StateRegistration: "123456789", CRT: "1", Model: "55", Environment: "homologation", Series: 1, Timezone: "America/Sao_Paulo"}, a.Token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.DB.Exec(`INSERT INTO products(tenant_id,id,sku,name,price_cents,unit) VALUES(?,?,?,?,?,?)`, a.Actor.TenantID, "freight-product", "synthetic-sku", "Synthetic product", 1599, "unit"); err != nil {
		t.Fatal(err)
	}
	zone, _ := time.LoadLocation(profile.Input.Timezone)
	now := time.Now().In(zone)
	rule := fiscalcore.TaxRule{ID: "freight-policy", ProductID: "freight-product", CFOP: "5102", NCM: "19059090", OriginUF: "SP", DestinationUF: "SP", CRT: "1", Model: "55", ValidFrom: now.AddDate(0, 0, -1).Format("2006-01-02"), ValidUntil: now.AddDate(0, 0, 1).Format("2006-01-02"), LegalReference: "Synthetic calculation only", ApprovalReference: "Synthetic reference only", BaseRounding: "before_tax", Goods: &fiscalcore.GoodsTaxClassification{Origin: "0", ICMS: "102", PIS: "07", COFINS: "07", IPIScope: "not_applicable", FCPScope: "not_applicable"}}
	for _, code := range []string{"ICMS", "PIS", "COFINS"} {
		rule.Components = append(rule.Components, fiscalcore.TaxComponent{Code: code, Method: "exempt", Rate: "0", MVA: "0", ReductionPercent: "0"})
	}
	policy := fiscal.TaxPolicyInput{OperationID: "freight-policy", ProfileID: profile.Input.ProfileID, ProfileRevision: 1, Active: true, Rule: rule, Case: fiscalcore.TaxCase{Operation: "sale", Purpose: "final_consumption", OriginMunicipality: "3550308", DestinationMunicipality: "3550308", RecipientStatus: "noncontributor", FinalConsumer: "yes", ProductOrigin: "0"}, Sources: []string{"https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp87.htm"}}
	if _, err = fiscal.PublishTaxPolicy(ctx, a.DB, a.License, a.Actor, a.Device, policy, a.Token); err != nil {
		t.Fatal(err)
	}
	freight := &fiscal.SaleFreightContract{SaleID: "freight-sale", Allocation: fiscalcore.FreightAllocationInput{Method: fiscalcore.FreightLargestRemainderV1, TotalCents: 101, Items: []fiscalcore.FreightWeight{{ItemID: "freight-quote", Weight: 1}}}, FreightMode: "0", RefundPolicy: "proportional_with_item", SourceReference: "synthetic-commercial-origin", ReviewReference: "synthetic-review"}
	q, err := fiscal.CreateTaxQuote(ctx, a.DB, a.License, a.Actor, a.Device, fiscal.TaxQuoteInput{OperationID: "freight-quote", PolicyID: rule.ID, PolicyRevision: 1, Context: fiscalcore.TaxContext{OriginUF: "SP", DestinationUF: "SP", CRT: "1", Model: "55", Date: now.Format("2006-01-02")}, Case: policy.Case, Item: fiscalcore.TaxItem{ProductID: rule.ProductID, Quantity: "2", UnitPrice: "15.99", Discount: "0", Freight: "1.01", Insurance: "0", Other: "0"}, Freight: freight}, a.Token)
	if err != nil {
		t.Fatal(err)
	}
	intent := fiscal.Intent{ID: "freight-intent", Snapshot: fiscal.Snapshot{Profile: profile.Input, ProfileRevision: 1, SaleID: "freight-sale", TotalCents: 3299, Items: []fiscal.SaleItem{{ProductID: rule.ProductID, QuantityMilli: 2000, UnitPriceCents: 1599, TotalCents: 3198, Tax: &fiscal.SaleTaxBinding{QuoteOperationID: q.Input.OperationID, ResultSHA256: q.ResultSHA256, ProfileID: profile.Input.ProfileID, ProfileRevision: 1, CatalogUnit: q.CatalogUnit, Freight: freight, AdditionalCents: 101, Taxes: q.Result.GoodsXML.Taxes, Totals: q.Result.GoodsXML.Totals}}}, Payments: []fiscal.Payment{{Method: "cash", Status: "confirmed", AmountCents: 3299}}}}
	invoice := fiscalcore.Invoice{Environment: 2, Nature: "VENDA", Presence: "1", Direction: "1", Destination: "1", FinalConsumer: "1", Recipient: &fiscalcore.Party{CPF: "12345678909", Name: "Synthetic buyer", IEDestination: "9", Address: fiscalcore.Address{Street: "Rua Teste", Number: "1", District: "Centro", City: "Sao Paulo", UF: "SP"}}}
	raw, _ := json.Marshal(invoice)
	req := DocumentRequest{Invoice: raw, Purpose: "1", FreightMode: "0"}
	payload, err := focusInvoiceRequest(ctx, a, intent, req)
	if err != nil {
		t.Fatal(err)
	}
	var mapped map[string]any
	if err = json.Unmarshal(payload, &mapped); err != nil {
		t.Fatal(err)
	}
	if mapped["valor_frete"] != "1.01" || mapped["valor_total"] != "32.99" {
		t.Fatal("freight totals lost")
	}
	items := mapped["items"].([]any)
	if items[0].(map[string]any)["valor_frete"] != "1.01" || items[0].(map[string]any)["valor_bruto"] != "31.98" {
		t.Fatal("merchandise and freight confused")
	}
	originalSource := freight.SourceReference
	freight.SourceReference = "different-commercial-source"
	if _, err = focusInvoiceRequest(ctx, a, intent, req); !errors.Is(err, fiscal.ErrIntegrity) {
		t.Fatal("quote/snapshot provenance replaced", err)
	}
	freight.SourceReference = originalSource
	req.FreightMode = "1"
	if _, err = focusInvoiceRequest(ctx, a, intent, req); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("responsibility changed after payment", err)
	}
	req.FreightMode = "0"
	intent.Snapshot.TotalCents--
	if _, err = focusInvoiceRequest(ctx, a, intent, req); !errors.Is(err, fiscal.ErrIntegrity) {
		t.Fatal("unbalanced total admitted", err)
	}
	intent.Snapshot.TotalCents++
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: now, CNPJ: profile.Input.CNPJ, Model: "55", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	ref := focusReference(a.Actor.TenantID, "freight-sale")
	var posts, gets atomic.Int32
	provider := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
			w.WriteHeader(http.StatusCreated)
			var sent map[string]any
			if json.NewDecoder(r.Body).Decode(&sent) != nil || sent["valor_frete"] != "1.01" {
				t.Error("wrong transmitted freight")
			}
		} else {
			gets.Add(1)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"ref": ref, "cnpj_emitente": profile.Input.CNPJ, "status": "autorizado", "chave_nfe": key})
	})
	for i := 0; i < 2; i++ {
		out, e := provider.submit(ctx, a, "freight-sale", intent, ref, payload)
		if e != nil || out.Key != key {
			t.Fatal("same-reference recovery failed", e)
		}
	}
	if posts.Load() != 1 || gets.Load() != 1 {
		t.Fatal("duplicate submission", posts.Load(), gets.Load())
	}
}
