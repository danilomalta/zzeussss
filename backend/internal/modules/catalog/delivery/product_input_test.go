package delivery

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProductInputExactBoundsAndCompatibleDefaults(t *testing.T) {
	for _, tc := range []struct {
		body  string
		price float64
		stock int
	}{
		{`{"nome":" Item ","sku":" sku "}`, 0, 0},
		{`{"nome":"Item","sku":"sku","preco":2.50,"estoque":1}`, 2.5, 1},
		{`{"nome":"Item","sku":"sku","preco":0.01}`, 0.01, 0},
		{`{"nome":"Item","sku":"sku","preco":9999999999.99,"estoque":2147483647}`, 9999999999.99, 2147483647},
	} {
		req, err := decodeProductInput("application/json; charset=utf-8", []byte(tc.body))
		if err != nil || req.Preco != tc.price || req.Estoque != tc.stock || req.Nome != "Item" || req.SKU != "sku" {
			t.Fatalf("valid boundary refused: %+v %v", req, err)
		}
	}
}

func TestProductInputRejectsAmbiguousOrUnsafeValues(t *testing.T) {
	bodies := []string{
		`null`, `[]`, `{}`, `{"nome":"Item","sku":"sku"} {}`,
		`{"nome":"Item","nome":"Other","sku":"sku"}`,
		`{"nome":"Item","sku":"sku","tenant_id":"foreign"}`,
		`{"nome":"Item","sku":"sku","ativo":false}`,
		`{"nome":null,"sku":"sku"}`, `{"nome":"Item","sku":"sku","descricao":null}`,
		`{"nome":42,"sku":"sku"}`, `{"nome":"Item","sku":"  "}`,
		`{"nome":"Item\u0000","sku":"sku"}`,
	}
	for _, value := range []string{`-1`, `-0`, `0.001`, `1e2`, `"2.50"`, `null`, `10000000000`, `184467440737095516160`, `{}`, `[]`, `true`} {
		bodies = append(bodies, `{"nome":"Item","sku":"sku","preco":`+value+`}`)
	}
	for _, value := range []string{`-1`, `1.0`, `1e2`, `"1"`, `2147483648`, `null`, `true`} {
		bodies = append(bodies, `{"nome":"Item","sku":"sku","estoque":`+value+`}`)
	}
	for _, tc := range []struct {
		key   string
		count int
	}{{"nome", 256}, {"sku", 101}, {"descricao", 2001}} {
		fields := map[string]string{"nome": "Item", "sku": "sku"}
		fields[tc.key] = strings.Repeat("a", tc.count)
		body, _ := json.Marshal(fields)
		bodies = append(bodies, string(body))
	}
	bodies = append(bodies, strings.Repeat(" ", 8193), string([]byte{'{', '"', 255, '"', ':', '0', '}'}))
	for i, body := range bodies {
		if _, err := decodeProductInput("application/json", []byte(body)); err == nil {
			t.Fatalf("unsafe case %d accepted", i)
		}
	}
	for _, media := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		if _, err := decodeProductInput(media, []byte(`{"nome":"Item","sku":"sku"}`)); err == nil {
			t.Fatal("wrong media type accepted")
		}
	}
}

func TestProductInputCharacterLimitsAreNotByteLimits(t *testing.T) {
	body := `{"nome":"` + strings.Repeat("é", 255) + `","sku":"sku"}`
	if _, err := decodeProductInput("application/json", []byte(body)); err != nil {
		t.Fatal(err)
	}
	body = `{"nome":"Item","sku":"` + strings.Repeat("a", 101) + `"}`
	if _, err := decodeProductInput("application/json", []byte(body)); err == nil {
		t.Fatal("oversized SKU accepted")
	}
}
