package onlinecatalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSupplierOfferStrictConditionsAndImmutablePair(t *testing.T) {
	v, r := offerFixture48()
	b, _ := json.Marshal(v)
	got, e := DecodeSupplierOffer("application/json", b, false)
	if e != nil || got != v {
		t.Fatal(got, e)
	}
	for _, body := range []string{`{}`, string(b) + `{}`, strings.Replace(string(b), `"minimum_units":2`, `"minimum_units":0`, 1), strings.Replace(string(b), `"unit_price_cents":250`, `"unit_price_cents":2.5`, 1), strings.Replace(string(b), `"unit_price_cents":250`, `"unit_price_cents":null`, 1), strings.Replace(string(b), `"lead_time_days":3`, `"lead_time_days":366`, 1), strings.Replace(string(b), `"supplier_sku":"SUP-1"`, `"supplier_sku":"SUP-1","supplier_sku":"x"`, 1)} {
		if _, e = DecodeSupplierOffer("application/json", []byte(body), false); e != ErrInput {
			t.Fatal(body, e)
		}
	}
	v.ExpectedVersion = 1
	b, _ = json.Marshal(v)
	if _, e = DecodeSupplierOffer("application/json", b, true); e != nil {
		t.Fatal(e)
	}
	if _, e = DecodeSupplierOffer("application/json", b, false); e != ErrInput {
		t.Fatal(e)
	}
	before := r.After
	r.Kind = "update"
	r.Before = &before
	r.After.Version = 2
	r.After.UnitPriceCents = 0
	if !validSupplierOfferReceipt(r) {
		t.Fatal("update")
	}
	r.After.ProductID++
	if validSupplierOfferReceipt(r) {
		t.Fatal("pair changed")
	}
	for _, q := range []SupplierOfferQuery{{Active: "all", Limit: 1, SupplierID: "bad"}, {Active: "all", Limit: 1, ProductID: -1}, {Active: "", Limit: 1}, {Active: "all", Limit: 101}} {
		if ValidateSupplierOfferQuery(q) != ErrInput {
			t.Fatal(q)
		}
	}
}
