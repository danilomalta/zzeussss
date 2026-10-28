package fiscalcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// ProductionRecorder must bind the reviewed invoice, certificate fingerprint,
// installation endpoint, original live session and durably assigned number.
// A FileRecorder alone is deliberately insufficient to authorize production.
type ProductionRecorder interface {
	InvoiceRecorder
	ValidateProduction(context.Context, Invoice, string, string) error
}

type ProductionEvidence struct {
	QueryOperationID string    `json:"query_operation_id,omitempty"`
	State            string    `json:"state"`
	Key              string    `json:"key"`
	Status           string    `json:"status,omitempty"`
	Reason           string    `json:"reason,omitempty"`
	OperationID      string    `json:"operation_id"`
	Protocol         *Protocol `json:"protocol,omitempty"`
	ProcessedXML     []byte    `json:"processed_xml,omitempty"`
}

func (c *Credential) Fingerprint() string {
	if c == nil || c.certificate == nil {
		return ""
	}
	sum := sha256.Sum256(c.certificate.Raw)
	return hex.EncodeToString(sum[:])
}

// CheckRevocation requires fresh signed CRLs for every non-root certificate.
// Missing, stale, future or revoked evidence fails closed. Roots belong to the
// installation trust policy; a self-signed leaf is not a production identity.
func (c *Credential) CheckRevocation(rawLists [][]byte, now time.Time) error {
	if !c.current(now) || len(c.verifiedChain) < 2 || c.certificate.IsCA || len(rawLists) == 0 || len(rawLists) > 16 {
		return ErrCredential
	}
	if len(c.certificate.ExtKeyUsage) > 0 {
		client := false
		for _, usage := range c.certificate.ExtKeyUsage {
			if usage == x509.ExtKeyUsageAny || usage == x509.ExtKeyUsageClientAuth {
				client = true
			}
		}
		if !client {
			return ErrCredential
		}
	}
	lists := make([]*x509.RevocationList, 0, len(rawLists))
	for _, raw := range rawLists {
		list, err := x509.ParseRevocationList(raw)
		if err != nil {
			return ErrCredential
		}
		for _, extension := range list.Extensions {
			// Delta/indirect/partitioned lists need additional coverage logic;
			// never treat a partial list as a complete revocation answer.
			if extension.Id.String() == "2.5.29.27" || extension.Id.String() == "2.5.29.28" {
				return ErrCredential
			}
			if extension.Critical && extension.Id.String() != "2.5.29.20" && extension.Id.String() != "2.5.29.35" {
				return ErrCredential
			}
		}
		lists = append(lists, list)
	}
	for i, cert := range c.verifiedChain[:len(c.verifiedChain)-1] {
		issuer := c.verifiedChain[i+1]
		var selected *x509.RevocationList
		for _, list := range lists {
			if !bytes.Equal(list.RawIssuer, issuer.RawSubject) || list.CheckSignatureFrom(issuer) != nil {
				continue
			}
			if list.ThisUpdate.After(now) || !list.NextUpdate.After(now) {
				continue
			}
			if selected == nil || list.ThisUpdate.After(selected.ThisUpdate) {
				selected = list
			}
		}
		if selected == nil {
			return ErrCredential
		}
		for _, entry := range selected.RevokedCertificateEntries {
			if entry.SerialNumber.Cmp(cert.SerialNumber) == 0 {
				return ErrCredential
			}
		}
		for _, entry := range selected.RevokedCertificates {
			if entry.SerialNumber.Cmp(cert.SerialNumber) == 0 {
				return ErrCredential
			}
		}
	}
	return nil
}

// IssueProductionNFe performs ONE normal synchronous request for one reviewed
// model55/65 document. Tax selection, configuration and numbering are supplied
// by the authenticated host. Uncertainty never causes a second POST.
func (c *Client) IssueProductionNFe(ctx context.Context, operationID, lot string, in Invoice, qrConfig *QRConfiguration, consultation string) (ProductionEvidence, error) {
	var out ProductionEvidence
	r, ok := c.recorder.(ProductionRecorder)
	if !ok || in.Environment != 1 || in.Key.Emission != 1 {
		return out, ErrUnsupported
	}
	if err := r.ValidateProduction(ctx, in, c.credential.Fingerprint(), c.endpoint.String()); err != nil {
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
	if in.Key.Model == "65" {
		if qrConfig == nil {
			return out, ErrInput
		}
		signed, err = c.credential.CompleteNFCeXML(*qrConfig, consultation, signed, time.Now())
		if err != nil {
			return out, err
		}
	} else if qrConfig != nil || consultation != "" {
		return out, ErrInput
	}
	out = ProductionEvidence{State: "not_sent", Key: key, OperationID: operationID}
	result, err := c.SubmitNFe(ctx, operationID, lot, signed)
	if errors.Is(err, ErrRecorded) {
		saved, e := RecoverProductionNFe(ctx, r)
		if e == nil {
			return saved, err
		}
		return ProductionEvidence{State: "recovery_required"}, err
	}
	if result.OperationID != "" {
		out.State = "uncertain"
	}
	if err != nil {
		return out, err
	}
	proof, parseErr := interpretProductionNFe(signed, result)
	if parseErr != nil {
		return proof, ErrUncertain
	} // A received invalid response is not a pre-send validation failure.
	return proof, nil
}

// RecoverProductionNFe only reads original journal evidence. No signer, live
// certificate, number allocation or POST is required to recover historical data.
func RecoverProductionNFe(ctx context.Context, reader InvoiceRecorder) (ProductionEvidence, error) {
	req, result, err := reader.Read(ctx)
	if err != nil {
		return ProductionEvidence{}, err
	}
	return ReconcileProductionIssuance(req, result)
}
func ReconcileProductionIssuance(req RecordedRequest, result ExchangeResult) (ProductionEvidence, error) {
	if req.OperationID != result.OperationID {
		return ProductionEvidence{}, ErrUncertain
	}
	original, err := originalNFeFromRequest(req, "1")
	if err != nil {
		return ProductionEvidence{}, err
	}
	return interpretProductionNFe(original.SignedXML, result)
}

func OriginalProductionNFeFromRequest(req RecordedRequest) (OriginalNFe, error) {
	return originalNFeFromRequest(req, "1")
}

func MatchProductionInvoice(in Invoice, signed []byte, fingerprint string) error {
	_, archived, err := inspectArchivedNFeEnvironment(signed, "1")
	if err != nil {
		return err
	}
	certNode := archived.Root().FindElement("Signature/KeyInfo/X509Data/X509Certificate")
	if certNode == nil {
		return ErrCredential
	}
	der, err := base64.StdEncoding.DecodeString(certNode.Text())
	if err != nil {
		return ErrCredential
	}
	sum := sha256.Sum256(der)
	if hex.EncodeToString(sum[:]) != fingerprint {
		return ErrCredential
	}
	raw, _, err := BuildInvoice(in)
	if err != nil {
		return err
	}
	d, err := parseXML(raw)
	if err != nil {
		return err
	}
	canonical := dsig.MakeC14N10RecCanonicalizer()
	expected, err := canonical.Canonicalize(d.Root().SelectElement("infNFe"))
	if err != nil {
		return ErrXML
	}
	actual, err := canonical.Canonicalize(archived.Root().SelectElement("infNFe"))
	if err != nil || !bytes.Equal(expected, actual) {
		return ErrInput
	}
	return nil
}

func interpretProductionNFe(signed []byte, result ExchangeResult) (ProductionEvidence, error) {
	out := ProductionEvidence{State: "uncertain", OperationID: result.OperationID}
	original, _, err := inspectArchivedNFeEnvironment(signed, "1")
	if err != nil {
		return out, err
	}
	out.Key = original.Key
	sum := sha256.Sum256(result.Body)
	if result.State != "response_received" || result.HTTPStatus != 200 || len(result.Body) > MaxXML || result.SHA256 != hex.EncodeToString(sum[:]) {
		return out, ErrUncertain
	}
	root, err := fiscalResponseRoot(result.Body, NFeNS+"/wsdl/NFeAutorizacao4", "retEnviNFe")
	if err != nil {
		return out, err
	}
	d := etree.NewDocument()
	d.SetRoot(root.Copy())
	raw, err := d.WriteToBytes()
	if err != nil {
		return out, ErrXML
	}
	if err = ValidateXML("nfe_authorization_response", raw); err != nil {
		return out, err
	}
	get := func(e *etree.Element, name string) string { return printXMLText(e, name) }
	if get(root, "tpAmb") != "1" {
		return out, ErrUncertain
	}
	out.Status, out.Reason = get(root, "cStat"), get(root, "xMotivo")
	if out.Status != "104" {
		switch out.Status {
		case "103", "105":
			out.State = "processing_requires_query"
		case "204", "539":
			out.State = "duplicate_requires_query"
		case "215", "225", "226", "242", "243", "249", "252", "404", "452":
			out.State = "batch_rejected_requires_review"
		default:
			out.State = "service_response_requires_review"
		}
		return out, nil
	}
	proto := root.SelectElement("protNFe")
	if proto == nil {
		return out, ErrUncertain
	}
	info := proto.SelectElement("infProt")
	if info == nil || get(info, "tpAmb") != "1" || get(info, "chNFe") != original.Key {
		return out, ErrUncertain
	}
	out.Status, out.Reason = get(info, "cStat"), get(info, "xMotivo")
	switch out.Status {
	case "100", "150":
		protocol, e := ParseProtocol(raw, original.Key, "1", original.Digest)
		if e != nil {
			return out, e
		}
		processed, e := assembleNFeProcess(signed, proto)
		if e != nil {
			return out, e
		}
		out.State, out.Protocol, out.ProcessedXML = "authorized", &protocol, processed
	case "204", "539":
		out.State = "duplicate_requires_query"
	case "110", "205", "301", "302":
		out.State = "denied_requires_review"
	case "101", "151", "155", "206":
		out.State = "document_closed_requires_review"
	case "215", "225", "228", "229", "230", "531", "532", "533", "534", "564", "590", "591", "629", "630", "694", "695", "696", "806":
		out.State = "rejected_requires_review"
	default:
		out.State = "status_requires_review"
	}
	return out, nil
}

func (c *Client) ConsultProductionNFe(ctx context.Context, id, key string) (ExchangeResult, error) {
	if !c.credential.current(time.Now()) || !ValidAccessKey(key) || key[6:20] != c.credential.document {
		return ExchangeResult{}, ErrCredential
	}
	raw, err := BuildNFeConsultation(key, "1")
	if err != nil {
		return ExchangeResult{}, err
	}
	return c.exchange(ctx, id, key, "1", `application/soap+xml; charset=utf-8; action="`+consultationNS+`/nfeConsultaNF"`, raw)
}
