package fiscalcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	stdhtml "html"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/boombuler/barcode/code128"
)

func printInvoice55() Invoice {
	in := testInvoice()
	in.Key.Model = "55"
	in.Recipient = &Party{CNPJ: "11222333000181", Name: "Synthetic recipient", IEDestination: "9", Address: in.Issuer.Address}
	return in
}

func TestFiscalDANFE55ExactArchiveAndBarcode(t *testing.T) {
	in := printInvoice55()
	in.Lines[0].Description = "Synthetic <script>alert(1)</script> & item"
	process := printProcess(t, in, nil)
	opts := PrintOptions{WidthMM: 210, Timezone: "America/Sao_Paulo"}
	out, err := RenderHomologationNFe55(process, opts)
	if err != nil {
		t.Fatal(err)
	}
	source, html := sha256.Sum256(process), sha256.Sum256(out.HTML)
	if out.Environment != "2" || out.OfficialHomologation || out.WidthMM != 210 || out.SourceSHA256 != hex.EncodeToString(source[:]) || out.HTMLSHA256 != hex.EncodeToString(html[:]) {
		t.Fatal("provenance")
	}
	compact := bytes.ReplaceAll(out.HTML, []byte("\n"), nil)
	for _, text := range []string{"SEM VALOR FISCAL", "AMBIENTE DE HOMOLOGAÇÃO", "CODE-128C", "size:A4 portrait", "12.ABC.345/01DE-35", "11.222.333/0001-81", "EMITIDA EM AMBIENTE DE HOMOLOGACAO", "CÁLCULO DO IMPOSTO", "TRANSPORTADOR", "09012100", "5102", "3,33333", "6,67", "0102", "&lt;script&gt;", "135260000000001", "26/10/2026 12:01:00 -0300", "Folha 1/"} {
		if !bytes.Contains(compact, []byte(text)) {
			t.Errorf("missing %q", text)
		}
	}
	for _, unsafe := range []string{"<script>", "ZgotmplZ", "<iframe", "NaN", "width:0%"} {
		if bytes.Contains(out.HTML, []byte(unsafe)) {
			t.Errorf("unsafe %q", unsafe)
		}
	}
	again, err := RenderHomologationNFe55(process, opts)
	if err != nil || !bytes.Equal(again.HTML, out.HTML) {
		t.Fatal("non-deterministic reprint", err)
	}
	key, _ := AccessKey(in.Key)
	raster, err := AccessKeyBarcodePNG(key)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raster))
	if err != nil || img.Bounds().Dx() < 891 || img.Bounds().Dx() > 1155 || img.Bounds().Dy() != 100 {
		t.Fatal("barcode dimensions", err)
	}
	for y := 0; y < 100; y++ {
		for x := 0; x < 30; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r != 65535 {
				t.Fatal("left quiet zone")
			}
			r, _, _, _ = img.At(img.Bounds().Dx()-1-x, y).RGBA()
			if r != 65535 {
				t.Fatal("right quiet zone")
			}
		}
	}
	if !strings.Contains(stdhtml.UnescapeString(string(out.HTML)), base64.StdEncoding.EncodeToString(raster)) {
		t.Fatal("wrong access key encoded")
	}
	if dir := os.Getenv("TITAN_DANFE_PREVIEW_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			t.Fatal("absolute preview path required")
		}
		if err = os.WriteFile(filepath.Join(dir, "nfe55-a4.html"), out.HTML, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFiscalDANFE55RejectsTamperedAndUnsupportedDocuments(t *testing.T) {
	process := printProcess(t, printInvoice55(), nil)
	for name, mutate := range map[string]func(*etree.Document){
		"digest": func(d *etree.Document) {
			d.Root().FindElement("protNFe/infProt/digVal").SetText("AAAAAAAAAAAAAAAAAAAAAAAAAAA=")
		},
		"key": func(d *etree.Document) {
			d.Root().FindElement("protNFe/infProt/chNFe").SetText(strings.Repeat("1", 44))
		},
		"environment": func(d *etree.Document) { d.Root().FindElement("protNFe/infProt/tpAmb").SetText("1") },
		"rejection":   func(d *etree.Document) { d.Root().FindElement("protNFe/infProt/cStat").SetText("302") },
		"amount":      func(d *etree.Document) { d.Root().FindElement("NFe/infNFe/total/ICMSTot/vNF").SetText("999.00") },
		"signature":   func(d *etree.Document) { d.Root().FindElement("NFe/Signature/SignatureValue").SetText("AAAA") },
	} {
		t.Run(name, func(t *testing.T) {
			d := etree.NewDocument()
			if err := d.ReadFromBytes(process); err != nil {
				t.Fatal(err)
			}
			mutate(d)
			raw, _ := d.WriteToBytes()
			if _, err := RenderHomologationNFe55(raw, PrintOptions{WidthMM: 210, Timezone: "America/Sao_Paulo"}); err == nil {
				t.Fatal("altered process printed")
			}
		})
	}
	for _, opts := range []PrintOptions{{WidthMM: 80, Timezone: "America/Sao_Paulo"}, {WidthMM: 210, Timezone: "UTC"}, {WidthMM: 210}, {WidthMM: 58, Timezone: "America/Sao_Paulo"}} {
		if _, err := RenderHomologationNFe55(process, opts); err == nil {
			t.Fatal("unsupported options")
		}
	}
	if _, err := RenderHomologationNFe55(printProcess(t, testInvoice(), nil), PrintOptions{WidthMM: 210, Timezone: "America/Sao_Paulo"}); err == nil {
		t.Fatal("NFC-e silently rendered on A4")
	}
	if _, err := RenderHomologationNFCe(process, PrintOptions{WidthMM: 210, Timezone: "America/Sao_Paulo"}); err == nil {
		t.Fatal("A4 silently rendered as NFC-e")
	}
	in := printInvoice55()
	in.Environment = 1
	if _, err := RenderHomologationNFe55(printProcess(t, in, nil), PrintOptions{WidthMM: 210, Timezone: "America/Sao_Paulo"}); err == nil {
		t.Fatal("production enabled")
	}
	if _, err := AccessKeyBarcodePNG(strings.Repeat("0", 44)); err == nil {
		t.Fatal("invalid barcode key")
	}
}

func TestFiscalDANFE55AllItemsAndLongAdditionalInformationPaginate(t *testing.T) {
	in := printInvoice55()
	in.Lines = nil
	for i := 0; i < 100; i++ {
		line := testInvoice().Lines[0]
		line.ProductID = fmt.Sprintf("SKU%03d", i)
		line.Description = fmt.Sprintf("Item%03d Synthetic product descrição longa sem corte", i)
		in.Lines = append(in.Lines, line)
	}
	in.Payments = []InvoicePayment{{Method: "01", Amount: "667.00"}}
	process := printProcess(t, in, nil)
	out, err := RenderHomologationNFe55(process, PrintOptions{WidthMM: 210, Timezone: "America/Sao_Paulo"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if !bytes.Contains(out.HTML, []byte(fmt.Sprintf("SKU%03d", i))) {
			t.Fatalf("lost item %d", i)
		}
	}
	if bytes.Count(out.HTML, []byte(`<article class="page">`)) < 3 {
		t.Fatal("not paginated")
	}
	if dir := os.Getenv("TITAN_DANFE_PREVIEW_DIR"); dir != "" {
		if err = os.WriteFile(filepath.Join(dir, "nfe55-a4-long.html"), out.HTML, 0600); err != nil {
			t.Fatal(err)
		}
	}
	text := strings.Repeat("CONTEUDO漢字", 500)
	wrapped, lines := wrapPrintText(text, 40)
	if strings.ReplaceAll(wrapped, "\n", "") != text || lines < 100 {
		t.Fatal("lost wide/long text")
	}
	// Additional fields must be signed, not appended after signature verification.
	c := testCredential(t)
	raw, _, err := BuildInvoice(printInvoice55())
	if err != nil {
		t.Fatal(err)
	}
	d, _ := parseXML(raw)
	additional := d.Root().SelectElement("infNFe").CreateElement("infAdic")
	field(additional, "infCpl", strings.Repeat("CONTINUACAO ", 350)+"FINAL_ADICIONAL")
	raw, _ = d.WriteToBytes()
	signed, err := c.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	base, _ := parseXML(process)
	sd, _ := parseXML(signed)
	proto := base.Root().SelectElement("protNFe").Copy()
	key := strings.TrimPrefix(sd.Root().SelectElement("infNFe").SelectAttrValue("Id", ""), "NFe")
	proto.FindElement("infProt/chNFe").SetText(key)
	proto.FindElement("infProt/digVal").SetText(sd.Root().FindElement("Signature/SignedInfo/Reference/DigestValue").Text())
	complete, err := assembleNFeProcess(signed, proto)
	if err != nil {
		t.Fatal(err)
	}
	printed, err := RenderHomologationNFe55(complete, PrintOptions{WidthMM: 210, Timezone: "America/Sao_Paulo"})
	if err != nil {
		t.Fatal(err)
	}
	compact := bytes.ReplaceAll(printed.HTML, []byte("\n"), nil)
	if !bytes.Contains(compact, []byte("FINAL_ADICIONAL")) || bytes.Count(compact, []byte("CONTINUACAO")) != 350 {
		t.Fatal("additional text truncated")
	}
}

func TestFiscalDANFE55BarcodeNumericReferenceAndAlphanumericRoundTrip(t *testing.T) {
	for _, document := range []string{"11222333000181", "12ABC34501DE35", "ABCDEFGHIJKL00", "A1B2C3D4E5G600"} {
		t.Run(document, func(t *testing.T) {
			in := printInvoice55().Key
			in.CNPJ = document
			key, err := AccessKey(in)
			if err != nil {
				t.Fatal(err)
			}
			symbols, err := accessBarcodeSymbols(key)
			if err != nil {
				t.Fatal(err)
			}
			if symbols[0] != 105 || symbols[len(symbols)-1] != 106 {
				t.Fatal("start/stop")
			}
			sum := symbols[0]
			var decoded strings.Builder
			setC := true
			for i, v := range symbols[1 : len(symbols)-2] {
				sum += (i + 1) * v
				if setC {
					if v == 101 {
						setC = false
					} else if v < 100 {
						fmt.Fprintf(&decoded, "%02d", v)
					} else {
						t.Fatal("unexpected C symbol", v)
					}
				} else {
					if v == 99 {
						setC = true
					} else if v >= 16 && v <= 58 {
						decoded.WriteByte(byte(v + 32))
					} else {
						t.Fatal("unexpected A symbol", v)
					}
				}
			}
			if decoded.String() != key || sum%103 != symbols[len(symbols)-2] {
				t.Fatal("wrong key or checksum")
			}
			raw, err := AccessKeyBarcodePNG(key)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			if float64(img.Bounds().Dx()/3)*0.2 > 115 {
				t.Fatal("narrow bar below 0.2mm at declared width")
			}
			if digitsRE.MatchString(key) {
				reference, err := code128.Encode(key)
				if err != nil {
					t.Fatal(err)
				}
				if img.Bounds().Dx() != (reference.Bounds().Dx()+20)*3 {
					t.Fatal("numeric reference length")
				}
				for x := 0; x < reference.Bounds().Dx(); x++ {
					expected, _, _, _ := reference.At(x, 0).RGBA()
					actual, _, _, _ := img.At(30+x*3, 50).RGBA()
					if expected != actual {
						t.Fatalf("reference disagrees at module %d", x)
					}
				}
			}
		})
	}
}
