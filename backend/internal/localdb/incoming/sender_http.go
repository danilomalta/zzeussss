package incoming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"time"

	"titansystem-backend/internal/localdb/outgoing"
)

var ErrTransport = errors.New("transporte local indisponivel ou resposta invalida")

const receiptHTTPBodyLimit = 8 * 1024

// PostSealedHTTP performs one bounded attempt and returns a VERIFIED receipt.
// It does not modify a database, acknowledge an outbox, follow redirects or
// retry automatically. The caller must acquire trust from approved local
// configuration and confirm through outgoing.Confirm after successful return.
func PostSealedHTTP(ctx context.Context, configured *http.Client, endpoint string, envelope SealedEnvelope, event outgoing.Event, verifier *ReceiptVerifier) (HTTPReceiptResult, error) {
	if ctx == nil || verifier == nil || verifier.destination != envelope.Destination || envelope.Version != 1 {
		return HTTPReceiptResult{}, ErrInvalid
	}
	if _, err := meta(Message{Event: event, Destination: envelope.Destination}); err != nil {
		return HTTPReceiptResult{}, err
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Path != SealedReceiverPath || u.RawPath != "" {
		return HTTPReceiptResult{}, ErrInvalid
	}
	switch u.Scheme {
	case "https":
	case "http":
		// Only literal loopback IPs are accepted without TLS; no DNS aliases.
		ip := net.ParseIP(u.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return HTTPReceiptResult{}, ErrInvalid
		}
	default:
		return HTTPReceiptResult{}, ErrInvalid
	}
	body, err := json.Marshal(envelope)
	if err != nil || len(body) > sealedHTTPBodyLimit {
		return HTTPReceiptResult{}, ErrInvalid
	}
	if _, err := decodeHTTPEnvelope(body); err != nil {
		return HTTPReceiptResult{}, ErrInvalid
	}
	client := http.Client{}
	if configured != nil {
		client = *configured
		if transport, ok := client.Transport.(*http.Transport); ok && transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
			return HTTPReceiptResult{}, ErrInvalid
		}
	}
	if client.Timeout <= 0 || client.Timeout > 10*time.Second {
		client.Timeout = 10 * time.Second
	}
	client.Jar = nil
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	attempt, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(attempt, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return HTTPReceiptResult{}, ErrInvalid
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(request)
	if err != nil {
		return HTTPReceiptResult{}, ErrTransport
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return HTTPReceiptResult{}, ErrTransport
	}
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return HTTPReceiptResult{}, ErrTransport
	}
	if response.Header.Get("Content-Encoding") != "" {
		return HTTPReceiptResult{}, ErrTransport
	}
	if response.ContentLength > receiptHTTPBodyLimit {
		return HTTPReceiptResult{}, ErrTransport
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, receiptHTTPBodyLimit+1))
	if err != nil || len(raw) > receiptHTTPBodyLimit {
		return HTTPReceiptResult{}, ErrTransport
	}
	result, err := decodeHTTPReceipt(raw)
	if err != nil {
		return HTTPReceiptResult{}, ErrTransport
	}
	if (response.StatusCode == http.StatusOK) != result.Repeated {
		return HTTPReceiptResult{}, ErrTransport
	}
	if err := verifier.Verify(attempt, event, result.Receipt); err != nil {
		return HTTPReceiptResult{}, ErrDenied
	}
	return result, nil
}

func decodeHTTPReceipt(raw []byte) (HTTPReceiptResult, error) {
	fields, err := exactKeyObject(raw, []string{"receipt", "repeated"})
	if err != nil {
		return HTTPReceiptResult{}, ErrInvalid
	}
	if _, err := exactKeyObject(fields["receipt"], []string{"ReceiptID", "EventID", "TenantID", "StoreID", "DeviceID", "PayloadSHA256", "Proof"}); err != nil {
		return HTTPReceiptResult{}, ErrInvalid
	}
	var result HTTPReceiptResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return HTTPReceiptResult{}, ErrInvalid
	}
	return result, nil
}
