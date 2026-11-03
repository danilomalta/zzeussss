package fiscalproduction

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

// Synthetic XML built/signed with the existing engine; not Focus acceptance.
func archiveXMLFixture(t *testing.T) ([]byte, []byte, string) {
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
	in := fiscalcore.Invoice{Key: fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: cnpj, Model: "55", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"}, Environment: 2, CRT: "1", Nature: "VENDA", Direction: "1", Destination: "1", FinalConsumer: "1", Presence: "1", Issuer: fiscalcore.Party{CNPJ: cnpj, Name: "Synthetic issuer", IE: "123456789", Address: address}, Recipient: &fiscalcore.Party{CNPJ: "11222333000181", Name: "Synthetic buyer", IEDestination: "9", Address: address}, Lines: []fiscalcore.InvoiceLine{{ProductID: "one", Description: "Synthetic private XML item", Unit: "UN", Quantity: "1", UnitPrice: "10", Discount: "0", Freight: "0", Insurance: "0", Other: "0", NCM: "09012100", CFOP: "5102", Taxes: []fiscalcore.TaxNode{{Name: "ICMS", Children: []fiscalcore.TaxNode{{Name: "ICMSSN102", Children: []fiscalcore.TaxNode{{Name: "orig", Value: "0"}, {Name: "CSOSN", Value: "102"}}}}}, {Name: "PIS", Children: []fiscalcore.TaxNode{{Name: "PISNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}, {Name: "COFINS", Children: []fiscalcore.TaxNode{{Name: "COFINSNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}}}}, Payments: []fiscalcore.InvoicePayment{{Method: "01", Amount: "10.00"}}, TaxTotals: totals}
	raw, key, e := fiscalcore.BuildInvoice(in)
	if e != nil {
		t.Fatal(e)
	}
	signed, e := cred.Credential.SignXML("nfe", raw, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	// The test authority supplies the existing schema's processed envelope.
	invoice := []byte(fmt.Sprintf(`<nfeProc xmlns="%s" versao="4.00">%s<protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><chNFe>%s</chNFe><dhRecbto>%s</dhRecbto><nProt>135260000000001</nProt><digVal>AAAAAAAAAAAAAAAAAAAAAAAAAAA=</digVal><cStat>100</cStat><xMotivo>Synthetic only</xMotivo></infProt></protNFe></nfeProc>`, fiscalcore.NFeNS, signed, key, time.Now().Format(time.RFC3339)))
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
func TestFocusXMLArchiveAuthorizedEventBackupAndLocalExport(t *testing.T) {
	invoice, event, key := archiveXMLFixture(t)
	ctx := context.Background()
	a := focusContext(t)
	keyFile := filepath.Join(t.TempDir(), "backup.key")
	if backup.InitKey(keyFile) != nil {
		t.Fatal("key")
	}
	var logOutput bytes.Buffer
	old := log.Writer()
	log.SetOutput(&logOutput)
	defer log.SetOutput(old)
	calls := 0
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("unexpected write")
		}
		if strings.HasSuffix(r.URL.Path, "-cancel.xml") {
			w.Write(event)
		} else {
			w.Write(invoice)
		}
	})
	p.configuration.XMLArchiveKeyFile = keyFile
	links := focusArchiveLinks{Invoice: "/arquivos/" + key[6:20] + "/202611/XMLs/" + key + "-nfe.xml", Cancellation: "/arquivos/" + key[6:20] + "/202611/XMLs/" + key + "-cancel.xml"}
	evidence, _ := json.Marshal(links)
	out := DocumentResult{State: fiscal.CheckoutCancelled, Key: key, Evidence: evidence}
	if state := p.archiveResult(ctx, a, "sale-one", "intent-one", out); state != "archived" {
		t.Fatal("archive", state)
	}
	if calls != 2 {
		t.Fatal("unexpected requests", calls)
	}
	// Both XMLs are encrypted in the selected DB; no plaintext in SQL or logs.
	rows, e := a.DB.Query(`SELECT payload_json FROM outbox WHERE event_type='checkout.focus_xml_archive'`)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for rows.Next() {
		var raw string
		rows.Scan(&raw)
		if strings.Contains(raw, "nfeProc") || strings.Contains(raw, "Synthetic private XML") || strings.Contains(raw, "procEventoNFe") {
			t.Fatal("plaintext in database")
		}
		count++
	}
	rows.Close()
	if count != 2 {
		t.Fatal("missing records", count)
	}
	if bytes.Contains(logOutput.Bytes(), invoice) || bytes.Contains(logOutput.Bytes(), event) || bytes.Contains(logOutput.Bytes(), []byte("Synthetic private XML")) {
		t.Fatal("XML in log")
	}
	registry := &ProviderRegistry{entries: map[string]ProviderRegistration{a.Actor.TenantID: {Configuration: p.configuration}}}
	beforeCalls := calls
	exported, e := registry.AccountingExport(ctx, a, time.Now().Add(-time.Hour).Format(time.RFC3339), time.Now().Add(time.Hour).Format(time.RFC3339), "pt-BR")
	if e != nil {
		t.Fatal("local export", e)
	}
	if calls != beforeCalls {
		t.Fatal("export called third party")
	}
	archive, e := zip.NewReader(bytes.NewReader(exported), int64(len(exported)))
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, file := range archive.File {
		if strings.HasSuffix(file.Name, ".xml") {
			f, _ := file.Open()
			data, _ := io.ReadAll(f)
			f.Close()
			if !bytes.Equal(data, invoice) && !bytes.Equal(data, event) {
				t.Fatal("XML altered")
			}
			found++
		}
	}
	if found != 2 {
		t.Fatal("missing export XML", found)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot.tytbak")
	keyBytes, e := backup.ReadKey(keyFile)
	if e != nil {
		t.Fatal(e)
	}
	defer clear(keyBytes)
	if e = backup.Create(ctx, a.DB, a.Device, keyBytes, snapshot); e != nil {
		t.Fatal("backup", e)
	}
	restored := filepath.Join(t.TempDir(), "restored.sqlite")
	if e = backup.Restore(ctx, snapshot, restored, a.Device, keyBytes); e != nil {
		t.Fatal("restore", e)
	}
	db, e := localdb.Open(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var raw string
	if db.QueryRow(`SELECT payload_json FROM outbox WHERE event_type='checkout.focus_xml_archive' LIMIT 1`).Scan(&raw) != nil {
		t.Fatal("archive missing after restore")
	}
	var record focusArchiveRecord
	json.Unmarshal([]byte(raw), &record)
	crypt, _ := archiveCrypt(keyFile)
	n := crypt.NonceSize()
	data, e := crypt.Open(nil, record.Ciphertext[:n], record.Ciphertext[n:], archiveAAD(a, record))
	if e != nil || !bytes.Equal(data, invoice) && !bytes.Equal(data, event) {
		t.Fatal("archive not recoverable", e)
	}
	foreign := a
	foreign.Actor.TenantID = "other-tenant"
	if _, e = crypt.Open(nil, record.Ciphertext[:n], record.Ciphertext[n:], archiveAAD(foreign, record)); e == nil {
		t.Fatal("tenant isolation")
	}
}
func TestFocusXMLArchiveDownloadFailureDoesNotLoseAuthorizedNote(t *testing.T) {
	a := focusContext(t)
	key := syntheticFocusKey(t)
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "synthetic archive unavailable", 503) })
	p.configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "backup.key")
	backup.InitKey(p.configuration.XMLArchiveKeyFile)
	links, _ := json.Marshal(focusArchiveLinks{Invoice: "/arquivos_development/" + key[6:20] + "/202611/XMLs/" + key + "-nfe.xml"})
	out, e := p.save(context.Background(), a, "sale-one", "intent-one", DocumentResult{State: "authorized_homologation", Key: key, Evidence: links}, nil)
	if e != nil || out.State != "authorized_homologation" || out.ArchiveState != "pending" {
		t.Fatal("authorization lost", out.State, out.ArchiveState, e)
	}
	var count int
	a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.fiscal_status'`).Scan(&count)
	if count != 1 {
		t.Fatal("note status lost")
	}
	for _, path := range []string{"https://outside.invalid/secret", "/arquivos/other/" + key + "-nfe.xml", "/arquivos/" + key[6:20] + "/../" + key + "-nfe.xml"} {
		if _, e = p.downloadArchiveXML(context.Background(), path, key, "invoice", 0); e == nil {
			t.Fatal("unsafe archive URL")
		}
	}
}

func TestFocusXMLArchiveSubmissionRecoveryNeverDuplicatesNote(t *testing.T) {
	invoice, _, key := archiveXMLFixture(t)
	a := focusContext(t)
	ctx := context.Background()
	ref := focusReference(a.Actor.TenantID, "sale-one")
	resource := "/arquivos/" + key[6:20] + "/202611/XMLs/" + key + "-nfe.xml"
	posts, gets := 0, 0
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == resource {
			gets++
			w.Write(invoice)
			return
		}
		if r.Method == "POST" {
			posts++
			w.WriteHeader(201)
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "autorizado", "ref": ref, "cnpj_emitente": key[6:20], "chave_nfe": "NFe" + key, "caminho_xml_nota_fiscal": resource})
	})
	p.configuration.DocumentModel = "55"
	p.configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "backup.key")
	backup.InitKey(p.configuration.XMLArchiveKeyFile)
	intent := fiscal.Intent{ID: "intent-one", Snapshot: fiscal.Snapshot{Profile: fiscal.ProfileInput{Model: "55", CNPJ: key[6:20]}}}
	for i := 0; i < 2; i++ {
		result, e := p.submit(ctx, a, "sale-one", intent, ref, []byte(`{"synthetic":"fault fixture"}`))
		if e != nil || result.State != "authorized_homologation" || result.ArchiveState != "archived" {
			t.Fatal("submission/archive", result.State, result.ArchiveState, e)
		}
	}
	if posts != 1 || gets != 2 {
		t.Fatal("recovery duplicated or lost archive", posts, gets)
	}
	var count int
	a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_xml_archive'`).Scan(&count)
	if count != 1 {
		t.Fatal("archive duplicated", count)
	}
}

func TestFocusXMLArchiveRefusesSharedCustomerKeysAndMissingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.key")
	if backup.InitKey(path) != nil {
		t.Fatal("key")
	}
	entries := []ProviderRegistration{{Configuration: TenantFiscalConfiguration{XMLArchiveKeyFile: path}}, {Configuration: TenantFiscalConfiguration{XMLArchiveKeyFile: path}}}
	if validateArchiveKeys(entries) == nil {
		t.Fatal("customers share key")
	}
	entries[1].Configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "other.key")
	backup.InitKey(entries[1].Configuration.XMLArchiveKeyFile)
	if validateArchiveKeys(entries) != nil {
		t.Fatal("independent keys refused")
	}
	entries[0].Configuration.XMLArchiveKeyFile = filepath.Join(t.TempDir(), "missing.key")
	if validateArchiveKeys(entries) == nil {
		t.Fatal("missing key accepted")
	}
}

func TestFocusXMLArchiveUsesExistingProtocolStatusesAndRejectsProduction(t *testing.T) {
	invoice, event, key := archiveXMLFixture(t)
	if validateFocusArchiveXML(bytes.Replace(invoice, []byte("<cStat>100</cStat>"), []byte("<cStat>150</cStat>"), 1), key, "invoice", 0) != nil {
		t.Fatal("existing authorization status refused")
	}
	if validateFocusArchiveXML(bytes.Replace(event, []byte("<cStat>135</cStat>"), []byte("<cStat>155</cStat>"), 1), key, "cancellation", 1) != nil {
		t.Fatal("existing cancellation status refused")
	}
	if validateFocusArchiveXML(bytes.ReplaceAll(invoice, []byte("<tpAmb>2</tpAmb>"), []byte("<tpAmb>1</tpAmb>")), key, "invoice", 0) == nil {
		t.Fatal("production archive accepted")
	}
}
