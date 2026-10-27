package fiscalcore

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/beevik/etree"
)

type DPSInput struct {
	Purpose               string // finNFSe; explicit only in the restricted-production profile
	Environment           int
	IssuedAt              time.Time
	Competence            string
	Municipality          string
	ServiceMunicipality   string
	CNPJ                  string
	MunicipalRegistration string
	Series                int
	Number                int64
	SimpleOption          string
	SimpleApportionment   string
	SpecialRegime         string
	NationalServiceCode   string
	MunicipalServiceCode  string
	Description           string
	ServiceValue          string
	Taxation              []TaxNode
}

// BuildDPS targets the pinned national 1.01 schema, not every legacy municipal
// provider. Municipal agreement, CNAE/service classification and regime must be
// approved separately. This schema's numeric-only identifier is not silently
// applied to alphanumeric CNPJ: unsupported values are refused without stripping.
func buildDPS(in DPSInput) ([]byte, string, error) { return buildDPSProfile(in, false) }

func buildDPSProfile(in DPSInput, homolog bool) ([]byte, string, error) {
	if (!homolog && !digitsRE.MatchString(in.CNPJ)) || !docRE.MatchString(in.CNPJ) {
		return nil, "", ErrUnsupported
	}
	if (in.Environment != 1 && in.Environment != 2) || in.IssuedAt.IsZero() || len(in.Municipality) != 7 || !digitsRE.MatchString(in.Municipality) || in.Series < 1 || in.Series > 99999 || in.Number < 1 || in.Number > 999999999999999 || len(in.Taxation) == 0 {
		return nil, "", ErrInput
	}
	if _, err := time.Parse("2006-01-02", in.Competence); err != nil {
		return nil, "", ErrInput
	}
	v, err := decimal(in.ServiceValue)
	if err != nil || v.Sign() <= 0 {
		return nil, "", ErrInput
	}
	if (!homolog && in.Purpose != "") || (homolog && (in.Environment != 2 || in.Series > 89999 || in.Purpose != "0")) {
		return nil, "", ErrInput
	}
	id := "DPS" + in.Municipality + "2" + in.CNPJ + fmt.Sprintf("%05d%015d", in.Series, in.Number)
	d := etree.NewDocument()
	root := d.CreateElement("DPS")
	root.CreateAttr("xmlns", NFSeNS)
	root.CreateAttr("versao", "1.01")
	info := root.CreateElement("infDPS")
	info.CreateAttr("Id", id)
	for _, v := range [][2]string{{"tpAmb", strconv.Itoa(in.Environment)}, {"dhEmi", in.IssuedAt.Format("2006-01-02T15:04:05-07:00")}, {"verAplic", "Titan-fiscal-0.2"}, {"serie", strconv.Itoa(in.Series)}, {"nDPS", strconv.FormatInt(in.Number, 10)}, {"dCompet", in.Competence}, {"tpEmit", "1"}} {
		field(info, v[0], v[1])
	}
	if homolog {
		field(info, "finNFSe", in.Purpose)
	}
	field(info, "cLocEmi", in.Municipality)
	prest := info.CreateElement("prest")
	field(prest, "CNPJ", in.CNPJ)
	if in.MunicipalRegistration != "" {
		field(prest, "IM", in.MunicipalRegistration)
	}
	reg := prest.CreateElement("regTrib")
	field(reg, "opSimpNac", in.SimpleOption)
	if in.SimpleApportionment != "" {
		field(reg, "regApTribSN", in.SimpleApportionment)
	}
	field(reg, "regEspTrib", in.SpecialRegime)
	serv := info.CreateElement("serv")
	field(serv.CreateElement("locPrest"), "cLocPrestacao", in.ServiceMunicipality)
	cserv := serv.CreateElement("cServ")
	field(cserv, "cTribNac", in.NationalServiceCode)
	if in.MunicipalServiceCode != "" {
		field(cserv, "cTribMun", in.MunicipalServiceCode)
	}
	field(cserv, "xDescServ", in.Description)
	values := info.CreateElement("valores")
	field(values.CreateElement("vServPrest"), "vServ", money(v))
	if err = taxNodes(values.CreateElement("trib"), in.Taxation, 0); err != nil {
		return nil, "", err
	}
	raw, err := d.WriteToBytes()
	if err != nil {
		return nil, "", ErrXML
	}
	return raw, id, nil
}

func BuildDPS(in DPSInput) ([]byte, string, error) {
	raw, id, err := buildDPS(in)
	if err != nil {
		return nil, "", err
	}
	if err = ValidateXML("nfse_dps", raw); err != nil {
		return nil, "", err
	}
	return raw, id, nil
}

// BuildDPSHomolog uses the unmodified official restricted-production bundle
// of 2026-10-20. It NEVER accepts environment 1 or changes the production schema.
func BuildDPSHomolog(in DPSInput) ([]byte, string, error) {
	raw, id, err := buildDPSProfile(in, true)
	if err != nil {
		return nil, "", err
	}
	if err = ValidateXML("nfse_dps_homolog_20261020", raw); err != nil {
		return nil, "", err
	}
	return raw, id, nil
}

// PackDPS implements the national JSON envelope. The signature is verified
// before compression. A JSON payload alone is never recorded as an issued NFS-e.
func (c *Credential) PackDPS(raw []byte, now time.Time) ([]byte, error) {
	return c.packDPS("nfse_dps", raw, now)
}

// PackDPSHomolog is an offline envelope, not proof of national acceptance.
func (c *Credential) PackDPSHomolog(raw []byte, now time.Time) ([]byte, error) {
	return c.packDPS("nfse_dps_homolog_20261020", raw, now)
}

func (c *Credential) packDPS(kind string, raw []byte, now time.Time) ([]byte, error) {
	if err := c.VerifyXML(kind, raw, now); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(raw); err != nil {
		return nil, ErrXML
	}
	if err := w.Close(); err != nil {
		return nil, ErrXML
	}
	return json.Marshal(struct {
		XML string `json:"dpsXmlGZipB64"`
	}{base64.StdEncoding.EncodeToString(b.Bytes())})
}

// UnpackNFSe caps decompression independently of the HTTP response size.
// It validates the official XML schema, but does not itself prove TLS origin,
// ownership, DPS association or authorization of a document supplied by a caller.
func UnpackNFSe(value string) ([]byte, error) {
	if len(value) > 2*MaxXML {
		return nil, ErrInput
	}
	compressed, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrXML
	}
	input := bytes.NewReader(compressed)
	reader, err := gzip.NewReader(input)
	if err != nil {
		return nil, ErrXML
	}
	defer reader.Close()
	reader.Multistream(false)
	raw, err := io.ReadAll(io.LimitReader(reader, MaxXML+1))
	if err != nil || len(raw) > MaxXML || input.Len() != 0 {
		return nil, ErrXML
	}
	if err = ValidateXML("nfse_document", raw); err != nil {
		return nil, err
	}
	return raw, nil
}

type Cancellation struct {
	Key         string
	CNPJ        string
	Environment int
	Time        time.Time
	Protocol    string
	Reason      string
}

func cancellationIdentity(info *etree.Element, document string) error {
	get := func(name string) string {
		e := info.SelectElement(name)
		if e == nil || e.NamespaceURI() != NFeNS {
			return ""
		}
		return e.Text()
	}
	key := get("chNFe")
	if !ValidAccessKey(key) || key[6:20] != document || get("CNPJ") != document {
		return ErrCredential
	}
	if get("tpEvento") != "110111" {
		return ErrUnsupported
	}
	if get("nSeqEvento") != "1" || get("cOrgao") != key[:2] || get("verEvento") != "1.00" || info.SelectAttrValue("Id", "") != "ID110111"+key+"01" {
		return ErrXML
	}
	det := info.SelectElement("detEvento")
	if det == nil || det.NamespaceURI() != NFeNS {
		return ErrXML
	}
	d := etree.NewDocument()
	d.SetRoot(det.Copy())
	d.Root().CreateAttr("xmlns", NFeNS)
	raw, err := d.WriteToBytes()
	if err != nil {
		return ErrXML
	}
	return ValidateXML("nfe_cancel_detail", raw)
}

func BuildCancellation(in Cancellation) ([]byte, error) {
	if !ValidAccessKey(in.Key) || in.CNPJ != in.Key[6:20] || (in.Environment != 1 && in.Environment != 2) || in.Time.IsZero() || (len(in.Protocol) != 15 && len(in.Protocol) != 17) || !digitsRE.MatchString(in.Protocol) || len(in.Reason) < 15 || len(in.Reason) > 255 {
		return nil, ErrInput
	}
	d := etree.NewDocument()
	root := d.CreateElement("evento")
	root.CreateAttr("xmlns", NFeNS)
	root.CreateAttr("versao", "1.00")
	info := root.CreateElement("infEvento")
	info.CreateAttr("Id", "ID110111"+in.Key+"01")
	for _, v := range [][2]string{{"cOrgao", in.Key[:2]}, {"tpAmb", strconv.Itoa(in.Environment)}, {"CNPJ", in.CNPJ}, {"chNFe", in.Key}, {"dhEvento", in.Time.Format("2006-01-02T15:04:05-07:00")}, {"tpEvento", "110111"}, {"nSeqEvento", "1"}, {"verEvento", "1.00"}} {
		field(info, v[0], v[1])
	}
	det := info.CreateElement("detEvento")
	det.CreateAttr("versao", "1.00")
	field(det, "descEvento", "Cancelamento")
	field(det, "nProt", in.Protocol)
	field(det, "xJust", in.Reason)
	detail := etree.NewDocument()
	detail.SetRoot(det.Copy())
	detail.Root().CreateAttr("xmlns", NFeNS)
	part, err := detail.WriteToBytes()
	if err != nil {
		return nil, ErrXML
	}
	if err = ValidateXML("nfe_cancel_detail", part); err != nil {
		return nil, err
	}
	return d.WriteToBytes()
}
