package fiscalcore

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This regression records a real interoperability gap, not a successful DPS.
// The pinned official TSSerieDPS contains ^/$; our strict XSD validator treats
// them literally. Do not weaken the official schema or sign/send a bypassed DPS.
func TestFiscalDPSKnownSchemaCompatibilityGapFailsClosed(t *testing.T) {
	c := testCredentialDocument(t, "11222333000181")
	in := DPSInput{Environment: 2, IssuedAt: time.Now(), Competence: "2026-10-26", Municipality: "3550308", ServiceMunicipality: "3550308", CNPJ: c.document, Series: 1, Number: 1, SimpleOption: "1", SpecialRegime: "0", NationalServiceCode: "010101", Description: "Synthetic service only <test>", ServiceValue: "100.00", Taxation: []TaxNode{
		{Name: "tribMun", Children: []TaxNode{{Name: "tribISSQN", Value: "1"}, {Name: "tpRetISSQN", Value: "1"}}},
		{Name: "totTrib", Children: []TaxNode{{Name: "indTotTrib", Value: "0"}}},
	}}
	raw, id, err := buildDPS(in)
	if err != nil || id == "" {
		t.Fatal("serialization", err)
	}
	doc, _ := helium.NewParser().Parse(context.Background(), raw)
	schema, _ := compileSchema("nfse_dps")
	collector := helium.NewErrorCollector(context.Background(), helium.ErrorLevelNone)
	err = xsd.NewValidator(schema).ErrorHandler(collector).Validate(context.Background(), doc)
	doc.Free()
	if err == nil || !strings.Contains(fmt.Sprint(collector.Errors()), "serie") {
		t.Fatalf("review known gap, diagnostics: %v", collector.Errors())
	}
	if _, _, err = BuildDPS(in); !errors.Is(err, ErrSchema) {
		t.Fatal("unvalidated DPS escaped", err)
	}
	if _, err = c.SignXML("nfse_dps", raw, time.Now()); !errors.Is(err, ErrSchema) {
		t.Fatal("bypassed schema to sign", err)
	}
	in.CNPJ = "12ABC34501DE35"
	if _, _, err = BuildDPS(in); !errors.Is(err, ErrUnsupported) {
		t.Fatal("numeric schema must not strip CNPJ letters", err)
	}
	if _, err = c.SignXML("nfse_event", raw, time.Now()); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported event signing", err)
	}
}

func TestFiscalCancellationUsesIssuerAndSpecificSchema(t *testing.T) {
	c := testCredential(t)
	key, _ := AccessKey(testInvoice().Key)
	in := Cancellation{Key: key, CNPJ: c.document, Environment: 2, Time: time.Now(), Protocol: "135260000000001", Reason: "Synthetic cancellation for test only"}
	raw, err := BuildCancellation(in)
	if err != nil {
		t.Fatal("build event", err)
	}
	signed, err := c.SignXML("nfe_event", raw, time.Now())
	if err != nil {
		t.Fatal("sign event", err)
	}
	if err = c.VerifyXML("nfe_event", signed, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Generic event schema has lax detail contents; the specific cancellation
	// schema must also be enforced when signing a caller-supplied event.
	bad := bytes.Replace(raw, []byte("Synthetic cancellation for test only"), []byte("short"), 1)
	if _, err = c.SignXML("nfe_event", bad, time.Now()); err == nil {
		t.Fatal("signed invalid cancellation detail")
	}
	bad = bytes.Replace(raw, []byte("<nSeqEvento>1</nSeqEvento>"), []byte("<nSeqEvento>2</nSeqEvento>"), 1)
	if _, err = c.SignXML("nfe_event", bad, time.Now()); err == nil {
		t.Fatal("signed inconsistent event identity")
	}
	in.CNPJ = "11222333000181"
	if _, err = BuildCancellation(in); err == nil {
		t.Fatal("foreign issuer")
	}
	in.CNPJ = c.document
	in.Reason = "short"
	if _, err = BuildCancellation(in); err == nil {
		t.Fatal("reason length")
	}
}

func TestFiscalQRVersionsOnlineOfflineAndPNG(t *testing.T) {
	c := testCredential(t)
	for _, emission := range []int{1, 9} {
		in := testInvoice()
		in.Key.Emission = emission
		if emission == 9 {
			in.ContingencyAt = in.Key.IssuedAt
			in.ContingencyReason = "Synthetic communication failure for test"
		}
		raw, _, err := BuildInvoice(in)
		if err != nil {
			t.Fatal(err)
		}
		signed, err := c.SignXML("nfe", raw, time.Now())
		if err != nil {
			t.Fatal("sign offline", err)
		}
		for _, version := range []int{2, 3} {
			config := QRConfiguration{BaseURL: "https://homologacao.nfce.fazenda.sp.gov.br/qrcode", Version: version, CSCID: "000001", CSC: "SYNTHETIC-NOT-A-REAL-CSC"}
			value, err := c.QRCode(config, signed, time.Now())
			if err != nil {
				t.Fatal(emission, version, err)
			}
			if strings.Contains(value, config.CSC) {
				t.Fatal("CSC exposed")
			}
			u, _ := url.Parse(value)
			parts := strings.Split(u.Query().Get("p"), "|")
			if version == 3 && emission == 9 {
				if len(parts) != 8 {
					t.Fatal(parts)
				}
				signature, err := base64.StdEncoding.DecodeString(parts[7])
				if err != nil {
					t.Fatal(err)
				}
				digest := sha1.Sum([]byte(strings.Join(parts[:7], "|")))
				if rsa.VerifyPKCS1v15(c.certificate.PublicKey.(*rsa.PublicKey), crypto.SHA1, digest[:], signature) != nil {
					t.Fatal("offline QR signature")
				}
			}
			image, err := QRPNG(value)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := png.Decode(bytes.NewReader(image))
			if err != nil || decoded.Bounds().Dx() < 100 {
				t.Fatal("PNG", err)
			}
			config.BaseURL = "https://example.com/qrcode"
			if _, err = c.QRCode(config, signed, time.Now()); err == nil {
				t.Fatal("untrusted consultation host")
			}
		}
	}
}

func TestFiscalMTLSDurableJournalNoAutomaticResend(t *testing.T) {
	c := testCredential(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	recorder, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.TLS == nil || len(r.TLS.PeerCertificates) != 1 {
			t.Error("missing mTLS client")
		}
		request, _, err := recorder.Read("synthetic-op-1")
		if err != nil || request.Key == "" {
			t.Error("POST preceded durable reserve", err)
		}
		w.Header().Set("Content-Type", "application/soap+xml")
		w.Write([]byte(`<synthetic-response/>`))
	}))
	clientRoots := x509.NewCertPool()
	clientRoots.AddCert(c.certificate)
	server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	client, err := newClient(server.URL, "tenant/store/device", c, recorder, roots, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	raw, _, _ := BuildInvoice(testInvoice())
	signed, err := c.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.SubmitNFe(context.Background(), "synthetic-op-1", "1", signed)
	if err != nil || result.State != "response_received" {
		t.Fatal(result.State, err)
	}
	if _, err = client.SubmitNFe(context.Background(), "synthetic-op-1", "1", signed); !errors.Is(err, ErrRecorded) {
		t.Fatal("duplicate re-POST", err)
	}
	reopened, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	request, stored, err := reopened.Read("synthetic-op-1")
	if err != nil || stored.SHA256 != result.SHA256 || len(request.Body) == 0 {
		t.Fatal("recovery", err)
	}
	server.Close()
	uncertain, err := client.SubmitNFe(context.Background(), "synthetic-op-2", "2", signed)
	if !errors.Is(err, ErrUncertain) || uncertain.OperationID != "synthetic-op-2" {
		t.Fatal("lost response", err)
	}
	_, stored, err = reopened.Read("synthetic-op-2")
	if err != nil || stored.State != "uncertain" {
		t.Fatal("uncertain not durable", err)
	}
	if _, err = client.SubmitNFe(context.Background(), "synthetic-op-2", "2", signed); !errors.Is(err, ErrRecorded) {
		t.Fatal("uncertain automatically resent", err)
	}
	if calls.Load() != 1 {
		t.Fatal("wrong POST count", calls.Load())
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		info, _ := entry.Info()
		if info.Mode().Perm() != 0600 {
			t.Fatal("journal permissions")
		}
	}
	if err = os.WriteFile(filepath.Join(dir, journalName("synthetic-op-1")+".response.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = reopened.Read("synthetic-op-1"); err == nil {
		t.Fatal("accepted corrupt recovery record")
	}
}

func TestFiscalProtocolExactAssociationAndNoSOAPFault(t *testing.T) {
	key, _ := AccessKey(testInvoice().Key)
	digest := base64.StdEncoding.EncodeToString(make([]byte, 20))
	raw := `<retEnviNFe xmlns="` + NFeNS + `" versao="4.00"><tpAmb>2</tpAmb><cStat>104</cStat><protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><chNFe>` + key + `</chNFe><dhRecbto>2026-10-26T12:00:00-03:00</dhRecbto><nProt>135260000000001</nProt><digVal>` + digest + `</digVal><cStat>100</cStat></infProt></protNFe></retEnviNFe>`
	p, err := ParseProtocol([]byte(raw), key, "2", digest)
	if err != nil || !p.AuthorizationIndicated {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(raw, "<cStat>100</cStat>", "<cStat>204</cStat>", 1), strings.Replace(raw, "<cStat>104</cStat>", "<cStat>105</cStat>", 1), `<wrapper>` + raw + `</wrapper>`, `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"><s:Body><s:Fault><s:Detail>` + raw + `</s:Detail></s:Fault></s:Body></s:Envelope>`} {
		if _, err = ParseProtocol([]byte(bad), key, "2", digest); !errors.Is(err, ErrUncertain) {
			t.Fatal("accepted pending/rejected/wrapped", err)
		}
	}
	if _, err = ParseProtocol([]byte(raw), key, "1", digest); err == nil {
		t.Fatal("wrong environment")
	}
}

type deniedSendRecorder struct {
	*FileRecorder
	calls int
}

func (r *deniedSendRecorder) BeforeSend(context.Context, string) error {
	r.calls++
	return ErrCredential
}
func TestFiscalTransportRechecksAuthorityAfterJournalAndBeforePOST(t *testing.T) {
	c := testCredential(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	disk, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	recorder := &deniedSendRecorder{FileRecorder: disk}
	var calls atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(200) }))
	clientRoots := x509.NewCertPool()
	clientRoots.AddCert(c.certificate)
	server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientRoots, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	client, err := newClient(server.URL, "synthetic-scope", c, recorder, roots, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	raw, _, err := BuildInvoice(testInvoice())
	if err != nil {
		t.Fatal(err)
	}
	signed, err := c.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.SubmitNFe(context.Background(), "guarded-op", "1", signed); !errors.Is(err, ErrCredential) {
		t.Fatal(err)
	}
	request, result, err := disk.Read("guarded-op")
	if err != nil || len(request.Body) == 0 || result.State != "uncertain" {
		t.Fatal("reservation lost", err)
	}
	if calls.Load() != 0 || recorder.calls != 1 {
		t.Fatal("HTTP occurred despite guard", calls.Load(), recorder.calls)
	}
	if _, err = client.SubmitNFe(context.Background(), "guarded-op", "1", signed); !errors.Is(err, ErrRecorded) {
		t.Fatal("guard refusal allowed retry", err)
	}
}
