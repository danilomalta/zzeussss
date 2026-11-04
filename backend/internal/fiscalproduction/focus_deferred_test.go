package fiscalproduction

import (
	"encoding/json"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestFocusDeferredPaymentExplicitDocumentedCodes(t *testing.T) {
	for _, code := range []string{"21", "91"} {
		snapshot := fiscal.Snapshot{Profile: fiscal.ProfileInput{Model: "65", Environment: "homologation"}, TotalCents: 3198, DeferredPayment: &fiscal.DeferredPayment{Code: code, DocumentID: "doc-one", SelectedBy: "owner-one", TotalCents: 3198}}
		rows, e := focusPayments(snapshot)
		if e != nil || len(rows) != 1 || rows[0]["forma_pagamento"] != code || rows[0]["indicador_pagamento"] != "1" || rows[0]["valor_pagamento"] != "31.98" {
			t.Fatal("deferred mapping", e)
		}
		raw, _ := json.Marshal(rows)
		if string(raw) == "" {
			t.Fatal("empty mapping")
		}
		snapshot.DeferredPayment.TotalCents++
		if _, e = focusPayments(snapshot); e == nil {
			t.Fatal("tampered amount accepted")
		}
		snapshot.DeferredPayment.TotalCents = 3198
		snapshot.Payments = []fiscal.Payment{{Method: "cash", Status: "confirmed", AmountCents: 3198}}
		if _, e = focusPayments(snapshot); e == nil {
			t.Fatal("deferred and received payment mixed")
		}
		snapshot.Payments = nil
		snapshot.DeferredPayment.Code = "01"
		if _, e = focusPayments(snapshot); e == nil {
			t.Fatal("implicit cash accepted")
		}
	}
	rows, e := focusPayments(fiscal.Snapshot{Payments: []fiscal.Payment{{Method: "cash", Status: "confirmed", AmountCents: 3198}}})
	if e != nil || len(rows) != 1 || rows[0]["forma_pagamento"] != "01" {
		t.Fatal("cash changed", e)
	}
	if _, e = focusPayments(fiscal.Snapshot{}); e == nil {
		t.Fatal("missing origin accepted")
	}
}
