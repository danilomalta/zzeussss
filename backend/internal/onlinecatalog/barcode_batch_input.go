package onlinecatalog

import (
	"encoding/json"
	"github.com/google/uuid"
	"mime"
	"sort"
	"strings"
	"unicode/utf8"
)

const MaxBarcodeBatchItems = 50

type BarcodeBatchItem struct {
	ProductID       int64  `json:"product_id"`
	ExpectedVersion int64  `json:"expected_version"`
	Code            string `json:"code"`
}
type BarcodeBatchInput struct {
	OperationID string             `json:"operation_id"`
	Reason      string             `json:"reason"`
	Items       []BarcodeBatchItem `json:"items"`
	PreviewHash string             `json:"preview_hash,omitempty"`
}

func barcodeBatchChild(op string, product int64) string {
	return uuid.NewSHA1(uuid.MustParse(op), []byte("barcode-batch:"+jsonID(product))).String()
}
func normalizeBarcodeBatch(b BarcodeBatchInput, apply bool) (BarcodeBatchInput, error) {
	b.Reason = strings.TrimSpace(b.Reason)
	if !ValidUUID(b.OperationID) || !clean(b.Reason, 500, true) || len(b.Items) < 1 || len(b.Items) > MaxBarcodeBatchItems {
		return BarcodeBatchInput{}, ErrInput
	}
	if apply {
		if !importDigest(b.PreviewHash) {
			return BarcodeBatchInput{}, ErrInput
		}
	} else if b.PreviewHash != "" {
		return BarcodeBatchInput{}, ErrInput
	}
	b.Items = append([]BarcodeBatchItem(nil), b.Items...)
	sort.Slice(b.Items, func(i, j int) bool { return b.Items[i].ProductID < b.Items[j].ProductID })
	codes := map[string]bool{}
	for i, v := range b.Items {
		key, e := CanonicalBarcode(v.Code)
		if e != nil || v.ProductID < 1 || v.ProductID > MaxVersion || v.ExpectedVersion < 1 || v.ExpectedVersion >= MaxVersion || (i > 0 && b.Items[i-1].ProductID == v.ProductID) || codes[key] {
			return BarcodeBatchInput{}, ErrInput
		}
		codes[key] = true
	}
	return b, nil
}
func DecodeBarcodeBatch(content string, body []byte, apply bool) (BarcodeBatchInput, error) {
	var b BarcodeBatchInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 32768 || !utf8.Valid(body) {
		return b, ErrInput
	}
	allowed := map[string]bool{"operation_id": true, "reason": true, "items": true}
	if apply {
		allowed["preview_hash"] = true
	}
	fields, e := batchObject(body, allowed)
	if e != nil || len(fields) != len(allowed) {
		return b, ErrInput
	}
	if json.Unmarshal(fields["operation_id"], &b.OperationID) != nil || json.Unmarshal(fields["reason"], &b.Reason) != nil {
		return b, ErrInput
	}
	if apply && json.Unmarshal(fields["preview_hash"], &b.PreviewHash) != nil {
		return b, ErrInput
	}
	var items []json.RawMessage
	if json.Unmarshal(fields["items"], &items) != nil || len(items) < 1 || len(items) > MaxBarcodeBatchItems {
		return b, ErrInput
	}
	for _, raw := range items {
		f, e := batchObject(raw, map[string]bool{"product_id": true, "expected_version": true, "code": true})
		var item BarcodeBatchItem
		if e != nil || len(f) != 3 || json.Unmarshal(raw, &item) != nil {
			return b, ErrInput
		}
		b.Items = append(b.Items, item)
	}
	return normalizeBarcodeBatch(b, apply)
}
