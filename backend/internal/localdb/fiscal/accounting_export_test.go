package fiscal

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestAccountingExportMakesMissingXMLVisibleAndNeutralizesCSVFormulas(t *testing.T) {
	body, e := buildAccountingZIP([]exportRow{{Kind: "invoice", ID: "=external-formula", State: "xml_unavailable"}}, exportNotice["en"])
	if e != nil {
		t.Fatal(e)
	}
	archive, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		t.Fatal(e)
	}
	for _, file := range archive.File {
		if strings.HasSuffix(file.Name, ".xml") {
			t.Fatal("invented XML")
		}
		if file.Name == "summary.csv" {
			r, _ := file.Open()
			data, _ := io.ReadAll(r)
			r.Close()
			if !bytes.Contains(data, []byte("'=external-formula")) || !bytes.Contains(data, []byte("xml_unavailable")) {
				t.Fatal("unsafe or hidden partial row")
			}
		}
	}
}
