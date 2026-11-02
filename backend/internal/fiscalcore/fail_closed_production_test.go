package fiscalcore

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

type failClosedTransport func(*http.Request) (*http.Response, error)

func (f failClosedTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProductionSubmissionRefusesGenericFileJournalBeforeReserveOrNetwork(t *testing.T) {
	credential := testCredential(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	recorder, err := NewFileRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	client, err := newClient("https://synthetic.fiscal.example.gov.br/issue", "synthetic-scope", credential, recorder, x509.NewCertPool(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	calls := 0
	client.http.Transport = failClosedTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("network must remain unused")
	})
	invoice := testInvoice()
	invoice.Environment = 1
	raw, _, err := BuildInvoice(invoice)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := credential.SignXML("nfe", raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.SubmitNFe(context.Background(), "blocked-issue", "1", signed); !errors.Is(err, ErrUnsupported) {
		t.Fatal("generic journal authorized production invoice", err)
	}
	key, err := AccessKey(invoice.Key)
	if err != nil {
		t.Fatal(err)
	}
	event, err := BuildCancellation(Cancellation{Key: key, CNPJ: credential.document, Environment: 1, Time: time.Now(), Protocol: "135260000000001", Reason: "Synthetic cancellation for refusal test"})
	if err != nil {
		t.Fatal(err)
	}
	signed, err = credential.SignXML("nfe_event", event, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.SubmitCancellation(context.Background(), "blocked-cancel", "1", signed); !errors.Is(err, ErrUnsupported) {
		t.Fatal("generic journal authorized production cancellation", err)
	}
	if calls != 0 {
		t.Fatal("unprovisioned production reached transport")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("unprovisioned production reached journal", err)
	}
}
