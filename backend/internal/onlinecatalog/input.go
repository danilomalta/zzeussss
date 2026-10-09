package onlinecatalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"mime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxVersion int64 = 9007199254740991
const MaxPrice int64 = 999999999999

type Change struct {
	OperationID     string `json:"operation_id"`
	ExpectedVersion int64  `json:"expected_version"`
	Name            string `json:"nome,omitempty"`
	Description     string `json:"descricao,omitempty"`
	SKU             string `json:"sku,omitempty"`
	PriceCents      int64  `json:"price_cents,omitempty"`
	Active          bool   `json:"ativo,omitempty"`
}

func ValidUUID(s string) bool {
	v, e := uuid.Parse(s)
	return e == nil && v != uuid.Nil && v.String() == s
}
func ProductID(s string) (int64, error) {
	if s == "" || len(s) > 16 || s[0] == '0' {
		return 0, ErrInput
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, ErrInput
		}
	}
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n < 1 || n > MaxVersion {
		return 0, ErrInput
	}
	return n, nil
}
func clean(s string, max int, required bool) bool {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || (required && strings.TrimSpace(s) == "") {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Decode rejects duplicate/unknown keys, nulls and coercion. Cents are integers.
func Decode(content string, body []byte, action string) (Change, error) {
	var c Change
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return c, ErrInput
	}
	allowed := map[string]bool{"operation_id": true, "expected_version": true}
	required := []string{}
	switch action {
	case "details":
		required = []string{"nome", "descricao", "sku"}
	case "price":
		required = []string{"price_cents"}
	case "active":
		required = []string{"ativo"}
	default:
		return c, ErrInput
	}
	for _, k := range required {
		allowed[k] = true
	}
	d := json.NewDecoder(bytes.NewReader(body))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return c, ErrInput
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		t, e = d.Token()
		if e != nil {
			return c, ErrInput
		}
		k, ok := t.(string)
		if !ok || !allowed[k] || fields[k] != nil {
			return c, ErrInput
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil || bytes.Equal(raw, []byte("null")) {
			return c, ErrInput
		}
		fields[k] = raw
	}
	if t, e = d.Token(); e != nil || t != json.Delim('}') {
		return c, ErrInput
	}
	if d.Decode(new(interface{})) != io.EOF {
		return c, ErrInput
	}
	if len(fields) != len(allowed) {
		return c, ErrInput
	}
	b, _ := json.Marshal(fields)
	if json.Unmarshal(b, &c) != nil || !ValidUUID(c.OperationID) || c.ExpectedVersion < 1 || c.ExpectedVersion >= MaxVersion {
		return c, ErrInput
	}
	if action == "details" {
		c.Name = strings.TrimSpace(c.Name)
		c.SKU = strings.TrimSpace(c.SKU)
		if !clean(c.Name, 255, true) || !clean(c.SKU, 100, true) || !clean(c.Description, 2000, false) {
			return c, ErrInput
		}
	}
	if action == "price" && (c.PriceCents < 0 || c.PriceCents > MaxPrice) {
		return c, ErrInput
	}
	return c, nil
}
func (c Change) hash(action string, id int64) string {
	b, _ := json.Marshal(struct {
		Action string
		ID     int64
		Change Change
	}{action, id, c})
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func cents(raw string) (int64, error) {
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || len(parts) == 0 {
		return 0, ErrUnavailable
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || whole < 0 || whole > MaxPrice/100 {
		return 0, ErrUnavailable
	}
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		if len(fraction) != 2 {
			return 0, ErrUnavailable
		}
	}
	n, e := strconv.ParseInt(fraction, 10, 64)
	if e != nil || n < 0 || n > 99 {
		return 0, ErrUnavailable
	}
	return whole*100 + n, nil
}
