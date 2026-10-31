package fiscalcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/beevik/etree"
)

const consultationNS = NFeNS + "/wsdl/NFeConsultaProtocolo4"

// BuildNFeConsultation builds ONLY consSitNFe. It never carries an NFe, event,
// signature or fresh number. Endpoints remain controlled by the trusted host.
func BuildNFeConsultation(key, environment string) ([]byte, error) {
	if !ValidAccessKey(key) || (environment != "2" && environment != "1") {
		return nil, ErrInput
	}
	d := etree.NewDocument()
	root := d.CreateElement("consSitNFe")
	root.CreateAttr("xmlns", NFeNS)
	root.CreateAttr("versao", "4.00")
	field(root, "tpAmb", environment)
	field(root, "xServ", "CONSULTAR")
	field(root, "chNFe", key)
	raw, err := d.WriteToBytes()
	if err != nil {
		return nil, ErrXML
	}
	if err = ValidateXML("nfe_query", raw); err != nil {
		return nil, err
	}
	return soap(consultationNS, root).WriteToBytes()
}

// ConsultNFe is one explicit status lookup, not a retry of original issuance.
// It uses its own durable Recorder, honors its SendGuard and sends no invoice.
func (c *Client) ConsultNFe(ctx context.Context, operationID, key string) (ExchangeResult, error) {
	if !c.credential.current(time.Now()) || !ValidAccessKey(key) || key[6:20] != c.credential.document {
		return ExchangeResult{}, ErrCredential
	}
	raw, err := BuildNFeConsultation(key, "2")
	if err != nil {
		return ExchangeResult{}, err
	}
	return c.exchange(ctx, operationID, key, "2", `application/soap+xml; charset=utf-8; action="`+consultationNS+`/nfeConsultaNF"`, raw)
}

type ConsultationEvidence struct {
	ParserVersion       string    `json:"parser_version"`
	OriginalOperationID string    `json:"original_operation_id"`
	QueryOperationID    string    `json:"query_operation_id"`
	Key                 string    `json:"key"`
	Environment         string    `json:"environment"`
	Status              string    `json:"status"`
	Reason              string    `json:"reason"`
	ReceivedAt          string    `json:"received_at"`
	Disposition         string    `json:"disposition"`
	Protocol            *Protocol `json:"protocol,omitempty"`
	ProcessedXML        []byte    `json:"processed_xml,omitempty"`
}

// ReconcileHomologationNFe interprets records retrieved from the trusted
// emission/query journals, not XML uploaded by a renderer. It never modifies
// either record, releases numbers, reemits, cancels money or adjusts stock.
func ReconcileHomologationNFe(originalRequest, queryRequest RecordedRequest, result ExchangeResult) (ConsultationEvidence, error) {
	return reconcileNFe(originalRequest, queryRequest, result, "2")
}

func ReconcileProductionNFe(originalRequest, queryRequest RecordedRequest, result ExchangeResult) (ConsultationEvidence, error) {
	return reconcileNFe(originalRequest, queryRequest, result, "1")
}

func reconcileNFe(originalRequest, queryRequest RecordedRequest, result ExchangeResult, environment string) (ConsultationEvidence, error) {
	out := ConsultationEvidence{ParserVersion: "nfe-query-v1", OriginalOperationID: originalRequest.OperationID, QueryOperationID: queryRequest.OperationID, Disposition: "uncertain"}
	original, err := originalNFeFromRequest(originalRequest, environment)
	if err != nil {
		return out, err
	}
	out.Key = original.Key
	out.Environment = original.Environment
	expected, err := BuildNFeConsultation(original.Key, environment)
	if err != nil {
		return out, err
	}
	hash := sha256.Sum256(queryRequest.Body)
	if queryRequest.OperationID == "" || queryRequest.OperationID == originalRequest.OperationID || queryRequest.OperationID != result.OperationID || queryRequest.Key != original.Key || queryRequest.Issuer != original.Issuer || queryRequest.Environment != environment || !bytes.Equal(queryRequest.Body, expected) || queryRequest.RequestSHA256 != hex.EncodeToString(hash[:]) {
		return out, ErrUncertain
	}
	if result.State != "response_received" || result.HTTPStatus != 200 || len(result.Body) > MaxXML {
		return out, ErrUncertain
	}
	hash = sha256.Sum256(result.Body)
	if result.SHA256 != hex.EncodeToString(hash[:]) {
		return out, ErrUncertain
	}
	root, err := fiscalResponseRoot(result.Body, consultationNS, "retConsSitNFe")
	if err != nil {
		return out, err
	}
	doc := etree.NewDocument()
	doc.SetRoot(root.Copy())
	raw, err := doc.WriteToBytes()
	if err != nil {
		return out, ErrXML
	}
	if err = ValidateXML("nfe_query_response", raw); err != nil {
		return out, err
	}
	get := func(name string) string {
		e := root.SelectElement(name)
		if e == nil {
			return ""
		}
		return e.Text()
	}
	if get("tpAmb") != environment || get("chNFe") != original.Key {
		return out, ErrUncertain
	}
	// Event/cancellation children must not smuggle another document/environment.
	for _, e := range root.FindElements(".//chNFe") {
		if e.NamespaceURI() != NFeNS || e.Text() != original.Key {
			return out, ErrUncertain
		}
	}
	for _, e := range root.FindElements(".//tpAmb") {
		if e.NamespaceURI() != NFeNS || e.Text() != environment {
			return out, ErrUncertain
		}
	}
	out.Status = get("cStat")
	out.Reason = get("xMotivo")
	out.ReceivedAt = get("dhRecbto")
	if _, err = time.Parse(time.RFC3339, out.ReceivedAt); err != nil {
		return out, ErrUncertain
	}
	switch out.Status {
	case "100", "150":
		if root.SelectElement("retCancNFe") != nil || len(root.SelectElements("procEventoNFe")) > 0 {
			out.Disposition = "event_review_required"
			return out, nil
		}
		proto := root.SelectElement("protNFe")
		if proto == nil {
			return out, ErrUncertain
		}
		// Reuse the exact key/environment/digest guard of the authorization parser.
		auth := etree.NewDocument()
		ret := auth.CreateElement("retEnviNFe")
		ret.CreateAttr("xmlns", NFeNS)
		ret.CreateAttr("versao", "4.00")
		field(ret, "tpAmb", environment)
		field(ret, "cStat", "104")
		ret.AddChild(proto.Copy())
		authRaw, _ := auth.WriteToBytes()
		protocol, err := ParseProtocol(authRaw, original.Key, environment, original.Digest)
		if err != nil {
			return out, err
		}
		if out.Status == "150" && protocol.Status != "150" {
			return out, ErrUncertain
		}
		processed, err := assembleNFeProcess(original.SignedXML, proto)
		if err != nil {
			return out, err
		}
		out.Disposition = "authorized_indicated_homologation"
		if environment == "1" {
			out.Disposition = "authorized_indicated_production"
		}
		out.Protocol = &protocol
		out.ProcessedXML = processed
	case "101":
		out.Disposition = "canceled_indicated_homologation"
		if environment == "1" {
			out.Disposition = "canceled_indicated_production"
		}
	case "110", "301", "302":
		out.Disposition = "denied_indicated_homologation"
		if environment == "1" {
			out.Disposition = "denied_indicated_production"
		}
	case "217":
		out.Disposition = "not_found_indicated"
	default:
		out.Disposition = "status_requires_review"
	}
	return out, nil
}

// Only one exact SOAP body/result shape is accepted; Fault/detail is not data.
func fiscalResponseRoot(raw []byte, namespace, rootName string) (*etree.Element, error) {
	return versionedFiscalResponseRoot(raw, namespace, rootName, "4.00")
}

func versionedFiscalResponseRoot(raw []byte, namespace, rootName, version string) (*etree.Element, error) {
	d, err := parseXML(raw)
	if err != nil {
		return nil, ErrUncertain
	}
	root := d.Root()
	if root.Tag == "Envelope" && root.NamespaceURI() == "http://www.w3.org/2003/05/soap-envelope" {
		children := root.ChildElements()
		if len(children) != 1 || children[0].Tag != "Body" || children[0].NamespaceURI() != root.NamespaceURI() {
			return nil, ErrUncertain
		}
		children = children[0].ChildElements()
		if len(children) != 1 || children[0].Tag != "nfeResultMsg" || children[0].NamespaceURI() != namespace {
			return nil, ErrUncertain
		}
		children = children[0].ChildElements()
		if len(children) != 1 {
			return nil, ErrUncertain
		}
		root = children[0]
	}
	if root.Tag != rootName || root.NamespaceURI() != NFeNS || root.SelectAttrValue("versao", "") != version {
		return nil, ErrUncertain
	}
	return root, nil
}

func assembleNFeProcess(signed []byte, protocol *etree.Element) ([]byte, error) {
	if protocol == nil {
		return nil, ErrUncertain
	}
	invoice, err := parseXML(signed)
	if err != nil {
		return nil, err
	}
	d := etree.NewDocument()
	root := d.CreateElement("nfeProc")
	root.CreateAttr("xmlns", NFeNS)
	root.CreateAttr("versao", "4.00")
	root.AddChild(invoice.Root().Copy())
	root.AddChild(protocol.Copy())
	raw, err := d.WriteToBytes()
	if err != nil {
		return nil, ErrXML
	}
	if err = ValidateXML("nfe_process", raw); err != nil {
		return nil, err
	}
	return raw, nil
}
