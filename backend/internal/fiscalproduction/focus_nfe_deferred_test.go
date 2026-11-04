package fiscalproduction

import (
	"context"
	"errors"
	"testing"
	"titansystem-backend/internal/localdb/fiscal"
)

func TestFocusNFe55DeferredPaymentMatchesDocumentedCodes(t *testing.T) {
	for _, code := range []string{"21", "91"} {
		s := fiscal.Snapshot{Profile: fiscal.ProfileInput{Model: "55", Environment: "homologation"}, TotalCents: 3198, DeferredPayment: &fiscal.DeferredPayment{Code: code, DocumentID: "ar-origin", SelectedBy: "owner-one", TotalCents: 3198}}
		rows, err := focusPayments(s)
		if err != nil || len(rows) != 1 || rows[0]["forma_pagamento"] != code || rows[0]["valor_pagamento"] != "31.98" || rows[0]["indicador_pagamento"] != "1" {
			t.Fatal("NF-e deferred mapping", err)
		}
		for _, invalid := range []string{"production", ""} {
			s.Profile.Environment = invalid
			if _, err = focusPayments(s); err == nil {
				t.Fatal("non-homologation deferred accepted")
			}
		}
		s.Profile.Environment = "homologation"
		s.Payments = []fiscal.Payment{{Method: "cash", Status: "confirmed", AmountCents: 3198}}
		if _, err = focusPayments(s); err == nil {
			t.Fatal("mixed cash/deferred accepted")
		}
		s.Payments = nil
		s.DeferredPayment.TotalCents++
		if _, err = focusPayments(s); err == nil {
			t.Fatal("unreconciled amount accepted")
		}
	}
}

func TestCustomerDeferredNFe55UsesModel55Gate(t *testing.T) {
	ctx := context.Background()
	r, a, _ := customerFixture(t)
	profile, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, "profile", 0)
	if err != nil {
		t.Fatal(err)
	}
	in := profile.Input
	in.OperationID = "deferred-profile55"
	in.ExpectedRevision = profile.Revision
	in.Model = "55"
	if _, err = fiscal.Configure(ctx, a.DB, a.License, a.Actor, a.Device, in, a.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = r.SaveCustomerCredentials(ctx, a, "profile", CustomerCredentialInput{OperationID: "deferred-nfe-token", ProfileRevision: 2, Token: []byte("SyntheticNFeToken123")}); err != nil {
		t.Fatal(err)
	}
	// No network or fixture success: a real Focus provider must pass its 55 gate
	// and reach the missing authoritative credit-sale origin.
	err = r.PrepareCustomerDeferredNFe55(ctx, a, "missing-sale", "21")
	if !errors.Is(err, fiscal.ErrNotFound) {
		t.Fatal("did not reach NF-e credit origin", err)
	}
}
