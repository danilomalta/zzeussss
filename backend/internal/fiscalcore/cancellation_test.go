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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beevik/etree"
)

type cancellationJournal struct {
	*FileRecorder
	operation              string
	validation, errorGuard error
	validations            int
}

func (r *cancellationJournal) ValidateCancellation(_ context.Context, _ Cancellation) error {
	r.validations++
	return r.validation
}
func (r *cancellationJournal) BeforeSend(context.Context, string) error { return r.errorGuard }
func (r *cancellationJournal) Read(context.Context) (RecordedRequest, ExchangeResult, error) {
	return r.FileRecorder.Read(r.operation)
}

func cancellationInput(t *testing.T, c *Credential) Cancellation {
	t.Helper()
	key, err := AccessKey(testInvoice().Key)
	if err != nil {
		t.Fatal(err)
	}
	return Cancellation{Key: key, CNPJ: c.document, Environment: 2, Time: time.Now().Add(-time.Minute).Truncate(time.Second), Protocol: "135260000000001", Reason: "Synthetic cancellation fixture only"}
}
func cancellationResponse(key, lot, status string) []byte {
	return []byte(fmt.Sprintf(`<retEnvEvento xmlns="%s" versao="1.00"><idLote>%s</idLote><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cOrgao>%s</cOrgao><cStat>128</cStat><xMotivo>Synthetic processed batch</xMotivo><retEvento versao="1.00"><infEvento><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cOrgao>%s</cOrgao><cStat>%s</cStat><xMotivo>Synthetic event result</xMotivo><chNFe>%s</chNFe><tpEvento>110111</tpEvento><xEvento>Cancelamento registrado</xEvento><nSeqEvento>1</nSeqEvento><dhRegEvento>%s</dhRegEvento><nProt>135260000000002</nProt></infEvento></retEvento></retEnvEvento>`, NFeNS, lot, key[:2], key[:2], status, key, time.Now().Format("2006-01-02T15:04:05-07:00")))
}
func cancellationRecord(t *testing.T, c *Credential) (RecordedRequest, ExchangeResult) {
	t.Helper()
	in := cancellationInput(t, c)
	raw, err := BuildCancellation(in)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := c.SignXML("nfe_event", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	event, _ := parseXML(signed)
	d := etree.NewDocument()
	batch := d.CreateElement("envEvento")
	batch.CreateAttr("xmlns", NFeNS)
	batch.CreateAttr("versao", "1.00")
	field(batch, "idLote", "1")
	batch.AddChild(event.Root().Copy())
	body, _ := soap(cancellationNS, batch).WriteToBytes()
	sum := sha256.Sum256(body)
	req := RecordedRequest{OperationID: "cancel-one", Scope: "synthetic-cancel", URL: "https://homologacao.example.gov.br/NFeRecepcaoEvento4", Key: in.Key, Issuer: in.CNPJ, Environment: "2", RequestSHA256: hex.EncodeToString(sum[:]), Body: body}
	response := cancellationResponse(in.Key, "1", "135")
	sum = sha256.Sum256(response)
	return req, ExchangeResult{OperationID: req.OperationID, State: "response_received", HTTPStatus: 200, SHA256: hex.EncodeToString(sum[:]), Body: response}
}
func TestFiscalCancellationMTLSOriginalJournalAndRecovery(t *testing.T) {
	for _, status := range []string{"135", "155", "136", "573"} {
		t.Run(status, func(t *testing.T) {
			c := testCredential(t)
			files, err := newIssuanceTestFileRecorder(t)
			if err != nil {
				t.Fatal(err)
			}
			r := &cancellationJournal{FileRecorder: files, operation: "cancel-one"}
			var calls atomic.Int32
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls.Add(1)
				stored, _, err := files.Read(r.operation)
				if err != nil {
					t.Error("missing durable request", err)
					w.WriteHeader(500)
					return
				}
				raw, _ := io.ReadAll(req.Body)
				if !bytes.Equal(raw, stored.Body) || !strings.Contains(req.Header.Get("Content-Type"), "nfeRecepcaoEvento") {
					t.Error("wire mismatch")
				}
				original, err := OriginalCancellationFromRequest(stored)
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				fmt.Fprintf(w, `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><nfeResultMsg xmlns="%s">%s</nfeResultMsg></soap:Body></soap:Envelope>`, cancellationNS, cancellationResponse(original.Input.Key, original.Lot, status))
			}))
			clientRoots := x509.NewCertPool()
			clientRoots.AddCert(c.certificate)
			server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots, MinVersion: tls.VersionTLS12}
			server.StartTLS()
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client, err := newClient(server.URL, "synthetic-cancel", c, r, roots, true)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			proof, err := client.CancelHomologationNFe(context.Background(), r.operation, "1", cancellationInput(t, c))
			if err != nil {
				t.Fatal(err)
			}
			accepted := status == "135" || status == "155"
			if (proof.State == "cancellation_indicated_homologation") != accepted || (len(proof.ProcessedXML) > 0) != accepted {
				t.Fatal("wrong disposition", proof.State)
			}
			if accepted && ValidateXML("nfe_event_process", proof.ProcessedXML) != nil {
				t.Fatal("invalid process")
			}
			duplicate, err := client.CancelHomologationNFe(context.Background(), r.operation, "2", cancellationInput(t, c))
			if !errors.Is(err, ErrRecorded) || calls.Load() != 1 || !bytes.Equal(duplicate.SignedXML, proof.SignedXML) || duplicate.Exchange.OperationID != r.operation {
				t.Fatal("duplicate changed/resubmitted", err)
			}
			server.Close()
			reopened, err := NewFileRecorder(files.directory)
			if err != nil {
				t.Fatal(err)
			}
			recovery := &cancellationJournal{FileRecorder: reopened, operation: r.operation}
			after, err := RecoverHomologationCancellation(context.Background(), recovery)
			if err != nil || !bytes.Equal(after.ProcessedXML, proof.ProcessedXML) || after.EventID != proof.EventID {
				t.Fatal("offline recovery", err)
			}
		})
	}
}

func TestFiscalCancellationRejectsMixedOrTamperedEvidence(t *testing.T) {
	base, answer := cancellationRecord(t, testCredential(t))
	mutations := map[string]func(*RecordedRequest, *ExchangeResult){
		"operation":     func(q *RecordedRequest, r *ExchangeResult) { r.OperationID = "different" },
		"request_hash":  func(q *RecordedRequest, r *ExchangeResult) { q.RequestSHA256 = "wrong" },
		"response_hash": func(q *RecordedRequest, r *ExchangeResult) { r.SHA256 = "wrong" },
		"signature": func(q *RecordedRequest, r *ExchangeResult) {
			q.Body = bytes.Replace(q.Body, []byte("fixture only"), []byte("altered only"), 1)
			s := sha256.Sum256(q.Body)
			q.RequestSHA256 = hex.EncodeToString(s[:])
		},
		"production": func(q *RecordedRequest, r *ExchangeResult) { q.Environment = "1" },
		"http_error": func(q *RecordedRequest, r *ExchangeResult) { r.HTTPStatus = 500 },
		"uncertain":  func(q *RecordedRequest, r *ExchangeResult) { r.State = "uncertain" },
		"lot": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<idLote>1</idLote>"), []byte("<idLote>2</idLote>"), 1)
		},
		"environment": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.ReplaceAll(r.Body, []byte("<tpAmb>2</tpAmb>"), []byte("<tpAmb>1</tpAmb>"))
		},
		"org": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.ReplaceAll(r.Body, []byte("<cOrgao>35</cOrgao>"), []byte("<cOrgao>33</cOrgao>"))
		},
		"event": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("110111"), []byte("110110"), 1)
		},
		"sequence": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<nSeqEvento>1</nSeqEvento>"), []byte("<nSeqEvento>2</nSeqEvento>"), 1)
		},
		"missing_protocol": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<nProt>135260000000002</nProt>"), nil, 1)
		},
		"fault": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = []byte(`<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><soap:Fault><detail>` + string(r.Body) + `</detail></soap:Fault></soap:Body></soap:Envelope>`)
		},
		"foreign_key": func(q *RecordedRequest, r *ExchangeResult) {
			k := testInvoice().Key
			k.Number++
			key, _ := AccessKey(k)
			r.Body = bytes.ReplaceAll(r.Body, []byte(q.Key), []byte(key))
		},
		"protocol_identity": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<infEvento>"), []byte(`<infEvento Id="ID135260000000999">`), 1)
		},
		"registration_before_event": func(q *RecordedRequest, r *ExchangeResult) {
			d, _ := parseXML(r.Body)
			d.Root().FindElement("retEvento/infEvento/dhRegEvento").SetText(time.Now().Add(-2 * time.Hour).Format("2006-01-02T15:04:05-07:00"))
			r.Body, _ = d.WriteToBytes()
		},
		"duplicate_event": func(q *RecordedRequest, r *ExchangeResult) {
			d, _ := parseXML(r.Body)
			d.Root().AddChild(d.Root().SelectElement("retEvento").Copy())
			r.Body, _ = d.WriteToBytes()
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			req, res := base, answer
			req.Body = append([]byte(nil), base.Body...)
			res.Body = append([]byte(nil), answer.Body...)
			mutate(&req, &res)
			if name != "response_hash" {
				s := sha256.Sum256(res.Body)
				res.SHA256 = hex.EncodeToString(s[:])
			}
			out, err := RecoverHomologationCancellation(context.Background(), &substitutedCancellationJournal{request: req, result: res})
			if err == nil || len(out.ProcessedXML) > 0 {
				t.Fatal("invalid cancellation accepted", out.State, err)
			}
		})
	}
}

type substitutedCancellationJournal struct {
	request RecordedRequest
	result  ExchangeResult
}

func (*substitutedCancellationJournal) Reserve(context.Context, RecordedRequest) error {
	return ErrUnsupported
}
func (*substitutedCancellationJournal) Complete(context.Context, ExchangeResult) error {
	return ErrUnsupported
}
func (*substitutedCancellationJournal) BeforeSend(context.Context, string) error {
	return ErrUnsupported
}
func (*substitutedCancellationJournal) ValidateCancellation(context.Context, Cancellation) error {
	return ErrUnsupported
}
func (r *substitutedCancellationJournal) Read(context.Context) (RecordedRequest, ExchangeResult, error) {
	return r.request, r.result, nil
}

func TestFiscalCancellationGuardAndProductionRefusal(t *testing.T) {
	c := testCredential(t)
	files, err := newIssuanceTestFileRecorder(t)
	if err != nil {
		t.Fatal(err)
	}
	r := &cancellationJournal{FileRecorder: files, operation: "cancel-one", validation: ErrCredential}
	roots := x509.NewCertPool()
	client, err := newClient("https://homologacao.example.gov.br/event", "synthetic-cancel", c, r, roots, false)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	in := cancellationInput(t, c)
	if _, err = client.CancelHomologationNFe(context.Background(), r.operation, "1", in); !errors.Is(err, ErrCredential) {
		t.Fatal(err)
	}
	r.validation = nil
	r.errorGuard = ErrUnsupported
	if _, err = client.CancelHomologationNFe(context.Background(), r.operation, "1", in); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if _, _, err = files.Read(r.operation); err != nil {
		t.Fatal("guard lost request", err)
	}
	in.Environment = 1
	if _, err = client.CancelHomologationNFe(context.Background(), "cancel-two", "1", in); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
}
