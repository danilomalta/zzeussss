package fiscalcore

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/beevik/etree"
)

const invalidationNS = NFeNS + "/wsdl/NFeInutilizacao4"

// MaxInvalidationRange is this component's resource bound, not a legal limit.
const MaxInvalidationRange int64 = 10000

type Invalidation struct {
	UF          string `json:"uf"`
	CNPJ        string `json:"cnpj"`
	Model       string `json:"model"`
	Series      int    `json:"series"`
	Year        int    `json:"year"`
	First       int64  `json:"first"`
	Last        int64  `json:"last"`
	Environment int    `json:"environment"`
	Reason      string `json:"reason"`
}

func InvalidationID(in Invalidation) (string, error) {
	uf, ok := ufCodes[in.UF]
	if !ok || !docRE.MatchString(in.CNPJ) || (in.Model != "55" && in.Model != "65") || in.Series < 0 || in.Series > 889 || in.Year < 2000 || in.Year > 2099 || in.First < 1 || in.Last < in.First || in.Last > 999999999 || in.Last-in.First+1 > MaxInvalidationRange || in.Environment != 2 || len(in.Reason) < 15 || len(in.Reason) > 255 {
		return "", ErrInput
	}
	return fmt.Sprintf("ID%s%02d%s%s%03d%09d%09d", uf, in.Year%100, in.CNPJ, in.Model, in.Series, in.First, in.Last), nil
}
func BuildInvalidation(in Invalidation) ([]byte, string, error) {
	id, err := InvalidationID(in)
	if err != nil {
		return nil, "", err
	}
	d := etree.NewDocument()
	root := d.CreateElement("inutNFe")
	root.CreateAttr("xmlns", NFeNS)
	root.CreateAttr("versao", "4.00")
	info := root.CreateElement("infInut")
	info.CreateAttr("Id", id)
	for _, v := range [][2]string{{"tpAmb", "2"}, {"xServ", "INUTILIZAR"}, {"cUF", ufCodes[in.UF]}, {"ano", fmt.Sprintf("%02d", in.Year%100)}, {"CNPJ", in.CNPJ}, {"mod", in.Model}, {"serie", strconv.Itoa(in.Series)}, {"nNFIni", strconv.FormatInt(in.First, 10)}, {"nNFFin", strconv.FormatInt(in.Last, 10)}, {"xJust", in.Reason}} {
		field(info, v[0], v[1])
	}
	raw, err := d.WriteToBytes()
	return raw, id, err
}
func invalidationInput(info *etree.Element) (Invalidation, error) {
	var in Invalidation
	get := func(name string) string {
		e := info.SelectElement(name)
		if e == nil || e.NamespaceURI() != NFeNS {
			return ""
		}
		return e.Text()
	}
	for uf, code := range ufCodes {
		if code == get("cUF") {
			in.UF = uf
		}
	}
	in.CNPJ = get("CNPJ")
	in.Model = get("mod")
	in.Reason = get("xJust")
	in.Environment = 2
	year, err := strconv.Atoi(get("ano"))
	if err != nil || len(get("ano")) != 2 {
		return in, ErrInput
	}
	in.Year = 2000 + year
	in.Series, err = strconv.Atoi(get("serie"))
	if err != nil {
		return in, ErrInput
	}
	in.First, err = strconv.ParseInt(get("nNFIni"), 10, 64)
	if err != nil {
		return in, ErrInput
	}
	in.Last, err = strconv.ParseInt(get("nNFFin"), 10, 64)
	if err != nil {
		return in, ErrInput
	}
	id, err := InvalidationID(in)
	if err != nil || info.SelectAttrValue("Id", "") != id || get("tpAmb") != "2" || get("xServ") != "INUTILIZAR" {
		return in, ErrInput
	}
	return in, nil
}

type InvalidationRecorder interface {
	Recorder
	SendGuard
	ValidateInvalidation(context.Context, Invalidation) error
	Read(context.Context) (RecordedRequest, ExchangeResult, error)
}
type InvalidationEvidence struct {
	State        string         `json:"state"`
	ID           string         `json:"id"`
	Input        Invalidation   `json:"input"`
	Status       string         `json:"status,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	Protocol     string         `json:"protocol,omitempty"`
	ReceivedAt   string         `json:"received_at,omitempty"`
	SignedXML    []byte         `json:"signed_xml"`
	ProcessedXML []byte         `json:"processed_xml,omitempty"`
	Exchange     ExchangeResult `json:"exchange"`
}

func (c *Client) InvalidateHomologation(ctx context.Context, operationID string, in Invalidation) (InvalidationEvidence, error) {
	var out InvalidationEvidence
	r, ok := c.recorder.(InvalidationRecorder)
	if !ok || in.Environment != 2 {
		return out, ErrUnsupported
	}
	if err := r.ValidateInvalidation(ctx, in); err != nil {
		return out, err
	}
	raw, id, err := BuildInvalidation(in)
	if err != nil {
		return out, err
	}
	signed, err := c.credential.SignXML("nfe_invalidation", raw, time.Now())
	if err != nil {
		return out, err
	}
	out = InvalidationEvidence{State: "not_sent", ID: id, Input: in, SignedXML: signed}
	result, err := c.SubmitInvalidation(ctx, operationID, signed)
	out.Exchange = result
	if result.OperationID != "" {
		out.State = result.State
	}
	if errors.Is(err, ErrRecorded) {
		recovered, _ := RecoverHomologationInvalidation(ctx, r)
		if recovered.ID != "" {
			return recovered, err
		}
		return InvalidationEvidence{State: "recovery_required"}, err
	}
	if err != nil {
		return out, err
	}
	return interpretInvalidation(signed, result)
}
func (c *Client) SubmitInvalidation(ctx context.Context, operationID string, signed []byte) (ExchangeResult, error) {
	if err := c.credential.VerifyXML("nfe_invalidation", signed, time.Now()); err != nil {
		return ExchangeResult{}, err
	}
	d, _ := parseXML(signed)
	info, _ := payload(d.Root(), "nfe_invalidation")
	in, err := invalidationInput(info)
	if err != nil {
		return ExchangeResult{}, err
	}
	id, _ := InvalidationID(in)
	raw, err := soap(invalidationNS, d.Root()).WriteToBytes()
	if err != nil {
		return ExchangeResult{}, ErrXML
	}
	return c.exchange(ctx, operationID, id, "2", `application/soap+xml; charset=utf-8; action="`+invalidationNS+`/nfeInutilizacaoNF"`, raw)
}
func RecoverHomologationInvalidation(ctx context.Context, r InvalidationRecorder) (InvalidationEvidence, error) {
	req, result, err := r.Read(ctx)
	if err != nil {
		return InvalidationEvidence{}, err
	}
	original, err := OriginalInvalidationFromRequest(req)
	if err != nil || req.OperationID != result.OperationID {
		return InvalidationEvidence{}, ErrUncertain
	}
	return interpretInvalidation(original.SignedXML, result)
}

type ArchivedInvalidation struct {
	Input     Invalidation
	SignedXML []byte
}

func OriginalInvalidationFromRequest(req RecordedRequest) (ArchivedInvalidation, error) {
	var out ArchivedInvalidation
	sum := sha256.Sum256(req.Body)
	if req.OperationID == "" || len(req.OperationID) > 128 || req.Scope == "" || req.URL == "" || req.Environment != "2" || req.RequestSHA256 != hex.EncodeToString(sum[:]) {
		return out, ErrUncertain
	}
	d, err := parseXMLLimits(req.Body, 2*MaxXML, 8200)
	if err != nil {
		return out, err
	}
	root := d.Root()
	if root.Tag != "Envelope" || root.NamespaceURI() != "http://www.w3.org/2003/05/soap-envelope" || len(root.ChildElements()) != 1 {
		return out, ErrUncertain
	}
	body := root.ChildElements()[0]
	if body.Tag != "Body" || body.NamespaceURI() != root.NamespaceURI() || len(body.ChildElements()) != 1 {
		return out, ErrUncertain
	}
	msg := body.ChildElements()[0]
	if msg.Tag != "nfeDadosMsg" || msg.NamespaceURI() != invalidationNS || len(msg.ChildElements()) != 1 {
		return out, ErrUncertain
	}
	nfe := msg.ChildElements()[0]
	if nfe.Tag != "inutNFe" || nfe.NamespaceURI() != NFeNS || nfe.SelectAttrValue("versao", "") != "4.00" {
		return out, ErrUncertain
	}
	event := etree.NewDocument()
	event.SetRoot(nfe.Copy())
	out.SignedXML, _ = event.WriteToBytes()
	out.Input, err = inspectArchivedInvalidation(out.SignedXML)
	if err != nil {
		return ArchivedInvalidation{}, err
	}
	id, _ := InvalidationID(out.Input)
	if req.Key != id || req.Issuer != out.Input.CNPJ {
		return ArchivedInvalidation{}, ErrUncertain
	}
	return out, nil
}
func inspectArchivedInvalidation(raw []byte) (Invalidation, error) {
	var in Invalidation
	if err := ValidateXML("nfe_invalidation", raw); err != nil {
		return in, err
	}
	d, err := parseXML(raw)
	if err != nil {
		return in, err
	}
	info, err := payload(d.Root(), "nfe_invalidation")
	if err != nil {
		return in, err
	}
	in, err = invalidationInput(info)
	if err != nil {
		return in, err
	}
	node := d.Root().FindElement("Signature/KeyInfo/X509Data/X509Certificate")
	if node == nil {
		return in, ErrXML
	}
	der, err := base64.StdEncoding.DecodeString(node.Text())
	if err != nil {
		return in, ErrXML
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil || certificateCNPJ(cert) != in.CNPJ {
		return in, ErrCredential
	}
	if _, ok := cert.PublicKey.(*rsa.PublicKey); !ok {
		return in, ErrCredential
	}
	c := &Credential{certificate: cert, document: in.CNPJ}
	if err = c.verifySignedXML("nfe_invalidation", raw); err != nil {
		return in, err
	}
	return in, nil
}
func interpretInvalidation(signed []byte, result ExchangeResult) (InvalidationEvidence, error) {
	out := InvalidationEvidence{State: "uncertain", SignedXML: append([]byte(nil), signed...), Exchange: result}
	in, err := inspectArchivedInvalidation(signed)
	if err != nil {
		return out, err
	}
	out.Input = in
	out.ID, _ = InvalidationID(in)
	sum := sha256.Sum256(result.Body)
	if result.OperationID == "" || result.State != "response_received" || result.HTTPStatus != 200 || len(result.Body) > MaxXML || result.SHA256 != hex.EncodeToString(sum[:]) {
		return out, ErrUncertain
	}
	root, err := fiscalResponseRoot(result.Body, invalidationNS, "retInutNFe")
	if err != nil {
		return out, err
	}
	d := etree.NewDocument()
	d.SetRoot(root.Copy())
	raw, _ := d.WriteToBytes()
	if err = ValidateXML("nfe_invalidation_response", raw); err != nil {
		return out, err
	}
	info := root.SelectElement("infInut")
	if info == nil {
		return out, ErrUncertain
	}
	get := func(name string) string {
		e := info.SelectElement(name)
		if e == nil {
			return ""
		}
		return e.Text()
	}
	if get("tpAmb") != "2" || get("cUF") != ufCodes[in.UF] {
		return out, ErrUncertain
	}
	out.Status = get("cStat")
	out.Reason = get("xMotivo")
	out.State = "response_requires_review"
	//563 and textual [nProt:...] are not exact structured success evidence.
	if out.Status != "102" {
		return out, nil
	}
	for _, v := range [][2]string{{"ano", fmt.Sprintf("%02d", in.Year%100)}, {"CNPJ", in.CNPJ}, {"mod", in.Model}, {"serie", strconv.Itoa(in.Series)}, {"nNFIni", strconv.FormatInt(in.First, 10)}, {"nNFFin", strconv.FormatInt(in.Last, 10)}} {
		if get(v[0]) != v[1] {
			return out, ErrUncertain
		}
	}
	protocol := get("nProt")
	if (len(protocol) != 15 && len(protocol) != 17) || !digitsRE.MatchString(protocol) || (info.SelectAttrValue("Id", "") != "" && info.SelectAttrValue("Id", "") != "ID"+protocol) {
		return out, ErrUncertain
	}
	if _, err = time.Parse(time.RFC3339, get("dhRecbto")); err != nil {
		return out, ErrUncertain
	}
	process := etree.NewDocument()
	p := process.CreateElement("ProcInutNFe")
	p.CreateAttr("xmlns", NFeNS)
	p.CreateAttr("versao", "4.00")
	original, _ := parseXML(signed)
	p.AddChild(original.Root().Copy())
	p.AddChild(root.Copy())
	complete, err := process.WriteToBytes()
	if err != nil {
		return out, ErrXML
	}
	if err = ValidateXML("nfe_invalidation_process", complete); err != nil {
		return out, err
	}
	out.State = "invalidation_indicated_homologation"
	out.Protocol = protocol
	out.ReceivedAt = get("dhRecbto")
	out.ProcessedXML = complete
	return out, nil
}
