package fiscalcore

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/beevik/etree"
)

const cancellationNS = NFeNS + "/wsdl/NFeRecepcaoEvento4"

// CancellationRecorder binds an explicit cancellation to an authenticated,
// authorized original invoice, current human authority and a durable journal.
// A fiscal cancellation never means refunding cash or returning stock.
type CancellationRecorder interface {
	Recorder
	SendGuard
	ValidateCancellation(context.Context, Cancellation) error
	Read(context.Context) (RecordedRequest, ExchangeResult, error)
}

type CancellationEvidence struct {
	RecoveryQueryOperationID string         `json:"recovery_query_operation_id,omitempty"`
	State                    string         `json:"state"`
	Key                      string         `json:"key"`
	EventID                  string         `json:"event_id"`
	Status                   string         `json:"status,omitempty"`
	Reason                   string         `json:"reason,omitempty"`
	Protocol                 string         `json:"protocol,omitempty"`
	RegisteredAt             string         `json:"registered_at,omitempty"`
	SignedXML                []byte         `json:"signed_xml"`
	ProcessedXML             []byte         `json:"processed_xml,omitempty"`
	Exchange                 ExchangeResult `json:"exchange"`
}

// CancelHomologationNFe performs one explicit event110111/sequence1 attempt.
// It does not decide legal deadlines, authorize production or repeat a POST.
func (c *Client) CancelHomologationNFe(ctx context.Context, operationID, lot string, in Cancellation) (CancellationEvidence, error) {
	return c.cancelNFe(ctx, operationID, lot, in, 2)
}

func (c *Client) cancelNFe(ctx context.Context, operationID, lot string, in Cancellation, environment int) (CancellationEvidence, error) {
	var out CancellationEvidence
	r, ok := c.recorder.(CancellationRecorder)
	if !ok || in.Environment != environment {
		return out, ErrUnsupported
	}
	if err := r.ValidateCancellation(ctx, in); err != nil {
		return out, err
	}
	raw, err := BuildCancellation(in)
	if err != nil {
		return out, err
	}
	signed, err := c.credential.SignXML("nfe_event", raw, time.Now())
	if err != nil {
		return out, err
	}
	out = CancellationEvidence{State: "not_sent", Key: in.Key, EventID: "ID110111" + in.Key + "01", SignedXML: signed}
	result, err := c.SubmitCancellation(ctx, operationID, lot, signed)
	out.Exchange = result
	if result.OperationID != "" {
		out.State = result.State
	}
	if errors.Is(err, ErrRecorded) {
		recovered, _ := recoverCancellation(ctx, r, environment)
		if recovered.Key != "" {
			return recovered, err
		}
		return CancellationEvidence{State: "recovery_required"}, err
	}
	if err != nil {
		return out, err
	}
	out, err = interpretCancellationEnvironment(signed, lot, result, environment)
	if err != nil {
		return out, ErrUncertain
	}
	return out, nil
}

func RecoverHomologationCancellation(ctx context.Context, r CancellationRecorder) (CancellationEvidence, error) {
	return recoverCancellation(ctx, r, 2)
}
func recoverCancellation(ctx context.Context, r CancellationRecorder, environment int) (CancellationEvidence, error) {
	req, result, err := r.Read(ctx)
	if err != nil {
		return CancellationEvidence{}, err
	}
	original, err := originalCancellationFromRequest(req, environment)
	if err != nil || req.OperationID != result.OperationID {
		return CancellationEvidence{}, ErrUncertain
	}
	return interpretCancellationEnvironment(original.SignedXML, original.Lot, result, environment)
}

type ArchivedCancellation struct {
	Input     Cancellation
	Lot       string
	SignedXML []byte
}

// OriginalCancellationFromRequest accepts only the exact archived SOAP batch,
// not an arbitrary uploaded event. Archived signature integrity is not new trust.
func OriginalCancellationFromRequest(req RecordedRequest) (ArchivedCancellation, error) {
	return originalCancellationFromRequest(req, 2)
}
func originalCancellationFromRequest(req RecordedRequest, environment int) (ArchivedCancellation, error) {
	var out ArchivedCancellation
	sum := sha256.Sum256(req.Body)
	if req.OperationID == "" || len(req.OperationID) > 128 || req.Scope == "" || req.URL == "" || (environment != 1 && environment != 2) || req.Environment != strconv.Itoa(environment) || !ValidAccessKey(req.Key) || req.Issuer != req.Key[6:20] || req.RequestSHA256 != hex.EncodeToString(sum[:]) {
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
	if msg.Tag != "nfeDadosMsg" || msg.NamespaceURI() != cancellationNS || len(msg.ChildElements()) != 1 {
		return out, ErrUncertain
	}
	batch := msg.ChildElements()[0]
	if batch.Tag != "envEvento" || batch.NamespaceURI() != NFeNS || batch.SelectAttrValue("versao", "") != "1.00" || len(batch.ChildElements()) != 2 {
		return out, ErrUncertain
	}
	part := etree.NewDocument()
	part.SetRoot(batch.Copy())
	raw, _ := part.WriteToBytes()
	if ValidateXML("nfe_event_batch", raw) != nil {
		return out, ErrSchema
	}
	out.Lot = batch.SelectElement("idLote").Text()
	event := batch.SelectElement("evento")
	if event == nil {
		return out, ErrUncertain
	}
	part = etree.NewDocument()
	part.SetRoot(event.Copy())
	out.SignedXML, _ = part.WriteToBytes()
	out.Input, err = inspectArchivedCancellationEnvironment(out.SignedXML, environment)
	if err != nil {
		return ArchivedCancellation{}, err
	}
	if out.Input.Key != req.Key || out.Input.CNPJ != req.Issuer {
		return ArchivedCancellation{}, ErrUncertain
	}
	return out, nil
}

func inspectArchivedCancellation(raw []byte) (Cancellation, error) {
	return inspectArchivedCancellationEnvironment(raw, 2)
}
func inspectArchivedCancellationEnvironment(raw []byte, environment int) (Cancellation, error) {
	var out Cancellation
	if err := ValidateXML("nfe_event", raw); err != nil {
		return out, err
	}
	d, err := parseXML(raw)
	if err != nil {
		return out, err
	}
	info, err := payload(d.Root(), "nfe_event")
	if err != nil {
		return out, err
	}
	issuer := info.SelectElement("CNPJ")
	if issuer == nil {
		return out, ErrCredential
	}
	certNode := d.Root().FindElement("Signature/KeyInfo/X509Data/X509Certificate")
	if certNode == nil {
		return out, ErrXML
	}
	der, err := base64.StdEncoding.DecodeString(certNode.Text())
	if err != nil {
		return out, ErrXML
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil || certificateCNPJ(cert) != issuer.Text() {
		return out, ErrCredential
	}
	if _, ok := cert.PublicKey.(*rsa.PublicKey); !ok {
		return out, ErrCredential
	}
	credential := &Credential{certificate: cert, document: issuer.Text()}
	if err = credential.verifySignedXML("nfe_event", raw); err != nil {
		return out, err
	}
	get := func(name string) string {
		e := info.FindElement(name)
		if e == nil {
			return ""
		}
		return e.Text()
	}
	when, err := time.Parse(time.RFC3339, get("dhEvento"))
	if err != nil {
		return out, ErrXML
	}
	if (environment != 1 && environment != 2) || get("tpAmb") != strconv.Itoa(environment) {
		return out, ErrUnsupported
	}
	return Cancellation{Key: get("chNFe"), CNPJ: issuer.Text(), Environment: environment, Time: when, Protocol: get("detEvento/nProt"), Reason: get("detEvento/xJust")}, nil
}

func interpretCancellation(signed []byte, lot string, result ExchangeResult) (CancellationEvidence, error) {
	return interpretCancellationEnvironment(signed, lot, result, 2)
}
func interpretCancellationEnvironment(signed []byte, lot string, result ExchangeResult, environment int) (CancellationEvidence, error) {
	out := CancellationEvidence{State: "uncertain", SignedXML: append([]byte(nil), signed...), Exchange: result}
	original, err := inspectArchivedCancellationEnvironment(signed, environment)
	if err != nil {
		return out, err
	}
	out.Key = original.Key
	out.EventID = "ID110111" + original.Key + "01"
	sum := sha256.Sum256(result.Body)
	if result.OperationID == "" || result.State != "response_received" || result.HTTPStatus != 200 || len(result.Body) > MaxXML || result.SHA256 != hex.EncodeToString(sum[:]) {
		return out, ErrUncertain
	}
	return interpretCancellationResponse(out, original, lot, result.Body)
}

func interpretCancellationResponse(out CancellationEvidence, original Cancellation, lot string, raw []byte) (CancellationEvidence, error) {
	root, err := eventResponseRoot(raw)
	if err != nil {
		return out, err
	}
	d := etree.NewDocument()
	d.SetRoot(root.Copy())
	part, _ := d.WriteToBytes()
	if err = ValidateXML("nfe_event_response", part); err != nil {
		return out, err
	}
	get := func(e *etree.Element, name string) string {
		v := e.SelectElement(name)
		if v == nil {
			return ""
		}
		return v.Text()
	}
	if get(root, "idLote") != lot || get(root, "tpAmb") != strconv.Itoa(original.Environment) || get(root, "cOrgao") != original.Key[:2] {
		return out, ErrUncertain
	}
	out.State = "response_requires_review"
	if get(root, "cStat") != "128" {
		out.Status = get(root, "cStat")
		out.Reason = get(root, "xMotivo")
		return out, nil
	}
	events := root.SelectElements("retEvento")
	if len(events) != 1 {
		return out, ErrUncertain
	}
	event := events[0]
	info := event.SelectElement("infEvento")
	if info == nil || get(info, "tpAmb") != strconv.Itoa(original.Environment) || get(info, "cOrgao") != original.Key[:2] || get(info, "chNFe") != original.Key || get(info, "tpEvento") != "110111" || get(info, "nSeqEvento") != "1" {
		return out, ErrUncertain
	}
	out.Status = get(info, "cStat")
	out.Reason = get(info, "xMotivo")
	if out.Status != "135" && out.Status != "155" {
		return out, nil
	}
	protocol := get(info, "nProt")
	when, err := time.Parse(time.RFC3339, get(info, "dhRegEvento"))
	if err != nil || when.Before(original.Time) || (len(protocol) != 15 && len(protocol) != 17) || !digitsRE.MatchString(protocol) || (info.SelectAttrValue("Id", "") != "" && info.SelectAttrValue("Id", "") != "ID"+protocol) {
		return out, ErrUncertain
	}
	process := etree.NewDocument()
	p := process.CreateElement("procEventoNFe")
	p.CreateAttr("xmlns", NFeNS)
	p.CreateAttr("versao", "1.00")
	originalDoc, _ := parseXML(out.SignedXML)
	p.AddChild(originalDoc.Root().Copy())
	p.AddChild(event.Copy())
	processed, err := process.WriteToBytes()
	if err != nil {
		return out, ErrXML
	}
	if err = ValidateXML("nfe_event_process", processed); err != nil {
		return out, err
	}
	out.State = "cancellation_indicated_homologation"
	if original.Environment == 1 {
		out.State = "cancellation_indicated_production"
	}
	out.Protocol = protocol
	out.RegisteredAt = get(info, "dhRegEvento")
	out.ProcessedXML = processed
	return out, nil
}

func eventResponseRoot(raw []byte) (*etree.Element, error) {
	// Use the same strict SOAP shape guard with an explicit version parameter.
	return versionedFiscalResponseRoot(raw, cancellationNS, "retEnvEvento", "1.00")
}
