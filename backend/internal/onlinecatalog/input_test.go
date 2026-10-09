package onlinecatalog

import (
	"strings"
	"testing"
)

const testOp = "11111111-1111-4111-8111-111111111111"

func TestStrictChangesRejectAmbiguousOrForeignInput(t *testing.T) {
	valid := `{"operation_id":"` + testOp + `","expected_version":1,"price_cents":250}`
	for _, body := range []string{
		strings.Replace(valid, `250`, `2.5`, 1), strings.Replace(valid, `250`, `2e2`, 1), strings.Replace(valid, `250`, `-1`, 1), strings.Replace(valid, `250`, `1000000000000`, 1), strings.Replace(valid, `250`, `null`, 1),
		strings.Replace(valid, `"expected_version":1`, `"expected_version":0`, 1), strings.Replace(valid, `"expected_version":1`, `"expected_version":1,"expected_version":2`, 1),
		strings.Replace(valid, `"price_cents":250`, `"tenant_id":"other","price_cents":250`, 1), valid + `{}`, `[]`, strings.Replace(valid, testOp, "00000000-0000-0000-0000-000000000000", 1),
	} {
		if _, e := Decode("application/json", []byte(body), "price"); e == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	if _, e := Decode("text/plain", []byte(valid), "price"); e == nil {
		t.Fatal("accepted non JSON")
	}
	for _, value := range []string{"0", "250", "999999999999"} {
		c, e := Decode("application/json; charset=utf-8", []byte(strings.Replace(valid, `250`, value, 1)), "price")
		if e != nil || c.ExpectedVersion != 1 {
			t.Fatal("exact cents rejected")
		}
	}
	for _, body := range []string{`{"operation_id":"` + testOp + `","expected_version":1,"ativo":false}`, `{"operation_id":"` + testOp + `","expected_version":1,"nome":" Item ","descricao":"","sku":" ABC "}`} {
		action := "details"
		if strings.Contains(body, "ativo") {
			action = "active"
		}
		if _, e := Decode("application/json", []byte(body), action); e != nil {
			t.Fatal(e)
		}
	}
	invalid := `{"operation_id":"` + testOp + `","expected_version":1,"nome":"Item","descricao":"","sku":"ABC"}`
	for _, body := range []string{strings.Replace(invalid, `"ABC"`, `" "`, 1), strings.Replace(invalid, `"Item"`, `"bad\nname"`, 1), strings.Replace(invalid, `"Item"`, `"`+strings.Repeat("a", 256)+`"`, 1), strings.Replace(invalid, `"descricao":"",`, "", 1)} {
		if _, e := Decode("application/json", []byte(body), "details"); e == nil {
			t.Fatal("invalid details accepted")
		}
	}
}
func TestExactPriceAndIdentityBoundaries(t *testing.T) {
	for raw, want := range map[string]int64{"0.00": 0, "2.50": 250, "9999999999.99": MaxPrice} {
		got, e := cents(raw)
		if e != nil || got != want {
			t.Fatal(raw, got, e)
		}
	}
	for _, raw := range []string{"NaN", "1.001", "-1.00", "10000000000.00"} {
		if _, e := cents(raw); e == nil {
			t.Fatal("unsafe price", raw)
		}
	}
	for _, id := range []string{"0", "01", "-1", "1e2", "9007199254740992"} {
		if _, e := ProductID(id); e == nil {
			t.Fatal("unsafe ID", id)
		}
	}
}
