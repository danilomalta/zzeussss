package onlinecatalog

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"strings"
	"unicode/utf8"
)

const MaxBarcodeCSVBytes = 16384
const BarcodeCSVHeader = "product_id;expected_version;code\n"

type BarcodeCSVInput struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
	CSV         string `json:"csv"`
	PreviewHash string `json:"preview_hash,omitempty"`
}

// Safe diagnostics never contain submitted fields, SQL or raw parser errors.
type BarcodeCSVError struct {
	Line  int    `json:"line"`
	Field string `json:"field"`
	Code  string `json:"code"`
}

func (e *BarcodeCSVError) Error() string { return "CSV de códigos inválido" }
func (e *BarcodeCSVError) Unwrap() error { return ErrInput }
func csvBarcodeError(line int, field, code string) error {
	return &BarcodeCSVError{Line: line, Field: field, Code: code}
}
func ParseBarcodeCSV(source string) ([]BarcodeBatchItem, error) {
	if len(source) == 0 || len(source) > MaxBarcodeCSVBytes || !utf8.ValidString(source) {
		return nil, csvBarcodeError(0, "csv", "size_or_encoding")
	}
	source = strings.TrimPrefix(source, "\ufeff")
	reader := csv.NewReader(strings.NewReader(source))
	reader.Comma = ';'
	reader.FieldsPerRecord = 3
	header, e := reader.Read()
	if e != nil || strings.Join(header, ";") != "product_id;expected_version;code" {
		line := 1
		if e == nil && len(header) > 0 {
			line, _ = reader.FieldPos(0)
		} else {
			var pe *csv.ParseError
			if errors.As(e, &pe) {
				line = pe.StartLine
			}
		}
		return nil, csvBarcodeError(line, "header", "header")
	}
	items := make([]BarcodeBatchItem, 0)
	seenProducts := map[int64]bool{}
	seenCodes := map[string]bool{}
	for {
		row, e := reader.Read()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			line := 0
			var pe *csv.ParseError
			if errors.As(e, &pe) {
				line = pe.StartLine
			}
			return nil, csvBarcodeError(line, "csv", "syntax_or_columns")
		}
		line, _ := reader.FieldPos(0)
		if len(items) >= MaxBarcodeBatchItems {
			return nil, csvBarcodeError(line, "csv", "row_limit")
		}
		product, e := ProductID(row[0])
		if e != nil {
			return nil, csvBarcodeError(line, "product_id", "product_id")
		}
		version, e := ProductID(row[1])
		if e != nil || version >= MaxVersion {
			return nil, csvBarcodeError(line, "expected_version", "version")
		}
		key, e := CanonicalBarcode(row[2])
		if e != nil {
			return nil, csvBarcodeError(line, "code", "gtin")
		}
		if seenProducts[product] {
			return nil, csvBarcodeError(line, "product_id", "duplicate_product")
		}
		if seenCodes[key] {
			return nil, csvBarcodeError(line, "code", "duplicate_gtin")
		}
		seenProducts[product] = true
		seenCodes[key] = true
		items = append(items, BarcodeBatchItem{ProductID: product, ExpectedVersion: version, Code: row[2]})
	}
	if len(items) == 0 {
		return nil, csvBarcodeError(0, "csv", "empty")
	}
	return items, nil
}
func DecodeBarcodeCSV(content string, body []byte, apply bool) (BarcodeBatchInput, error) {
	var v BarcodeCSVInput
	kind, _, e := mime.ParseMediaType(content)
	if e != nil || kind != "application/json" || len(body) > 32768 || !utf8.Valid(body) {
		return BarcodeBatchInput{}, ErrInput
	}
	allowed := map[string]bool{"operation_id": true, "reason": true, "csv": true}
	if apply {
		allowed["preview_hash"] = true
	}
	fields, e := batchObject(body, allowed)
	if e != nil || len(fields) != len(allowed) || json.Unmarshal(body, &v) != nil {
		return BarcodeBatchInput{}, ErrInput
	}
	// Validate metadata before parsing the supplied file.
	if !ValidUUID(v.OperationID) || !clean(strings.TrimSpace(v.Reason), 500, true) || (apply && !importDigest(v.PreviewHash)) {
		return BarcodeBatchInput{}, ErrInput
	}
	items, e := ParseBarcodeCSV(v.CSV)
	if e != nil {
		return BarcodeBatchInput{}, e
	}
	return normalizeBarcodeBatch(BarcodeBatchInput{OperationID: v.OperationID, Reason: v.Reason, Items: items, PreviewHash: v.PreviewHash}, apply)
}
func (s Store) BarcodeCSVTemplate(ctx context.Context, a Actor) (string, error) {
	if !a.allowed("details") {
		return "", ErrDenied
	}
	if s.DB == nil {
		return "", ErrUnavailable
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", ErrUnavailable
	}
	defer tx.Rollback()
	if e = batchLive(ctx, tx, a); e != nil {
		return "", e
	}
	if tx.Commit() != nil {
		return "", ErrUnavailable
	}
	return BarcodeCSVHeader, nil
}
