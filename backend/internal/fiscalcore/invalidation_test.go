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
)

type invalidationJournal struct {
	*FileRecorder
	operation string
	guard     error
}

func (r *invalidationJournal) ValidateInvalidation(context.Context, Invalidation) error { return nil }
func (r *invalidationJournal) BeforeSend(context.Context, string) error                 { return r.guard }
func (r *invalidationJournal) Read(context.Context) (RecordedRequest, ExchangeResult, error) {
	return r.FileRecorder.Read(r.operation)
}
func invalidationFixture(c *Credential) Invalidation {
	return Invalidation{UF: "SP", CNPJ: c.document, Model: "65", Series: 1, Year: 2026, First: 2, Last: 5, Environment: 2, Reason: "Synthetic unused range for tests only"}
}
func invalidationResponse(in Invalidation, status string) []byte {
	return []byte(fmt.Sprintf(`<retInutNFe xmlns="%s" versao="4.00"><infInut><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><cStat>%s</cStat><xMotivo>Synthetic invalidation result only</xMotivo><cUF>%s</cUF><ano>%02d</ano><CNPJ>%s</CNPJ><mod>%s</mod><serie>%d</serie><nNFIni>%d</nNFIni><nNFFin>%d</nNFFin><dhRecbto>%s</dhRecbto><nProt>135260000000003</nProt></infInut></retInutNFe>`, NFeNS, status, ufCodes[in.UF], in.Year%100, in.CNPJ, in.Model, in.Series, in.First, in.Last, time.Now().Format("2006-01-02T15:04:05-07:00")))
}
func TestFiscalInvalidationMTLSJournalProcessAndOfflineRecovery(t *testing.T) {
	for _, status := range []string{"102", "563", "201"} {
		t.Run(status, func(t *testing.T) {
			c := testCredential(t)
			in := invalidationFixture(c)
			files, err := newIssuanceTestFileRecorder(t)
			if err != nil {
				t.Fatal(err)
			}
			r := &invalidationJournal{FileRecorder: files, operation: "invalidate-one"}
			var calls atomic.Int32
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				stored, _, err := files.Read(r.operation)
				if err != nil {
					t.Error("missing reservation", err)
					w.WriteHeader(500)
					return
				}
				raw, _ := io.ReadAll(request.Body)
				if !bytes.Equal(raw, stored.Body) || !strings.Contains(request.Header.Get("Content-Type"), "nfeInutilizacaoNF") {
					t.Error("wire mismatch")
				}
				original, err := OriginalInvalidationFromRequest(stored)
				if err != nil || original.Input != in {
					t.Error("wrong original", err)
					w.WriteHeader(500)
					return
				}
				fmt.Fprintf(w, `<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><nfeResultMsg xmlns="%s">%s</nfeResultMsg></soap:Body></soap:Envelope>`, invalidationNS, invalidationResponse(in, status))
			}))
			clients := x509.NewCertPool()
			clients.AddCert(c.certificate)
			server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients}
			server.StartTLS()
			defer server.Close()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			client, err := newClient(server.URL, "synthetic-invalidation", c, r, roots, true)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			proof, err := client.InvalidateHomologation(context.Background(), r.operation, in)
			if err != nil {
				t.Fatal(err)
			}
			accepted := status == "102"
			if (proof.State == "invalidation_indicated_homologation") != accepted || (len(proof.ProcessedXML) > 0) != accepted {
				t.Fatal(proof.State)
			}
			if accepted && ValidateXML("nfe_invalidation_process", proof.ProcessedXML) != nil {
				t.Fatal("invalid process")
			}
			duplicate, err := client.InvalidateHomologation(context.Background(), r.operation, in)
			if !errors.Is(err, ErrRecorded) || calls.Load() != 1 || duplicate.Exchange.OperationID != r.operation || !bytes.Equal(duplicate.SignedXML, proof.SignedXML) {
				t.Fatal("duplicate reissued/rewritten", err)
			}
			server.Close()
			reopened, err := NewFileRecorder(files.directory)
			if err != nil {
				t.Fatal(err)
			}
			after, err := RecoverHomologationInvalidation(context.Background(), &invalidationJournal{FileRecorder: reopened, operation: r.operation})
			if err != nil || !bytes.Equal(after.ProcessedXML, proof.ProcessedXML) || after.ID != proof.ID {
				t.Fatal("offline recovery", err)
			}
		})
	}
}
func TestFiscalInvalidationIdentityBoundsAndIssuerSignature(t *testing.T) {
	for _, document := range []string{"12ABC34501DE35", "11222333000181"} {
		c := testCredentialDocument(t, document)
		in := invalidationFixture(c)
		raw, id, err := BuildInvalidation(in)
		if err != nil || len(id) != 43 {
			t.Fatal(id, err)
		}
		signed, err := c.SignXML("nfe_invalidation", raw, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err = c.VerifyXML("nfe_invalidation", signed, time.Now()); err != nil {
			t.Fatal(err)
		}
		bad := bytes.Replace(raw, []byte(`<nNFFin>5</nNFFin>`), []byte(`<nNFFin>6</nNFFin>`), 1)
		if _, err = c.SignXML("nfe_invalidation", bad, time.Now()); err == nil {
			t.Fatal("ID not bound to exact range")
		}
	}
	base := invalidationFixture(testCredential(t))
	for name, mutate := range map[string]func(*Invalidation){"production": func(i *Invalidation) { i.Environment = 1 }, "range_order": func(i *Invalidation) { i.Last = 1 }, "range_limit": func(i *Invalidation) { i.Last = i.First + MaxInvalidationRange }, "zero": func(i *Invalidation) { i.First = 0 }, "max": func(i *Invalidation) { i.Last = 1000000000 }, "UF": func(i *Invalidation) { i.UF = "XX" }, "model": func(i *Invalidation) { i.Model = "57" }, "series": func(i *Invalidation) { i.Series = 999 }, "year": func(i *Invalidation) { i.Year = 2100 }, "reason": func(i *Invalidation) { i.Reason = "short" }} {
		t.Run(name, func(t *testing.T) {
			in := base
			mutate(&in)
			if _, _, err := BuildInvalidation(in); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}
func TestFiscalInvalidationRejectsMixedResponseAndOriginal(t *testing.T) {
	c := testCredential(t)
	in := invalidationFixture(c)
	raw, id, err := BuildInvalidation(in)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := c.SignXML("nfe_invalidation", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	d, _ := parseXML(signed)
	body, _ := soap(invalidationNS, d.Root()).WriteToBytes()
	sum := sha256.Sum256(body)
	base := RecordedRequest{OperationID: "invalidate-one", Scope: "synthetic-invalidation", URL: "https://homologacao.example.gov.br/inut", Issuer: in.CNPJ, Key: id, Environment: "2", Body: body, RequestSHA256: hex.EncodeToString(sum[:])}
	response := invalidationResponse(in, "102")
	sum = sha256.Sum256(response)
	result := ExchangeResult{OperationID: base.OperationID, State: "response_received", HTTPStatus: 200, Body: response, SHA256: hex.EncodeToString(sum[:])}
	for name, mutate := range map[string]func(*RecordedRequest, *ExchangeResult){
		"operation": func(q *RecordedRequest, r *ExchangeResult) { r.OperationID = "foreign" }, "request_hash": func(q *RecordedRequest, r *ExchangeResult) { q.RequestSHA256 = "wrong" }, "response_hash": func(q *RecordedRequest, r *ExchangeResult) { r.SHA256 = "wrong" }, "http": func(q *RecordedRequest, r *ExchangeResult) { r.HTTPStatus = 500 }, "uncertain": func(q *RecordedRequest, r *ExchangeResult) { r.State = "uncertain" },
		"environment": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<tpAmb>2</tpAmb>"), []byte("<tpAmb>1</tpAmb>"), 1)
		}, "range": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<nNFFin>5</nNFFin>"), []byte("<nNFFin>6</nNFFin>"), 1)
		}, "issuer": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte(in.CNPJ), []byte("11222333000181"), 1)
		}, "UF": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<cUF>35</cUF>"), []byte("<cUF>33</cUF>"), 1)
		}, "protocol": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = bytes.Replace(r.Body, []byte("<nProt>135260000000003</nProt>"), nil, 1)
		}, "signature": func(q *RecordedRequest, r *ExchangeResult) {
			q.Body = bytes.Replace(q.Body, []byte("tests only"), []byte("tampered!!"), 1)
			s := sha256.Sum256(q.Body)
			q.RequestSHA256 = hex.EncodeToString(s[:])
		}, "fault": func(q *RecordedRequest, r *ExchangeResult) {
			r.Body = []byte(`<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><soap:Fault><detail>` + string(r.Body) + `</detail></soap:Fault></soap:Body></soap:Envelope>`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			q, r := base, result
			q.Body = append([]byte(nil), q.Body...)
			r.Body = append([]byte(nil), r.Body...)
			mutate(&q, &r)
			if name != "response_hash" {
				s := sha256.Sum256(r.Body)
				r.SHA256 = hex.EncodeToString(s[:])
			}
			out, err := RecoverHomologationInvalidation(context.Background(), &substitutedInvalidationJournal{request: q, result: r})
			if err == nil || len(out.ProcessedXML) > 0 {
				t.Fatal("invalid evidence accepted", out.State, err)
			}
		})
	}
}

type substitutedInvalidationJournal struct {
	request RecordedRequest
	result  ExchangeResult
}

func (*substitutedInvalidationJournal) Reserve(context.Context, RecordedRequest) error {
	return ErrUnsupported
}
func (*substitutedInvalidationJournal) Complete(context.Context, ExchangeResult) error {
	return ErrUnsupported
}
func (*substitutedInvalidationJournal) BeforeSend(context.Context, string) error {
	return ErrUnsupported
}
func (*substitutedInvalidationJournal) ValidateInvalidation(context.Context, Invalidation) error {
	return ErrUnsupported
}
func (r *substitutedInvalidationJournal) Read(context.Context) (RecordedRequest, ExchangeResult, error) {
	return r.request, r.result, nil
}

func TestFiscalInvalidationGuardKeepsUnsentOriginal(t *testing.T) {
	c := testCredential(t)
	files, err := newIssuanceTestFileRecorder(t)
	if err != nil {
		t.Fatal(err)
	}
	r := &invalidationJournal{FileRecorder: files, operation: "invalidate-one", guard: ErrCredential}
	roots := x509.NewCertPool()
	client, err := newClient("https://homologacao.example.gov.br/inut", "synthetic-invalidation", c, r, roots, false)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err = client.InvalidateHomologation(context.Background(), r.operation, invalidationFixture(c)); !errors.Is(err, ErrCredential) {
		t.Fatal(err)
	}
	req, res, err := files.Read(r.operation)
	if err != nil || req.OperationID != r.operation || res.State != "uncertain" {
		t.Fatal("guard lost original", err)
	}
	in := invalidationFixture(c)
	in.Environment = 1
	if _, err = client.InvalidateHomologation(context.Background(), "production-attempt", in); !errors.Is(err, ErrUnsupported) {
		t.Fatal("production permitted", err)
	}
}
