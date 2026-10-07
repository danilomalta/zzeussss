package localapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Validate the schema subset used by this domain's OpenAPI against actual HTTP
// responses. This is not a general OpenAPI validator: unused features must not
// silently be introduced without extending this checker.
func domainSchema(t *testing.T, name string, body []byte) {
	domainSchemaFile(t, "catalog-stock.openapi.json", name, body)
}

func domainSchemaFile(t *testing.T, file, name string, body []byte) {
	t.Helper()
	raw, e := os.ReadFile("../../../docs/api/" + file)
	if e != nil {
		t.Fatal(e)
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		t.Fatal("invalid contract")
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		t.Fatal("invalid response JSON")
	}
	var check func(map[string]any, any) error
	check = func(s map[string]any, v any) error {
		if choices, ok := s["oneOf"].([]any); ok {
			matches := 0
			for _, candidate := range choices {
				if check(candidate.(map[string]any), v) == nil {
					matches++
				}
			}
			if matches != 1 {
				return fmt.Errorf("oneOf matched %d schemas", matches)
			}
			return nil
		}
		if v == nil && s["nullable"] == true {
			return nil
		}
		if ref, ok := s["$ref"].(string); ok {
			return check(schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any), v)
		}
		switch s["type"] {
		case "object":
			o, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("expected object")
			}
			if required, ok := s["required"].([]any); ok {
				for _, key := range required {
					if _, present := o[key.(string)]; !present {
						return fmt.Errorf("missing %s", key)
					}
				}
			}
			properties := s["properties"].(map[string]any)
			for key, val := range o {
				prop, exists := properties[key]
				if !exists {
					if s["additionalProperties"] == false {
						return fmt.Errorf("unknown %s", key)
					}
					continue
				}
				if e := check(prop.(map[string]any), val); e != nil {
					return fmt.Errorf("%s: %w", key, e)
				}
			}
		case "array":
			a, ok := v.([]any)
			if !ok {
				return fmt.Errorf("expected array")
			}
			if min, ok := s["minItems"].(float64); ok && len(a) < int(min) {
				return fmt.Errorf("too few items")
			}
			if max, ok := s["maxItems"].(float64); ok && len(a) > int(max) {
				return fmt.Errorf("too many items")
			}
			for _, item := range a {
				if e := check(s["items"].(map[string]any), item); e != nil {
					return e
				}
			}
		case "string":
			if _, ok := v.(string); !ok {
				return fmt.Errorf("expected string")
			}
		case "boolean":
			if _, ok := v.(bool); !ok {
				return fmt.Errorf("expected boolean")
			}
		case "integer":
			n, ok := v.(json.Number)
			if !ok {
				return fmt.Errorf("expected integer")
			}
			i, e := strconv.ParseInt(string(n), 10, 64)
			if e != nil {
				return fmt.Errorf("inexact integer")
			}
			if min, ok := s["minimum"].(float64); ok && i < int64(min) {
				return fmt.Errorf("below minimum")
			}
			if max, ok := s["maximum"].(float64); ok && i > int64(max) {
				return fmt.Errorf("above maximum")
			}
		default:
			return fmt.Errorf("unsupported schema type")
		}
		if values, ok := s["enum"].([]any); ok {
			matched := false
			for _, item := range values {
				if fmt.Sprint(v) == fmt.Sprint(item) {
					matched = true
				}
			}
			if !matched {
				return fmt.Errorf("invalid enum")
			}
		}
		return nil
	}
	if e := check(schemas[name].(map[string]any), value); e != nil {
		t.Fatalf("%s contract: %v", name, e)
	}
}

func TestCatalogStockOpenAPIValidatesRealPayloadsAndReplay(t *testing.T) {
	f := stockHTTPSetup(t)
	for _, item := range []struct{ path, schema string }{
		{"/local/v1/products?limit=1&offset=0", "ProductPage"},
		{"/local/v1/products?limit=1&offset=100", "ProductPage"},
		{"/local/v1/locations", "LocationList"},
		{"/local/v1/catalog/search?q=", "SearchPage"},
		{"/local/v1/catalog/search?q=not-found-contract-unique", "SearchPage"},
		{"/local/v1/stock/balance?product_id=" + f.product + "&location_id=" + f.back, "StockBalance"},
	} {
		status, body := request(t, f.app, "GET", item.path, "", f.token)
		if status != 200 {
			t.Fatal("documented read failed")
		}
		domainSchema(t, item.schema, body)
	}
	for _, item := range []struct{ path, input, body string }{
		{"/local/v1/products", "ProductInput", `{"sku":"contract-product","name":"Produto","unit":"kg","price_cents":1050,"cost_cents":700}`},
		{"/local/v1/locations", "LocationInput", `{"kind":"receiving","name":"Recebimento"}`},
	} {
		domainSchema(t, item.input, []byte(item.body))
		status, body := request(t, f.app, "POST", item.path, item.body, f.token)
		if status != 201 {
			t.Fatal("documented creation failed")
		}
		domainSchema(t, "Created", body)
	}
	in := stockEntry(f, "contract-entry", 1000)
	domainSchema(t, "StockInput", []byte(stockJSON(t, in)))
	first := stockRequest(t, f, in, 201)
	domainSchema(t, "StockResult", first)
	if !bytes.Contains(first, []byte(`"repeated":false`)) {
		t.Fatal("first operation not new")
	}
	replay := stockRequest(t, f, in, 200)
	domainSchema(t, "StockResult", replay)
	if !bytes.Contains(replay, []byte(`"repeated":true`)) {
		t.Fatal("replay not confirmed")
	}
	in.QuantityMilli = 2000
	stockRequest(t, f, in, 409)
	assertStockRows(t, f, 1, 1, 1)
	assertStockBalance(t, f, f.back, 1000)
}
