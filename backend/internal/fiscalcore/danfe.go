package fiscalcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"html/template"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata" // Desktop installations must not depend on host zoneinfo files.

	"github.com/beevik/etree"
)

// PrintOptions is presentation configuration, not authority to issue a document.
// The installation supplies its local timezone explicitly; UF is insufficient
// to select a timezone for every Brazilian municipality.
type PrintOptions struct {
	WidthMM  int
	Timezone string
}

type DANFEArtifact struct {
	Key                  string `json:"key"`
	SourceSHA256         string `json:"source_sha256"`
	HTMLSHA256           string `json:"html_sha256"`
	HTML                 []byte `json:"html"`
	WidthMM              int    `json:"width_mm"`
	Timezone             string `json:"timezone"`
	Environment          string `json:"environment"`
	OfficialHomologation bool   `json:"official_homologation"`
}

// CompleteNFCeXML attaches the installation-selected consultation URLs to a
// verified signed normal NFC-e. Supplement is outside infNFe: the original
// signature/digest must remain byte-equivalent and verify afterwards. This does
// not choose a UF endpoint, approve CSC or enable offline contingency.
func (c *Credential) CompleteNFCeXML(config QRConfiguration, consultation string, signed []byte, now time.Time) ([]byte, error) {
	if !publicFiscalURL(consultation, false) {
		return nil, ErrInput
	}
	qrURL, err := c.QRCode(config, signed, now)
	if err != nil {
		return nil, err
	}
	d, err := parseXML(signed)
	if err != nil {
		return nil, err
	}
	info, err := payload(d.Root(), "nfe")
	if err != nil {
		return nil, err
	}
	key := strings.TrimPrefix(info.SelectAttrValue("Id", ""), "NFe")
	if key[34:35] != "1" {
		return nil, ErrUnsupported
	}
	if d.Root().SelectElement("infNFeSupl") != nil {
		return nil, ErrInput
	}
	supl := etree.NewElement("infNFeSupl")
	field(supl, "qrCode", qrURL)
	field(supl, "urlChave", consultation)
	d.Root().InsertChild(d.Root().SelectElement("Signature"), supl)
	out, err := d.WriteToBytes()
	if err != nil {
		return nil, ErrXML
	}
	if err = c.VerifyXML("nfe", out, now); err != nil {
		return nil, err
	}
	return out, nil
}

type printLine struct{ Code, Description, Quantity, Unit, UnitPrice, Total string }
type printValue struct{ Label, Value string }
type nfcePrint struct {
	ContentWidth                                                                     int
	Width                                                                            int
	Issuer, CNPJ, IE, Address, Key, Number, Series, IssuedAt, Protocol, AuthorizedAt string
	Consumer, ConsumerName, ConsumerAddress, DeliveryAddress                         string
	Consultation, QR, FiscalMessage, Additional, ProtocolMessage, ApproximateTaxes   string
	Count                                                                            int
	Lines                                                                            []printLine
	Totals, Payments                                                                 []printValue
}

// RenderHomologationNFCe renders ONLY an archived, authorized-indicated normal
// model65 process selected by a trusted journal. It does not authenticate an
// uploaded protocol against SEFAZ, emit, authorize or change the document.
// Production/contingency/model55 are deliberately not silently approximated.
func RenderHomologationNFCe(process []byte, opts PrintOptions) (DANFEArtifact, error) {
	var out DANFEArtifact
	zone, err := printLocation(opts)
	if err != nil {
		return out, ErrInput
	}
	if err = ValidateXML("nfe_process", process); err != nil {
		return out, err
	}
	d, err := parseXMLLimits(process, 2*MaxXML, 8200)
	if err != nil {
		return out, err
	}
	root := d.Root()
	if root.Tag != "nfeProc" || root.NamespaceURI() != NFeNS || len(root.ChildElements()) != 2 {
		return out, ErrUncertain
	}
	nfe, proto := root.SelectElement("NFe"), root.SelectElement("protNFe")
	if nfe == nil || proto == nil {
		return out, ErrUncertain
	}
	signedDoc := etree.NewDocument()
	signedDoc.SetRoot(nfe.Copy())
	signed, err := signedDoc.WriteToBytes()
	if err != nil {
		return out, ErrXML
	}
	original, _, err := inspectArchivedNFe(signed)
	if err != nil {
		return out, err
	}
	if original.Key[20:22] != "65" || original.Key[34:35] != "1" {
		return out, ErrUnsupported
	}
	// Reuse the issuance guard; an unsigned protocol with another key/digest,
	// environment, rejection or malformed date must not label this print active.
	auth := etree.NewDocument()
	response := auth.CreateElement("retEnviNFe")
	response.CreateAttr("xmlns", NFeNS)
	response.CreateAttr("versao", "4.00")
	field(response, "tpAmb", "2")
	field(response, "cStat", "104")
	response.AddChild(proto.Copy())
	authRaw, _ := auth.WriteToBytes()
	protocol, err := ParseProtocol(authRaw, original.Key, "2", original.Digest)
	if err != nil {
		return out, err
	}
	info := nfe.SelectElement("infNFe")
	text := func(path string) string {
		if e := info.FindElement(path); e != nil {
			return e.Text()
		}
		return ""
	}
	supl := nfe.SelectElement("infNFeSupl")
	if supl == nil || supl.SelectElement("qrCode") == nil || supl.SelectElement("urlChave") == nil {
		return out, ErrInput
	}
	qrURL, consult := supl.SelectElement("qrCode").Text(), supl.SelectElement("urlChave").Text()
	if !validPrintQR(qrURL, original.Key) || !publicFiscalURL(consult, false) {
		return out, ErrInput
	}
	qrPNG, err := QRPNG(qrURL)
	if err != nil {
		return out, err
	}
	issued, err := time.Parse(time.RFC3339, text("ide/dhEmi"))
	if err != nil {
		return out, ErrInput
	}
	received, err := time.Parse(time.RFC3339, protocol.ReceivedAt)
	if err != nil {
		return out, ErrInput
	}
	view := nfcePrint{Width: opts.WidthMM, ContentWidth: opts.WidthMM - 4, Issuer: text("emit/xNome"), CNPJ: formatDocument(text("emit/CNPJ")), IE: text("emit/IE"), Address: printAddress(info.FindElement("emit/enderEmit")), Key: groupKey(original.Key), Number: text("ide/nNF"), Series: text("ide/serie"), IssuedAt: issued.In(zone).Format("02/01/2006 15:04:05 -0700"), Protocol: protocol.Number, AuthorizedAt: received.In(zone).Format("02/01/2006 15:04:05 -0700"), Consultation: consult, QR: base64.StdEncoding.EncodeToString(qrPNG), FiscalMessage: text("infAdic/infAdFisco"), Additional: text("infAdic/infCpl"), Consumer: "CONSUMIDOR NÃO IDENTIFICADO", ConsumerName: text("dest/xNome"), ConsumerAddress: printAddress(info.FindElement("dest/enderDest")), DeliveryAddress: printAddress(info.SelectElement("entrega"))}
	for _, kind := range []string{"CNPJ", "CPF", "idEstrangeiro"} {
		if value := text("dest/" + kind); value != "" {
			if kind != "idEstrangeiro" {
				value = formatDocument(value)
			}
			view.Consumer = "CONSUMIDOR " + kind + ": " + value
			break
		}
	}
	for _, detail := range info.SelectElements("det") {
		product := detail.SelectElement("prod")
		if product == nil {
			return out, ErrInput
		}
		get := func(name string) string {
			if e := product.SelectElement(name); e != nil {
				return e.Text()
			}
			return ""
		}
		view.Lines = append(view.Lines, printLine{Code: get("cProd"), Description: get("xProd"), Quantity: printDecimal(get("qCom")), Unit: get("uCom"), UnitPrice: printDecimal(get("vUnCom")), Total: printDecimal(get("vProd"))})
	}
	view.Count = len(view.Lines)
	if view.Count == 0 || view.Count > 990 {
		return out, ErrInput
	}
	view.Totals = append(view.Totals, printValue{"Valor total R$", printDecimal(text("total/ICMSTot/vProd"))})
	for _, value := range []struct{ field, label string }{{"vFrete", "Frete R$"}, {"vSeg", "Seguro R$"}, {"vOutro", "Outras despesas R$"}, {"vDesc", "Desconto R$"}} {
		raw := text("total/ICMSTot/" + value.field)
		if n, e := decimal(raw); e == nil && n.Sign() != 0 {
			view.Totals = append(view.Totals, printValue{value.label, printDecimal(raw)})
		}
	}
	view.Totals = append(view.Totals, printValue{"Valor a pagar R$", printDecimal(text("total/ICMSTot/vNF"))})
	for _, pay := range info.FindElements("pag/detPag") {
		method, amount := pay.SelectElement("tPag"), pay.SelectElement("vPag")
		if method == nil || amount == nil {
			return out, ErrInput
		}
		label := paymentLabel(method.Text())
		if method.Text() == "99" {
			if desc := pay.SelectElement("xPag"); desc != nil {
				label = desc.Text()
			}
		}
		view.Payments = append(view.Payments, printValue{label, printDecimal(amount.Text())})
	}
	if len(view.Payments) == 0 {
		return out, ErrInput
	}
	change := text("pag/vTroco")
	if change == "" {
		view.Payments = append(view.Payments, printValue{"Troco", "Não informado no XML"})
	} else {
		view.Payments = append(view.Payments, printValue{"Troco R$", printDecimal(change)})
	}
	if amount := text("total/ICMSTot/vTotTrib"); amount != "" {
		view.ApproximateTaxes = printDecimal(amount)
	}
	if message := proto.FindElement("infProt/xMsg"); message != nil {
		view.ProtocolMessage = message.Text()
	}
	var html bytes.Buffer
	if err = nfceTemplate.Execute(&html, view); err != nil {
		return out, err
	}
	sourceSum, htmlSum := sha256.Sum256(process), sha256.Sum256(html.Bytes())
	return DANFEArtifact{Key: original.Key, SourceSHA256: hex.EncodeToString(sourceSum[:]), HTMLSHA256: hex.EncodeToString(htmlSum[:]), HTML: html.Bytes(), WidthMM: opts.WidthMM, Timezone: opts.Timezone, Environment: "2"}, nil
}

func publicFiscalURL(raw string, query bool) bool {
	if len(raw) == 0 || len(raw) > 1024 || strings.TrimSpace(raw) != raw {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.User == nil && u.Hostname() != "" && strings.HasSuffix(strings.ToLower(u.Hostname()), ".gov.br") && u.Port() == "" && u.Fragment == "" && (query || u.RawQuery == "")
}
func validPrintQR(raw, key string) bool {
	if !publicFiscalURL(raw, true) {
		return false
	}
	u, _ := url.Parse(raw)
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query) != 1 || len(query["p"]) != 1 {
		return false
	}
	parts := strings.Split(query.Get("p"), "|")
	if len(parts) < 3 || parts[0] != key || parts[2] != "2" {
		return false
	}
	if parts[1] == "3" {
		return len(parts) == 3
	}
	if parts[1] != "2" || len(parts) != 5 || len(parts[3]) < 1 || len(parts[3]) > 6 || !digitsRE.MatchString(parts[3]) || len(parts[4]) != 40 {
		return false
	}
	_, err = hex.DecodeString(parts[4])
	return err == nil
}
func groupKey(key string) string {
	var parts []string
	for i := 0; i < len(key); i += 4 {
		parts = append(parts, key[i:i+4])
	}
	return strings.Join(parts, " ")
}
func formatDocument(value string) string {
	if len(value) == 14 {
		return value[:2] + "." + value[2:5] + "." + value[5:8] + "/" + value[8:12] + "-" + value[12:]
	}
	if len(value) == 11 && digitsRE.MatchString(value) {
		return value[:3] + "." + value[3:6] + "." + value[6:9] + "-" + value[9:]
	}
	return value
}
func printDecimal(value string) string {
	parts := strings.SplitN(value, ".", 2)
	whole := parts[0]
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "." + whole[i:]
	}
	if len(parts) == 2 {
		return whole + "," + parts[1]
	}
	return whole
}
func printAddress(e *etree.Element) string {
	if e == nil {
		return ""
	}
	var parts []string
	for _, name := range []string{"xLgr", "nro", "xCpl", "xBairro", "xMun", "UF", "CEP"} {
		if c := e.SelectElement(name); c != nil && c.Text() != "" {
			parts = append(parts, c.Text())
		}
	}
	return strings.Join(parts, ", ")
}
func paymentLabel(code string) string {
	names := map[string]string{"01": "Dinheiro", "02": "Cheque", "03": "Cartão de crédito", "04": "Cartão de débito", "05": "Crédito loja", "10": "Vale alimentação", "11": "Vale refeição", "12": "Vale presente", "13": "Vale combustível", "14": "Duplicata mercantil", "15": "Boleto bancário", "16": "Depósito bancário", "17": "Pix dinâmico", "18": "Transferência / carteira digital", "19": "Fidelidade / cashback / crédito virtual", "20": "Pix estático", "21": "Crédito em loja por devolução", "22": "Pagamento eletrônico não informado", "90": "Sem pagamento", "99": "Outros"}
	if name, ok := names[code]; ok {
		return name
	}
	return "Forma de pagamento " + code
}

var nfceTemplate = template.Must(template.New("nfce").Parse(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'"><title>DANFE NFC-e - homologação</title>
<style>@page{margin:2mm}*{box-sizing:border-box}html{color:#000;background:#fff;font:9pt Arial,sans-serif}body{width:{{.Width}}mm;margin:0 auto;padding:2mm;overflow-wrap:anywhere}h1{font-size:11pt;margin:0}h2{font-size:9pt;margin:2mm 0}p{margin:1mm 0}.center{text-align:center}.section{border-top:1px dashed #000;margin-top:2mm;padding-top:2mm}.warning{font-weight:bold;border:2px solid #000;padding:2mm;text-align:center;margin:2mm 0}.item{break-inside:avoid;margin:2mm 0}.item-values,.value{display:flex;justify-content:space-between;gap:1mm}.item-values{flex-wrap:wrap;font-size:8pt}.value strong{white-space:nowrap}.qr{display:block;width:30mm;height:30mm;margin:3mm auto;padding:1.5mm;image-rendering:pixelated}.message{white-space:pre-wrap}.key{font-family:monospace;font-size:8pt}.consult{font-size:8pt}@media screen{body{box-shadow:0 0 8px #aaa;margin:8px auto}}@media print{body{width:{{.ContentWidth}}mm;margin:0;padding:0}.center,.warning{break-inside:avoid}}</style></head><body>
<header class="center"><h1>{{.Issuer}}</h1><p>CNPJ {{.CNPJ}}</p><p>IE {{.IE}}</p><p>{{.Address}}</p><h2>Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica</h2></header>
<div class="warning">EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL</div>
<section class="section" aria-label="Produtos">{{range .Lines}}<div class="item"><strong>{{.Code}} - {{.Description}}</strong><div class="item-values"><span>{{.Quantity}} {{.Unit}}</span><span>× R$ {{.UnitPrice}}</span><strong>R$ {{.Total}}</strong></div></div>{{end}}</section>
<section class="section" aria-label="Totais"><div class="value"><span>Quantidade total de itens</span><strong>{{.Count}}</strong></div>{{range .Totals}}<div class="value"><span>{{.Label}}</span><strong>{{.Value}}</strong></div>{{end}}{{range .Payments}}<div class="value"><span>{{.Label}}</span><strong>{{.Value}}</strong></div>{{end}}</section>
<section class="section center"><p class="consult">Consulte pela chave de acesso em<br>{{.Consultation}}</p><p class="key">{{.Key}}</p><p>{{.Consumer}}</p>{{if .ConsumerName}}<p>{{.ConsumerName}}</p>{{end}}{{if .ConsumerAddress}}<p>{{.ConsumerAddress}}</p>{{end}}{{if .DeliveryAddress}}<p>Entrega: {{.DeliveryAddress}}</p>{{end}}<img class="qr" alt="QR Code para consulta da NFC-e de homologação" src="data:image/png;base64,{{.QR}}"><p>NFC-e nº {{.Number}} - Série {{.Series}}</p><p>Emissão: {{.IssuedAt}}</p><p>Protocolo de autorização: {{.Protocol}}</p><p>{{.AuthorizedAt}}</p></section>
<div class="warning">EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL</div>
{{if .FiscalMessage}}<section class="section message">{{.FiscalMessage}}</section>{{end}}{{if .ProtocolMessage}}<section class="section message">{{.ProtocolMessage}}</section>{{end}}{{if .ApproximateTaxes}}<section class="section">Tributos aproximados R$ {{.ApproximateTaxes}}</section>{{end}}{{if .Additional}}<section class="section message">{{.Additional}}</section>{{end}}
</body></html>`))

// Print timezones follow the Brazilian zones in IANA zone.tab. Selection remains
// explicit; no state-wide inference is made for multi-zone UFs.
func printLocation(opts PrintOptions) (*time.Location, error) {
	if opts.WidthMM != 58 && opts.WidthMM != 80 {
		return nil, ErrInput
	}
	switch opts.Timezone {
	case "America/Noronha", "America/Belem", "America/Fortaleza", "America/Recife", "America/Araguaina", "America/Maceio", "America/Bahia", "America/Sao_Paulo", "America/Campo_Grande", "America/Cuiaba", "America/Santarem", "America/Porto_Velho", "America/Boa_Vista", "America/Manaus", "America/Eirunepe", "America/Rio_Branco":
		return time.LoadLocation(opts.Timezone)
	default:
		return nil, ErrInput
	}
}
func ValidatePrintOptions(opts PrintOptions) error { _, err := printLocation(opts); return err }
