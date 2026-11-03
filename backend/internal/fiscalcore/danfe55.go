package fiscalcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/mattn/go-runewidth"
)

// printableHomologationProcess checks the signed issuer payload and its exact
// authorization-indicating protocol. Official transport provenance still comes
// from the authenticated journal; arbitrary caller-uploaded XML is not trusted.
func printableHomologationProcess(process []byte) (*etree.Element, Protocol, OriginalNFe, error) {
	var protocol Protocol
	var original OriginalNFe
	if err := ValidateXML("nfe_process", process); err != nil {
		return nil, protocol, original, err
	}
	d, err := parseXMLLimits(process, 2*MaxXML, 8200)
	if err != nil {
		return nil, protocol, original, err
	}
	root := d.Root()
	if root.Tag != "nfeProc" || root.NamespaceURI() != NFeNS || len(root.ChildElements()) != 2 {
		return nil, protocol, original, ErrUncertain
	}
	nfe, proto := root.SelectElement("NFe"), root.SelectElement("protNFe")
	if nfe == nil || proto == nil {
		return nil, protocol, original, ErrUncertain
	}
	signed := etree.NewDocument()
	signed.SetRoot(nfe.Copy())
	raw, err := signed.WriteToBytes()
	if err != nil {
		return nil, protocol, original, ErrXML
	}
	original, _, err = inspectArchivedNFe(raw)
	if err != nil {
		return nil, protocol, original, err
	}
	auth := etree.NewDocument()
	response := auth.CreateElement("retEnviNFe")
	response.CreateAttr("xmlns", NFeNS)
	response.CreateAttr("versao", "4.00")
	field(response, "tpAmb", "2")
	field(response, "cStat", "104")
	response.AddChild(proto.Copy())
	raw, err = auth.WriteToBytes()
	if err != nil {
		return nil, protocol, original, ErrXML
	}
	protocol, err = ParseProtocol(raw, original.Key, "2", original.Digest)
	return nfe, protocol, original, err
}

// Code128 symbol widths are the standard symbol table (MOC Anexo II). Start C
// and switching C/A preserve numeric keys and alphanumeric CNPJ (NTC2025.001).
var accessBarcodeWidths = strings.Fields("212222 222122 222221 121223 121322 131222 122213 122312 132212 221213 221312 231212 112232 122132 122231 113222 123122 123221 223211 221132 221231 213212 223112 312131 311222 321122 321221 312212 322112 322211 212123 212321 232121 111323 131123 131321 112313 132113 132311 211313 231113 231311 112133 112331 132131 113123 113321 133121 313121 211331 231131 213113 213311 213131 311123 311321 331121 312113 312311 332111 314111 221411 431111 111224 111422 121124 121421 141122 141221 112214 112412 122114 122411 142112 142211 241211 221114 413111 241112 134111 111242 121142 121241 114212 124112 124211 411212 421112 421211 212141 214121 412121 111143 111341 131141 114113 114311 411113 411311 113141 114131 311141 411131 211412 211214 211232 2331112")

func accessBarcodeSymbols(key string) ([]int, error) {
	if !ValidAccessKey(key) || key[20:22] != "55" {
		return nil, ErrInput
	}
	values, setC := []int{105}, true
	for i := 0; i < len(key); {
		digits := 0
		for j := i; j < len(key) && key[j] >= '0' && key[j] <= '9'; j++ {
			digits++
		}
		if setC {
			if digits >= 2 {
				values = append(values, int(key[i]-'0')*10+int(key[i+1]-'0'))
				i += 2
				continue
			}
			values = append(values, 101)
			setC = false
		} else if digits >= 4 {
			values = append(values, 99)
			setC = true
			continue
		}
		values = append(values, int(key[i])-32)
		i++
	}
	checksum := values[0]
	for i := 1; i < len(values); i++ {
		checksum += i * values[i]
	}
	return append(values, checksum%103, 106), nil
}

// AccessKeyBarcodePNG uses C for numeric pairs and A for uppercase characters,
// with 10-module quiet zones and integer scaling. Even the widest supported
// key at 115x10mm retains a narrow module above 0.2mm. No interpolated bars.
func AccessKeyBarcodePNG(key string) ([]byte, error) {
	values, err := accessBarcodeSymbols(key)
	if err != nil {
		return nil, err
	}
	modules := 0
	for _, v := range values {
		for _, w := range accessBarcodeWidths[v] {
			modules += int(w - '0')
		}
	}
	out := image.NewGray(image.Rect(0, 0, (modules+20)*3, 100))
	draw.Draw(out, out.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	x := 30
	for _, v := range values {
		for i, w := range accessBarcodeWidths[v] {
			width := int(w-'0') * 3
			if i%2 == 0 {
				draw.Draw(out, image.Rect(x, 0, x+width, 100), image.NewUniform(color.Black), image.Point{}, draw.Src)
			}
			x += width
		}
	}
	var b bytes.Buffer
	if err = png.Encode(&b, out); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type a4Cell struct {
	Width int
	Text  string
}
type a4Row struct {
	Cells []a4Cell
	Units int
	Title bool
}
type a4Page struct {
	Rows  []a4Row
	Index int
}
type nfePrint struct {
	Issuer, IssuerAddress, CNPJ, IE, Key, Number, Series, Direction string
	Protocol, AuthorizedAt, Barcode                                 string
	Pages                                                           []a4Page
}

// wrapPrintText preserves every non-control rune, including long unbroken
// identifiers. Newlines are presentation, never HTML; template escaping remains
// mandatory. Wide CJK characters consume two cells in the print grid.
func wrapPrintText(text string, width int) (string, int) {
	var b strings.Builder
	column, lines := 0, 1
	for _, r := range text {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			b.WriteRune(r)
			column, lines = 0, lines+1
			continue
		}
		w := runewidth.RuneWidth(r)
		if column+w > width {
			b.WriteByte('\n')
			column, lines = 0, lines+1
		}
		b.WriteRune(r)
		column += w
	}
	return b.String(), lines
}

func a4Cells(title bool, widths []int, values ...string) a4Row {
	row := a4Row{Title: title}
	for i, text := range values {
		// 196mm content / 1000 units, at 7pt mono and 1mm cell padding.
		capacity := (widths[i]*196/1000 - 2) * 100 / 150
		if capacity < 1 {
			capacity = 1
		}
		wrapped, lines := wrapPrintText(text, capacity)
		if lines > row.Units {
			row.Units = lines
		}
		row.Cells = append(row.Cells, a4Cell{Width: widths[i], Text: wrapped})
	}
	row.Units++ // border and cell padding
	return row
}

// RenderHomologationNFe55 provides an A4 technical archive copy of normal,
// portrait model55. It neither enables production nor claims official approval
// of the layout, printer or protocol. Width210 is explicit; NFC-e widths fail.
func RenderHomologationNFe55(process []byte, opts PrintOptions) (DANFEArtifact, error) {
	var out DANFEArtifact
	if opts.WidthMM != 210 {
		return out, ErrInput
	}
	zone, err := printLocation(opts)
	if err != nil {
		return out, err
	}
	nfe, protocol, original, err := printableHomologationProcess(process)
	if err != nil {
		return out, err
	}
	if original.Key[20:22] != "55" || original.Key[34:35] != "1" {
		return out, ErrUnsupported
	}
	info := nfe.SelectElement("infNFe")
	get := func(path string) string { return printXMLText(info, path) }
	if get("ide/tpImp") != "1" {
		return out, ErrUnsupported
	}
	barcode, err := AccessKeyBarcodePNG(original.Key)
	if err != nil {
		return out, err
	}
	received, err := time.Parse(time.RFC3339, protocol.ReceivedAt)
	if err != nil {
		return out, ErrInput
	}
	view := nfePrint{Issuer: get("emit/xNome"), IssuerAddress: printAddress(info.FindElement("emit/enderEmit")), CNPJ: formatDocument(get("emit/CNPJ")), IE: get("emit/IE"), Key: groupKey(original.Key), Number: get("ide/nNF"), Series: get("ide/serie"), Direction: get("ide/tpNF"), Protocol: protocol.Number, AuthorizedAt: received.In(zone).Format("02/01/2006 15:04:05 -0700"), Barcode: base64.StdEncoding.EncodeToString(barcode)}
	// Fixed pages and explicit line budgets avoid silent truncation and browser
	// page counters. A single long field is split over continuation rows/pages.
	page := a4Page{Index: 1}
	used := 0
	const budget = 60 // 180mm maximum rows; header/footer reserve 90mm.
	var repeat *a4Row
	nextPage := func() {
		view.Pages = append(view.Pages, page)
		page, used = a4Page{Index: len(view.Pages) + 1}, 0
		if repeat != nil {
			page.Rows = append(page.Rows, *repeat)
			used = repeat.Units
		}
	}
	add := func(row a4Row) {
		remaining := row
		for {
			if used == budget {
				nextPage()
			}
			if remaining.Units <= budget-used {
				page.Rows = append(page.Rows, remaining)
				used += remaining.Units
				return
			}
			// Preserve complete rows where they fit on a fresh page.
			capacity := budget
			if repeat != nil {
				capacity -= repeat.Units
			}
			if remaining.Units <= capacity || budget-used < 3 {
				nextPage()
				continue
			}
			count := budget - used - 1
			part := a4Row{Title: remaining.Title, Units: count + 1}
			next := a4Row{Title: remaining.Title}
			for _, cell := range remaining.Cells {
				lines := strings.Split(cell.Text, "\n")
				cut := min(count, len(lines))
				part.Cells = append(part.Cells, a4Cell{cell.Width, strings.Join(lines[:cut], "\n")})
				next.Cells = append(next.Cells, a4Cell{cell.Width, strings.Join(lines[cut:], "\n")})
				next.Units = max(next.Units, len(lines)-cut+1)
			}
			page.Rows = append(page.Rows, part)
			used, remaining = budget, next
		}
	}
	full := func(label, value string) { add(a4Cells(false, []int{1000}, label+": "+value)) }
	section := func(label string) { add(a4Cells(true, []int{1000}, label)) }
	section("IDENTIFICAÇÃO DA OPERAÇÃO")
	full("Natureza da operação", get("ide/natOp"))
	full("Inscrição estadual / IE substituto / CNPJ", get("emit/IE")+" / "+get("emit/IEST")+" / "+formatDocument(get("emit/CNPJ")))
	section("DESTINATÁRIO / REMETENTE")
	full("Nome / razão social", get("dest/xNome"))
	full("CNPJ / CPF / identificação estrangeira", formatDocument(get("dest/CNPJ")+get("dest/CPF"))+get("dest/idEstrangeiro"))
	full("Endereço", printAddress(info.FindElement("dest/enderDest")))
	full("IE / fone", get("dest/IE")+" / "+get("dest/enderDest/fone"))
	for _, path := range []string{"dhEmi", "dhSaiEnt"} {
		if raw := get("ide/" + path); raw != "" {
			t, e := time.Parse(time.RFC3339, raw)
			if e != nil {
				return out, ErrInput
			}
			full(path+" (data/hora)", t.In(zone).Format("02/01/2006 15:04:05 -0700"))
		}
	}
	for _, path := range []string{"retirada", "entrega"} {
		if e := info.SelectElement(path); e != nil {
			full("Local de "+path, printXMLText(e, "xNome")+" / "+formatDocument(printXMLText(e, "CNPJ")+printXMLText(e, "CPF"))+" / "+printAddress(e)+" / IE "+printXMLText(e, "IE"))
		}
	}
	if billing := info.SelectElement("cobr"); billing != nil {
		section("FATURA / DUPLICATAS")
		full("Fatura / original / desconto / líquido", get("cobr/fat/nFat")+" / "+printDecimal(get("cobr/fat/vOrig"))+" / "+printDecimal(get("cobr/fat/vDesc"))+" / "+printDecimal(get("cobr/fat/vLiq")))
		for _, e := range billing.SelectElements("dup") {
			full("Duplicata / vencimento / valor", printXMLText(e, "nDup")+" / "+printXMLText(e, "dVenc")+" / "+printDecimal(printXMLText(e, "vDup")))
		}
	}
	section("CÁLCULO DO IMPOSTO — VALORES DO XML, SEM RECÁLCULO")
	labels := map[string]string{"vBC": "Base ICMS", "vICMS": "ICMS", "vBCST": "Base ICMS ST", "vST": "ICMS ST", "vProd": "Produtos", "vFrete": "Frete", "vSeg": "Seguro", "vDesc": "Desconto", "vOutro": "Outras despesas", "vIPI": "IPI", "vNF": "Total da nota", "vTotTrib": "Tributos aproximados"}
	var cells []string
	flush := func() {
		if len(cells) > 0 {
			widths := make([]int, len(cells))
			for i := range widths {
				widths[i] = 1000 / len(cells)
			}
			widths[len(widths)-1] += 1000 % len(cells)
			add(a4Cells(false, widths, cells...))
			cells = nil
		}
	}
	for _, e := range info.FindElement("total/ICMSTot").ChildElements() {
		label := labels[e.Tag]
		if label == "" {
			label = e.Tag
		}
		cells = append(cells, label+" R$\n"+printDecimal(e.Text()))
		if len(cells) == 4 {
			flush()
		}
	}
	flush()
	section("TRANSPORTADOR / VOLUMES TRANSPORTADOS")
	full("Frete por conta (código XML)", get("transp/modFrete"))
	if e := info.FindElement("transp/transporta"); e != nil {
		full("Transportador", printXMLText(e, "xNome")+" / "+formatDocument(printXMLText(e, "CNPJ")+printXMLText(e, "CPF"))+" / IE "+printXMLText(e, "IE"))
		full("Endereço / município / UF", printXMLText(e, "xEnder")+" / "+printXMLText(e, "xMun")+" / "+printXMLText(e, "UF"))
	}
	if e := info.FindElement("transp/veicTransp"); e != nil {
		full("Placa / UF / RNTC", printXMLText(e, "placa")+" / "+printXMLText(e, "UF")+" / "+printXMLText(e, "RNTC"))
	}
	for _, e := range info.FindElements("transp/vol") {
		full("Volumes / espécie / marca / numeração / peso bruto / líquido", printXMLText(e, "qVol")+" / "+printXMLText(e, "esp")+" / "+printXMLText(e, "marca")+" / "+printXMLText(e, "nVol")+" / "+printDecimal(printXMLText(e, "pesoB"))+" / "+printDecimal(printXMLText(e, "pesoL")))
	}
	section("DADOS DOS PRODUTOS / SERVIÇOS")
	widths := []int{65, 200, 65, 50, 50, 40, 70, 80, 70, 70, 60, 60, 60, 60}
	headings := []string{"Código", "Descrição", "NCM", "CST/CSOSN", "CFOP", "UN", "Qtd", "V.unit", "V.total", "BC ICMS", "V.ICMS", "V.IPI", "%ICMS", "%IPI"}
	header := a4Cells(true, widths, headings...)
	add(header)
	repeat = &header
	for _, det := range info.SelectElements("det") {
		p := det.SelectElement("prod")
		icms := det.FindElement("imposto/ICMS")
		var group *etree.Element
		if icms != nil && len(icms.ChildElements()) == 1 {
			group = icms.ChildElements()[0]
		}
		status := printXMLText(group, "orig") + printXMLText(group, "CST") + printXMLText(group, "CSOSN")
		ipi := det.FindElement("imposto/IPI/IPITrib")
		add(a4Cells(false, widths, printXMLText(p, "cProd"), printXMLText(p, "xProd"), printXMLText(p, "NCM"), status, printXMLText(p, "CFOP"), printXMLText(p, "uCom"), printDecimal(printXMLText(p, "qCom")), printDecimal(printXMLText(p, "vUnCom")), printDecimal(printXMLText(p, "vProd")), printDecimal(printXMLText(group, "vBC")), printDecimal(printXMLText(group, "vICMS")), printDecimal(printXMLText(ipi, "vIPI")), printDecimal(printXMLText(group, "pICMS")), printDecimal(printXMLText(ipi, "pIPI"))))
		if get := printXMLText(det, "infAdProd"); get != "" {
			full("Informações do produto "+printXMLText(p, "cProd"), get)
		}
		if printXMLText(p, "uTrib") != printXMLText(p, "uCom") || printXMLText(p, "qTrib") != printXMLText(p, "qCom") || printXMLText(p, "vUnTrib") != printXMLText(p, "vUnCom") {
			full("Unidade / quantidade / valor unitário tributáveis", printXMLText(p, "uTrib")+" / "+printDecimal(printXMLText(p, "qTrib"))+" / "+printDecimal(printXMLText(p, "vUnTrib")))
		}
	}
	repeat = nil
	section("DADOS ADICIONAIS / INFORMAÇÕES COMPLEMENTARES")
	full("Informações complementares", get("infAdic/infCpl"))
	full("Reservado ao fisco", get("infAdic/infAdFisco"))
	// Preserve additional total groups (e.g. ISSQN / RTC) as named source data;
	// they are not interpreted as approved calculations by the renderer.
	for _, e := range info.SelectElement("total").ChildElements() {
		if e.Tag != "ICMSTot" {
			printNamedXML(full, e, "total")
		}
	}
	view.Pages = append(view.Pages, page)
	var html bytes.Buffer
	if err = nfe55Template.Execute(&html, view); err != nil {
		return out, err
	}
	source, rendered := sha256.Sum256(process), sha256.Sum256(html.Bytes())
	return DANFEArtifact{Key: original.Key, SourceSHA256: hex.EncodeToString(source[:]), HTMLSHA256: hex.EncodeToString(rendered[:]), HTML: html.Bytes(), WidthMM: 210, Timezone: opts.Timezone, Environment: "2"}, nil
}

func printXMLText(e *etree.Element, path string) string {
	if e != nil {
		if n := e.FindElement(path); n != nil {
			return n.Text()
		}
	}
	return ""
}
func printNamedXML(add func(string, string), e *etree.Element, prefix string) {
	name := prefix + "/" + e.Tag
	if len(e.ChildElements()) == 0 {
		add(name, e.Text())
		return
	}
	for _, child := range e.ChildElements() {
		printNamedXML(add, child, name)
	}
}

var nfe55Template = template.Must(template.New("nfe55").Funcs(template.FuncMap{"height": func(units int) int { return units * 3 }, "percent": func(units int) string { return fmt.Sprintf("%.1f", float64(units)/10) }}).Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"><title>DANFE NF-e 55 — homologação</title><style>
@page{size:A4 portrait;margin:7mm}*{box-sizing:border-box}body{margin:0;background:#ddd;color:#000;font:7pt monospace}.page{width:196mm;min-height:280mm;margin:5mm auto;background:white;break-after:page}.page:last-child{break-after:auto}.identity{display:flex;border:1px solid #000;min-height:55mm}.issuer{width:23%;padding:2mm;overflow-wrap:anywhere}.issuer h1{font:bold 12pt Arial;margin:0}.document{width:17%;padding:2mm;border-left:1px solid #000;border-right:1px solid #000;text-align:center;font:8pt Arial}.document strong{font-size:14pt}.access{width:60%;padding:1mm;overflow-wrap:anywhere;font:7pt Arial}.barcode{width:115mm;height:10mm;display:block;image-rendering:pixelated;margin:1mm auto}.warning{border:2px solid #000;font:bold 10pt Arial;text-align:center;padding:2mm;margin:1mm 0}.row{display:flex;border-left:1px solid #000;border-top:1px solid #000;break-inside:avoid}.cell{border-right:1px solid #000;white-space:pre;line-height:3mm;padding:0 1mm;flex-shrink:0;overflow:visible}.title{font-weight:bold;background:#eee}.data{border-bottom:1px solid #000}.footer{border:1px solid #000;margin-top:2mm;padding:1mm;font:7pt Arial}.receipt{border:1px dashed #000;padding:2mm;margin-top:2mm;font:7pt Arial}@media print{body{background:white}.page{margin:0;min-height:0}.title{background:white}}</style></head><body>
{{range .Pages}}<article class="page"><header class="identity"><div class="issuer"><h1>{{$.Issuer}}</h1><p>{{$.IssuerAddress}}</p><p>CNPJ {{$.CNPJ}} — IE {{$.IE}}</p></div><div class="document"><strong>DANFE</strong><p>Documento Auxiliar da Nota Fiscal Eletrônica</p><p>0 — Entrada<br>1 — Saída<br>Tipo: {{$.Direction}}</p><b>Nº {{$.Number}}<br>Série {{$.Series}}</b><p>Folha {{.Index}}/{{len $.Pages}}</p></div><div class="access"><img class="barcode" alt="Chave de acesso em CODE-128C/A" src="data:image/png;base64,{{$.Barcode}}"><b>CHAVE DE ACESSO</b><p>{{$.Key}}</p><p>Consulta de autenticidade no portal nacional da NF-e<br>www.nfe.fazenda.gov.br/portal ou no site da SEFAZ autorizadora</p><b>PROTOCOLO DE AUTORIZAÇÃO DE USO</b><p>{{$.Protocol}} — {{$.AuthorizedAt}}</p></div></header>
<div class="warning">SEM VALOR FISCAL — AMBIENTE DE HOMOLOGAÇÃO</div><main class="data">{{range .Rows}}<div class="row{{if .Title}} title{{end}}" style="min-height:{{height .Units}}mm">{{range .Cells}}<div class="cell" style="width:{{percent .Width}}%">{{.Text}}</div>{{end}}</div>{{end}}</main>
<footer class="footer">Cópia técnica do XML arquivado. Leiaute e impressão física ainda sujeitos a validação. Folha {{.Index}}/{{len $.Pages}}. Não é autorização da SEFAZ nem comprovante de pagamento.</footer>{{if eq .Index 1}}<div class="receipt">RECEBEMOS DE {{$.Issuer}} OS PRODUTOS / SERVIÇOS CONSTANTES DA NF-e Nº {{$.Number}} SÉRIE {{$.Series}}<br><br>DATA DO RECEBIMENTO: __________________ IDENTIFICAÇÃO E ASSINATURA: __________________</div>{{end}}</article>{{end}}</body></html>`))
