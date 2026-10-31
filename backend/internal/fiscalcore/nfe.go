package fiscalcore

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

var keyRE = regexp.MustCompile(`^[0-9]{6}[0-9A-Z]{12}[0-9]{26}$`)
var xmlTagRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,40}$`)

type KeyInput struct {
	UF          string
	IssuedAt    time.Time
	CNPJ        string
	Model       string
	Series      int
	Number      int64
	Emission    int
	NumericCode string
}

// AccessKey calculates the official modulo-11 key, including alphanumeric CNPJ.
// It does not allocate a number: the caller must reserve it durably beforehand.
func AccessKey(in KeyInput) (string, error) {
	uf, ok := ufCodes[in.UF]
	if !ok || in.IssuedAt.IsZero() || !docRE.MatchString(in.CNPJ) || (in.Model != "55" && in.Model != "65") || in.Series < 0 || in.Series > 889 || in.Number < 1 || in.Number > 999999999 || in.Emission < 1 || in.Emission > 9 || len(in.NumericCode) != 8 || !digitsRE.MatchString(in.NumericCode) {
		return "", ErrInput
	}
	body := uf + in.IssuedAt.Format("0601") + in.CNPJ + in.Model + fmt.Sprintf("%03d%09d%d", in.Series, in.Number, in.Emission) + in.NumericCode
	return body + strconv.Itoa(accessDigit(body)), nil
}
func accessDigit(body string) int {
	sum, weight := 0, 2
	for i := len(body) - 1; i >= 0; i-- {
		sum += int(body[i]-48) * weight
		weight++
		if weight == 10 {
			weight = 2
		}
	}
	d := 11 - sum%11
	if d >= 10 {
		return 0
	}
	return d
}
func ValidAccessKey(key string) bool {
	return keyRE.MatchString(key) && accessDigit(key[:43]) == int(key[43]-'0')
}

func invoiceIdentity(info *etree.Element, key string) error {
	get := func(path string) string {
		e := info.FindElement(path)
		if e == nil || e.NamespaceURI() != NFeNS {
			return ""
		}
		return e.Text()
	}
	issued, err := time.Parse(time.RFC3339, get("ide/dhEmi"))
	if err != nil {
		return ErrXML
	}
	series, e1 := strconv.Atoi(get("ide/serie"))
	number, e2 := strconv.ParseInt(get("ide/nNF"), 10, 64)
	emission, e3 := strconv.Atoi(get("ide/tpEmis"))
	if e1 != nil || e2 != nil || e3 != nil {
		return ErrXML
	}
	uf := ""
	for name, code := range ufCodes {
		if code == get("ide/cUF") {
			uf = name
		}
	}
	computed, err := AccessKey(KeyInput{UF: uf, IssuedAt: issued, CNPJ: get("emit/CNPJ"), Model: get("ide/mod"), Series: series, Number: number, Emission: emission, NumericCode: get("ide/cNF")})
	if err != nil || computed != key || get("ide/cDV") != key[43:] || get("emit/enderEmit/UF") != uf || !strings.HasPrefix(get("emit/enderEmit/cMun"), key[:2]) {
		return ErrXML
	}
	return nil
}

type Address struct{ Street, Number, District, CityCode, City, UF, PostalCode string }
type Party struct {
	CNPJ, CPF, Name, IE, IEDestination string
	Address                            Address
}
type TaxNode struct {
	Name     string
	Value    string
	Children []TaxNode
}
type InvoiceLine struct {
	FCI                                                                                    string `json:"FCI,omitempty"`
	TaxQuoteOperationID                                                                    string `json:"TaxQuoteOperationID,omitempty"`
	CEST                                                                                   string `json:"CEST,omitempty"`
	BenefitCode                                                                            string `json:"BenefitCode,omitempty"`
	ProductID, Description, Unit, Quantity, UnitPrice, Discount, Freight, Insurance, Other string
	NCM, CFOP, GTIN                                                                        string
	Taxes                                                                                  []TaxNode
}
type InvoicePayment struct{ Method, Amount string }
type Invoice struct {
	Key                                                          KeyInput
	Environment                                                  int
	CRT, Nature, Direction, Destination, FinalConsumer, Presence string
	Issuer                                                       Party
	Recipient                                                    *Party
	Lines                                                        []InvoiceLine
	Payments                                                     []InvoicePayment
	TaxTotals                                                    map[string]string
	ContingencyAt                                                time.Time
	ContingencyReason                                            string
}

func field(parent *etree.Element, name, value string) *etree.Element {
	e := parent.CreateElement(name)
	e.SetText(value)
	return e
}
func address(parent *etree.Element, name string, a Address) {
	e := parent.CreateElement(name)
	field(e, "xLgr", a.Street)
	field(e, "nro", a.Number)
	field(e, "xBairro", a.District)
	field(e, "cMun", a.CityCode)
	field(e, "xMun", a.City)
	field(e, "UF", a.UF)
	if a.PostalCode != "" {
		field(e, "CEP", a.PostalCode)
	}
	field(e, "cPais", "1058")
	field(e, "xPais", "BRASIL")
}
func taxNodes(parent *etree.Element, nodes []TaxNode, depth int) error {
	if depth > 8 || len(nodes) > 64 {
		return ErrInput
	}
	for _, n := range nodes {
		if !xmlTagRE.MatchString(n.Name) || (n.Value != "" && len(n.Children) > 0) {
			return ErrInput
		}
		e := parent.CreateElement(n.Name)
		if len(n.Children) > 0 {
			if err := taxNodes(e, n.Children, depth+1); err != nil {
				return err
			}
		} else {
			e.SetText(n.Value)
		}
	}
	return nil
}

// BuildInvoice serializes supplied, approved fiscal data. Tax groups are typed
// XML nodes checked against the official XSD, never raw XML concatenation.
// It does not select the legal tax regime or manufacture missing tax groups.
func buildInvoice(in Invoice) ([]byte, string, error) {
	key, err := AccessKey(in.Key)
	if err != nil {
		return nil, "", err
	}
	if (in.Environment != 1 && in.Environment != 2) || in.Issuer.CNPJ != in.Key.CNPJ || in.Issuer.Address.UF != in.Key.UF || len(in.Lines) == 0 || len(in.Lines) > 990 || len(in.Payments) == 0 || len(in.Payments) > 100 {
		return nil, "", ErrInput
	}
	if in.Key.Model == "55" && in.Recipient == nil {
		return nil, "", ErrInput
	}
	if in.Key.Emission != 1 && in.Key.Emission != 9 {
		return nil, "", ErrUnsupported
	}
	if in.Key.Emission == 9 && (in.Key.Model != "65" || in.ContingencyAt.IsZero() || len(in.ContingencyReason) < 15 || len(in.ContingencyReason) > 256) {
		return nil, "", ErrInput
	}
	d := etree.NewDocument()
	root := d.CreateElement("NFe")
	root.CreateAttr("xmlns", NFeNS)
	info := root.CreateElement("infNFe")
	info.CreateAttr("Id", "NFe"+key)
	info.CreateAttr("versao", "4.00")
	ide := info.CreateElement("ide")
	for _, v := range [][2]string{{"cUF", ufCodes[in.Key.UF]}, {"cNF", in.Key.NumericCode}, {"natOp", in.Nature}, {"mod", in.Key.Model}, {"serie", strconv.Itoa(in.Key.Series)}, {"nNF", strconv.FormatInt(in.Key.Number, 10)}, {"dhEmi", in.Key.IssuedAt.Format("2006-01-02T15:04:05-07:00")}, {"tpNF", in.Direction}, {"idDest", in.Destination}, {"cMunFG", in.Issuer.Address.CityCode}} {
		field(ide, v[0], v[1])
	}
	printType := "1"
	if in.Key.Model == "65" {
		printType = "4"
	}
	for _, v := range [][2]string{{"tpImp", printType}, {"tpEmis", strconv.Itoa(in.Key.Emission)}, {"cDV", key[43:]}, {"tpAmb", strconv.Itoa(in.Environment)}, {"finNFe", "1"}, {"indFinal", in.FinalConsumer}, {"indPres", in.Presence}, {"procEmi", "0"}, {"verProc", "Titan-fiscal-0.2"}} {
		field(ide, v[0], v[1])
	}
	if in.Key.Emission == 9 {
		field(ide, "dhCont", in.ContingencyAt.Format("2006-01-02T15:04:05-07:00"))
		field(ide, "xJust", in.ContingencyReason)
	}
	emit := info.CreateElement("emit")
	field(emit, "CNPJ", in.Issuer.CNPJ)
	field(emit, "xNome", in.Issuer.Name)
	address(emit, "enderEmit", in.Issuer.Address)
	field(emit, "IE", in.Issuer.IE)
	field(emit, "CRT", in.CRT)
	if in.Recipient != nil {
		p := in.Recipient
		dest := info.CreateElement("dest")
		if p.CNPJ != "" {
			if p.CPF != "" {
				return nil, "", ErrInput
			}
			field(dest, "CNPJ", p.CNPJ)
		} else {
			field(dest, "CPF", p.CPF)
		}
		name := p.Name
		if in.Environment == 2 {
			name = "NF-E EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
		}
		field(dest, "xNome", name)
		if p.Address.CityCode != "" {
			address(dest, "enderDest", p.Address)
		}
		field(dest, "indIEDest", p.IEDestination)
		if p.IE != "" {
			field(dest, "IE", p.IE)
		}
	}
	products, discounts, freight, insurance, others := zero(), zero(), zero(), zero(), zero()
	for i, line := range in.Lines {
		if len(line.Taxes) == 0 {
			return nil, "", ErrRule
		}
		q, e := decimal(line.Quantity)
		if e != nil || q.Sign() <= 0 {
			return nil, "", ErrInput
		}
		u, e := decimal(line.UnitPrice)
		if e != nil {
			return nil, "", e
		}
		gross, _ := decimal(money(new(big.Rat).Mul(q, u)))
		products.Add(products, gross)
		det := info.CreateElement("det")
		det.CreateAttr("nItem", strconv.Itoa(i+1))
		prod := det.CreateElement("prod")
		gtin := line.GTIN
		if gtin == "" {
			gtin = "SEM GTIN"
		}
		for _, v := range [][2]string{{"cProd", line.ProductID}, {"cEAN", gtin}, {"xProd", line.Description}, {"NCM", line.NCM}, {"CFOP", line.CFOP}, {"uCom", line.Unit}, {"qCom", line.Quantity}, {"vUnCom", line.UnitPrice}, {"vProd", money(gross)}, {"cEANTrib", gtin}, {"uTrib", line.Unit}, {"qTrib", line.Quantity}, {"vUnTrib", line.UnitPrice}} {
			field(prod, v[0], v[1])
			if v[0] == "NCM" {
				if line.CEST != "" {
					field(prod, "CEST", line.CEST)
				}
				if line.BenefitCode != "" {
					field(prod, "cBenef", line.BenefitCode)
				}
			}
		}
		for _, v := range []struct {
			name, value string
			sum         *big.Rat
		}{{"vFrete", line.Freight, freight}, {"vSeg", line.Insurance, insurance}, {"vDesc", line.Discount, discounts}, {"vOutro", line.Other, others}} {
			n, e := decimal(v.value)
			if e != nil {
				return nil, "", e
			}
			if v.name == "vDesc" && n.Cmp(gross) > 0 {
				return nil, "", ErrInput
			}
			n, _ = decimal(money(n))
			v.sum.Add(v.sum, n)
			if n.Sign() != 0 {
				field(prod, v.name, money(n))
			}
		}
		field(prod, "indTot", "1")
		if line.FCI != "" {
			field(prod, "nFCI", line.FCI)
		}
		tax := det.CreateElement("imposto")
		if err = taxNodes(tax, line.Taxes, 0); err != nil {
			return nil, "", err
		}
	}
	totals := info.CreateElement("total").CreateElement("ICMSTot")
	required := []string{"vBC", "vICMS", "vICMSDeson", "vFCP", "vBCST", "vST", "vFCPST", "vFCPSTRet", "vII", "vIPI", "vIPIDevol", "vPIS", "vCOFINS"}
	for _, name := range required {
		if _, ok := in.TaxTotals[name]; !ok {
			return nil, "", ErrRule
		}
	}
	sharingNames := []string{"vFCPUFDest", "vICMSUFDest", "vICMSUFRemet"}
	sharingCount := 0
	for _, name := range sharingNames {
		if _, ok := in.TaxTotals[name]; ok {
			sharingCount++
		}
	}
	if sharingCount != 0 && sharingCount != 3 {
		return nil, "", ErrRule
	}
	if sharingCount == 3 {
		required = append(required, sharingNames...)
	}
	total := new(big.Rat).Set(products)
	total.Sub(total, discounts)
	total.Add(total, freight)
	total.Add(total, insurance)
	total.Add(total, others)
	for _, name := range []string{"vST", "vFCPST", "vII", "vIPI", "vIPIDevol"} {
		n, e := decimal(in.TaxTotals[name])
		if e != nil {
			return nil, "", e
		}
		total.Add(total, n)
	}
	// Deduction of desonerated ICMS depends on indDeduzDeson; not inferred here.
	if deson, e := decimal(in.TaxTotals["vICMSDeson"]); e != nil || deson.Sign() != 0 {
		return nil, "", ErrUnsupported
	}
	values := map[string]string{"vProd": money(products), "vFrete": money(freight), "vSeg": money(insurance), "vDesc": money(discounts), "vOutro": money(others), "vNF": money(total)}
	for _, name := range []string{"vBC", "vICMS", "vICMSDeson", "vFCP", "vBCST", "vST", "vFCPST", "vFCPSTRet", "vProd", "vFrete", "vSeg", "vDesc", "vII", "vIPI", "vIPIDevol", "vPIS", "vCOFINS", "vOutro", "vNF"} {
		v, ok := values[name]
		if !ok {
			n, e := decimal(in.TaxTotals[name])
			if e != nil {
				return nil, "", e
			}
			v = money(n)
		}
		field(totals, name, v)
		if name == "vICMSDeson" && sharingCount == 3 {
			for _, extra := range sharingNames {
				n, err := decimal(in.TaxTotals[extra])
				if err != nil {
					return nil, "", err
				}
				field(totals, extra, money(n))
			}
		}
	}
	for name := range in.TaxTotals {
		found := false
		for _, x := range required {
			if x == name {
				found = true
			}
		}
		if !found {
			return nil, "", ErrUnsupported
		}
	}
	field(info.CreateElement("transp"), "modFrete", "9")
	pag := info.CreateElement("pag")
	paid := zero()
	for _, p := range in.Payments {
		n, e := decimal(p.Amount)
		if e != nil {
			return nil, "", e
		}
		paid.Add(paid, n)
		det := pag.CreateElement("detPag")
		field(det, "tPag", p.Method)
		field(det, "vPag", money(n))
	}
	if paid.Cmp(total) < 0 {
		return nil, "", ErrInput
	}
	if paid.Cmp(total) > 0 {
		field(pag, "vTroco", money(new(big.Rat).Sub(paid, total)))
	}
	raw, err := d.WriteToBytes()
	if err != nil {
		return nil, "", ErrXML
	}
	return raw, key, nil
}

func BuildInvoice(in Invoice) ([]byte, string, error) {
	raw, key, err := buildInvoice(in)
	if err != nil {
		return nil, "", err
	}
	doc, err := parseXML(raw)
	if err != nil {
		return nil, "", err
	}
	info, _ := payload(doc.Root(), "nfe")
	if err = reconcileTaxTotals(info); err != nil {
		return nil, "", err
	}
	return raw, key, nil
}
