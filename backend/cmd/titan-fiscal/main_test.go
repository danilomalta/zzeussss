package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
)

func TestFiscalOfflineCLICapabilitiesAndSafeFiles(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"capabilities"}, &output); err != nil || !strings.Contains(output.String(), `"official_issuance":false`) {
		t.Fatal(output.String(), err)
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "in.json")
	target := filepath.Join(dir, "out.json")
	input := `{"context":{"origin_uf":"SP","destination_uf":"SP","crt":"3","model":"55","date":"2026-10-26"},"item":{"product_id":"p","quantity":"2","unit_price":"50","discount":"0","freight":"0","insurance":"0","other":"0"},"rules":[{"id":"synthetic","product_id":"p","cfop":"5102","ncm":"09012100","origin_uf":"SP","destination_uf":"SP","crt":"3","model":"55","valid_from":"2026-01-01","valid_until":"2026-12-31","legal_reference":"synthetic test only","approval_reference":"test","components":[{"code":"TEST","method":"percentage","rate":"18","reduction_percent":"0","mva_percent":"0","adds_to_total":true}]}]}`
	if err := os.WriteFile(source, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"tax-quote", "--in", source, "--out", target}
	if err := run(args, &output); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(target)
	if !bytes.Contains(raw, []byte(`"total": "118.00"`)) {
		t.Fatal(string(raw))
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0600 {
		t.Fatal("permissions")
	}
	if err := run(args, &output); err == nil {
		t.Fatal("overwrote existing output")
	}
	link := filepath.Join(dir, "linked.json")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readRegular(link); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := run([]string{"transmit", "--in", source}, &output); err == nil {
		t.Fatal("unexpected transmission command")
	}
	if err := run([]string{"tax-quote", "--in", source, "--out", filepath.Join(dir, "other.json"), "extra"}, &output); err == nil {
		t.Fatal("extra arguments")
	}
}

func TestFiscalCLIValidatesEmbeddedSchemaOutsideRepository(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC), CNPJ: "12ABC34501DE35", Model: "65", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "query.xml")
	xml := fmt.Sprintf(`<consSitNFe xmlns="%s" versao="4.00"><tpAmb>2</tpAmb><xServ>CONSULTAR</xServ><chNFe>%s</chNFe></consSitNFe>`, fiscalcore.NFeNS, key)
	if err = os.WriteFile(source, []byte(xml), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = run([]string{"validate", "--kind", "nfe_query", "--in", source}, &output); err != nil {
		t.Fatal("embedded XSD unavailable from installed working directory", err)
	}
	if !strings.Contains(output.String(), "XSD valid") {
		t.Fatal("missing validation result")
	}
}
