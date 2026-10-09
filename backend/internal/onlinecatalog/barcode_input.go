package onlinecatalog

import (
	"encoding/json"
	"mime"
	"strings"
	"unicode/utf8"
)

const MaxProductBarcodes = 50

// Preserve the scanned form; compare and look up the zero-padded 14 digit key.
// Check digit: alternate 3 and 1 from the rightmost data digit (GS1 modulo 10).
func CanonicalBarcode(code string) (string, error) {
	if len(code) != 8 && len(code) != 12 && len(code) != 13 && len(code) != 14 {
		return "", ErrInput
	}
	sum := 0
	weight := 3
	nonzero := false
	for i := len(code) - 1; i >= 0; i-- {
		if code[i] < '0' || code[i] > '9' {
			return "", ErrInput
		}
		nonzero = nonzero || code[i] != '0'
		if i < len(code)-1 {
			sum += int(code[i]-'0') * weight
			weight = 4 - weight
		}
	}
	if !nonzero || int(code[len(code)-1]-'0') != (10-sum%10)%10 {
		return "", ErrInput
	}
	return strings.Repeat("0", 14-len(code)) + code, nil
}

type BarcodeInput struct {
	OperationID         string `json:"operation_id"`
	ExpectedVersion     int64  `json:"expected_version"`
	Code                string `json:"code,omitempty"`
	ExpectedCodeVersion int64  `json:"expected_code_version,omitempty"`
	Active              *bool  `json:"ativo,omitempty"`
	Reason              string `json:"reason"`
}

func normalizeBarcode(v BarcodeInput, action string) (BarcodeInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || v.ExpectedVersion < 1 || v.ExpectedVersion >= MaxVersion || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	if action == "add" {
		if _, e := CanonicalBarcode(v.Code); e != nil || v.Active != nil || v.ExpectedCodeVersion != 0 {
			return v, ErrInput
		}
	} else if action == "active" {
		if v.Code != "" || v.Active == nil || v.ExpectedCodeVersion < 1 || v.ExpectedCodeVersion >= MaxVersion {
			return v, ErrInput
		}
	} else {
		return v, ErrInput
	}
	return v, nil
}
func DecodeBarcode(content string, body []byte, action string) (BarcodeInput, error) {
	var v BarcodeInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return v, ErrInput
	}
	keys := map[string]bool{"operation_id": true, "expected_version": true, "reason": true}
	if action == "add" {
		keys["code"] = true
	} else if action == "active" {
		keys["expected_code_version"] = true
		keys["ativo"] = true
	} else {
		return v, ErrInput
	}
	fields, e := batchObject(body, keys)
	if e != nil || len(fields) != len(keys) || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeBarcode(v, action)
}
