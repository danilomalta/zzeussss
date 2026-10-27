package localapi

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
)

func boundFiscalInvoice(intent fiscal.Intent) fiscalcore.Invoice {
	s := intent.Snapshot
	p := s.Profile
	in := fiscalcore.Invoice{Key: fiscalcore.KeyInput{UF: p.State, IssuedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC), CNPJ: p.CNPJ, Model: p.Model, Series: p.Series, Number: 1, Emission: 1, NumericCode: "12345678"}, Environment: 2, CRT: p.CRT, Nature: "VENDA", Direction: "1", Destination: "1", FinalConsumer: "1", Presence: "1", Issuer: fiscalcore.Party{CNPJ: p.CNPJ, Name: p.LegalName, IE: p.StateRegistration, Address: fiscalcore.Address{UF: p.State, CityCode: p.Municipality}}, TaxTotals: map[string]string{}}
	for _, n := range []string{"vBC", "vICMS", "vICMSDeson", "vFCP", "vBCST", "vST", "vFCPST", "vFCPSTRet", "vII", "vIPI", "vIPIDevol", "vPIS", "vCOFINS"} {
		in.TaxTotals[n] = "0"
	}
	for _, item := range s.Items {
		in.Lines = append(in.Lines, fiscalcore.InvoiceLine{ProductID: item.ProductID, Quantity: new(big.Rat).SetFrac(big.NewInt(item.QuantityMilli), big.NewInt(1000)).FloatString(3), UnitPrice: new(big.Rat).SetFrac(big.NewInt(item.UnitPriceCents), big.NewInt(100)).FloatString(2), Discount: "0", Freight: "0", Insurance: "0", Other: "0"})
	}
	for _, p := range s.Payments {
		in.Payments = append(in.Payments, fiscalcore.InvoicePayment{Method: "01", Amount: new(big.Rat).SetFrac(big.NewInt(p.AmountCents), big.NewInt(100)).FloatString(2)})
	}
	return in
}

func TestFiscalInvoiceBindingUsesOriginalSaleAndExactAmounts(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	base := boundFiscalInvoice(intent)
	if err := r.ValidateInvoice(context.Background(), base); err != nil {
		t.Fatal("original sale", err)
	}
	tests := map[string]func(*fiscalcore.Invoice){
		"issuer":          func(in *fiscalcore.Invoice) { in.Issuer.CNPJ = "11222333000181" },
		"name":            func(in *fiscalcore.Invoice) { in.Issuer.Name = "Different issuer" },
		"registration":    func(in *fiscalcore.Invoice) { in.Issuer.IE = "000000000" },
		"municipality":    func(in *fiscalcore.Invoice) { in.Issuer.Address.CityCode = "3509502" },
		"state":           func(in *fiscalcore.Invoice) { in.Issuer.Address.UF = "RJ" },
		"regime":          func(in *fiscalcore.Invoice) { in.CRT = "3" },
		"production":      func(in *fiscalcore.Invoice) { in.Environment = 1 },
		"number":          func(in *fiscalcore.Invoice) { in.Key.Number = 2 },
		"contingency":     func(in *fiscalcore.Invoice) { in.Key.Emission = 9 },
		"direction":       func(in *fiscalcore.Invoice) { in.Direction = "0" },
		"product":         func(in *fiscalcore.Invoice) { in.Lines[0].ProductID = "other-product" },
		"quantity":        func(in *fiscalcore.Invoice) { in.Lines[0].Quantity = "1.9999999999" },
		"price":           func(in *fiscalcore.Invoice) { in.Lines[0].UnitPrice = "15.9899999999" },
		"exponent":        func(in *fiscalcore.Invoice) { in.Lines[0].UnitPrice = "1.599e1" },
		"negative":        func(in *fiscalcore.Invoice) { in.Lines[0].Quantity = "-2" },
		"extra_line":      func(in *fiscalcore.Invoice) { in.Lines = append(in.Lines, in.Lines[0]) },
		"missing_line":    func(in *fiscalcore.Invoice) { in.Lines = nil },
		"discount":        func(in *fiscalcore.Invoice) { in.Lines[0].Discount = "0.01" },
		"freight":         func(in *fiscalcore.Invoice) { in.Lines[0].Freight = "0.01" },
		"insurance":       func(in *fiscalcore.Invoice) { in.Lines[0].Insurance = "0.01" },
		"other_charge":    func(in *fiscalcore.Invoice) { in.Lines[0].Other = "0.01" },
		"credit":          func(in *fiscalcore.Invoice) { in.Payments[0].Method = "03" },
		"payment_amount":  func(in *fiscalcore.Invoice) { in.Payments[0].Amount = "31.99" },
		"missing_payment": func(in *fiscalcore.Invoice) { in.Payments = nil },
		"extra_payment":   func(in *fiscalcore.Invoice) { in.Payments = append(in.Payments, in.Payments[0]) },
		"additional_tax":  func(in *fiscalcore.Invoice) { in.TaxTotals["vST"] = "0.01" },
		"missing_total":   func(in *fiscalcore.Invoice) { delete(in.TaxTotals, "vII") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(base)
			var in fiscalcore.Invoice
			if err := json.Unmarshal(raw, &in); err != nil {
				t.Fatal(err)
			}
			mutate(&in)
			if err := r.ValidateInvoice(context.Background(), in); err == nil {
				t.Fatal("mismatched sale accepted")
			}
		})
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_requests`).Scan(&n); err != nil || n != 0 {
		t.Fatal("validation reserved or sent a request", n, err)
	}
}

func TestFiscalInvoiceBindingLogoutBlocksPreparationWithoutLosingIntent(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	r := fiscalJournal(t, f, f.db, intent, f.token)
	if err := localauth.Logout(context.Background(), f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateInvoice(context.Background(), boundFiscalInvoice(intent)); err == nil {
		t.Fatal("stale session built invoice")
	}
	nextToken := loginToken(t, f.app, f.owner.OwnerID)
	next := fiscalJournal(t, f, f.db, intent, nextToken)
	if err := next.ValidateInvoice(context.Background(), boundFiscalInvoice(intent)); err == nil {
		t.Fatal("new session took over old intent")
	}
	var n int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_intents WHERE id=?`, intent.ID).Scan(&n); err != nil || n != 1 {
		t.Fatal("original intent lost", err)
	}
}
