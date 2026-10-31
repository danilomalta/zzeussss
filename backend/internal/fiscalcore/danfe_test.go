package fiscalcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
)

func printProcess(t *testing.T, in Invoice, mutate func(*etree.Document)) []byte {
	t.Helper()
	c := testCredential(t)
	raw, key, err := BuildInvoice(in)
	if err != nil {
		t.Fatal("build", err)
	}
	signed, err := c.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if in.Key.Model == "65" && in.Key.Emission == 1 {
		signed, err = c.CompleteNFCeXML(QRConfiguration{BaseURL: "https://homologacao.example.gov.br/qrcode", Version: 3}, "https://homologacao.example.gov.br/consulta", signed, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}
	d, _ := parseXML(signed)
	if mutate != nil {
		mutate(d)
	}
	signed, _ = d.WriteToBytes()
	digest := d.Root().FindElement("Signature/SignedInfo/Reference/DigestValue").Text()
	proto := etree.NewDocument()
	if err = proto.ReadFromString(fmt.Sprintf(`<protNFe xmlns="%s" versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><chNFe>%s</chNFe><dhRecbto>2026-10-26T15:01:00+00:00</dhRecbto><nProt>135260000000001</nProt><digVal>%s</digVal><cStat>100</cStat><xMotivo>Synthetic only</xMotivo></infProt></protNFe>`, NFeNS, key, digest)); err != nil {
		t.Fatal(err)
	}
	process, err := assembleNFeProcess(signed, proto.Root())
	if err != nil {
		t.Fatal("process", err)
	}
	return process
}

func TestFiscalDANFEUnsupportedModesNeverClaimOfficialPrint(t *testing.T) {
	for _, mode := range []string{"production", "model55"} {
		t.Run(mode, func(t *testing.T) {
			in := testInvoice()
			if mode == "production" {
				in.Environment = 1
			} else {
				in.Key.Model = "55"
				in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic buyer", IEDestination: "9", Address: in.Issuer.Address}
			}
			process := printProcess(t, in, nil)
			if _, err := RenderHomologationNFCe(process, PrintOptions{WidthMM: 80, Timezone: "America/Sao_Paulo"}); err == nil {
				t.Fatal("unsupported document printed", mode)
			}
		})
	}
	if ValidatePrintOptions(PrintOptions{WidthMM: 80, Timezone: "America/New_York"}) == nil {
		t.Fatal("non-Brazilian print timezone")
	}
}

func TestFiscalDANFEExactSourceLayoutAndEscapedData(t *testing.T) {
	in := testInvoice()
	in.Lines[0].Description = `Synthetic <script>alert('x')</script> & text`
	in.Payments = []InvoicePayment{{Method: "01", Amount: "10.00"}}
	process := printProcess(t, in, nil)
	for _, width := range []int{58, 80} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			out, err := RenderHomologationNFCe(process, PrintOptions{WidthMM: width, Timezone: "America/Sao_Paulo"})
			if err != nil {
				t.Fatal(err)
			}
			source, html := sha256.Sum256(process), sha256.Sum256(out.HTML)
			if out.SourceSHA256 != hex.EncodeToString(source[:]) || out.HTMLSHA256 != hex.EncodeToString(html[:]) || out.OfficialHomologation || out.Environment != "2" {
				t.Fatal("artifact provenance")
			}
			for _, expected := range []string{"SEM VALOR FISCAL", "6,67", "10,00", "3,33", "3,33333", "26/10/2026 12:01:00 -0300", "135260000000001", "data:image/png;base64,", "CONSUMIDOR NÃO IDENTIFICADO", "&lt;script&gt;", "12.ABC.345/01DE-35", "Rua Teste, 1, Centro, Sao Paulo, SP, 01001000"} {
				if !bytes.Contains(out.HTML, []byte(expected)) {
					t.Errorf("missing %q", expected)
				}
			}
			for _, forbidden := range []string{"<script>", "ZgotmplZ", "<iframe", "<link", "fetch("} {
				if bytes.Contains(out.HTML, []byte(forbidden)) {
					t.Errorf("unsafe %q", forbidden)
				}
			}
			again, err := RenderHomologationNFCe(process, PrintOptions{WidthMM: width, Timezone: "America/Sao_Paulo"})
			if err != nil || !bytes.Equal(out.HTML, again.HTML) {
				t.Fatal("nondeterministic reprint", err)
			}
			if dir := os.Getenv("TITAN_DANFE_PREVIEW_DIR"); dir != "" {
				if !filepath.IsAbs(dir) {
					t.Fatal("preview directory must be absolute")
				}
				if err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("nfce-%d.html", width)), out.HTML, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestFiscalDANFERejectsAlteredProtocolAndSignature(t *testing.T) {
	process := printProcess(t, testInvoice(), nil)
	mutations := map[string]func(*etree.Document){
		"digest": func(d *etree.Document) {
			d.Root().FindElement("protNFe/infProt/digVal").SetText("AAAAAAAAAAAAAAAAAAAAAAAAAAA=")
		},
		"rejection":   func(d *etree.Document) { d.Root().FindElement("protNFe/infProt/cStat").SetText("302") },
		"environment": func(d *etree.Document) { d.Root().FindElement("protNFe/infProt/tpAmb").SetText("1") },
		"amount":      func(d *etree.Document) { d.Root().FindElement("NFe/infNFe/total/ICMSTot/vNF").SetText("999.00") },
		"signature":   func(d *etree.Document) { d.Root().FindElement("NFe/Signature/SignatureValue").SetText("AAAA") },
		"qr_host": func(d *etree.Document) {
			d.Root().FindElement("NFe/infNFeSupl/qrCode").SetText("https://evil.example/qr?p=anything")
		},
		"qr_environment": func(d *etree.Document) {
			e := d.Root().FindElement("NFe/infNFeSupl/qrCode")
			e.SetText(strings.ReplaceAll(e.Text(), "|2", "|1"))
		},
		"qr_missing": func(d *etree.Document) {
			n := d.Root().SelectElement("NFe")
			n.RemoveChild(n.SelectElement("infNFeSupl"))
		},
		"consult_host": func(d *etree.Document) {
			d.Root().FindElement("NFe/infNFeSupl/urlChave").SetText("https://evil.example/")
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			d := etree.NewDocument()
			if err := d.ReadFromBytes(process); err != nil {
				t.Fatal(err)
			}
			mutate(d)
			raw, _ := d.WriteToBytes()
			if _, err := RenderHomologationNFCe(raw, PrintOptions{WidthMM: 80, Timezone: "America/Sao_Paulo"}); err == nil {
				t.Fatal("accepted altered process")
			}
		})
	}
	for _, opts := range []PrintOptions{{WidthMM: 57, Timezone: "America/Sao_Paulo"}, {WidthMM: 80}, {WidthMM: 80, Timezone: "UTC"}, {WidthMM: 80, Timezone: "America/Missing"}} {
		if _, err := RenderHomologationNFCe(process, opts); err == nil {
			t.Fatal("accepted invalid print options")
		}
	}
}

func TestFiscalDANFEQRBindingAndFormatting(t *testing.T) {
	key, _ := AccessKey(testInvoice().Key)
	for _, suffix := range []string{key + "|3|2", key + "|2|2|1|" + strings.Repeat("A", 40)} {
		if !validPrintQR("https://homologacao.example.gov.br/q?p="+url.QueryEscape(suffix), key) {
			t.Fatal("valid archived QR")
		}
	}
	for _, raw := range []string{"javascript:alert(1)", "https://gov.br.evil.example/q?p=" + key + "%7C3%7C2", "https://a.gov.br:443/q?p=" + key + "%7C3%7C2", "https://a.gov.br/q?p=" + key + "%7C3%7C2&p=another", "https://a.gov.br/q?p=" + key + "%7C3%7C1", "https://a.gov.br/q?p=" + key + "%7C3%7C2&x=1"} {
		if validPrintQR(raw, key) {
			t.Fatal("unsafe QR", raw)
		}
	}
	if printDecimal("1234567.8900") != "1.234.567,8900" || formatDocument("12345678901") != "123.456.789-01" {
		t.Fatal("decimal/document formatting")
	}
}

func TestFiscalDANFECompleteSupplementBindsOriginalSignature(t *testing.T) {
	c := testCredential(t)
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			raw, _, err := BuildInvoice(testInvoice())
			if err != nil {
				t.Fatal(err)
			}
			signed, err := c.SignXML("nfe", raw, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			original, _ := parseXML(signed)
			digest := original.Root().FindElement("Signature/SignedInfo/Reference/DigestValue").Text()
			config := QRConfiguration{Version: version, BaseURL: "https://homologacao.example.gov.br/qrcode", CSCID: "1", CSC: "synthetic-csc-for-tests-only"}
			complete, err := c.CompleteNFCeXML(config, "https://homologacao.example.gov.br/consulta", signed, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			d, _ := parseXML(complete)
			if digest != d.Root().FindElement("Signature/SignedInfo/Reference/DigestValue").Text() {
				t.Fatal("supplement changed signature digest")
			}
			if strings.Contains(d.Root().FindElement("infNFeSupl/qrCode").Text(), "%7C") {
				t.Fatal("XML QR pipe separators escaped")
			}
			if _, err = c.CompleteNFCeXML(config, "https://homologacao.example.gov.br/consulta", complete, time.Now()); err == nil {
				t.Fatal("replaced archived supplement")
			}
		})
	}
}

func TestFiscalDANFELargeReceiptPreservesAllItems(t *testing.T) {
	in := testInvoice()
	sample := in.Lines[0]
	in.Lines = nil
	for i := 0; i < 100; i++ {
		line := sample
		line.ProductID = fmt.Sprintf("item-%03d", i)
		line.Description = fmt.Sprintf("Synthetic item %03d - descricao longa para conferir quebra de linha e paginacao sem perder dados", i)
		in.Lines = append(in.Lines, line)
	}
	in.Payments = []InvoicePayment{{Method: "01", Amount: "667.00"}}
	process := printProcess(t, in, nil)
	out, err := RenderHomologationNFCe(process, PrintOptions{WidthMM: 58, Timezone: "America/Rio_Branco"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range in.Lines {
		if !bytes.Contains(out.HTML, []byte(item.ProductID+" - ")) {
			t.Fatal("missing item", item.ProductID)
		}
	}
	if !bytes.Contains(out.HTML, []byte("667,00")) || !bytes.Contains(out.HTML, []byte("Não informado no XML")) {
		t.Fatal("wrong totals or fabricated change")
	}
	if dir := os.Getenv("TITAN_DANFE_PREVIEW_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			t.Fatal("preview directory must be absolute")
		}
		if err = os.WriteFile(filepath.Join(dir, "nfce-large.html"), out.HTML, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
