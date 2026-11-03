package fiscalcore

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
	"testing"
	"time"
)

func TestFiscalDPSOfficialHomologProfileBuildSignAndPack(t *testing.T) {
	for _, document := range []string{"11222333000181", "12ABC34501DE35"} {
		c := testCredentialDocument(t, document)
		in := DPSInput{Purpose: "0", Environment: 2, IssuedAt: time.Now(), Competence: "2026-10-26", Municipality: "3550308", ServiceMunicipality: "3550308", CNPJ: document, Series: 1, Number: 1, SimpleOption: "1", SpecialRegime: "0", NationalServiceCode: "010101", Description: "Synthetic service only <test>", ServiceValue: "100.00", Taxation: []TaxNode{
			{Name: "tribMun", Children: []TaxNode{{Name: "tribISSQN", Value: "1"}, {Name: "tpRetISSQN", Value: "1"}}},
			{Name: "totTrib", Children: []TaxNode{{Name: "indTotTrib", Value: "0"}}},
		}}
		raw, id, err := BuildDPSHomolog(in)
		if err != nil || len(id) != 45 {
			r, _, _ := buildDPSProfile(in, true)
			doc, _ := helium.NewParser().Parse(context.Background(), r)
			schema, _ := compileSchema("nfse_dps_homolog_20261020")
			collector := helium.NewErrorCollector(context.Background(), helium.ErrorLevelNone)
			_ = xsd.NewValidator(schema).ErrorHandler(collector).Validate(context.Background(), doc)
			doc.Free()
			t.Fatalf("synthetic schema diagnostics: %v; %v", collector.Errors(), err)
		}
		signed, err := c.SignXML("nfse_dps_homolog_20261020", raw, time.Now())
		if err != nil {
			t.Fatal("sign", err)
		}
		packed, err := c.PackDPSHomolog(signed, time.Now())
		if err != nil {
			t.Fatal("pack", err)
		}
		var body map[string]string
		if json.Unmarshal(packed, &body) != nil || body["dpsXmlGZipB64"] == "" {
			t.Fatal("envelope")
		}
		if c.VerifyXML("nfse_dps_homolog_20261020", bytes.Replace(signed, []byte("100.00"), []byte("101.00"), 1), time.Now()) == nil {
			t.Fatal("signed amount changed")
		}
		if _, err = c.PackDPS(signed, time.Now()); err == nil {
			t.Fatal("homologation silently accepted as old production profile")
		}
		in.Environment = 1
		if _, _, err = BuildDPSHomolog(in); err == nil {
			t.Fatal("homologation used as production")
		}
		in.Environment = 2
		in.Series = 90000
		if _, _, err = BuildDPSHomolog(in); err == nil {
			t.Fatal("reserved series")
		}
		in.Series = 89999
		if _, _, err = BuildDPSHomolog(in); err != nil {
			t.Fatal("valid series boundary", err)
		}
	}
}
