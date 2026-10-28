package fiscalcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/beevik/etree"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func queryOriginal(t *testing.T, credential *Credential) RecordedRequest {
	t.Helper()
	raw, key, err := BuildInvoice(testInvoice())
	if err != nil {
		t.Fatal(err)
	}
	signed, err := credential.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	d, _ := parseXML(signed)
	batch := etree.NewDocument()
	root := batch.CreateElement("enviNFe")
	root.CreateAttr("xmlns", NFeNS)
	root.CreateAttr("versao", "4.00")
	field(root, "idLote", "1")
	field(root, "indSinc", "1")
	root.AddChild(d.Root().Copy())
	body, err := soap(NFeNS+"/wsdl/NFeAutorizacao4", root).WriteToBytes()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return RecordedRequest{OperationID: "issue-original", Scope: "synthetic-original", URL: "https://homologacao.example.gov.br/NFeAutorizacao4", Issuer: credential.document, Environment: "2", Key: key, RequestSHA256: hex.EncodeToString(sum[:]), Body: body}
}
func queryResponse(t *testing.T, original RecordedRequest, status string) []byte {
	t.Helper()
	invoice, err := OriginalNFeFromRequest(original)
	if err != nil {
		t.Fatal(err)
	}
	protocol := ""
	if status == "100" || status == "101" || status == "150" || status == "110" {
		pstatus := status
		if status == "101" {
			pstatus = "100"
		}
		protocol = fmt.Sprintf(`<protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><chNFe>%s</chNFe><dhRecbto>2026-10-27T12:00:00-03:00</dhRecbto><nProt>135260000000001</nProt><digVal>%s</digVal><cStat>%s</cStat><xMotivo>Synthetic protocol only</xMotivo></infProt></protNFe>`, invoice.Key, invoice.Digest, pstatus)
	}
	return []byte(fmt.Sprintf(`<retConsSitNFe xmlns="%s" versao="4.00"><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cStat>%s</cStat><xMotivo>Synthetic query only</xMotivo><cUF>35</cUF><dhRecbto>2026-10-27T12:01:00-03:00</dhRecbto><chNFe>%s</chNFe>%s</retConsSitNFe>`, NFeNS, status, invoice.Key, protocol))
}
func queryRecords(t *testing.T, original RecordedRequest, status string) (RecordedRequest, ExchangeResult) {
	t.Helper()
	body, err := BuildNFeConsultation(original.Key, "2")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	req := RecordedRequest{OperationID: "query-one", Scope: "synthetic-query", URL: "https://homologacao.example.gov.br/NFeConsultaProtocolo4", Issuer: original.Issuer, Environment: "2", Key: original.Key, RequestSHA256: hex.EncodeToString(sum[:]), Body: body}
	response := queryResponse(t, original, status)
	sum = sha256.Sum256(response)
	return req, ExchangeResult{OperationID: req.OperationID, State: "response_received", HTTPStatus: 200, Body: response, SHA256: hex.EncodeToString(sum[:])}
}

func TestFiscalQueryMTLSPersistsBeforeLookupWithoutReemission(t *testing.T) {
	credential := testCredential(t)
	original := queryOriginal(t, credential)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	journal, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		req, _, err := journal.Read("query-one")
		if err != nil {
			t.Error("query not persisted before network", err)
			w.WriteHeader(500)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		expected, _ := BuildNFeConsultation(original.Key, "2")
		if !bytes.Equal(raw, expected) || !bytes.Equal(raw, req.Body) || bytes.Contains(raw, []byte("<NFe")) || !strings.Contains(r.Header.Get("Content-Type"), "nfeConsultaNF") {
			t.Error("lookup carried issuance or mismatched bytes")
		}
		w.Header().Set("Content-Type", "application/soap+xml")
		fmt.Fprintf(w, `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><nfeResultMsg xmlns="%s">%s</nfeResultMsg></soap:Body></soap:Envelope>`, consultationNS, queryResponse(t, original, "100"))
	}))
	clientRoots := x509.NewCertPool()
	clientRoots.AddCert(credential.certificate)
	server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	client, err := newClient(server.URL, "synthetic-query", credential, journal, roots, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result, err := client.ConsultNFe(context.Background(), "query-one", original.Key)
	if err != nil {
		t.Fatal(err)
	}
	req, stored, err := journal.Read("query-one")
	if err != nil || stored.SHA256 != result.SHA256 {
		t.Fatal(err)
	}
	proof, err := ReconcileHomologationNFe(original, req, stored)
	if err != nil || proof.Disposition != "authorized_indicated_homologation" || proof.Protocol == nil || len(proof.ProcessedXML) == 0 || ValidateXML("nfe_process", proof.ProcessedXML) != nil {
		t.Fatal("protocol reconciliation", proof.Disposition, err)
	}
	if _, err = client.ConsultNFe(context.Background(), "query-one", original.Key); !errors.Is(err, ErrRecorded) {
		t.Fatal("automatic repeated lookup", err)
	}
	server.Close()
	reopened, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	req, stored, err = reopened.Read("query-one")
	if err != nil {
		t.Fatal(err)
	}
	offline, err := ReconcileHomologationNFe(original, req, stored)
	if err != nil || !bytes.Equal(proof.ProcessedXML, offline.ProcessedXML) || calls.Load() != 1 {
		t.Fatal("offline recovery changed proof or emitted", err, calls.Load())
	}
}

func TestFiscalQueryDispositionsNeverReleaseOriginal(t *testing.T) {
	original := queryOriginal(t, testCredential(t))
	want := map[string]string{"100": "authorized_indicated_homologation", "150": "authorized_indicated_homologation", "101": "canceled_indicated_homologation", "110": "denied_indicated_homologation", "217": "not_found_indicated", "999": "status_requires_review"}
	for status, disposition := range want {
		t.Run(status, func(t *testing.T) {
			req, result := queryRecords(t, original, status)
			proof, err := ReconcileHomologationNFe(original, req, result)
			if err != nil || proof.Disposition != disposition {
				t.Fatal(proof.Disposition, err)
			}
			if status != "100" && status != "150" && (proof.Protocol != nil || len(proof.ProcessedXML) != 0) {
				t.Fatal("non-authorized status returned fiscal process")
			}
			if proof.OriginalOperationID != original.OperationID {
				t.Fatal("lost original operation")
			}
		})
	}
}

func TestFiscalQueryRejectsMixedResponsesAndCorruptOriginal(t *testing.T) {
	original := queryOriginal(t, testCredential(t))
	baseReq, baseResult := queryRecords(t, original, "100")
	mutations := map[string]func(*RecordedRequest, *RecordedRequest, *ExchangeResult){
		"response_hash": func(o, q *RecordedRequest, r *ExchangeResult) { r.SHA256 = strings.Repeat("0", 64) },
		"operation":     func(o, q *RecordedRequest, r *ExchangeResult) { r.OperationID = "foreign-query" },
		"same_operation_as_issue": func(o, q *RecordedRequest, r *ExchangeResult) {
			q.OperationID = o.OperationID
			r.OperationID = q.OperationID
		},
		"http_status":       func(o, q *RecordedRequest, r *ExchangeResult) { r.HTTPStatus = 500 },
		"uncertain":         func(o, q *RecordedRequest, r *ExchangeResult) { r.State = "uncertain" },
		"query_environment": func(o, q *RecordedRequest, r *ExchangeResult) { q.Environment = "1" },
		"query_issuer":      func(o, q *RecordedRequest, r *ExchangeResult) { q.Issuer = "11222333000181" },
		"query_body": func(o, q *RecordedRequest, r *ExchangeResult) {
			q.Body = bytes.Replace(q.Body, []byte("CONSULTAR"), []byte("AUTORIZAR"), 1)
			sum := sha256.Sum256(q.Body)
			q.RequestSHA256 = hex.EncodeToString(sum[:])
		},
		"outer_environment": func(o, q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<tpAmb>2</tpAmb>"), []byte("<tpAmb>1</tpAmb>"), 1)
		},
		"missing_protocol": func(o, q *RecordedRequest, r *ExchangeResult) {
			d, _ := parseXML(r.Body)
			d.Root().RemoveChild(d.Root().SelectElement("protNFe"))
			r.Body, _ = d.WriteToBytes()
		},
		"foreign_key": func(o, q *RecordedRequest, r *ExchangeResult) {
			key := testInvoice().Key
			key.Number = 2
			foreign, _ := AccessKey(key)
			r.Body = bytes.ReplaceAll(r.Body, []byte(o.Key), []byte(foreign))
		},
		"protocol_digest": func(o, q *RecordedRequest, r *ExchangeResult) {
			d, _ := parseXML(r.Body)
			d.Root().FindElement("protNFe/infProt/digVal").SetText("AAAAAAAAAAAAAAAAAAAAAAAAAAA=")
			r.Body, _ = d.WriteToBytes()
		},
		"duplicate_protocol": func(o, q *RecordedRequest, r *ExchangeResult) {
			d, _ := parseXML(r.Body)
			d.Root().AddChild(d.Root().SelectElement("protNFe").Copy())
			r.Body, _ = d.WriteToBytes()
		},
		"fault_wrapper": func(o, q *RecordedRequest, r *ExchangeResult) {
			r.Body = []byte(`<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><soap:Fault><detail>` + string(r.Body) + `</detail></soap:Fault></soap:Body></soap:Envelope>`)
		},
		"original_signature": func(o, q *RecordedRequest, r *ExchangeResult) {
			o.Body = bytes.Replace(o.Body, []byte("6.67"), []byte("6.68"), 1)
			sum := sha256.Sum256(o.Body)
			o.RequestSHA256 = hex.EncodeToString(sum[:])
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			o, q, r := original, baseReq, baseResult
			o.Body = append([]byte(nil), o.Body...)
			q.Body = append([]byte(nil), q.Body...)
			r.Body = append([]byte(nil), r.Body...)
			mutate(&o, &q, &r)
			if name != "response_hash" {
				sum := sha256.Sum256(r.Body)
				r.SHA256 = hex.EncodeToString(sum[:])
			}
			proof, err := ReconcileHomologationNFe(o, q, r)
			if err == nil || proof.Protocol != nil || len(proof.ProcessedXML) > 0 {
				t.Fatal("corrupt proof accepted", err)
			}
		})
	}
}

func TestFiscalQueryEmbeddedCancellationRequiresReview(t *testing.T) {
	original := queryOriginal(t, testCredential(t))
	req, result := queryRecords(t, original, "100")
	d, _ := parseXML(result.Body)
	cancel := d.Root().CreateElement("retCancNFe")
	cancel.CreateAttr("versao", "2.00")
	info := cancel.CreateElement("infCanc")
	for _, pair := range [][2]string{{"tpAmb", "2"}, {"verAplic", "SYNTHETIC1"}, {"cStat", "101"}, {"xMotivo", "Synthetic cancellation only"}, {"cUF", "35"}, {"chNFe", original.Key}, {"dhRecbto", "2026-10-27T12:01:00-03:00"}, {"nProt", "135260000000002"}} {
		field(info, pair[0], pair[1])
	}
	result.Body, _ = d.WriteToBytes()
	sum := sha256.Sum256(result.Body)
	result.SHA256 = hex.EncodeToString(sum[:])
	proof, err := ReconcileHomologationNFe(original, req, result)
	if err != nil || proof.Disposition != "event_review_required" || proof.Protocol != nil || len(proof.ProcessedXML) > 0 {
		t.Fatal("embedded cancellation ignored", proof.Disposition, err)
	}
}

func TestFiscalEmbeddedSchemaRootsWorkOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, kind := range []string{"nfe", "nfe_event", "nfse_dps", "nfse_dps_homolog_20261020", "nfe_query", "nfe_query_response", "nfe_process", "nfe_event_batch", "nfe_event_response", "nfe_event_process", "nfe_invalidation", "nfe_invalidation_response", "nfe_invalidation_process"} {
		if _, err := compileSchema(kind); err != nil {
			t.Fatalf("embedded schema %s outside repository: %v", kind, err)
		}
	}
	credential := testCredential(t)
	raw, _, err := BuildInvoice(testInvoice())
	if err != nil {
		t.Fatal(err)
	}
	signed, err := credential.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal("sign outside repository", err)
	}
	if err = credential.VerifyXML("nfe", signed, time.Now()); err != nil {
		t.Fatal("verify outside repository", err)
	}
}
