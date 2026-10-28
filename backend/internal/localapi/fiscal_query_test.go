package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beevik/etree"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
)

const fiscalQueryEndpoint = "https://homologacao.example.gov.br/NFeConsultaProtocolo4"

func querySyntheticCredential(t *testing.T) *fiscalcore.Credential {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	oid, _ := asn1.Marshal(asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3})
	value, _ := asn1.Marshal("12ABC34501DE35")
	explicit, _ := asn1.Marshal(asn1.RawValue{Class: 2, Tag: 0, IsCompound: true, Bytes: value})
	san, _ := asn1.Marshal([]asn1.RawValue{{Class: 2, Tag: 0, IsCompound: true, Bytes: append(oid, explicit...)}})
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Synthetic query fixture only"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtraExtensions: []pkix.Extension{{Id: asn1.ObjectIdentifier{2, 5, 29, 17}, Value: san}}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	credential, err := fiscalcore.NewCredential(der, key, "12ABC34501DE35", roots, now)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func queryOriginalFixture(t *testing.T) (*stockHTTPFixture, fiscal.Intent, fiscal.NumberAssignment, fiscalcore.RecordedRequest) {
	return queryOriginalFixtureWithPrint(t, false)
}

func queryOriginalFixtureWithPrint(t *testing.T, withPrint bool, direct ...bool) (*stockHTTPFixture, fiscal.Intent, fiscal.NumberAssignment, fiscalcore.RecordedRequest) {
	t.Helper()
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	number := numberingPrepare(t, f, intent, 1)
	recorder := numberedFiscalJournal(t, f, f.db, intent, f.token, number)
	in := boundFiscalInvoice(intent)
	in.Key.Number = number.Number
	in.Key.NumericCode = number.AccessKey[35:43]
	in.Issuer.Address.Street = "Rua Sintetica"
	in.Issuer.Address.Number = "1"
	in.Issuer.Address.District = "Centro"
	in.Issuer.Address.City = "Sao Paulo"
	in.Issuer.Address.PostalCode = "01001000"
	for i := range in.Lines {
		in.Lines[i].Description = "Synthetic item only"
		in.Lines[i].Unit = "UN"
		in.Lines[i].NCM = "09012100"
		in.Lines[i].CFOP = "5102"
		in.Lines[i].Taxes = []fiscalcore.TaxNode{{Name: "ICMS", Children: []fiscalcore.TaxNode{{Name: "ICMSSN102", Children: []fiscalcore.TaxNode{{Name: "orig", Value: "0"}, {Name: "CSOSN", Value: "102"}}}}}, {Name: "PIS", Children: []fiscalcore.TaxNode{{Name: "PISNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}, {Name: "COFINS", Children: []fiscalcore.TaxNode{{Name: "COFINSNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}}
	}
	if err := recorder.ValidateInvoice(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	raw, _, err := fiscalcore.BuildInvoice(in)
	if err != nil {
		t.Fatal(err)
	}
	credential := querySyntheticCredential(t)
	signed, err := credential.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal("sign synthetic fixture", err)
	}
	if withPrint {
		signed, err = credential.CompleteNFCeXML(fiscalcore.QRConfiguration{BaseURL: "https://homologacao.example.gov.br/qrcode", Version: 3}, "https://homologacao.example.gov.br/consulta", signed, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}
	invoice := etree.NewDocument()
	if err = invoice.ReadFromBytes(signed); err != nil {
		t.Fatal(err)
	}
	d := etree.NewDocument()
	envelope := d.CreateElement("soap:Envelope")
	envelope.CreateAttr("xmlns:soap", "http://www.w3.org/2003/05/soap-envelope")
	body := envelope.CreateElement("soap:Body")
	msg := body.CreateElement("nfeDadosMsg")
	msg.CreateAttr("xmlns", fiscalcore.NFeNS+"/wsdl/NFeAutorizacao4")
	batch := msg.CreateElement("enviNFe")
	batch.CreateAttr("xmlns", fiscalcore.NFeNS)
	batch.CreateAttr("versao", "4.00")
	batch.CreateElement("idLote").SetText("1")
	batch.CreateElement("indSinc").SetText("1")
	batch.AddChild(invoice.Root().Copy())
	soap, err := d.WriteToBytes()
	if err != nil {
		t.Fatal(err)
	}
	req := fiscalRecorded(t, recorder)
	req.OperationID = "issue-original"
	req.Key = number.AccessKey
	req.Body = soap
	sum := sha256.Sum256(soap)
	req.RequestSHA256 = hex.EncodeToString(sum[:])
	if err = recorder.Reserve(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	result := fiscalcore.ExchangeResult{OperationID: req.OperationID, State: "uncertain"}
	if len(direct) > 0 && direct[0] {
		consultation := querySQLResult(t, req, req.OperationID, "100")
		parsed := etree.NewDocument()
		if err = parsed.ReadFromBytes(consultation.Body); err != nil {
			t.Fatal(err)
		}
		auth := etree.NewDocument()
		ret := auth.CreateElement("retEnviNFe")
		ret.CreateAttr("xmlns", fiscalcore.NFeNS)
		ret.CreateAttr("versao", "4.00")
		ret.CreateElement("tpAmb").SetText("2")
		ret.CreateElement("cStat").SetText("104")
		ret.AddChild(parsed.Root().SelectElement("protNFe").Copy())
		body, err := auth.WriteToBytes()
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		result = fiscalcore.ExchangeResult{OperationID: req.OperationID, State: "response_received", HTTPStatus: 200, Body: body, SHA256: hex.EncodeToString(digest[:])}
	}
	if err = recorder.Complete(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	return f, intent, number, req
}
func querySQLRecorder(t *testing.T, f *stockHTTPFixture, db *sql.DB, intent fiscal.Intent, queryID, token string) *fiscal.HomologationQueryRecorder {
	t.Helper()
	r, err := fiscal.NewHomologationQueryRecorder(context.Background(), db, journalLicense(t, f, db), numberingActor(f), f.device, intent.ID, queryID, fiscalQueryEndpoint, token)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func querySQLRequest(t *testing.T, r *fiscal.HomologationQueryRecorder, queryID string) fiscalcore.RecordedRequest {
	t.Helper()
	body, err := fiscalcore.BuildNFeConsultation(r.Key(), "2")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return fiscalcore.RecordedRequest{OperationID: queryID, Scope: r.Scope(), URL: fiscalQueryEndpoint, Issuer: r.Key()[6:20], Environment: "2", Key: r.Key(), RequestSHA256: hex.EncodeToString(sum[:]), Body: body}
}
func querySQLResult(t *testing.T, original fiscalcore.RecordedRequest, queryID, status string) fiscalcore.ExchangeResult {
	t.Helper()
	invoice, err := fiscalcore.OriginalNFeFromRequest(original)
	if err != nil {
		t.Fatal(err)
	}
	protocol := ""
	if status == "100" {
		protocol = fmt.Sprintf(`<protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><chNFe>%s</chNFe><dhRecbto>2026-10-27T12:00:00-03:00</dhRecbto><nProt>135260000000001</nProt><digVal>%s</digVal><cStat>100</cStat><xMotivo>Synthetic protocol only</xMotivo></infProt></protNFe>`, invoice.Key, invoice.Digest)
	}
	body := []byte(fmt.Sprintf(`<retConsSitNFe xmlns="%s" versao="4.00"><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cStat>%s</cStat><xMotivo>Synthetic lookup only</xMotivo><cUF>35</cUF><dhRecbto>2026-10-27T12:01:00-03:00</dhRecbto><chNFe>%s</chNFe>%s</retConsSitNFe>`, fiscalcore.NFeNS, status, invoice.Key, protocol))
	sum := sha256.Sum256(body)
	return fiscalcore.ExchangeResult{OperationID: queryID, State: "response_received", HTTPStatus: 200, Body: body, SHA256: hex.EncodeToString(sum[:])}
}

func TestFiscalQuerySQLConcurrentReservationAndIndependentReconciliation(t *testing.T) {
	f, intent, number, original := queryOriginalFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	req := querySQLRequest(t, r, "query-one")
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.Reserve(ctx, req) }()
	}
	wg.Wait()
	close(results)
	created := 0
	for err := range results {
		if err == nil {
			created++
		} else if !errors.Is(err, fiscalcore.ErrRecorded) {
			t.Fatal(err)
		}
	}
	if created != 1 {
		t.Fatal("duplicate query reservations", created)
	}
	if err := r.BeforeSend(ctx, req.OperationID); err != nil {
		t.Fatal(err)
	}
	result := querySQLResult(t, original, "query-one", "100")
	if err := r.Complete(ctx, result); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, result); err != nil {
		t.Fatal("idempotent callback", err)
	}
	if err := r.Complete(ctx, querySQLResult(t, original, "query-one", "217")); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("overwrote query evidence", err)
	}
	proof, err := r.Reconcile(ctx)
	if err != nil || proof.Key != number.AccessKey || proof.Disposition != "authorized_indicated_homologation" || fiscalcore.ValidateXML("nfe_process", proof.ProcessedXML) != nil {
		t.Fatal("SQL reconciliation", proof.Disposition, err)
	}
	originalReader := numberedFiscalJournal(t, f, f.db, intent, f.token, number)
	saved, pending, err := originalReader.Read(ctx)
	if err != nil || saved.RequestSHA256 != original.RequestSHA256 || pending.State != "uncertain" {
		t.Fatal("query overwrote original issuance", err)
	}
	for _, query := range []string{`UPDATE fiscal_query_requests SET query_id='changed'`, `DELETE FROM fiscal_query_requests`, `UPDATE fiscal_query_results SET result_json='{}'`, `DELETE FROM fiscal_query_results`} {
		if _, err = f.db.Exec(query); err == nil {
			t.Fatal("query evidence mutable", query)
		}
	}
}

func TestFiscalQuerySQLLogoutKeepsInflightResponseAndNewSessionCanRead(t *testing.T) {
	f, intent, _, original := queryOriginalFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	ctx := context.Background()
	req := querySQLRequest(t, r, "query-one")
	if err := r.Reserve(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := localauth.Logout(ctx, f.db, f.token); err != nil {
		t.Fatal(err)
	}
	if err := r.BeforeSend(ctx, req.OperationID); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("logged-out lookup sent", err)
	}
	if err := r.Complete(ctx, querySQLResult(t, original, "query-one", "100")); err != nil {
		t.Fatal("lost in-flight query result", err)
	}
	token := loginToken(t, f.app, f.owner.OwnerID)
	recovered := querySQLRecorder(t, f, f.db, intent, "query-one", token)
	if _, err := recovered.Reconcile(ctx); err != nil {
		t.Fatal("new authorized session cannot recover", err)
	}
	if err := recovered.BeforeSend(ctx, req.OperationID); !errors.Is(err, localauth.ErrDenied) {
		t.Fatal("new session took over old lookup", err)
	}
	if err := recovered.Reserve(ctx, querySQLRequest(t, recovered, "query-one")); !errors.Is(err, fiscalcore.ErrRecorded) {
		t.Fatal("duplicate query re-sent", err)
	}
	fresh := querySQLRecorder(t, f, f.db, intent, "query-two", token)
	if err := fresh.Reserve(ctx, querySQLRequest(t, fresh, "query-two")); err != nil {
		t.Fatal("explicit new lookup denied", err)
	}
}

func TestFiscalQuerySQLRejectsIssuanceBodyAndForeignContext(t *testing.T) {
	f, intent, _, original := queryOriginalFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	base := querySQLRequest(t, r, "query-one")
	for name, mutate := range map[string]func(*fiscalcore.RecordedRequest){"issuance_body": func(q *fiscalcore.RecordedRequest) {
		q.Body = original.Body
		sum := sha256.Sum256(q.Body)
		q.RequestSHA256 = hex.EncodeToString(sum[:])
	}, "endpoint": func(q *fiscalcore.RecordedRequest) { q.URL = "https://foreign.gov.br/path" }, "issuer": func(q *fiscalcore.RecordedRequest) { q.Issuer = "11222333000181" }, "scope": func(q *fiscalcore.RecordedRequest) { q.Scope = original.Scope }, "operation": func(q *fiscalcore.RecordedRequest) { q.OperationID = "foreign-query" }, "environment": func(q *fiscalcore.RecordedRequest) { q.Environment = "1" }} {
		t.Run(name, func(t *testing.T) {
			q := base
			mutate(&q)
			if err := r.Reserve(context.Background(), q); err == nil {
				t.Fatal("unbound query accepted")
			}
		})
	}
	for _, dimension := range []string{"tenant", "store", "actor", "device"} {
		t.Run(dimension, func(t *testing.T) {
			a, d := numberingActor(f), f.device
			switch dimension {
			case "tenant":
				a.TenantID = "foreign"
				d.TenantID = "foreign"
			case "store":
				a.StoreID = "foreign"
				d.StoreID = "foreign"
			case "actor":
				a.IdentityID = "foreign"
			case "device":
				d.DeviceID = "foreign"
			}
			if _, err := fiscal.NewHomologationQueryRecorder(context.Background(), f.db, journalLicense(t, f, f.db), a, d, intent.ID, "query-one", fiscalQueryEndpoint, f.token); err == nil {
				t.Fatal("foreign context opened query")
			}
		})
	}
	var count int
	if err := f.db.QueryRow(`SELECT count(*) FROM fiscal_query_requests`).Scan(&count); err != nil || count != 0 {
		t.Fatal("rejection left query record", count, err)
	}
}

func TestFiscalQuerySQLContractRevocationBlocksSendButPreservesEvidence(t *testing.T) {
	f, intent, _, original := queryOriginalFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	ctx := context.Background()
	if err := r.Reserve(ctx, querySQLRequest(t, r, "query-one")); err != nil {
		t.Fatal(err)
	}
	f.install(t, signedTestContract(t, f.private, f.owner.TenantID, 4, []modules.ID{modules.Core, modules.Inventory, modules.POS, modules.Staff}), 200)
	if err := r.BeforeSend(ctx, "query-one"); err == nil {
		t.Fatal("revoked contract queried network")
	}
	if err := r.Complete(ctx, querySQLResult(t, original, "query-one", "100")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx); err != nil {
		t.Fatal("expired contract hid archived result", err)
	}
	fresh := querySQLRecorder(t, f, f.db, intent, "query-two", f.token)
	if err := fresh.Reserve(ctx, querySQLRequest(t, fresh, "query-two")); err == nil {
		t.Fatal("revoked contract opened new network lookup")
	}
}

func TestFiscalQuerySQLNotFoundAndLateResponseNeverRewriteAnotherAttempt(t *testing.T) {
	f, intent, number, original := queryOriginalFixture(t)
	ctx := context.Background()
	older := querySQLRecorder(t, f, f.db, intent, "query-old", f.token)
	newer := querySQLRecorder(t, f, f.db, intent, "query-new", f.token)
	for id, r := range map[string]*fiscal.HomologationQueryRecorder{"query-old": older, "query-new": newer} {
		if err := r.Reserve(ctx, querySQLRequest(t, r, id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := newer.Complete(ctx, querySQLResult(t, original, "query-new", "100")); err != nil {
		t.Fatal(err)
	}
	if err := older.Complete(ctx, querySQLResult(t, original, "query-old", "217")); err != nil {
		t.Fatal(err)
	}
	freshProof, err := newer.Reconcile(ctx)
	if err != nil || freshProof.Disposition != "authorized_indicated_homologation" {
		t.Fatal(err)
	}
	oldProof, err := older.Reconcile(ctx)
	if err != nil || oldProof.Disposition != "not_found_indicated" || len(oldProof.ProcessedXML) > 0 {
		t.Fatal(err)
	}
	assigned, err := fiscal.ReadHomologationNumber(ctx, f.db, numberingActor(f), f.device, intent.ID, f.token)
	if err != nil || assigned.AccessKey != number.AccessKey {
		t.Fatal("not-found released number", err)
	}
	var count int
	if err = f.db.QueryRow(`SELECT count(*) FROM fiscal_transmission_requests`).Scan(&count); err != nil || count != 1 {
		t.Fatal("query reissued original", count, err)
	}
}

func TestFiscalQuerySQLRestartKeepsOriginalProcess(t *testing.T) {
	f, intent, _, original := queryOriginalFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	ctx := context.Background()
	if err := r.Reserve(ctx, querySQLRequest(t, r, "query-one")); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, querySQLResult(t, original, "query-one", "100")); err != nil {
		t.Fatal(err)
	}
	before, err := r.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var seq int
	var name, path string
	if err = f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err = f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reopened := querySQLRecorder(t, f, db, intent, "query-one", f.token)
	after, err := reopened.Reconcile(ctx)
	if err != nil || after.Key != before.Key || !bytes.Equal(after.ProcessedXML, before.ProcessedXML) || after.OriginalOperationID != before.OriginalOperationID || after.QueryOperationID != before.QueryOperationID {
		t.Fatal("restart changed fiscal proof", err)
	}
}

func TestFiscalQuerySQLRestoreAllowsLookupWithoutReleasingNumberBarrier(t *testing.T) {
	f, intent, number, original := queryOriginalFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	ctx := context.Background()
	if err := r.Reserve(ctx, querySQLRequest(t, r, "query-one")); err != nil {
		t.Fatal(err)
	}
	if err := r.Complete(ctx, querySQLResult(t, original, "query-one", "100")); err != nil {
		t.Fatal(err)
	}
	before, err := r.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var dest string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, f)
		var seq int
		var name, path string
		if err := f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
			t.Fatal(err)
		}
		dest = pilotCLIBackupRoundTrip(t, f, path, station)
	} else {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			t.Fatal(err)
		}
		archive := filepath.Join(dir, "query.tytbak")
		dest = filepath.Join(dir, "restored.sqlite")
		if err := backup.Create(ctx, f.db, f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err := backup.Restore(ctx, archive, dest, f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	db, err := localdb.Open(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewWithVerifier(db, f.device, verifier)
	if err != nil {
		t.Fatal(err)
	}
	token := loginToken(t, app, f.owner.OwnerID)
	restored := querySQLRecorder(t, f, db, intent, "query-one", token)
	after, err := restored.Reconcile(ctx)
	if err != nil || !bytes.Equal(after.ProcessedXML, before.ProcessedXML) {
		t.Fatal("restore lost fiscal evidence", err)
	}
	fresh := querySQLRecorder(t, f, db, intent, "query-two", token)
	if err := fresh.Reserve(ctx, querySQLRequest(t, fresh, "query-two")); err != nil {
		t.Fatal("restored read-only lookup blocked", err)
	}
	if err := fresh.BeforeSend(ctx, "query-two"); err != nil {
		t.Fatal(err)
	}
	assigned, err := fiscal.ReadHomologationNumber(ctx, db, numberingActor(f), f.device, intent.ID, token)
	if err != nil || assigned.AccessKey != number.AccessKey {
		t.Fatal(err)
	}
	if err := fiscal.ConfigureHomologationSeries(ctx, db, journalLicense(t, f, db), numberingActor(f), f.device, numberingOrigin(1), token); !errors.Is(err, fiscal.ErrRestored) {
		t.Fatal("query released restore barrier", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM fiscal_restore_barriers`).Scan(&count); err != nil || count != 1 {
		t.Fatal("lost restore barrier", count, err)
	}
}

func TestFiscalQuerySQLHistoricalProfileCanBeConsultedWithoutOldIssuanceAuthority(t *testing.T) {
	f, intent, number, _ := queryOriginalFixture(t)
	revised := strings.Replace(fiscalProfileBody, "issuer-create", "query-profile-revise", 1)
	revised = strings.Replace(revised, `"expected_revision":0`, `"expected_revision":1`, 1)
	revised = strings.Replace(revised, "Synthetic test issuer", "Updated synthetic issuer", 1)
	code, _ := request(t, f.app, "POST", "/local/v1/fiscal/profiles", revised, f.token)
	if code != 201 {
		t.Fatal(code)
	}
	query := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	if err := query.Reserve(context.Background(), querySQLRequest(t, query, "query-one")); err != nil {
		t.Fatal("historical lookup blocked", err)
	}
	old := numberedFiscalJournal(t, f, f.db, intent, f.token, number)
	if err := old.BeforeSend(context.Background(), "issue-original"); !errors.Is(err, fiscal.ErrConflict) {
		t.Fatal("profile change resumed issuance", err)
	}
}

func TestFiscalQuerySQLAccountingPermissionDoesNotResumeAnotherActorsLookup(t *testing.T) {
	f, intent, _, original := queryOriginalFixture(t)
	ctx := context.Background()
	ownerQuery := querySQLRecorder(t, f, f.db, intent, "query-owner", f.token)
	if err := ownerQuery.Reserve(ctx, querySQLRequest(t, ownerQuery, "query-owner")); err != nil {
		t.Fatal(err)
	}
	if err := ownerQuery.Complete(ctx, querySQLResult(t, original, "query-owner", "100")); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"cashier", "accountant"} {
		t.Run(role, func(t *testing.T) {
			id := "query-" + role
			code, _ := request(t, f.app, "POST", "/local/v1/staff", `{"operation_id":"create-`+id+`","identity_id":"`+id+`","name":"Synthetic query staff","role":"`+role+`","password":"synthetic-query-password-only"}`, f.token)
			if code != 201 {
				t.Fatal(code)
			}
			code, body := request(t, f.app, "POST", "/local/v1/login", `{"identity_id":"`+id+`","password":"synthetic-query-password-only"}`, "")
			if code != 200 {
				t.Fatal(code)
			}
			var auth struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(body, &auth); err != nil {
				t.Fatal(err)
			}
			a := numberingActor(f)
			a.IdentityID = id
			reader, err := fiscal.NewHomologationQueryRecorder(ctx, f.db, journalLicense(t, f, f.db), a, f.device, intent.ID, "query-owner", fiscalQueryEndpoint, auth.Token)
			if role == "cashier" {
				if err == nil {
					t.Fatal("cashier read accounting query")
				}
				return
			}
			if err != nil {
				t.Fatal("accountant cannot read historical query", err)
			}
			if _, err = reader.Reconcile(ctx); err != nil {
				t.Fatal(err)
			}
			if err = reader.BeforeSend(ctx, "query-owner"); !errors.Is(err, localauth.ErrDenied) {
				t.Fatal("accountant resumed owner's lookup", err)
			}
			fresh, err := fiscal.NewHomologationQueryRecorder(ctx, f.db, journalLicense(t, f, f.db), a, f.device, intent.ID, "query-accountant-new", fiscalQueryEndpoint, auth.Token)
			if err != nil {
				t.Fatal(err)
			}
			if err = fresh.Reserve(ctx, querySQLRequest(t, fresh, "query-accountant-new")); err != nil {
				t.Fatal("explicit accountant lookup denied", err)
			}
		})
	}
}

func TestFiscalQuerySQLRejectsMalformedCallbacksWithoutDiscardingRequest(t *testing.T) {
	f, intent, _, original := queryOriginalFixture(t)
	r := querySQLRecorder(t, f, f.db, intent, "query-one", f.token)
	ctx := context.Background()
	req := querySQLRequest(t, r, "query-one")
	if err := r.Reserve(ctx, req); err != nil {
		t.Fatal(err)
	}
	base := querySQLResult(t, original, "query-one", "100")
	for _, mutate := range []func(*fiscalcore.ExchangeResult){func(r *fiscalcore.ExchangeResult) { r.OperationID = "foreign" }, func(r *fiscalcore.ExchangeResult) { r.State = "authorized" }, func(r *fiscalcore.ExchangeResult) { r.HTTPStatus = 0 }, func(r *fiscalcore.ExchangeResult) { r.SHA256 = "wrong" }, func(r *fiscalcore.ExchangeResult) { r.State = "uncertain" }, func(r *fiscalcore.ExchangeResult) { r.HTTPStatus = 600 }} {
		result := base
		mutate(&result)
		if err := r.Complete(ctx, result); err == nil {
			t.Fatal("invalid callback accepted")
		}
	}
	saved, pending, err := r.Read(ctx)
	if err != nil || saved.RequestSHA256 != req.RequestSHA256 || pending.State != "uncertain" {
		t.Fatal("invalid callback removed original lookup", err)
	}
	if err = r.Complete(ctx, base); err != nil {
		t.Fatal("valid response after rejected callback", err)
	}
}
