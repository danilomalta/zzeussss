package onlinecatalog

import (
	"context"
	"encoding/csv"
	"strconv"
	"strings"
)

const BarcodeCSVReportHeader = "product_id;product_version;barcode_id;barcode_version;code;canonical_code;barcode_active;product_active\n"

type BarcodeCSVReport struct {
	CSV            string
	Count          int
	ProductID      int64
	ProductVersion int64
	SourceHash     string
}

func buildBarcodeCSVReport(page BarcodePage) (BarcodeCSVReport, error) {
	if page.Product.ID < 1 || page.Product.ID > MaxVersion || page.Product.Version < 1 || page.Product.Version > MaxVersion || page.Total < 0 || page.Total > MaxProductBarcodes || page.HasMore || page.Offset != 0 || page.Total != len(page.Items) {
		return BarcodeCSVReport{}, ErrUnavailable
	}
	var out strings.Builder
	w := csv.NewWriter(&out)
	w.Comma = ';'
	if w.Write(strings.Split(strings.TrimSuffix(BarcodeCSVReportHeader, "\n"), ";")) != nil {
		return BarcodeCSVReport{}, ErrUnavailable
	}
	seen := map[string]bool{}
	for _, b := range page.Items {
		if !validBarcode(b) || b.ProductID != page.Product.ID || seen[b.CanonicalCode] {
			return BarcodeCSVReport{}, ErrUnavailable
		}
		seen[b.CanonicalCode] = true
		if w.Write([]string{strconv.FormatInt(b.ProductID, 10), strconv.FormatInt(page.Product.Version, 10), b.ID, strconv.FormatInt(b.Version, 10), b.Code, b.CanonicalCode, strconv.FormatBool(b.Active), strconv.FormatBool(page.Product.Active)}) != nil {
			return BarcodeCSVReport{}, ErrUnavailable
		}
	}
	w.Flush()
	if w.Error() != nil || out.Len() > MaxBarcodeCSVBytes {
		return BarcodeCSVReport{}, ErrUnavailable
	}
	return BarcodeCSVReport{CSV: out.String(), Count: len(page.Items), ProductID: page.Product.ID, ProductVersion: page.Product.Version, SourceHash: creationHash([]byte(out.String()))}, nil
}
func (s Store) ExportBarcodeCSV(ctx context.Context, a Actor, product int64) (BarcodeCSVReport, error) {
	if !a.allowed("details") {
		return BarcodeCSVReport{}, ErrDenied
	}
	p, e := s.ListBarcodes(ctx, a, product, "all", MaxProductBarcodes, 0)
	if e != nil {
		return BarcodeCSVReport{}, e
	}
	return buildBarcodeCSVReport(p)
}
