package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var invalidProductInput = errors.New("invalid product input")
var productPriceSyntax = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)
var productStockSyntax = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)

// Limits match NUMERIC(12,2), INTEGER and character columns in migration 000002.
// Price is checked in integer cents before adapting to the existing float64 model.
func decodeProductInput(contentType string, body []byte) (CreateProductRequest, error) {
	var req CreateProductRequest
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return req, invalidProductInput
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return req, invalidProductInput
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		token, err = dec.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return req, invalidProductInput
		}
		if _, exists := fields[key]; exists {
			return req, invalidProductInput
		}
		switch key {
		case "nome", "sku", "descricao", "preco", "estoque":
		default:
			return req, invalidProductInput
		}
		var value json.RawMessage
		if dec.Decode(&value) != nil || bytes.Equal(value, []byte("null")) {
			return req, invalidProductInput
		}
		fields[key] = value
	}
	if token, err = dec.Token(); err != nil || token != json.Delim('}') {
		return req, invalidProductInput
	}
	var extra interface{}
	if dec.Decode(&extra) != io.EOF {
		return req, invalidProductInput
	}
	for key, target := range map[string]*string{"nome": &req.Nome, "sku": &req.SKU, "descricao": &req.Descricao} {
		if value, exists := fields[key]; exists && json.Unmarshal(value, target) != nil {
			return req, invalidProductInput
		}
	}
	req.Nome = strings.TrimSpace(req.Nome)
	req.SKU = strings.TrimSpace(req.SKU)
	if !validProductText(req.Nome, 255, false) || !validProductText(req.SKU, 100, false) || !validProductText(req.Descricao, 2000, true) {
		return req, invalidProductInput
	}
	if value, exists := fields["preco"]; exists {
		text := string(value)
		if !productPriceSyntax.MatchString(text) {
			return req, invalidProductInput
		}
		parts := strings.SplitN(text, ".", 2)
		whole, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil || whole > 9999999999 {
			return req, invalidProductInput
		}
		var fraction uint64
		if len(parts) == 2 {
			digits := parts[1]
			if len(digits) == 1 {
				digits += "0"
			}
			fraction, err = strconv.ParseUint(digits, 10, 64)
			if err != nil {
				return req, invalidProductInput
			}
		}
		req.priceCents = int64(whole*100 + fraction)
		req.Preco = float64(req.priceCents) / 100
	}
	if value, exists := fields["estoque"]; exists {
		text := string(value)
		if !productStockSyntax.MatchString(text) {
			return req, invalidProductInput
		}
		stock, err := strconv.ParseUint(text, 10, 31)
		if err != nil {
			return req, invalidProductInput
		}
		req.Estoque = int(stock)
	}
	return req, nil
}

func validProductText(value string, limit int, optional bool) bool {
	if !utf8.ValidString(value) || (!optional && value == "") || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
