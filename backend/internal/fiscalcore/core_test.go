package fiscalcore

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
	"math/big"
	"strings"
	"testing"
	"time"
)

func testCredential(t *testing.T) *Credential { return testCredentialDocument(t, "12ABC34501DE35") }
func testCredentialDocument(t *testing.T, document string) *Credential {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	oid, _ := asn1.Marshal(asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3})
	value, _ := asn1.Marshal(document)
	explicit, _ := asn1.Marshal(asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: value})
	san, _ := asn1.Marshal([]asn1.RawValue{{Class: 2, Tag: 0, IsCompound: true, Bytes: append(oid, explicit...)}})
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic fiscal test"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 17}, Value: san}}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	c, err := NewCredential(der, key, document, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func testInvoice() Invoice {
	a := Address{Street: "Rua Teste", Number: "1", District: "Centro", CityCode: "3550308", City: "Sao Paulo", UF: "SP", PostalCode: "01001000"}
	totals := map[string]string{}
	for _, k := range []string{"vBC", "vICMS", "vICMSDeson", "vFCP", "vBCST", "vST", "vFCPST", "vFCPSTRet", "vII", "vIPI", "vIPIDevol", "vPIS", "vCOFINS"} {
		totals[k] = "0.00"
	}
	return Invoice{Key: KeyInput{UF: "SP", IssuedAt: time.Date(2026, 10, 26, 12, 0, 0, 0, time.FixedZone("BRT", -3*3600)), CNPJ: "12ABC34501DE35", Model: "65", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"}, Environment: 2, CRT: "1", Nature: "VENDA", Direction: "1", Destination: "1", FinalConsumer: "1", Presence: "1", Issuer: Party{CNPJ: "12ABC34501DE35", Name: "Synthetic issuer", IE: "123456789", Address: a}, Lines: []InvoiceLine{{ProductID: "coffee", Description: "Synthetic coffee <test>", Unit: "UN", Quantity: "2", UnitPrice: "3.33333", Discount: "0", Freight: "0", Insurance: "0", Other: "0", NCM: "09012100", CFOP: "5102", Taxes: []TaxNode{{Name: "ICMS", Children: []TaxNode{{Name: "ICMSSN102", Children: []TaxNode{{Name: "orig", Value: "0"}, {Name: "CSOSN", Value: "102"}}}}}, {Name: "PIS", Children: []TaxNode{{Name: "PISNT", Children: []TaxNode{{Name: "CST", Value: "07"}}}}}, {Name: "COFINS", Children: []TaxNode{{Name: "COFINSNT", Children: []TaxNode{{Name: "CST", Value: "07"}}}}}}}}, Payments: []InvoicePayment{{Method: "01", Amount: "6.67"}}, TaxTotals: totals}
}

func TestFiscalOfficialSchemaCompilation(t *testing.T) {
	for _, kind := range []string{"nfe", "nfse_dps", "nfse_document", "nfse_event", "nfe_event", "nfse_dps_homolog_20261020"} {
		_, err := compileSchema(kind)
		if err != nil {
			t.Fatalf("schema %s: %v", kind, err)
		}
	}
}
func TestFiscalXMLBuildSignValidateAndTamper(t *testing.T) {
	cred := testCredential(t)
	for _, model := range []string{"55", "65"} {
		in := testInvoice()
		in.Key.Model = model
		if model == "55" {
			in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic buyer", IEDestination: "9", Address: in.Issuer.Address}
		}
		raw, key, err := BuildInvoice(in)
		if err != nil {
			raw, _, _ := buildInvoice(in)
			doc, _ := helium.NewParser().Parse(context.Background(), raw)
			schema, _ := compileSchema("nfe")
			collector := helium.NewErrorCollector(context.Background(), helium.ErrorLevelNone)
			detail := xsd.NewValidator(schema).ErrorHandler(collector).Validate(context.Background(), doc)
			t.Logf("synthetic-only schema errors: %v", collector.Errors())
			doc.Free()
			t.Fatalf("build %s: %v; synthetic schema diagnostics: %v", model, err, detail)
		}
		if !ValidAccessKey(key) || !bytes.Contains(raw, []byte("&lt;test&gt;")) {
			t.Fatal("bad serialization")
		}
		signed, err := cred.SignXML("nfe", raw, time.Now())
		if err != nil {
			t.Fatal("sign", model, err)
		}
		if err = cred.VerifyXML("nfe", signed, time.Now()); err != nil {
			t.Fatal("verify", err)
		}
		changed := bytes.Replace(signed, []byte("6.67"), []byte("6.68"), 1)
		if cred.VerifyXML("nfe", changed, time.Now()) == nil {
			t.Fatal("accepted altered signed payload")
		}
		if _, err = cred.SignXML("nfe", signed, time.Now()); err == nil {
			t.Fatal("signed twice")
		}
		if _, err = cred.SignXML("nfe", raw, time.Now().Add(2*time.Hour)); !errors.Is(err, ErrCredential) {
			t.Fatal("expired credential", err)
		}
	}
}
func TestFiscalXMLRejectsEntitiesWrappingAndUnknownFields(t *testing.T) {
	for _, raw := range []string{`<!DOCTYPE x [<!ENTITY y SYSTEM "file:///etc/passwd">]><x>&y;</x>`, `<x Id="same"><y Id="same"/></x>`, `<x a="1" a="2"/>`, `<x/><y/>`, `<x>` + strings.Repeat(`<n>`, 70) + strings.Repeat(`</n>`, 70) + `</x>`} {
		if _, err := parseXML([]byte(raw)); err == nil {
			t.Fatal("accepted unsafe XML")
		}
	}
	in := testInvoice()
	raw, _, err := BuildInvoice(in)
	if err != nil {
		t.Fatal(err)
	}
	cred := testCredential(t)
	if _, err := cred.SignXML("nfe", bytes.Replace(raw, []byte("<CRT>1</CRT>"), []byte("<CRT>9</CRT>"), 1), time.Now()); err == nil {
		t.Fatal("schema validation skipped")
	}
	in.Lines[0].Taxes = nil
	if _, _, err = BuildInvoice(in); !errors.Is(err, ErrRule) {
		t.Fatal("missing tax policy", err)
	}
}
func TestFiscalAccessKeysAllUFsAndAlphanumericDV(t *testing.T) {
	if len(ufCodes) != 27 {
		t.Fatal("lost UF")
	}
	for uf := range ufCodes {
		in := testInvoice().Key
		in.UF = uf
		key, err := AccessKey(in)
		if err != nil || !ValidAccessKey(key) || key[:2] != ufCodes[uf] {
			t.Fatal(uf, key, err)
		}
		bad := key[:43] + string('0'+byte((int(key[43]-'0')+1)%10))
		if ValidAccessKey(bad) {
			t.Fatal("wrong DV")
		}
	}
}
func TestFiscalExactTaxRulesSTAndNoImplicitZero(t *testing.T) {
	c := TaxContext{OriginUF: "SP", DestinationUF: "SP", CRT: "3", Model: "55", Date: "2026-10-26"}
	r := TaxRule{ID: "reviewed-test-1", ProductID: "p", CFOP: "5102", NCM: "09012100", OriginUF: "SP", DestinationUF: "SP", CRT: "3", Model: "55", ValidFrom: "2026-01-01", ValidUntil: "2026-12-31", LegalReference: "synthetic mathematical scenario; not a tax rate recommendation", ApprovalReference: "synthetic-review", Components: []TaxComponent{
		{Code: "ICMS", Method: "percentage", Rate: "18", ReductionPercent: "0", MVA: "0"},
		{Code: "ICMS_ST", Method: "percentage", Rate: "18", ReductionPercent: "0", MVA: "40", DeductApplied: []string{"ICMS"}, AddsToTotal: true},
	}}
	item := TaxItem{ProductID: "p", Quantity: "2", UnitPrice: "50", Discount: "0", Freight: "0", Insurance: "0", Other: "0"}
	out, err := Calculate(c, item, []TaxRule{r})
	if err != nil || out.Taxes[0].Amount != "18.00" || out.Taxes[1].Base != "140.00" || out.Taxes[1].Amount != "7.20" || out.Total != "107.20" {
		t.Fatal(out, err)
	}
	if _, err = Calculate(c, item, nil); !errors.Is(err, ErrRule) {
		t.Fatal("unknown-as-zero", err)
	}
	if _, err = Calculate(c, item, []TaxRule{r, r}); !errors.Is(err, ErrRule) {
		t.Fatal("ambiguous policy", err)
	}
	c.Date = "2027-01-01"
	if _, err = Calculate(c, item, []TaxRule{r}); !errors.Is(err, ErrRule) {
		t.Fatal("expired policy", err)
	}
	c.Date = "2026-10-26"
	r.Components[1].DeductApplied = []string{"MISSING"}
	if _, err = Calculate(c, item, []TaxRule{r}); !errors.Is(err, ErrRule) {
		t.Fatal("unknown dependency", err)
	}
}
