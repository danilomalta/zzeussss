package fiscalcore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"github.com/beevik/etree"
)

// ProductionCancellationRecorder is supplied by the authenticated trusted host.
// A generic file journal cannot authorize cancellation of a production invoice.
type ProductionCancellationRecorder interface {
	CancellationRecorder
	ValidateProductionCancellation(context.Context, Cancellation, string, string) error
}

func (c *Client) CancelProductionNFe(ctx context.Context, operation, lot string, in Cancellation) (CancellationEvidence, error) {
	r, ok := c.recorder.(ProductionCancellationRecorder)
	if !ok || in.Environment != 1 {
		return CancellationEvidence{}, ErrUnsupported
	}
	if err := r.ValidateProductionCancellation(ctx, in, c.credential.Fingerprint(), c.endpoint.String()); err != nil {
		return CancellationEvidence{}, err
	}
	return c.cancelNFe(ctx, operation, lot, in, 1)
}
func OriginalProductionCancellation(req RecordedRequest) (ArchivedCancellation, error) {
	return originalCancellationFromRequest(req, 1)
}
func ReconcileProductionCancellation(req RecordedRequest, result ExchangeResult) (CancellationEvidence, error) {
	in, err := OriginalProductionCancellation(req)
	if err != nil || req.OperationID != result.OperationID {
		return CancellationEvidence{}, ErrUncertain
	}
	return interpretCancellationEnvironment(in.SignedXML, in.Lot, result, 1)
}
func MatchProductionCancellation(in Cancellation, signed []byte, fingerprint string) error {
	original, err := inspectArchivedCancellationEnvironment(signed, 1)
	if err != nil || original.Key != in.Key || original.CNPJ != in.CNPJ || original.Environment != in.Environment || original.Protocol != in.Protocol || original.Reason != in.Reason || !original.Time.Equal(in.Time) {
		return ErrInput
	}
	d, err := parseXML(signed)
	if err != nil {
		return err
	}
	node := d.Root().FindElement("Signature/KeyInfo/X509Data/X509Certificate")
	if node == nil {
		return ErrCredential
	}
	der, err := base64.StdEncoding.DecodeString(node.Text())
	if err != nil {
		return ErrCredential
	}
	sum := sha256.Sum256(der)
	if hex.EncodeToString(sum[:]) != fingerprint {
		return ErrCredential
	}
	return nil
}

// ReconcileProductionCancellationQuery accepts only a schema-checked archived
// consultation carrying the exact originally signed event and its registration.
// A document-level canceled status alone does not reconstruct an event receipt.
func ReconcileProductionCancellationQuery(event, invoice, query RecordedRequest, result ExchangeResult) (CancellationEvidence, error) {
	original, err := OriginalProductionCancellation(event)
	if err != nil {
		return CancellationEvidence{}, err
	}
	proof, err := ReconcileProductionNFe(invoice, query, result)
	if err != nil || original.Input.Key != proof.Key || proof.Disposition != "canceled_indicated_production" {
		return CancellationEvidence{}, ErrUncertain
	}
	root, err := fiscalResponseRoot(result.Body, consultationNS, "retConsSitNFe")
	if err != nil {
		return CancellationEvidence{}, err
	}
	sent, err := parseXML(original.SignedXML)
	if err != nil {
		return CancellationEvidence{}, err
	}
	var matched *etree.Element
	for _, process := range root.SelectElements("procEventoNFe") {
		document := process.SelectElement("evento")
		registration := process.SelectElement("retEvento")
		if document == nil || registration == nil {
			continue
		}
		candidate := etree.NewDocument()
		candidate.SetRoot(document.Copy())
		raw, _ := candidate.WriteToBytes()
		input, e := inspectArchivedCancellationEnvironment(raw, 1)
		if e != nil || input.Key != original.Input.Key || input.Protocol != original.Input.Protocol || input.Reason != original.Input.Reason || !input.Time.Equal(original.Input.Time) {
			continue
		}
		// Match the signature/certificate/digest too, not just an event ID shared by
		// every cancellation110111/sequence1 for the same invoice.
		equal := true
		for _, path := range []string{"Signature/SignatureValue", "Signature/KeyInfo/X509Data/X509Certificate", "Signature/SignedInfo/Reference/DigestValue"} {
			a, b := sent.Root().FindElement(path), document.FindElement(path)
			if a == nil || b == nil || a.Text() != b.Text() {
				equal = false
				break
			}
		}
		if !equal {
			continue
		}
		if matched != nil {
			return CancellationEvidence{}, ErrUncertain
		}
		matched = registration
	}
	if matched == nil {
		return CancellationEvidence{}, ErrUncertain
	}
	batch := etree.NewDocument()
	ret := batch.CreateElement("retEnvEvento")
	ret.CreateAttr("xmlns", NFeNS)
	ret.CreateAttr("versao", "1.00")
	for _, v := range [][2]string{{"idLote", original.Lot}, {"tpAmb", "1"}, {"verAplic", "query-archive"}, {"cOrgao", original.Input.Key[:2]}, {"cStat", "128"}, {"xMotivo", "Registration recovered from exact archived consultation"}} {
		field(ret, v[0], v[1])
	}
	ret.AddChild(matched.Copy())
	raw, _ := batch.WriteToBytes()
	out := CancellationEvidence{State: "uncertain", Key: event.Key, EventID: "ID110111" + event.Key + "01", SignedXML: original.SignedXML}
	recovered, err := interpretCancellationResponse(out, original.Input, original.Lot, raw)
	if err != nil || recovered.State != "cancellation_indicated_production" {
		return recovered, ErrUncertain
	}
	recovered.RecoveryQueryOperationID = query.OperationID
	return recovered, nil
}
