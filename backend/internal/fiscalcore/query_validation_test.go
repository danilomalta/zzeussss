package fiscalcore

import (
	"github.com/beevik/etree"
	"testing"
)

func TestFiscalQueryComposedEventSchemaPreservesStrictEnvelope(t *testing.T) {
	c := testCredential(t)
	invoice := queryOriginal(t, c)
	req, result := cancellationRecord(t, c)
	event, err := OriginalCancellationFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := interpretCancellation(event.SignedXML, event.Lot, result)
	if err != nil {
		t.Fatal(err)
	}
	base := etree.NewDocument()
	base.ReadFromBytes(queryResponse(t, invoice, "101"))
	process := etree.NewDocument()
	process.ReadFromBytes(proof.ProcessedXML)
	base.Root().AddChild(process.Root().Copy())
	raw, _ := base.WriteToBytes()
	if err = validateEmbeddedXML("nfe_query_response", raw); err == nil {
		t.Fatal("fixture does not exercise legacy event ID constraint")
	}
	if err = ValidateXML("nfe_query_response", raw); err != nil {
		t.Fatal("new event schema rejected", err)
	}
	for _, mode := range []string{"outer-extra", "event-extra", "wrong-namespace", "too-many", "unsigned"} {
		t.Run(mode, func(t *testing.T) {
			d := base.Copy()
			p := d.Root().SelectElement("procEventoNFe")
			switch mode {
			case "outer-extra":
				d.Root().CreateElement("unknown")
			case "event-extra":
				p.CreateElement("unknown")
			case "wrong-namespace":
				p.CreateAttr("xmlns", "https://invalid.example")
			case "too-many":
				for i := 0; i < 50; i++ {
					d.Root().AddChild(p.Copy())
				}
			case "unsigned":
				e := p.SelectElement("evento")
				e.RemoveChild(e.SelectElement("Signature"))
			}
			invalid, _ := d.WriteToBytes()
			if ValidateXML("nfe_query_response", invalid) == nil {
				t.Fatal("invalid composite accepted", mode)
			}
		})
	}
}
