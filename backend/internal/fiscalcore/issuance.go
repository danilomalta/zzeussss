package fiscalcore

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/beevik/etree"
	"strings"
	"time"
)

// InvoiceRecorder is supplied by the trusted host, never by a renderer. Read
// rechecks current recovery permissions; ValidateInvoice checks frozen sale
// economics and the original send authority. No production route is enabled.
type InvoiceRecorder interface {
	Recorder
	SendGuard
	ValidateInvoice(context.Context, Invoice) error
	Read(context.Context) (RecordedRequest, ExchangeResult, error)
}

type IssuanceEvidence struct {
	State     string         `json:"state"`
	Key       string         `json:"key"`
	SignedXML []byte         `json:"signed_xml"`
	Exchange  ExchangeResult `json:"exchange"`
	Protocol  *Protocol      `json:"protocol,omitempty"`
}

// IssueHomologationNFe is a one-attempt orchestration: frozen sale binding,
// build, official XSD/signature checks, durable reserve, send guard and mTLS.
// A number must already have been selected by the trusted installation. This
// method neither allocates it nor selects/approves taxes. No automatic retries.
func (c *Client) IssueHomologationNFe(ctx context.Context, operationID, lot string, in Invoice) (IssuanceEvidence, error) {
	var out IssuanceEvidence
	r, ok := c.recorder.(InvoiceRecorder)
	if !ok || in.Environment != 2 || in.Key.Emission != 1 {
		return out, ErrUnsupported
	}
	if err := r.ValidateInvoice(ctx, in); err != nil {
		return out, err
	}
	raw, key, err := BuildInvoice(in)
	if err != nil {
		return out, err
	}
	signed, err := c.credential.SignXML("nfe", raw, time.Now())
	if err != nil {
		return out, err
	}
	out = IssuanceEvidence{State: "not_sent", Key: key, SignedXML: signed}
	exchange, err := c.SubmitNFe(ctx, operationID, lot, signed)
	out.Exchange = exchange
	if exchange.OperationID != "" {
		out.State = exchange.State
	}
	if errors.Is(err, ErrRecorded) {
		// A rebuilt candidate is not the original transmitted document. Return only
		// the existing immutable evidence on duplicate, preserving its operation ID.
		recovered, _ := RecoverHomologationNFe(ctx, r)
		if recovered.Key != "" {
			return recovered, err
		}
		return IssuanceEvidence{State: "recovery_required"}, err
	}
	if err != nil {
		return out, err
	}
	return interpretHomologationNFe(signed, exchange)
}

// RecoverHomologationNFe reads original durable bytes and authenticated result,
// and never sends HTTP, signs again, reserves a number or requires a live key.
// Even an expired signing credential must not make the original evidence vanish.
// Its reader must be the trusted journal; arbitrary uploaded XML is not evidence.
func RecoverHomologationNFe(ctx context.Context, reader InvoiceRecorder) (IssuanceEvidence, error) {
	req, result, err := reader.Read(ctx)
	if err != nil {
		return IssuanceEvidence{}, err
	}
	digest := sha256.Sum256(req.Body)
	if len(req.Body) > 2*MaxXML || req.OperationID != result.OperationID || req.Environment != "2" || !ValidAccessKey(req.Key) || req.Issuer != req.Key[6:20] || req.RequestSHA256 != hex.EncodeToString(digest[:]) {
		return IssuanceEvidence{}, ErrUncertain
	}
	d, err := parseXMLLimits(req.Body, 2*MaxXML, 8200)
	if err != nil {
		return IssuanceEvidence{}, ErrUncertain
	}
	root := d.Root()
	if root.Tag != "Envelope" || root.NamespaceURI() != "http://www.w3.org/2003/05/soap-envelope" || len(root.ChildElements()) != 1 {
		return IssuanceEvidence{}, ErrUncertain
	}
	body := root.ChildElements()[0]
	if body.Tag != "Body" || body.NamespaceURI() != root.NamespaceURI() || len(body.ChildElements()) != 1 {
		return IssuanceEvidence{}, ErrUncertain
	}
	msg := body.ChildElements()[0]
	if msg.Tag != "nfeDadosMsg" || msg.NamespaceURI() != NFeNS+"/wsdl/NFeAutorizacao4" || len(msg.ChildElements()) != 1 {
		return IssuanceEvidence{}, ErrUncertain
	}
	batch := msg.ChildElements()[0]
	children := batch.ChildElements()
	if batch.Tag != "enviNFe" || batch.NamespaceURI() != NFeNS || batch.SelectAttrValue("versao", "") != "4.00" || len(children) != 3 || children[0].Tag != "idLote" || children[0].NamespaceURI() != NFeNS || len(children[0].Text()) < 1 || len(children[0].Text()) > 15 || !digitsRE.MatchString(children[0].Text()) || children[1].NamespaceURI() != NFeNS || children[1].Tag != "indSinc" || children[1].Text() != "1" || children[2].Tag != "NFe" || children[2].NamespaceURI() != NFeNS {
		return IssuanceEvidence{}, ErrUncertain
	}
	signedDoc := etree.NewDocument()
	signedDoc.SetRoot(children[2].Copy())
	signed, err := signedDoc.WriteToBytes()
	if err != nil {
		return IssuanceEvidence{}, ErrXML
	}
	out, err := interpretHomologationNFe(signed, result)
	if out.Key != req.Key {
		return IssuanceEvidence{}, ErrUncertain
	}
	return out, err
}

func interpretHomologationNFe(signed []byte, result ExchangeResult) (IssuanceEvidence, error) {
	out := IssuanceEvidence{State: "uncertain", SignedXML: append([]byte(nil), signed...), Exchange: result}
	if err := ValidateXML("nfe", signed); err != nil {
		return out, err
	}
	d, err := parseXML(signed)
	if err != nil {
		return out, err
	}
	info, err := payload(d.Root(), "nfe")
	if err != nil {
		return out, err
	}
	key := strings.TrimPrefix(info.SelectAttrValue("Id", ""), "NFe")
	out.Key = key
	environment := info.FindElement("ide/tpAmb")
	if !ValidAccessKey(key) || environment == nil || environment.Text() != "2" || invoiceIdentity(info, key) != nil {
		return out, ErrInput
	}
	if result.State != "response_received" || result.HTTPStatus != 200 || len(result.Body) > MaxXML {
		return out, ErrUncertain
	}
	digest := sha256.Sum256(result.Body)
	if result.SHA256 != hex.EncodeToString(digest[:]) {
		return out, ErrUncertain
	}
	// Verify archived signature integrity without demanding the old private key
	// or current certificate validity. Trust comes from the authenticated journal;
	// this does NOT validate ICP-Brasil chain/revocation or grant new authority.
	certNode := d.Root().FindElement("Signature/KeyInfo/X509Data/X509Certificate")
	if certNode == nil {
		return out, ErrXML
	}
	der, err := base64.StdEncoding.DecodeString(certNode.Text())
	if err != nil {
		return out, ErrXML
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil || certificateCNPJ(cert) != key[6:20] {
		return out, ErrCredential
	}
	if _, ok := cert.PublicKey.(*rsa.PublicKey); !ok {
		return out, ErrCredential
	}
	archived := &Credential{certificate: cert, document: key[6:20]}
	if err = archived.verifySignedXML("nfe", signed); err != nil {
		return out, err
	}
	signature := d.Root().FindElement("Signature/SignedInfo/Reference/DigestValue")
	if signature == nil {
		return out, ErrXML
	}
	protocol, err := ParseProtocol(result.Body, key, "2", signature.Text())
	if err != nil {
		out.State = "response_received"
		return out, err
	}
	// This is transport evidence for homologation, never commercial authorization.
	out.State = "protocol_received_homologation"
	out.Protocol = &protocol
	return out, nil
}
