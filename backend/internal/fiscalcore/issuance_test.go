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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type orchestrationJournal struct {
	*FileRecorder
	operation  string
	validation error
	guard      error
	validated  int
}

func (r *orchestrationJournal) ValidateInvoice(context.Context, Invoice) error {
	r.validated++
	return r.validation
}
func (r *orchestrationJournal) BeforeSend(context.Context, string) error { return r.guard }
func (r *orchestrationJournal) Read(context.Context) (RecordedRequest, ExchangeResult, error) {
	return r.FileRecorder.Read(r.operation)
}

func newIssuanceTestFileRecorder(t *testing.T) (*FileRecorder, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return NewFileRecorder(dir)
}

type substitutedIssuanceJournal struct {
	InvoiceRecorder
	request RecordedRequest
	result  ExchangeResult
	readErr error
}

func (r *substitutedIssuanceJournal) Read(context.Context) (RecordedRequest, ExchangeResult, error) {
	return r.request, r.result, r.readErr
}

func TestFiscalIssuanceMTLSJournalProtocolAndOfflineRecovery(t *testing.T) {
	for _, scenario := range []string{"protocol", "wrong_digest", "wrong_key", "rejection", "http_error", "fault"} {
		t.Run(scenario, func(t *testing.T) {
			credential := testCredential(t)
			journal, err := newIssuanceTestFileRecorder(t)
			if err != nil {
				t.Fatal(err)
			}
			r := &orchestrationJournal{FileRecorder: journal, operation: "issue-one"}
			var calls atomic.Int32
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls.Add(1)
				original, _, err := journal.Read(r.operation)
				if err != nil {
					t.Error("POST before journal", err)
				}
				raw, _ := io.ReadAll(req.Body)
				if !bytes.Equal(raw, original.Body) {
					t.Error("sent bytes differ")
				}
				d, err := parseXML(raw)
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				nfe := d.Root().FindElement("Body/nfeDadosMsg/enviNFe/NFe")
				digest := nfe.FindElement("Signature/SignedInfo/Reference/DigestValue").Text()
				key := original.Key
				if scenario == "wrong_digest" {
					digest = "AAAAAAAAAAAAAAAAAAAAAAAAAAA="
				}
				if scenario == "wrong_key" {
					in := testInvoice().Key
					in.Number = 2
					key, _ = AccessKey(in)
				}
				status := "100"
				if scenario == "rejection" {
					status = "302"
				}
				if scenario == "http_error" {
					w.WriteHeader(500)
				}
				if scenario == "fault" {
					w.Write([]byte(`<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><soap:Fault/></soap:Body></soap:Envelope>`))
					return
				}
				fmt.Fprintf(w, `<retEnviNFe xmlns="%s" versao="4.00"><tpAmb>2</tpAmb><cStat>104</cStat><protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><chNFe>%s</chNFe><dhRecbto>2026-10-27T12:00:00-03:00</dhRecbto><nProt>135260000000001</nProt><digVal>%s</digVal><cStat>%s</cStat><xMotivo>Synthetic protocol only</xMotivo></infProt></protNFe></retEnviNFe>`, NFeNS, key, digest, status)
			}))
			clientRoots := x509.NewCertPool()
			clientRoots.AddCert(credential.certificate)
			server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots, MinVersion: tls.VersionTLS12}
			server.StartTLS()
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client, err := newClient(server.URL, "synthetic-scope", credential, r, roots, true)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			result, err := client.IssueHomologationNFe(context.Background(), r.operation, "1", testInvoice())
			if scenario == "protocol" {
				if err != nil || result.State != "protocol_received_homologation" || result.Protocol == nil || result.Protocol.Key != result.Key {
					t.Fatalf("protocol: %s %v", result.State, err)
				}
			} else if !errors.Is(err, ErrUncertain) || result.Protocol != nil {
				t.Fatalf("false authorization: %s %v", result.State, err)
			}
			if calls.Load() != 1 || r.validated != 1 {
				t.Fatal("unexpected calls", calls.Load(), r.validated)
			}
			duplicate, duplicateErr := client.IssueHomologationNFe(context.Background(), r.operation, "1", testInvoice())
			if !errors.Is(duplicateErr, ErrRecorded) || duplicate.Exchange.OperationID != result.Exchange.OperationID || !bytes.Equal(duplicate.SignedXML, result.SignedXML) || !bytes.Equal(duplicate.Exchange.Body, result.Exchange.Body) {
				t.Fatal("duplicate lost original or re-POSTed", duplicateErr)
			}
			server.Close()
			// Recovery uses no client/certificate/network, even after signing key expiry.
			credential.certificate.NotAfter = time.Now().Add(-time.Hour)
			recovered, recoveryErr := RecoverHomologationNFe(context.Background(), r)
			if scenario == "protocol" {
				if recoveryErr != nil || recovered.Protocol == nil || recovered.Key != result.Key || !bytes.Equal(recovered.SignedXML, result.SignedXML) {
					t.Fatal("original recovery", recoveryErr)
				}
			} else if !errors.Is(recoveryErr, ErrUncertain) || recovered.Protocol != nil {
				t.Fatal("false recovered authorization", recoveryErr)
			}
			if calls.Load() != 1 {
				t.Fatal("recovery sent HTTP")
			}
			if scenario == "protocol" {
				original, stored, err := r.Read(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				// SOAP overhead is allowed beyond the 1MiB document limit;
				// the extracted signed NFe still receives strict validation.
				padded := original
				padded.Body = bytes.Replace(original.Body, []byte("<soap:Body>"), []byte(strings.Repeat(" ", MaxXML)+"<soap:Body>"), 1)
				paddingHash := sha256.Sum256(padded.Body)
				padded.RequestSHA256 = hex.EncodeToString(paddingHash[:])
				large := &substitutedIssuanceJournal{InvoiceRecorder: r, request: padded, result: stored}
				if proof, err := RecoverHomologationNFe(context.Background(), large); err != nil || proof.Protocol == nil {
					t.Fatal("SOAP envelope overhead lost recovery", err)
				}
				for name, mutate := range map[string]func(*RecordedRequest, *ExchangeResult){
					"operation":   func(q *RecordedRequest, s *ExchangeResult) { s.OperationID = "foreign" },
					"environment": func(q *RecordedRequest, s *ExchangeResult) { q.Environment = "1" },
					"issuer":      func(q *RecordedRequest, s *ExchangeResult) { q.Issuer = "11222333000181" },
					"key": func(q *RecordedRequest, s *ExchangeResult) {
						in := testInvoice().Key
						in.Number = 2
						q.Key, _ = AccessKey(in)
					},
					"signed_amount": func(q *RecordedRequest, s *ExchangeResult) {
						q.Body = bytes.Replace(q.Body, []byte("6.67"), []byte("6.68"), 1)
					},
					"lot_namespace": func(q *RecordedRequest, s *ExchangeResult) {
						q.Body = bytes.Replace(q.Body, []byte("<idLote>"), []byte(`<idLote xmlns="urn:foreign">`), 1)
					},
					"asynchronous": func(q *RecordedRequest, s *ExchangeResult) {
						q.Body = bytes.Replace(q.Body, []byte("<indSinc>1</indSinc>"), []byte("<indSinc>0</indSinc>"), 1)
					},
					"response_hash": func(q *RecordedRequest, s *ExchangeResult) { s.SHA256 = strings.Repeat("0", 64) },
				} {
					t.Run("recover_"+name, func(t *testing.T) {
						q, s := original, stored
						q.Body = append([]byte(nil), q.Body...)
						s.Body = append([]byte(nil), s.Body...)
						mutate(&q, &s)
						hash := sha256.Sum256(q.Body)
						q.RequestSHA256 = hex.EncodeToString(hash[:])
						bad := &substitutedIssuanceJournal{InvoiceRecorder: r, request: q, result: s}
						evidence, err := RecoverHomologationNFe(context.Background(), bad)
						if err == nil || evidence.Protocol != nil {
							t.Fatal("corrupt evidence became protocol", name, err)
						}
					})
				}
				denied := &substitutedIssuanceJournal{InvoiceRecorder: r, readErr: ErrCredential}
				if _, err = RecoverHomologationNFe(context.Background(), denied); !errors.Is(err, ErrCredential) {
					t.Fatal("recovery bypassed reader authorization", err)
				}
			}

		})
	}
}

func TestFiscalIssuanceRejectsBindingProductionAndUnsupportedRecorder(t *testing.T) {
	credential := testCredential(t)
	journal, err := newIssuanceTestFileRecorder(t)
	if err != nil {
		t.Fatal(err)
	}
	r := &orchestrationJournal{FileRecorder: journal, operation: "issue-denied", validation: ErrInput}
	client, err := NewClient("https://homologacao.example.gov.br/authorization", "scope", credential, r)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err = client.IssueHomologationNFe(context.Background(), r.operation, "1", testInvoice()); !errors.Is(err, ErrInput) {
		t.Fatal(err)
	}
	if _, _, err = r.Read(context.Background()); err == nil {
		t.Fatal("binding refusal wrote journal")
	}
	in := testInvoice()
	in.Environment = 1
	if _, err = client.IssueHomologationNFe(context.Background(), r.operation, "1", in); !errors.Is(err, ErrUnsupported) {
		t.Fatal("production", err)
	}
	client.recorder = journal
	if _, err = client.IssueHomologationNFe(context.Background(), r.operation, "1", testInvoice()); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unbound recorder", err)
	}
}

func TestFiscalIssuanceRecoveryRejectsCorruptAssociation(t *testing.T) {
	credential := testCredential(t)
	raw, _, _ := BuildInvoice(testInvoice())
	signed, err := credential.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt response hashing cannot create protocol evidence.
	sum := sha256.Sum256([]byte("original"))
	out, err := interpretHomologationNFe(signed, ExchangeResult{OperationID: "op", State: "response_received", HTTPStatus: 200, Body: []byte("changed"), SHA256: hex.EncodeToString(sum[:])})
	if !errors.Is(err, ErrUncertain) || out.Protocol != nil {
		t.Fatal("hash mismatch", err)
	}
	// A pending journal keeps its original XML without claiming success.
	out, err = interpretHomologationNFe(signed, ExchangeResult{OperationID: "op", State: "uncertain"})
	if !errors.Is(err, ErrUncertain) || out.Key == "" || !strings.Contains(string(out.SignedXML), "Signature") {
		t.Fatal("lost uncertain evidence", err)
	}
}
