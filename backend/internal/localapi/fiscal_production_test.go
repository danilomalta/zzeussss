package localapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beevik/etree"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/backup"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/testsupport/fiscalcert"
)

const productionTestHost = "synthetic.fiscal.example.gov.br"

type productionFixture struct {
	f                *stockHTTPFixture
	intent           fiscal.Intent
	input            fiscalcore.Invoice
	binding          fiscalcore.ProductionBinding
	runtime          *fiscalproduction.Runtime
	cert             *fiscalcert.Fixture
	authority        *httptest.Server
	mode             atomic.Value
	issues           atomic.Int32
	queries          atomic.Int32
	cancels          atomic.Int32
	crlGuard         atomic.Bool
	logoutGuard      atomic.Bool
	crlCalls         atomic.Int32
	lock             sync.Mutex
	lastSigned       []byte
	lastEvent        []byte
	lastCancellation []byte
	authorizedAt     string
}

func newProductionFixture(t *testing.T, model string, crt ...string) *productionFixture {
	t.Helper()
	p := &productionFixture{f: fiscalFixture(t, model)}
	p.mode.Store("authorize")
	body := strings.ReplaceAll(fiscalProfileBody, `"model":"65"`, `"model":"`+model+`"`)
	body = strings.ReplaceAll(body, `"issuer-create"`, `"production-profile"`)
	body = strings.ReplaceAll(body, `"expected_revision":0`, `"expected_revision":1`)
	body = strings.ReplaceAll(body, `"environment":"homologation"`, `"environment":"production"`)
	if len(crt) > 0 {
		body = strings.ReplaceAll(body, `"crt":"1"`, `"crt":"`+crt[0]+`"`)
	}
	if len(crt) > 1 {
		body = strings.TrimSuffix(body, "}") + `,"timezone":"` + crt[1] + `"}`
	}
	if len(crt) > 2 && crt[2] == "required" {
		body = strings.TrimSuffix(body, "}") + `,"require_tax_pricing":true}`
	}
	if code, b := request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", body, p.f.token); code != 201 {
		t.Fatalf("profile %d %s", code, b)
	}
	p.intent = fiscalIntent(t, p.f, strings.Replace(fiscalIntentBody, `"profile_revision":1`, `"profile_revision":2`, 1), 201)
	p.input = boundFiscalInvoice(p.intent)
	p.input.Environment = 1
	p.input.Key = fiscalcore.KeyInput{}
	a := &p.input.Issuer.Address
	a.Street = "Rua Sintetica"
	a.Number = "1"
	a.District = "Centro"
	a.City = "Sao Paulo"
	a.PostalCode = "01001000"
	if model == "55" {
		p.input.Recipient = &fiscalcore.Party{CNPJ: "11222333000181", Name: "Synthetic recipient", IEDestination: "9", Address: *a}
	}
	for i := range p.input.Lines {
		line := &p.input.Lines[i]
		line.Description = "Synthetic reviewed item"
		line.Unit = "UN"
		line.NCM = "09012100"
		line.CFOP = "5102"
		line.Taxes = []fiscalcore.TaxNode{{Name: "ICMS", Children: []fiscalcore.TaxNode{{Name: "ICMSSN102", Children: []fiscalcore.TaxNode{{Name: "orig", Value: "0"}, {Name: "CSOSN", Value: "102"}}}}}, {Name: "PIS", Children: []fiscalcore.TaxNode{{Name: "PISNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}, {Name: "COFINS", Children: []fiscalcore.TaxNode{{Name: "COFINSNT", Children: []fiscalcore.TaxNode{{Name: "CST", Value: "07"}}}}}}
	}
	var err error
	p.cert, err = fiscalcert.New(p.intent.Snapshot.Profile.CNPJ, productionTestHost)
	if err != nil {
		t.Fatal(err)
	}
	p.authority = httptest.NewUnstartedServer(http.HandlerFunc(p.authorityHandler))
	p.authority.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{p.cert.Server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: p.cert.Roots}
	p.authority.StartTLS()
	t.Cleanup(p.authority.Close)
	now := time.Now()
	p.binding = fiscalcore.ProductionBinding{ID: "synthetic-production", TenantID: p.f.device.TenantID, StoreID: p.f.device.StoreID, DeviceID: p.f.device.DeviceID, ProfileID: "issuer-one", ProfileRevision: 2, Issuer: p.input.Issuer.CNPJ, UF: "SP", Timezone: "America/Sao_Paulo", Model: model, Series: 1, InitialNumber: 17, SoleAuthorityReference: "synthetic-single-authority", EndpointRegistryReference: "synthetic-endpoint-registry", HomologationReference: "synthetic-homologation-only", DeveloperReference: "synthetic-developer-reference", AuthorizationURL: "https://" + productionTestHost + "/issue", ConsultationURL: "https://" + productionTestHost + "/query", CancellationURL: "https://" + productionTestHost + "/cancel", CertificateSHA256: p.cert.Credential.Fingerprint(), ValidFrom: now.Add(-time.Minute).UTC().Format(time.RFC3339), ValidUntil: now.Add(time.Hour).UTC().Format(time.RFC3339)}
	valid, err := p.cert.CRL(false, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := p.cert.CRL(true, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	loader := func() ([][]byte, error) {
		if (p.crlGuard.Load() || p.logoutGuard.Load()) && p.crlCalls.Add(1) >= 3 {
			if p.logoutGuard.Load() {
				if err := localauth.Logout(context.Background(), p.f.db, p.f.token); err != nil {
					return nil, err
				}
			} else {
				return [][]byte{revoked}, nil
			}
		}
		return [][]byte{valid}, nil
	}
	var qr *fiscalcore.QRConfiguration
	consult := ""
	if model == "65" {
		qr = &fiscalcore.QRConfiguration{Version: 3, BaseURL: "https://" + productionTestHost + "/qr"}
		consult = "https://" + productionTestHost + "/consult"
	}
	p.runtime, err = fiscalproduction.Provision(p.binding, p.cert.Credential, loader, qr, consult, fiscalproduction.NetworkTrust{ServerRoots: p.cert.Roots, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", p.authority.Listener.Addr().String())
	}})
	if err != nil {
		t.Fatal(err)
	}
	p.binding = p.runtime.Binding()
	p.installApp(t, p.runtime)
	return p
}
func (p *productionFixture) installApp(t *testing.T, runtime *fiscalproduction.Runtime) {
	t.Helper()
	v, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": p.f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	p.f.app, err = NewWithProductionAndGate(p.f.db, p.f.device, v, nil, nil, runtime)
	if err != nil {
		t.Fatal(err)
	}
}
func (p *productionFixture) path(suffix string) string {
	return "/local/v1/fiscal/intents/" + p.intent.ID + "/production" + suffix
}
func (p *productionFixture) prepare(t *testing.T, op string, expected, want int) fiscal.ProductionPreparation {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"operation_id": op, "expected_revision": expected, "invoice": p.input})
	code, body := request(t, p.f.app, "POST", p.path("/prepare"), string(raw), p.f.token)
	if code != want {
		t.Fatalf("prepare %d want%d %s", code, want, body)
	}
	var out fiscal.ProductionPreparation
	if want == 201 {
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
func (p *productionFixture) approve(t *testing.T, prep fiscal.ProductionPreparation, want int) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"operation_id": fmt.Sprintf("approve-%d", prep.Revision), "revision": prep.Revision, "review_sha256": prep.ReviewSHA256, "tax_reference": "SYNTHETIC-TAX-REVIEW-NOT-LEGAL-APPROVAL"})
	code, body := request(t, p.f.app, "POST", p.path("/approve"), string(raw), p.f.token)
	if code != want {
		t.Fatalf("approve %d %s", code, body)
	}
}
func (p *productionFixture) emit(t *testing.T, prep fiscal.ProductionPreparation, op string, want int) fiscalcore.ProductionEvidence {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"operation_id": op, "revision": prep.Revision, "review_sha256": prep.ReviewSHA256})
	code, body := request(t, p.f.app, "POST", p.path("/emit"), string(raw), p.f.token)
	if code != want {
		t.Fatalf("emit %d want%d %s", code, want, body)
	}
	var out fiscalcore.ProductionEvidence
	if code == 200 || code == 202 {
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
func (p *productionFixture) query(t *testing.T, prep fiscal.ProductionPreparation, op string, want int) fiscal.ProductionStatus {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"operation_id": op, "revision": prep.Revision})
	code, body := request(t, p.f.app, "POST", p.path("/query"), string(raw), p.f.token)
	if code != want {
		t.Fatalf("query %d %s", code, body)
	}
	var out fiscal.ProductionStatus
	if code == 200 {
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}
func (p *productionFixture) authorityHandler(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
		w.WriteHeader(403)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2*fiscalcore.MaxXML+1))
	if err != nil {
		w.WriteHeader(400)
		return
	}
	if r.URL.Path == "/cancel" {
		p.cancels.Add(1)
		var archived int
		if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_production_cancellation_requests`).Scan(&archived); err != nil || archived != 1 {
			w.WriteHeader(500)
			return
		}
		doc := etree.NewDocument()
		if doc.ReadFromBytes(body) != nil {
			w.WriteHeader(400)
			return
		}
		info := doc.Root().FindElement(".//evento/infEvento")
		if info == nil || info.SelectElement("tpAmb").Text() != "1" {
			w.WriteHeader(400)
			return
		}
		key := info.SelectElement("chNFe").Text()
		mode := p.mode.Load().(string)

		if mode == "cancel-logout-inflight" {
			if err := localauth.Logout(context.Background(), p.f.db, p.f.token); err != nil {
				w.WriteHeader(500)
				return
			}
		}
		if mode == "cancel-invalid-xml" {
			fmt.Fprint(w, "<bad>")
			return
		}
		if mode == "cancel-http500" {
			w.WriteHeader(500)
		}
		if mode == "cancel-redirect" {
			w.Header().Set("Location", "https://"+productionTestHost+"/cancel")
			w.WriteHeader(302)
			return
		}
		status := "135"
		if strings.HasPrefix(mode, "cancel-status-") {
			status = strings.TrimPrefix(mode, "cancel-status-")
		}
		response := []byte(fmt.Sprintf(`<retEnvEvento xmlns="%s" versao="1.00"><idLote>1</idLote><tpAmb>1</tpAmb><verAplic>SYNTHETIC1</verAplic><cOrgao>%s</cOrgao><cStat>128</cStat><xMotivo>Synthetic batch only</xMotivo><retEvento versao="1.00"><infEvento><tpAmb>1</tpAmb><verAplic>SYNTHETIC1</verAplic><cOrgao>%s</cOrgao><cStat>%s</cStat><xMotivo>Synthetic event only</xMotivo><chNFe>%s</chNFe><tpEvento>110111</tpEvento><xEvento>Cancelamento registrado</xEvento><nSeqEvento>1</nSeqEvento><dhRegEvento>%s</dhRegEvento><nProt>135260000000002</nProt></infEvento></retEvento></retEnvEvento>`, fiscalcore.NFeNS, key[:2], key[:2], status, key, time.Now().Format("2006-01-02T15:04:05-07:00")))
		event := etree.NewDocument()
		event.SetRoot(doc.Root().FindElement(".//evento").Copy())
		eventRaw, _ := event.WriteToBytes()
		p.lock.Lock()
		p.lastEvent = eventRaw
		p.lastCancellation = response
		p.lock.Unlock()
		if mode == "cancel-drop" {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				connection.Close()
			}
			return
		}
		w.Write(response)
		return
	}
	if r.URL.Path == "/issue" {
		p.issues.Add(1)
		doc := etree.NewDocument()
		if doc.ReadFromBytes(body) != nil {
			w.WriteHeader(400)
			return
		}
		nfe := doc.Root().FindElement(".//NFe")
		if nfe == nil {
			w.WriteHeader(400)
			return
		}
		signed := etree.NewDocument()
		signed.SetRoot(nfe.Copy())
		raw, _ := signed.WriteToBytes()
		p.lock.Lock()
		p.lastSigned = raw
		p.authorizedAt = time.Now().UTC().Format("2006-01-02T15:04:05-07:00")
		p.lock.Unlock()
		if p.mode.Load().(string) == "drop" {
			connection, _, e := w.(http.Hijacker).Hijack()
			if e == nil {
				_ = connection.Close()
			}
			return
		}
		status := "100"
		if p.mode.Load().(string) == "reject" {
			status = "531"
		}
		if p.mode.Load().(string) == "duplicate" {
			status = "204"
		}
		if p.mode.Load().(string) == "unknown" {
			status = "999"
		}
		w.Header().Set("Content-Type", "application/soap+xml")
		response := p.response(false, status)
		switch p.mode.Load().(string) {
		case "tamper":
			doc := etree.NewDocument()
			_ = doc.ReadFromBytes(response)
			doc.Root().FindElement("protNFe/infProt/digVal").SetText("AAAAAAAAAAAAAAAAAAAAAAAAAAA=")
			response, _ = doc.WriteToBytes()
		case "malformed":
			response = []byte("<not-an-official-response/>")
		case "http500":
			w.WriteHeader(500)
		case "redirect":
			w.Header().Set("Location", "https://"+productionTestHost+"/redirect-target")
			w.WriteHeader(302)
		}
		_, _ = w.Write(response)
		return
	}
	if r.URL.Path == "/query" {
		p.queries.Add(1)
		w.Header().Set("Content-Type", "application/soap+xml")
		if p.mode.Load().(string) == "query-canceled-event" || p.mode.Load().(string) == "query-other-event" {
			response := etree.NewDocument()
			response.ReadFromBytes(p.response(true, "101"))
			root := response.Root()
			root.RemoveChild(root.SelectElement("protNFe"))
			p.lock.Lock()
			eventRaw := append([]byte(nil), p.lastEvent...)
			registrationRaw := append([]byte(nil), p.lastCancellation...)
			p.lock.Unlock()
			event, registered := etree.NewDocument(), etree.NewDocument()
			event.ReadFromBytes(eventRaw)
			registered.ReadFromBytes(registrationRaw)
			if p.mode.Load().(string) == "query-other-event" {
				event.Root().FindElement("infEvento/detEvento/xJust").SetText("Different event content cannot recover original")
				event.Root().RemoveChild(event.Root().SelectElement("Signature"))
				raw, _ := event.WriteToBytes()
				signed, err := p.cert.Credential.SignXML("nfe_event", raw, time.Now())
				if err != nil {
					w.WriteHeader(500)
					return
				}
				event.ReadFromBytes(signed)
			}
			process := root.CreateElement("procEventoNFe")
			process.CreateAttr("versao", "1.00")
			process.AddChild(event.Root().Copy())
			process.AddChild(registered.Root().SelectElement("retEvento").Copy())
			raw, _ := response.WriteToBytes()
			w.Write(raw)
		} else {
			_, _ = w.Write(p.response(true, "100"))
		}
		return
	}
	w.WriteHeader(404)
}
func (p *productionFixture) response(query bool, status string) []byte {
	p.lock.Lock()
	signed := append([]byte(nil), p.lastSigned...)
	authorizedAt := p.authorizedAt
	p.lock.Unlock()
	doc := etree.NewDocument()
	_ = doc.ReadFromBytes(signed)
	key := strings.TrimPrefix(doc.Root().SelectElement("infNFe").SelectAttrValue("Id", ""), "NFe")
	digest := doc.Root().FindElement("Signature/SignedInfo/Reference/DigestValue").Text()
	now := time.Now().UTC().Format("2006-01-02T15:04:05-07:00")
	protocolFields := ""
	if status == "100" {
		protocolFields = "<nProt>135260000000001</nProt><digVal>" + digest + "</digVal>"
	}
	proto := fmt.Sprintf(`<protNFe versao="4.00"><infProt><tpAmb>1</tpAmb><verAplic>SYNTHETIC</verAplic><chNFe>%s</chNFe><dhRecbto>%s</dhRecbto>%s<cStat>%s</cStat><xMotivo>Synthetic authority for tests only</xMotivo></infProt></protNFe>`, key, authorizedAt, protocolFields, status)
	if query {
		return []byte(fmt.Sprintf(`<retConsSitNFe xmlns="%s" versao="4.00"><tpAmb>1</tpAmb><verAplic>SYNTHETIC</verAplic><cStat>%s</cStat><xMotivo>Synthetic query only</xMotivo><cUF>35</cUF><dhRecbto>%s</dhRecbto><chNFe>%s</chNFe>%s</retConsSitNFe>`, fiscalcore.NFeNS, status, now, key, proto))
	}
	return []byte(fmt.Sprintf(`<retEnviNFe xmlns="%s" versao="4.00"><tpAmb>1</tpAmb><verAplic>SYNTHETIC</verAplic><cStat>104</cStat><xMotivo>Synthetic batch only</xMotivo><cUF>35</cUF><dhRecbto>%s</dhRecbto>%s</retEnviNFe>`, fiscalcore.NFeNS, now, proto))
}

func TestHTTPFiscalProductionReviewedInvoiceRealMTLSAndNoDuplicate(t *testing.T) {
	for _, model := range []string{"55", "65"} {
		t.Run(model, func(t *testing.T) {
			p := newProductionFixture(t, model)
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.emit(t, prep, "emit-one", 409)
			if p.issues.Load() != 0 {
				t.Fatal("unapproved send")
			}
			wrong := prep
			wrong.ReviewSHA256 = strings.Repeat("0", 64)
			p.approve(t, wrong, 409)
			p.approve(t, prep, 204)
			out := p.emit(t, prep, "emit-one", 200)
			if out.State != "authorized" || out.Protocol == nil || len(out.ProcessedXML) == 0 || fiscalcore.ValidateXML("nfe_process", out.ProcessedXML) != nil {
				t.Fatal("authorized document not assembled")
			}
			again := p.emit(t, prep, "emit-one", 200)
			if !bytes.Equal(out.ProcessedXML, again.ProcessedXML) || p.issues.Load() != 1 {
				t.Fatal("duplicate POST or changed original")
			}
			p.emit(t, prep, "emit-another", 409)
			if code, body := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token); code != 200 || !bytes.Equal(body, out.ProcessedXML) {
				t.Fatal("download differs", code)
			}
			p.prepare(t, "forbidden-reprepare", 1, 409)
			if model == "65" && !bytes.Contains(out.ProcessedXML, []byte("|3|1")) {
				t.Fatal("production QR binding")
			}
		})
	}
}

func TestHTTPFiscalProductionDropRestartAndQueryNeverReposts(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	p.mode.Store("drop")
	out := p.emit(t, prep, "emit-one", 202)
	if out.State != "uncertain" || p.issues.Load() != 1 {
		t.Fatal("lost response not uncertain")
	}
	var seq int
	var name, path string
	if err := p.f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := p.f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p.f.db = db
	p.installApp(t, p.runtime)
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	p.emit(t, prep, "emit-one", 200)
	p.emit(t, prep, "not-a-retry", 409)
	p.prepare(t, "forbidden-reprepare", 1, 409)
	proof := p.query(t, prep, "query-after-restart", 200)
	if proof.Evidence.State != "authorized" || p.issues.Load() != 1 || p.queries.Load() != 1 {
		t.Fatal("query did not recover original")
	}
	p.query(t, prep, "query-after-restart", 200)
	if p.queries.Load() != 1 {
		t.Fatal("duplicate query POST")
	}
}

func TestHTTPFiscalProductionExplicitCorrectionPreservesNumberAndOldRequest(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	p.mode.Store("reject")
	if out := p.emit(t, prep, "issue-rejected", 200); out.State != "rejected_requires_review" {
		t.Fatal(out.State)
	}
	var original string
	if err := p.f.db.QueryRow(`SELECT request_json FROM fiscal_production_requests WHERE kind='issue'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	p.input.Lines[0].Description = "Explicit synthetic correction"
	next := p.prepare(t, "prepare-corrected", 1, 201)
	if prep.Invoice.Key != next.Invoice.Key || prep.ReviewSHA256 == next.ReviewSHA256 {
		t.Fatal("number changed or review reused")
	}
	p.emit(t, prep, "stale-callback", 409)
	p.emit(t, next, "corrected-without-review", 409)
	p.approve(t, next, 204)
	p.mode.Store("authorize")
	out := p.emit(t, next, "corrected-one", 200)
	if out.State != "authorized" || p.issues.Load() != 2 {
		t.Fatal("explicit correction failed")
	}
	var after string
	if err := p.f.db.QueryRow(`SELECT request_json FROM fiscal_production_requests WHERE operation_id='issue-rejected'`).Scan(&after); err != nil || after != original {
		t.Fatal("original rejected document overwritten")
	}
	var allocations int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_production_allocations`).Scan(&allocations); err != nil || allocations != 1 {
		t.Fatal("number reallocated")
	}
}

func TestHTTPFiscalProductionLogoutRequiresNewExplicitReview(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	old := p.f.token
	if code, _ := request(t, p.f.app, "POST", "/local/v1/logout", "", old); code != 204 {
		t.Fatal(code)
	}
	p.emit(t, prep, "old-callback", 401)
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	p.emit(t, prep, "new-session-old-review", 403)
	if p.issues.Load() != 0 {
		t.Fatal("old review used after session change")
	}
	next := p.prepare(t, "explicit-new-session", 1, 201)
	p.approve(t, next, 204)
	if out := p.emit(t, next, "new-reviewed-operation", 200); out.State != "authorized" {
		t.Fatal(out.State)
	}
}

func TestHTTPFiscalProductionRevocationAtSendKeepsDurableUncertainty(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	p.crlCalls.Store(0)
	p.crlGuard.Store(true)
	p.emit(t, prep, "revoked-before-send", 503)
	if p.issues.Load() != 0 {
		t.Fatal("revoked credential sent")
	}
	var count int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_production_requests WHERE kind='issue'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("reserved request missing")
	}
	p.crlGuard.Store(false)
	out := p.emit(t, prep, "revoked-before-send", 200)
	if out.State != "uncertain" || p.issues.Load() != 0 {
		t.Fatal("revocation triggered retry")
	}
}

func TestHTTPFiscalProductionUnavailableByDefault(t *testing.T) {
	f := fiscalFixture(t)
	intent := fiscalIntent(t, f, fiscalIntentBody, 201)
	for _, action := range []string{"prepare", "approve", "emit", "query"} {
		code, body := request(t, f.app, "POST", "/local/v1/fiscal/intents/"+intent.ID+"/production/"+action, `{}`, f.token)
		if code != 503 || !bytes.Contains(body, []byte("fiscal_production_unavailable")) {
			t.Fatal(action, code)
		}
	}
}

func TestHTTPFiscalProductionImmutableJournalAndPreparationRollback(t *testing.T) {
	p := newProductionFixture(t, "55")
	if _, err := p.f.db.Exec(`CREATE TRIGGER test_production_abort BEFORE INSERT ON fiscal_production_preparations BEGIN SELECT RAISE(ABORT,'synthetic fault'); END`); err != nil {
		t.Fatal(err)
	}
	p.prepare(t, "prepare-one", 0, 500)
	for _, table := range []string{"fiscal_production_allocations", "fiscal_production_series", "fiscal_production_preparations"} {
		var count int
		if err := p.f.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("partial preparation", table, count)
		}
	}
	if _, err := p.f.db.Exec(`DROP TRIGGER test_production_abort`); err != nil {
		t.Fatal(err)
	}
	prep := p.prepare(t, "prepare-one", 0, 201)
	if prep.Invoice.Key.Number != 17 {
		t.Fatal("rolled back number consumed")
	}
	p.approve(t, prep, 204)
	p.emit(t, prep, "emit-one", 200)
	for _, table := range []string{"fiscal_production_allocations", "fiscal_production_series", "fiscal_production_preparations", "fiscal_production_approvals", "fiscal_production_requests", "fiscal_production_results"} {
		if _, err := p.f.db.Exec(`DELETE FROM ` + table); err == nil {
			t.Fatal("removed journal", table)
		}
	}
}

func TestHTTPFiscalProductionRejectsChangedEconomicsAndClientConfiguration(t *testing.T) {
	p := newProductionFixture(t, "55")
	p.input.Lines[0].UnitPrice = "0.01"
	p.prepare(t, "bad-economics", 0, 400)
	if p.issues.Load() != 0 {
		t.Fatal("invalid economics sent")
	}
	for _, body := range []string{`{"operation_id":"x","expected_revision":0,"endpoint":"https://attacker.gov.br/issue"}`, `{"operation_id":"x","expected_revision":0,"private_key":"forbidden"}`} {
		if code, _ := request(t, p.f.app, "POST", p.path("/prepare"), body, p.f.token); code != 400 {
			t.Fatal(code)
		}
	}
}

func productionBusinessHash(t *testing.T, p *productionFixture) string {
	t.Helper()
	h := sha256.New()
	for _, table := range []string{"fiscal_production_series", "fiscal_production_allocations", "fiscal_production_preparations", "fiscal_production_approvals", "fiscal_production_requests", "fiscal_production_results"} {
		rows, err := p.f.db.Query(`SELECT * FROM ` + table + ` ORDER BY 1,2,3`)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			values := make([]any, len(cols))
			targets := make([]any, len(cols))
			for i := range values {
				targets[i] = &values[i]
			}
			if err = rows.Scan(targets...); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(values)
			h.Write(raw)
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestHTTPFiscalProductionBackupRestorePreservesXMLAndBlocksNewEmission(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	original := p.emit(t, prep, "issue-one", 200)
	before := productionBusinessHash(t, p)
	var seq int
	var name, source string
	if err := p.f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &source); err != nil {
		t.Fatal(err)
	}
	var destination string
	if *pilotBackupCLI != "" {
		station := pilotPrepareCLIStation(t, p.f)
		destination = pilotCLIBackupRoundTrip(t, p.f, source, station)
	} else {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			t.Fatal(err)
		}
		defer clear(key)
		dir := t.TempDir()
		archive := filepath.Join(dir, "production.tytbak")
		destination = filepath.Join(dir, "restored.sqlite")
		if err := backup.Create(context.Background(), p.f.db, p.f.device, key, archive); err != nil {
			t.Fatal(err)
		}
		if err := backup.Restore(context.Background(), archive, destination, p.f.device, key); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := localdb.Open(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	p.f.db = restored
	p.installApp(t, p.runtime)
	if code, _ := request(t, p.f.app, "GET", p.path(""), "", p.f.token); code != 401 {
		t.Fatal("restored session survived", code)
	}
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	if productionBusinessHash(t, p) != before {
		t.Fatal("production journal changed on restore")
	}
	if code, raw := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token); code != 200 || !bytes.Equal(raw, original.ProcessedXML) {
		t.Fatal("restored XML differs", code)
	}
	p.prepare(t, "after-restore", 1, 409)
	p.emit(t, prep, "different-operation", 409)
	proof := p.query(t, prep, "explicit-restored-query", 200)
	if proof.Evidence.State != "authorized" || !bytes.Equal(proof.Evidence.ProcessedXML, original.ProcessedXML) || p.issues.Load() != 1 {
		t.Fatal("restoration caused reissue or changed document")
	}
	var barriers int
	if err = restored.QueryRow(`SELECT count(*) FROM fiscal_restore_barriers`).Scan(&barriers); err != nil || barriers != 1 {
		t.Fatal("query cleared restoration barrier")
	}
}

func TestHTTPFiscalProductionNativeHTTPFlowAndAuthorization(t *testing.T) {
	p := newProductionFixture(t, "55")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- p.f.app.Listener(listener) }()
	t.Cleanup(func() {
		_ = p.f.app.Shutdown()
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("HTTP server did not stop")
		}
	})
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}
	defer client.CloseIdleConnections()
	call := func(method, path string, value any, token string, want int) []byte {
		t.Helper()
		var body []byte
		if value != nil {
			body, _ = json.Marshal(value)
		}
		req, e := http.NewRequest(method, "http://"+listener.Addr().String()+path, bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		raw, e := io.ReadAll(resp.Body)
		if e != nil {
			t.Fatal(e)
		}
		if resp.StatusCode != want {
			t.Fatalf("%s %s ->%d want%d %s", method, path, resp.StatusCode, want, raw)
		}
		if resp.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("fiscal response cached")
		}
		t.Logf("REAL HTTP %s %s -> %d", method, path, resp.StatusCode)
		return raw
	}
	call("GET", p.path(""), nil, "", 401)
	raw := call("POST", p.path("/prepare"), map[string]any{"operation_id": "native-prepare", "expected_revision": 0, "invoice": p.input}, p.f.token, 201)
	var prep fiscal.ProductionPreparation
	if json.Unmarshal(raw, &prep) != nil {
		t.Fatal("prepare response")
	}
	call("POST", p.path("/approve"), map[string]any{"operation_id": "native-approve", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256, "tax_reference": "SYNTHETIC-REVIEW-ONLY"}, p.f.token, 204)
	raw = call("POST", p.path("/emit"), map[string]any{"operation_id": "native-emit", "revision": prep.Revision, "review_sha256": prep.ReviewSHA256}, p.f.token, 200)
	var evidence fiscalcore.ProductionEvidence
	if json.Unmarshal(raw, &evidence) != nil || evidence.State != "authorized" {
		t.Fatal("authorization response")
	}
	if !bytes.Equal(call("GET", p.path("/xml"), nil, p.f.token, 200), evidence.ProcessedXML) {
		t.Fatal("native XML mismatch")
	}
	call("POST", p.path("/emit")+"?tenant_id=foreign", map[string]any{}, p.f.token, 400)
	if p.issues.Load() != 1 {
		t.Fatal("native duplicate issue")
	}
}

func TestHTTPFiscalProductionAccountantCanReviewButCannotUseSellerAuthority(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	for _, role := range []string{"cashier", "accountant"} {
		name := "production-" + role
		if code, _ := request(t, p.f.app, "POST", "/local/v1/staff", `{"operation_id":"`+name+`","identity_id":"`+name+`","name":"Synthetic staff","role":"`+role+`","password":"synthetic-staff-password-1"}`, p.f.token); code != 201 {
			t.Fatal(code)
		}
		code, body := request(t, p.f.app, "POST", "/local/v1/login", `{"identity_id":"`+name+`","password":"synthetic-staff-password-1"}`, "")
		if code != 200 {
			t.Fatal(code)
		}
		var ss struct {
			Token string `json:"token"`
		}
		if json.Unmarshal(body, &ss) != nil {
			t.Fatal("login")
		}
		original := p.f.token
		p.f.token = ss.Token
		if role == "cashier" {
			p.approve(t, prep, 404)
			if code, _ := request(t, p.f.app, "GET", p.path(""), "", ss.Token); code != 404 {
				t.Fatal("other seller read document", code)
			}
		} else {
			p.approve(t, prep, 204)
			p.emit(t, prep, "accountant-cannot-impersonate", 404)
		}
		p.f.token = original
	}
	if out := p.emit(t, prep, "owner-reviewed-send", 200); out.State != "authorized" {
		t.Fatal(out.State)
	}
}

func TestHTTPFiscalProductionConcurrentPreparationOneDurableNumber(t *testing.T) {
	p := newProductionFixture(t, "55")
	var seq int
	var name, path string
	if err := p.f.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	second, err := localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	firstLicense, secondLicense := journalLicense(t, p.f, p.f.db), journalLicense(t, p.f, second)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			db, license := p.f.db, firstLicense
			if index%2 == 1 {
				db, license = second, secondLicense
			}
			_, err := fiscal.PrepareProduction(context.Background(), db, license, numberingActor(p.f), p.f.device, p.intent.ID, p.f.token, fmt.Sprintf("concurrent-%d", index), 0, p.input, p.binding)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	passed := 0
	for err := range results {
		if err == nil {
			passed++
		} else if err != fiscal.ErrConflict {
			t.Fatal(err)
		}
	}
	if passed != 1 {
		t.Fatal("preparation winners", passed)
	}
	var count int
	if err := p.f.db.QueryRow(`SELECT count(*) FROM fiscal_production_allocations`).Scan(&count); err != nil || count != 1 {
		t.Fatal("allocated duplicate numbers")
	}
}

func TestHTTPFiscalProductionInvalidAuthorityResponseRequiresQuery(t *testing.T) {
	for _, mode := range []string{"tamper", "malformed", "http500", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			p := newProductionFixture(t, "55")
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.approve(t, prep, 204)
			p.mode.Store(mode)
			if out := p.emit(t, prep, "issue-one", 202); out.State != "uncertain" {
				t.Fatal("invalid response became certainty", out.State)
			}
			p.emit(t, prep, "forbidden-retry", 409)
			p.prepare(t, "forbidden-correction", 1, 409)
			if code, _ := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token); code != 409 {
				t.Fatal("invalid authority response made authorized XML", code)
			}
			if proof := p.query(t, prep, "explicit-query", 200); proof.Evidence.State != "authorized" || proof.Evidence.QueryOperationID != "explicit-query" {
				t.Fatal("explicit query did not recover original")
			}
			if p.issues.Load() != 1 {
				t.Fatal("POST repeated")
			}
		})
	}
}

func TestHTTPFiscalProductionDuplicateAndUnknownStatusNeverEnableCorrection(t *testing.T) {
	for _, mode := range []string{"duplicate", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			p := newProductionFixture(t, "55")
			prep := p.prepare(t, "prepare-one", 0, 201)
			p.approve(t, prep, 204)
			p.mode.Store(mode)
			out := p.emit(t, prep, "issue-one", 200)
			if out.State == "authorized" || out.State == "rejected_requires_review" {
				t.Fatal("unsafe classification", out.State)
			}
			p.prepare(t, "not-a-correction", 1, 409)
			p.emit(t, prep, "not-a-retry", 409)
			if p.issues.Load() != 1 {
				t.Fatal("duplicate/unknown replayed")
			}
		})
	}
}

func TestHTTPFiscalProductionLogoutBetweenReservationAndSend(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	p.crlCalls.Store(0)
	p.logoutGuard.Store(true)
	p.emit(t, prep, "logout-before-send", 401)
	if p.issues.Load() != 0 {
		t.Fatal("logout after durable reservation sent")
	}
	p.logoutGuard.Store(false)
	p.f.token = loginToken(t, p.f.app, p.f.owner.OwnerID)
	if out := p.emit(t, prep, "logout-before-send", 200); out.State != "uncertain" {
		t.Fatal("uncertainty was erased")
	}
	p.emit(t, prep, "new-session-retry", 409)
	p.prepare(t, "new-session-correction", 1, 409)
	if p.issues.Load() != 0 {
		t.Fatal("new session replayed")
	}
}

func TestHTTPFiscalProductionHistoricalCopyDoesNotRequireLiveCredentials(t *testing.T) {
	p := newProductionFixture(t, "55")
	prep := p.prepare(t, "prepare-one", 0, 201)
	p.approve(t, prep, 204)
	out := p.emit(t, prep, "issue-one", 200)
	p.installApp(t, nil)
	if code, body := request(t, p.f.app, "GET", p.path("/xml"), "", p.f.token); code != 200 || !bytes.Equal(body, out.ProcessedXML) {
		t.Fatal("historical credential dependence", code)
	}
	p.emit(t, prep, "disabled-runtime", 503)
	if p.issues.Load() != 1 {
		t.Fatal("disabled runtime sent")
	}
}
