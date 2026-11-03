package fiscalproduction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/beevik/etree"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

func checkoutPrintXMLFixture(t *testing.T) ([]byte, []byte, string) {
	t.Helper()
	cnpj := "12ABC34501DE35"
	cred, e := fiscalcert.New(cnpj, "synthetic.gov.br")
	if e != nil {
		t.Fatal(e)
	}
	address := fiscalcore.Address{Street: "Rua Teste", Number: "1", District: "Centro", CityCode: "3550308", City: "Sao Paulo", UF: "SP", PostalCode: "01001000"}
	totals := map[string]string{}
	for _, name := range strings.Fields("vBC vICMS vICMSDeson vFCP vBCST vST vFCPST vFCPSTRet vII vIPI vIPIDevol vPIS vCOFINS") {
		totals[name] = "0.00"
	}
	in := fiscalcore.Invoice{Key: fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: cnpj, Model: "65", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"}, Environment: 2, CRT: "1", Nature: "VENDA", Direction: "1", Destination: "1", FinalConsumer: "1", Presence: "1", Issuer: fiscalcore.Party{CNPJ: cnpj, Name: "Synthetic issuer", IE: "123456789", Address: address}, Recipient: &fiscalcore.Party{CNPJ: "11222333000181", Name: "Synthetic buyer", IEDestination: "9", Address: address}, Lines: []fiscalcore.InvoiceLine{{ProductID: "one", Description: "Synthetic private XML item", Unit: "UN", Quantity: "1", UnitPrice: "10", Discount: "0", Freight: "0", Insurance: "0", Other: "0", NCM: "09012100", CFOP: "5102", Taxes: []fiscalcore.TaxNode{{Name: "ICMS", Children: []fiscalcore.TaxNode{{Name: "ICMSSN102", Children: []fiscalcore.TaxNode{{Name: "orig", Value: "0"}, {Name: "CSOSN", Value: "102"}}}}}, {Name: "PIS", Children: []fiscalcore.TaxNode{{Name: "PISNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}, {Name: "COFINS", Children: []fiscalcore.TaxNode{{Name: "COFINSNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}}}}, Payments: []fiscalcore.InvoicePayment{{Method: "01", Amount: "10.00"}}, TaxTotals: totals}
	raw, key, e := fiscalcore.BuildInvoice(in)
	if e != nil {
		t.Fatal(e)
	}
	signed, e := cred.Credential.SignXML("nfe", raw, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	signed, e = cred.Credential.CompleteNFCeXML(fiscalcore.QRConfiguration{BaseURL: "https://synthetic.gov.br/qr", Version: 3}, "https://synthetic.gov.br/consult", signed, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	doc := etree.NewDocument()
	if doc.ReadFromBytes(signed) != nil {
		t.Fatal("signed fixture")
	}
	digest := doc.Root().FindElement("Signature/SignedInfo/Reference/DigestValue").Text()
	// The test authority supplies the existing schema's processed envelope.
	invoice := []byte(fmt.Sprintf(`<nfeProc xmlns="%s" versao="4.00">%s<protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><chNFe>%s</chNFe><dhRecbto>%s</dhRecbto><nProt>135260000000001</nProt><digVal>%s</digVal><cStat>100</cStat><xMotivo>Synthetic only</xMotivo></infProt></protNFe></nfeProc>`, fiscalcore.NFeNS, signed, key, time.Now().Format(time.RFC3339), digest))
	cancel, e := fiscalcore.BuildCancellation(fiscalcore.Cancellation{Key: key, CNPJ: cnpj, Environment: 2, Time: time.Now(), Protocol: "135260000000001", Reason: "Synthetic cancellation only"})
	if e != nil {
		t.Fatal(e)
	}
	cancel, e = cred.Credential.SignXML("nfe_event", cancel, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	event := []byte(fmt.Sprintf(`<procEventoNFe xmlns="%s" versao="1.00">%s<retEvento versao="1.00"><infEvento><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cOrgao>35</cOrgao><cStat>135</cStat><xMotivo>Synthetic only</xMotivo><chNFe>%s</chNFe><tpEvento>110111</tpEvento><xEvento>Cancelamento</xEvento><nSeqEvento>1</nSeqEvento><dhRegEvento>%s</dhRegEvento><nProt>135260000000002</nProt></infEvento></retEvento></procEventoNFe>`, fiscalcore.NFeNS, cancel, key, time.Now().Format(time.RFC3339)))
	if e = validateFocusArchiveXML(invoice, key, "invoice", 0); e != nil {
		t.Fatal("invoice fixture", e)
	}
	if e = validateFocusArchiveXML(event, key, "cancellation", 1); e != nil {
		t.Fatal("event fixture", e)
	}
	return invoice, event, key
}

func TestCheckoutEncryptedNFCePrintArchiveIsolationIntegrityAndNoNetwork(t *testing.T) {
	data, _, key := checkoutPrintXMLFixture(t)
	a := focusContext(t)
	file := filepath.Join(t.TempDir(), "archive.key")
	if backup.InitKey(file) != nil {
		t.Fatal("key")
	}
	crypt, err := archiveCrypt(file)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	record := focusArchiveRecord{Version: 1, Sale: "sale", Intent: "intent", Key: key, Kind: "invoice", SHA256: hex.EncodeToString(sum[:])}
	nonce := make([]byte, crypt.NonceSize())
	record.Ciphertext = crypt.Seal(nonce, nonce, data, archiveAAD(a, record))
	opts := fiscalcore.PrintOptions{WidthMM: 80, Timezone: "America/Sao_Paulo"}
	out, err := renderCheckoutArchive(a, record, file, opts)
	if err != nil || out.Key != key || !strings.Contains(string(out.HTML), "SEM VALOR FISCAL") || !strings.Contains(string(out.HTML), "data:image/png;base64,") {
		t.Fatal("archive print", err)
	}
	foreign := a
	foreign.Actor.TenantID = "other"
	if _, err = renderCheckoutArchive(foreign, record, file, opts); err == nil {
		t.Fatal("cross-tenant decryption")
	}
	bad := record
	bad.SHA256 = strings.Repeat("0", 64)
	if _, err = renderCheckoutArchive(a, bad, file, opts); err == nil {
		t.Fatal("corrupt digest accepted")
	}
	bad = record
	bad.Ciphertext = append([]byte(nil), record.Ciphertext...)
	bad.Ciphertext[len(bad.Ciphertext)-1] ^= 1
	if _, err = renderCheckoutArchive(a, bad, file, opts); err == nil {
		t.Fatal("corrupt ciphertext accepted")
	}
	if _, err = renderCheckoutArchive(a, record, file, fiscalcore.PrintOptions{WidthMM: 210, Timezone: "UTC"}); err == nil {
		t.Fatal("A4 NFC-e accepted")
	}
	raw, _ := json.Marshal(record)
	if strings.Contains(string(raw), "nfeProc") {
		t.Fatal("XML plaintext persisted")
	}
	var registry *ProviderRegistry
	a.Token = "revoked"
	if _, err = registry.CheckoutNFCeDANFE(context.Background(), a, "sale", opts); err == nil {
		t.Fatal("unauthenticated print")
	}
}
