package fiscalproduction

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// An outbound sale snapshot must never become an incoming invoice through a
// caller-controlled direction flag. No database or transport is supplied:
// rejection must happen before reading taxes or producing any external effect.
func TestFocusIncomingRejectsSaleDirectionFlagBeforeEffects(t *testing.T) {
	invoice := fiscalcore.Invoice{
		Environment: 2, Nature: "OPERACAO SINTETICA", Presence: "1",
		Direction: "0", Destination: "1", FinalConsumer: "1",
		Recipient: &fiscalcore.Party{
			CPF: "12345678909", Name: "Synthetic consumer", IEDestination: "9",
			Address: fiscalcore.Address{Street: "Rua Teste", Number: "1", District: "Centro", City: "Sao Paulo", UF: "SP"},
		},
	}
	intent := fiscal.Intent{Snapshot: fiscal.Snapshot{
		SaleID:  "synthetic-outbound-sale",
		Profile: fiscal.ProfileInput{Model: "55", Environment: "homologation", State: "SP"},
		Items:   []fiscal.SaleItem{{ProductID: "synthetic-product", QuantityMilli: 1000, UnitPriceCents: 100, TotalCents: 100}},
	}}
	for _, purpose := range []string{"1", "2", "3", "4"} {
		t.Run(purpose, func(t *testing.T) {
			raw, err := json.Marshal(invoice)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := focusInvoiceRequest(context.Background(), RequestContext{}, intent, DocumentRequest{
				Invoice: raw, Purpose: purpose, FreightMode: "9",
			})
			if !errors.Is(err, ErrFocusNFeIncomplete) || len(payload) != 0 {
				t.Fatalf("sale direction flag accepted: payload bytes=%d, error=%v", len(payload), err)
			}
		})
	}
}

func TestApplyOriginReferenceUsesExplicitAccessKey(t *testing.T) {
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: "12345678000123", Model: "55", Series: 1, Number: 8, Emission: 1, NumericCode: "87654321"})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{}
	if err := applyOriginReference(payload, &fiscal.NFeOrigin{Kind: "inbound_receipt", ReferencedKey: key}); err != nil {
		t.Fatal(err)
	}
	refs, _ := payload["notas_referenciadas"].([]map[string]string)
	if len(refs) != 1 || refs[0]["chave_nfe"] != key {
		t.Fatal(payload)
	}
	if err := applyOriginReference(payload, &fiscal.NFeOrigin{Kind: "inbound_receipt", ReferencedKey: "short"}); !errors.Is(err, ErrReferencedOperationUnavailable) {
		t.Fatal(err)
	}
}
