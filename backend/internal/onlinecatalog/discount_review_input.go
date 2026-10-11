package onlinecatalog

import (
	"encoding/json"
	"mime"
	"strings"
	"unicode/utf8"
)

type DiscountReviewInput struct {
	OperationID            string `json:"operation_id"`
	ExpectedProductVersion int64  `json:"expected_product_version"`
	ExpectedDiscountBPS    int64  `json:"expected_discount_bps"`
	Reason                 string `json:"reason"`
}

func normalizeDiscountReview(v DiscountReviewInput) (DiscountReviewInput, error) {
	v.Reason = strings.TrimSpace(v.Reason)
	if !ValidUUID(v.OperationID) || v.ExpectedProductVersion < 1 || v.ExpectedProductVersion > MaxVersion ||
		v.ExpectedDiscountBPS < 0 || v.ExpectedDiscountBPS > 10000 || !clean(v.Reason, 500, true) {
		return v, ErrInput
	}
	return v, nil
}
func DecodeDiscountReview(content string, body []byte) (DiscountReviewInput, error) {
	var v DiscountReviewInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 8192 || !utf8.Valid(body) {
		return v, ErrInput
	}
	fields, e := batchObject(body, map[string]bool{"operation_id": true, "expected_product_version": true, "expected_discount_bps": true, "reason": true})
	if e != nil || len(fields) != 4 || json.Unmarshal(body, &v) != nil {
		return v, ErrInput
	}
	return normalizeDiscountReview(v)
}
