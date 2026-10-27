package fiscalcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestFiscalSchemaManifestCoversUnmodifiedEmbeddedFiles(t *testing.T) {
	raw, err := schemaFiles.ReadFile("schemas/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Copied map[string]struct {
			SHA256 string `json:"sha256"`
		} `json:"copied"`
		Generated map[string]struct {
			SHA256 string `json:"sha256"`
		} `json:"generated_root_bindings"`
	}
	// Read actual top-level member name without assuming a provenance field.
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		t.Fatal("manifest")
	}
	for key, value := range top {
		if key != "official_packages" && key != "generated_root_bindings" {
			if json.Unmarshal(value, &manifest.Copied) != nil {
				t.Fatal("hash registry")
			}
		}
	}
	if json.Unmarshal(top["generated_root_bindings"], &manifest.Generated) != nil {
		t.Fatal("generated binding")
	}
	seen := 0
	err = fs.WalkDir(schemaFiles, "schemas", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".xsd") {
			return nil
		}
		name := strings.TrimPrefix(path, "schemas/")
		expected := manifest.Copied[name].SHA256
		if expected == "" {
			expected = manifest.Generated[name].SHA256
		}
		raw, err := schemaFiles.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		if expected != hex.EncodeToString(sum[:]) {
			t.Fatalf("unrecorded/altered schema: %s", path)
		}
		seen++
		return nil
	})
	if err != nil || seen != 34 {
		t.Fatal(seen, err)
	}
}

func TestFiscalInvoiceRejectsTaxTotalMismatchAndUnsupportedGroups(t *testing.T) {
	in := testInvoice()
	in.TaxTotals["vICMS"] = "1.00"
	if _, _, err := BuildInvoice(in); err == nil {
		t.Fatal("invented aggregate")
	}
	in = testInvoice()
	in.Lines[0].Taxes = append(in.Lines[0].Taxes, TaxNode{Name: "IBSCBS", Children: []TaxNode{{Name: "CST", Value: "000"}}})
	if _, _, err := BuildInvoice(in); err == nil {
		t.Fatal("RTC omitted from totals")
	}
	in = testInvoice()
	raw, _, err := BuildInvoice(in)
	if err != nil {
		t.Fatal(err)
	}
	c := testCredential(t)
	raw = []byte(strings.Replace(string(raw), "<nNF>1</nNF>", "<nNF>2</nNF>", 1))
	if _, err = c.SignXML("nfe", raw, time.Now()); err == nil {
		t.Fatal("key/document mismatch signed")
	}
}

func TestFiscalJSONRejectsDuplicateAndCaseCollidingFields(t *testing.T) {
	for _, raw := range []string{`{"x":1,"x":2}`, `{"x":1,"X":2}`, `{"x":{"a":1,"a":2}}`, `{} {}`, `[` + strings.Repeat(`[`, 35) + `0` + strings.Repeat(`]`, 35) + `]`} {
		var target map[string]any
		if DecodeInput([]byte(raw), &target) == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
}
