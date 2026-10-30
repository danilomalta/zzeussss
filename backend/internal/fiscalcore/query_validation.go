package fiscalcore

import (
	"errors"
	"github.com/beevik/etree"
)

// The archived query layout embeds an old numeric-only event Id restriction.
// Keep every official file unchanged: validate its outer response separately
// and each cancellation process with the newer official event schema already
// embedded by the cancellation component. Unknown event formats fail closed.
func validateNFeQueryResponse(raw []byte) error {
	err := validateEmbeddedXML("nfe_query_response", raw)
	if err == nil || !errors.Is(err, ErrSchema) {
		return err
	}
	d, e := parseXML(raw)
	if e != nil {
		return e
	}
	root := d.Root()
	if root.Tag != "retConsSitNFe" || root.NamespaceURI() != NFeNS {
		return ErrSchema
	}
	processes := root.SelectElements("procEventoNFe")
	if len(processes) == 0 || len(processes) > 50 {
		return ErrSchema
	}
	for _, process := range processes {
		if process.NamespaceURI() != NFeNS {
			return ErrSchema
		}
		doc := etree.NewDocument()
		copy := process.Copy()
		copy.CreateAttr("xmlns", NFeNS)
		doc.SetRoot(copy)
		part, e := doc.WriteToBytes()
		if e != nil {
			return ErrXML
		}
		if e = validateEmbeddedXML("nfe_event_process", part); e != nil {
			return e
		}
		root.RemoveChild(process)
	}
	trimmed, e := d.WriteToBytes()
	if e != nil {
		return ErrXML
	}
	return validateEmbeddedXML("nfe_query_response", trimmed)
}
