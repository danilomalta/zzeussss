package onlinecatalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func TestBarcodeCSVPreservesZerosAndNormalizesSameOperationAsJSON(t *testing.T) {
	source := BarcodeCSVHeader + "2;1;036000291452\n1;1;4006381333931\n"
	for _, csv := range []string{source, "\ufeff" + strings.ReplaceAll(source, "\n", "\r\n"), BarcodeCSVHeader + "\"2\";\"1\";\"036000291452\"\n\n1;1;4006381333931\n"} {
		body, _ := json.Marshal(BarcodeCSVInput{OperationID: testOp, Reason: " Conferido ", CSV: csv})
		b, e := DecodeBarcodeCSV("application/json", body, false)
		if e != nil || len(b.Items) != 2 || b.Items[0].ProductID != 1 || b.Items[1].Code != "036000291452" || b.Reason != "Conferido" {
			t.Fatal(b, e)
		}
		expected := BarcodeBatchInput{OperationID: testOp, Reason: "Conferido", Items: []BarcodeBatchItem{{ProductID: 1, ExpectedVersion: 1, Code: "4006381333931"}, {ProductID: 2, ExpectedVersion: 1, Code: "036000291452"}}}
		actual, _ := json.Marshal(b)
		want, _ := json.Marshal(expected)
		if string(actual) != string(want) {
			t.Fatal("identity differs from JSON")
		}
	}
}
func TestBarcodeCSVRejectsEveryInvalidRowWithoutLeakingSource(t *testing.T) {
	cases := []struct {
		csv, code string
		line      int
	}{
		{"", "size_or_encoding", 0}, {"sku;expected_version;code\n1;1;4006381333931\n", "header", 1},
		{"\n\nsku;expected_version;code\n", "header", 3},
		{BarcodeCSVHeader, "empty", 0}, {BarcodeCSVHeader + "1;1;4006381333931;extra\n", "syntax_or_columns", 2},
		{BarcodeCSVHeader + "01;1;4006381333931\n", "product_id", 2},
		{BarcodeCSVHeader + "1;1.0;4006381333931\n", "version", 2},
		{BarcodeCSVHeader + "1;9007199254740991;4006381333931\n", "version", 2},
		{BarcodeCSVHeader + "1;1;4006381333932\n", "gtin", 2},
		{BarcodeCSVHeader + "1;1;=4006381333931\n", "gtin", 2},
		{BarcodeCSVHeader + "1;1;36000291452\n", "gtin", 2},
		{BarcodeCSVHeader + "1;1; 036000291452\n", "gtin", 2},
		{BarcodeCSVHeader + "1;1;4006381333931\n1;1;036000291452\n", "duplicate_product", 3},
		{BarcodeCSVHeader + "1;1;036000291452\n2;1;0036000291452\n", "duplicate_gtin", 3},
		{BarcodeCSVHeader + "1;1;\"private", "syntax_or_columns", 2},
	}
	for _, tc := range cases {
		items, e := ParseBarcodeCSV(tc.csv)
		var diagnostic *BarcodeCSVError
		if items != nil || !errors.Is(e, ErrInput) || !errors.As(e, &diagnostic) || diagnostic.Code != tc.code || diagnostic.Line != tc.line {
			t.Fatal(tc.code, diagnostic, e)
		}
		if strings.Contains(e.Error(), "private") || strings.Contains(e.Error(), "400638") {
			t.Fatal("raw source leaked")
		}
	}
	for _, source := range []string{strings.Repeat("x", MaxBarcodeCSVBytes+1), string([]byte{0xff})} {
		if _, e := ParseBarcodeCSV(source); !errors.Is(e, ErrInput) {
			t.Fatal(e)
		}
	}
}
func csvGTIN34(base string) string {
	sum := 0
	w := 3
	for i := len(base) - 1; i >= 0; i-- {
		sum += int(base[i]-'0') * w
		w = 4 - w
	}
	return base + fmt.Sprint((10-sum%10)%10)
}
func TestBarcodeCSVBoundedRowsAndStrictEnvelope(t *testing.T) {
	source := BarcodeCSVHeader
	for i := 1; i <= 50; i++ {
		source += fmt.Sprintf("%d;1;%s\n", i, csvGTIN34(fmt.Sprintf("78900000%04d", i)))
	}
	if items, e := ParseBarcodeCSV(source); e != nil || len(items) != 50 {
		t.Fatal(e)
	}
	if _, e := ParseBarcodeCSV(source + "51;1;4006381333931\n"); !errors.Is(e, ErrInput) {
		t.Fatal("51 rows accepted")
	}
	v := BarcodeCSVInput{OperationID: testOp, Reason: "Conferido", CSV: BarcodeCSVHeader + "1;1;4006381333931\n"}
	raw, _ := json.Marshal(v)
	bad := []string{string(raw) + `{}`, strings.Replace(string(raw), `"reason":`, `"tenant_id":"x","reason":`, 1), strings.Replace(string(raw), `"csv":`, `"csv":"x","csv":`, 1), strings.Replace(string(raw), `"csv":`, `"csv":null,"csv":`, 1), strings.Replace(string(raw), `"reason":"Conferido"`, `"reason":" "`, 1), strings.Replace(string(raw), testOp, "invalid", 1)}
	for _, body := range bad {
		if _, e := DecodeBarcodeCSV("application/json", []byte(body), false); !errors.Is(e, ErrInput) {
			t.Fatal("ambiguous accepted", e)
		}
	}
	if _, e := DecodeBarcodeCSV("text/csv", raw, false); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := DecodeBarcodeCSV("application/json", make([]byte, 32769), false); e != ErrInput {
		t.Fatal(e)
	}
	if _, e := DecodeBarcodeCSV("application/json", raw, true); e != ErrInput {
		t.Fatal("apply without hash accepted")
	}
	v.PreviewHash = strings.Repeat("a", 64)
	raw, _ = json.Marshal(v)
	if _, e := DecodeBarcodeCSV("application/json", raw, true); e != nil {
		t.Fatal(e)
	}
	if _, e := DecodeBarcodeCSV("application/json", raw, false); e != ErrInput {
		t.Fatal("preview extra hash accepted")
	}
}
func TestBarcodeCSVTemplateRequiresLiveRoleAndNeverReadsProducts(t *testing.T) {
	for _, mode := range []string{"ok", "revoked", "commit"} {
		s, m := mockStore(t)
		m.ExpectBegin()
		if mode == "revoked" {
			m.ExpectQuery(`SELECT s.id`).WillReturnError(sql.ErrNoRows)
			m.ExpectRollback()
		} else {
			undoLive(m)
			if mode == "commit" {
				m.ExpectCommit().WillReturnError(errors.New("private"))
			} else {
				m.ExpectCommit()
			}
		}
		result, e := s.BarcodeCSVTemplate(context.Background(), fixtureActor)
		if mode == "ok" {
			if e != nil || result != BarcodeCSVHeader {
				t.Fatal(e)
			}
		} else if result != "" || e == nil {
			t.Fatal("false success")
		}
	}
	a := fixtureActor
	a.Role = "cashier"
	if _, e := (Store{}).BarcodeCSVTemplate(context.Background(), a); e != ErrDenied {
		t.Fatal(e)
	}
}
func TestBarcodeCSVReportIncludesBothStatesAndPreservesDigitStrings(t *testing.T) {
	_, b, r := barcodeFixture()
	b.Code = "036000291452"
	b.CanonicalCode, _ = CanonicalBarcode(b.Code)
	b.Active = false
	page := BarcodePage{Product: r.ProductBefore, Items: []Barcode{b}, Total: 1, Limit: 50}
	report, e := buildBarcodeCSVReport(page)
	if e != nil || report.Count != 1 || report.ProductVersion != 1 || !strings.Contains(report.CSV, ";036000291452;00036000291452;false;true\n") || !importDigest(report.SourceHash) {
		t.Fatal(report, e)
	}
	empty := page
	empty.Total = 0
	empty.Items = nil
	report, e = buildBarcodeCSVReport(empty)
	if e != nil || report.CSV != BarcodeCSVReportHeader {
		t.Fatal(e)
	}
	for _, mode := range []string{"incomplete", "foreign", "duplicate", "malformed"} {
		bad := page
		bad.Items = append([]Barcode(nil), page.Items...)
		switch mode {
		case "incomplete":
			bad.HasMore = true
		case "foreign":
			bad.Items[0].ProductID = 2
		case "duplicate":
			bad.Items = append(bad.Items, b)
			bad.Total = 2
		case "malformed":
			bad.Items[0].Code = "=private"
		}
		if report, e := buildBarcodeCSVReport(bad); e != ErrUnavailable || report.CSV != "" {
			t.Fatal(mode, e)
		}
	}
	a := fixtureActor
	a.Role = "cashier"
	if _, e := (Store{}).ExportBarcodeCSV(context.Background(), a, 1); e != ErrDenied {
		t.Fatal(e)
	}
}
func TestBarcodeCSVReportUsesSingleScopedSnapshot(t *testing.T) {
	s, m := mockStore(t)
	_, b, r := barcodeFixture()
	m.ExpectBegin()
	undoLive(m)
	batchRow(m, r.ProductBefore)
	m.ExpectQuery(`SELECT count`).WithArgs(fixtureActor.Tenant, int64(1), "all").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	m.ExpectQuery(`SELECT id::text.*online_catalog_barcodes`).WithArgs(fixtureActor.Tenant, int64(1), "all", 50, 0).WillReturnRows(barcodeRows(b))
	m.ExpectCommit()
	if report, e := s.ExportBarcodeCSV(context.Background(), fixtureActor, 1); e != nil || report.Count != 1 {
		t.Fatal(report, e)
	}
}
