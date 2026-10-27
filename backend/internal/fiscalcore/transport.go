package fiscalcore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/beevik/etree"
)

type RecordedRequest struct {
	OperationID   string `json:"operation_id"`
	Scope         string `json:"scope"`
	URL           string `json:"url"`
	Issuer        string `json:"issuer"`
	Environment   string `json:"environment"`
	Key           string `json:"key"`
	RequestSHA256 string `json:"request_sha256"`
	Body          []byte `json:"body"`
}
type ExchangeResult struct {
	OperationID string `json:"operation_id"`
	HTTPStatus  int    `json:"http_status"`
	SHA256      string `json:"sha256"`
	Body        []byte `json:"body"`
	State       string `json:"state"` // response_received or uncertain; never authorized
}

// Recorder must durably reserve the original request BEFORE any network I/O.
// A duplicate ID must fail, never silently initiate another transmission.
// This lower-level component is not the application's SQLite fiscal outbox.
type Recorder interface {
	Reserve(context.Context, RecordedRequest) error
	Complete(context.Context, ExchangeResult) error
}

type Client struct {
	endpoint   *url.URL
	http       *http.Client
	credential *Credential
	recorder   Recorder
	scope      string
}

// NewClient accepts installation-controlled official endpoints only. It never
// follows redirects, inherits an environment proxy, disables TLS validation or
// retries a POST. Endpoint selection/vigência and client authorization belong to
// the trusted application configuration; a browser must not supply this URL.
func NewClient(endpoint, scope string, credential *Credential, recorder Recorder) (*Client, error) {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, ErrCredential
	}
	return newClient(endpoint, scope, credential, recorder, roots, false)
}
func newClient(endpoint, scope string, c *Credential, recorder Recorder, serverRoots *x509.CertPool, loopbackTest bool) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Host == "" || scope == "" || len(scope) > 128 || recorder == nil || serverRoots == nil || !c.current(time.Now()) {
		return nil, ErrInput
	}
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".gov.br") && !(loopbackTest && net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback()) {
		return nil, ErrInput
	}
	transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: serverRoots, Certificates: []tls.Certificate{{Certificate: c.chain, PrivateKey: c.signer}}}, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 20 * time.Second, MaxResponseHeaderBytes: 32 << 10}
	return &Client{endpoint: u, http: &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, credential: c, recorder: recorder, scope: scope}, nil
}
func (c *Client) Close() {
	if c != nil && c.http != nil {
		c.http.CloseIdleConnections()
	}
}

func (c *Client) exchange(ctx context.Context, id, key, environment, contentType string, body []byte) (ExchangeResult, error) {
	var out ExchangeResult
	if id == "" || len(id) > 128 || len(body) > 2*MaxXML || len(body) == 0 || (environment != "1" && environment != "2") {
		return out, ErrInput
	}
	sum := sha256.Sum256(body)
	record := RecordedRequest{OperationID: id, Scope: c.scope, URL: c.endpoint.String(), Issuer: c.credential.document, Environment: environment, Key: key, RequestSHA256: hex.EncodeToString(sum[:]), Body: append([]byte(nil), body...)}
	if err := c.recorder.Reserve(ctx, record); err != nil {
		return out, err
	}
	out = ExchangeResult{OperationID: id, State: "uncertain"}
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return out, ErrUncertain
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept-Encoding", "identity")
	response, err := c.http.Do(request)
	if err != nil {
		_ = c.recorder.Complete(context.WithoutCancel(ctx), out)
		return out, ErrUncertain
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 2*MaxXML+1))
	out.HTTPStatus = response.StatusCode
	if err != nil || len(raw) > 2*MaxXML {
		_ = c.recorder.Complete(context.WithoutCancel(ctx), out)
		return out, ErrUncertain
	}
	digest := sha256.Sum256(raw)
	out.Body = raw
	out.SHA256 = hex.EncodeToString(digest[:])
	out.State = "response_received"
	if err = c.recorder.Complete(context.WithoutCancel(ctx), out); err != nil {
		out.State = "uncertain"
		return out, ErrUncertain
	}
	if response.StatusCode != 200 {
		return out, ErrUncertain
	}
	return out, nil
}

// SubmitNFe validates/signature-checks the exact original document and writes
// its SOAP request through Recorder before attempting its one HTTP POST.
func (c *Client) SubmitNFe(ctx context.Context, operationID, lot string, signed []byte) (ExchangeResult, error) {
	if len(lot) < 1 || len(lot) > 15 || !digitsRE.MatchString(lot) {
		return ExchangeResult{}, ErrInput
	}
	if err := c.credential.VerifyXML("nfe", signed, time.Now()); err != nil {
		return ExchangeResult{}, err
	}
	doc, _ := parseXML(signed)
	info, _ := payload(doc.Root(), "nfe")
	key := strings.TrimPrefix(info.SelectAttrValue("Id", ""), "NFe")
	env := info.FindElement("ide/tpAmb").Text()
	batch := etree.NewDocument()
	root := batch.CreateElement("enviNFe")
	root.CreateAttr("xmlns", NFeNS)
	root.CreateAttr("versao", "4.00")
	field(root, "idLote", lot)
	field(root, "indSinc", "1")
	root.AddChild(doc.Root().Copy())
	namespace := NFeNS + "/wsdl/NFeAutorizacao4"
	envelope := soap(namespace, root)
	raw, err := envelope.WriteToBytes()
	if err != nil {
		return ExchangeResult{}, ErrXML
	}
	return c.exchange(ctx, operationID, key, env, `application/soap+xml; charset=utf-8; action="`+namespace+`/nfeAutorizacaoLote"`, raw)
}
func soap(namespace string, data *etree.Element) *etree.Document {
	d := etree.NewDocument()
	root := d.CreateElement("soap:Envelope")
	root.CreateAttr("xmlns:soap", "http://www.w3.org/2003/05/soap-envelope")
	body := root.CreateElement("soap:Body")
	msg := body.CreateElement("nfeDadosMsg")
	msg.CreateAttr("xmlns", namespace)
	msg.AddChild(data)
	return d
}
func (c *Client) SubmitCancellation(ctx context.Context, id, lot string, signed []byte) (ExchangeResult, error) {
	if len(lot) < 1 || len(lot) > 15 || !digitsRE.MatchString(lot) {
		return ExchangeResult{}, ErrInput
	}
	if err := c.credential.VerifyXML("nfe_event", signed, time.Now()); err != nil {
		return ExchangeResult{}, err
	}
	d, _ := parseXML(signed)
	info, _ := payload(d.Root(), "nfe_event")
	key := info.FindElement("chNFe").Text()
	env := info.FindElement("tpAmb").Text()
	if info.FindElement("tpEvento").Text() != "110111" || !ValidAccessKey(key) || key[6:20] != c.credential.document {
		return ExchangeResult{}, ErrInput
	}
	batch := etree.NewDocument()
	root := batch.CreateElement("envEvento")
	root.CreateAttr("xmlns", NFeNS)
	root.CreateAttr("versao", "1.00")
	field(root, "idLote", lot)
	root.AddChild(d.Root().Copy())
	namespace := NFeNS + "/wsdl/NFeRecepcaoEvento4"
	raw, err := soap(namespace, root).WriteToBytes()
	if err != nil {
		return ExchangeResult{}, ErrXML
	}
	return c.exchange(ctx, id, key, env, `application/soap+xml; charset=utf-8; action="`+namespace+`/nfeRecepcaoEvento"`, raw)
}
func (c *Client) SubmitDPS(ctx context.Context, id string, signed []byte) (ExchangeResult, error) {
	raw, err := c.credential.PackDPS(signed, time.Now())
	if err != nil {
		return ExchangeResult{}, err
	}
	d, _ := parseXML(signed)
	info, _ := payload(d.Root(), "nfse_dps")
	return c.exchange(ctx, id, info.SelectAttrValue("Id", ""), info.FindElement("tpAmb").Text(), "application/json; charset=utf-8", raw)
}

type Protocol struct {
	Key                    string `json:"key"`
	Environment            string `json:"environment"`
	Status                 string `json:"status"`
	Number                 string `json:"number"`
	ReceivedAt             string `json:"received_at"`
	Digest                 string `json:"digest"`
	AuthorizationIndicated bool   `json:"authorization_indicated"`
}

// ParseProtocol requires an exact key/environment/digest association. Only call
// on an authenticated official transport response, not browser-uploaded XML.
// Uncertain/duplicate/rejected/pending responses are not authorization evidence.
func ParseProtocol(raw []byte, key, environment, digest string) (Protocol, error) {
	var out Protocol
	if !ValidAccessKey(key) || (environment != "1" && environment != "2") {
		return out, ErrInput
	}
	if decoded, err := base64.StdEncoding.DecodeString(digest); err != nil || len(decoded) != 20 {
		return out, ErrInput
	}
	d, err := parseXML(raw)
	if err != nil {
		return out, ErrUncertain
	}
	root := d.Root()
	// Only an authorization response in the exact SOAP 1.2 response shape is
	// accepted; a nested infProt inside Fault/detail must never grant success.
	if root.Tag == "Envelope" && root.NamespaceURI() == "http://www.w3.org/2003/05/soap-envelope" {
		children := root.ChildElements()
		if len(children) != 1 || children[0].Tag != "Body" || children[0].NamespaceURI() != root.NamespaceURI() {
			return out, ErrUncertain
		}
		children = children[0].ChildElements()
		if len(children) != 1 {
			return out, ErrUncertain
		}
		root = children[0]
		if root.Tag != "nfeResultMsg" || root.NamespaceURI() != NFeNS+"/wsdl/NFeAutorizacao4" {
			return out, ErrUncertain
		}
		children = root.ChildElements()
		if len(children) != 1 {
			return out, ErrUncertain
		}
		root = children[0]
	}
	if root.Tag != "retEnviNFe" || root.NamespaceURI() != NFeNS || root.SelectAttrValue("versao", "") != "4.00" {
		return out, ErrUncertain
	}
	var found *etree.Element
	seen := map[string]bool{}
	for _, child := range root.ChildElements() {
		if child.NamespaceURI() != NFeNS || seen[child.Tag] {
			return out, ErrUncertain
		}
		seen[child.Tag] = true
		switch child.Tag {
		case "tpAmb":
			if child.Text() != environment {
				return out, ErrUncertain
			}
		case "cStat":
			if child.Text() != "104" {
				return out, ErrUncertain
			}
		case "protNFe":
			if child.SelectAttrValue("versao", "") != "4.00" || len(child.ChildElements()) != 1 {
				return out, ErrUncertain
			}
			found = child.ChildElements()[0]
			if found.Tag != "infProt" || found.NamespaceURI() != NFeNS {
				return out, ErrUncertain
			}
		case "verAplic", "cUF", "xMotivo", "dhRecbto":
		default:
			return out, ErrUncertain
		}
	}
	if !seen["tpAmb"] || !seen["cStat"] || found == nil {
		return out, ErrUncertain
	}
	get := func(name string) string {
		var value string
		n := 0
		for _, e := range found.ChildElements() {
			if e.Tag == name && e.NamespaceURI() == NFeNS {
				n++
				value = e.Text()
			}
		}
		if n != 1 {
			return ""
		}
		return value
	}
	out = Protocol{Key: get("chNFe"), Environment: get("tpAmb"), Status: get("cStat"), Number: get("nProt"), ReceivedAt: get("dhRecbto"), Digest: get("digVal")}
	if out.Key != key || out.Environment != environment || out.Digest != digest || (out.Status != "100" && out.Status != "150") || (len(out.Number) != 15 && len(out.Number) != 17) || !digitsRE.MatchString(out.Number) {
		return Protocol{}, ErrUncertain
	}
	if _, err = time.Parse(time.RFC3339, out.ReceivedAt); err != nil {
		return Protocol{}, ErrUncertain
	}
	out.AuthorizationIndicated = true
	return out, nil
}

// NFSeResponseXML extracts only the documented compressed XML response member;
// it never treats an HTTP status or access-key string as an authorized document.
func NFSeResponseXML(raw []byte) ([]byte, error) {
	if len(raw) > 2*MaxXML {
		return nil, ErrInput
	}
	if uniqueJSON(raw) != nil {
		return nil, ErrUncertain
	}
	var payload struct {
		XML string `json:"nfseXmlGZipB64"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.XML == "" {
		return nil, ErrUncertain
	}
	return UnpackNFSe(payload.XML)
}
