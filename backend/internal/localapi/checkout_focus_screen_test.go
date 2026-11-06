package localapi

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beevik/etree"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/backup"
)

// Browser/Focus simulation opt-in. All transport overrides and credentials are
// TEST-ONLY. The production adapter retains its fixed homologation URL and TLS.
func TestHTTPCheckoutFocusRenderedScreenRecoveryAndArchive(t *testing.T) {
	script := os.Getenv("TITAN_CHECKOUT_FOCUS_SCREEN")
	if script == "" {
		t.Skip("optional: set TITAN_CHECKOUT_FOCUS_SCREEN to checkoutFocus.screen.mjs; requires Chromium and frontend dependencies")
	}
	p := newProductionFixture(t, "65", "1", "America/Sao_Paulo")
	body := strings.Replace(strings.Replace(fiscalProfileBody, `"issuer-create"`, `"focus-screen-profile"`, 1), `"expected_revision":0`, `"expected_revision":2`, 1)
	body = strings.TrimSuffix(body, "}") + `,"timezone":"America/Sao_Paulo","require_tax_pricing":true}`
	if code, _ := request(t, p.f.app, "POST", "/local/v1/fiscal/profiles", body, p.f.token); code != 201 {
		t.Fatal("homologation profile", code)
	}
	policy := productionTaxPolicyInput(p)
	policy.ProfileRevision = 3
	policy.Rule.ValidFrom = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	policy.Rule.ValidUntil = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	taxRequest(t, p.f, "POST", "/local/v1/fiscal/tax-policies", policy, 201, nil)

	// Existing builder/signature/QR engine, never a hand-written fiscal engine.
	processed := func(number int64) ([]byte, string) {
		in := p.input
		in.Environment = 2
		in.Key = fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: p.intent.Snapshot.Profile.CNPJ, Model: "65", Series: 1, Number: number, Emission: 1, NumericCode: "12345678"}
		in.Lines = append([]fiscalcore.InvoiceLine(nil), p.input.Lines...)
		in.Lines[0].Quantity, in.Lines[0].UnitPrice = "1", "15.99"
		in.Payments = []fiscalcore.InvoicePayment{{Method: "01", Amount: "15.99"}}
		xml, key, err := fiscalcore.BuildInvoice(in)
		if err != nil {
			t.Fatal(err)
		}
		xml, err = p.cert.Credential.SignXML("nfe", xml, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		xml, err = p.cert.Credential.CompleteNFCeXML(fiscalcore.QRConfiguration{Version: 3, BaseURL: "https://synthetic.gov.br/qr"}, "https://synthetic.gov.br/consult", xml, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		doc := etree.NewDocument()
		if err = doc.ReadFromBytes(xml); err != nil {
			t.Fatal(err)
		}
		digest := doc.Root().FindElement("Signature/SignedInfo/Reference/DigestValue").Text()
		out := []byte(fmt.Sprintf(`<nfeProc xmlns="%s" versao="4.00">%s<protNFe versao="4.00"><infProt><tpAmb>2</tpAmb><verAplic>SYNTHETIC1</verAplic><chNFe>%s</chNFe><dhRecbto>%s</dhRecbto><nProt>135260000000001</nProt><digVal>%s</digVal><cStat>100</cStat><xMotivo>SYNTHETIC ONLY</xMotivo></infProt></protNFe></nfeProc>`, fiscalcore.NFeNS, xml, key, time.Now().Format("2006-01-02T15:04:05-07:00"), digest))
		if err = fiscalcore.ValidateXML("nfe_process", out); err != nil {
			t.Fatal("processed fixture", err)
		}
		if _, err = fiscalcore.RenderHomologationNFCe(out, fiscalcore.PrintOptions{WidthMM: 80, Timezone: "America/Sao_Paulo"}); err != nil {
			t.Fatal("print fixture", err)
		}
		return out, key
	}
	xml1, key1 := processed(990)
	xml2, key2 := processed(991)
	type remoteRecord struct {
		key string
		xml []byte
	}
	ledger := map[string]remoteRecord{}
	var mu sync.Mutex
	posts, queries, downloads := 0, 0, 0
	focus := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		user, password, ok := r.BasicAuth()
		if !ok || user != "SyntheticFocusScreenToken123" || password != "" {
			t.Error("invalid synthetic basic auth")
			w.WriteHeader(401)
			return
		}
		var record remoteRecord
		ref := r.URL.Query().Get("ref")
		switch {
		case r.Method == "POST" && r.URL.Path == "/v2/nfce":
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil || input["cnpj_emitente"] != p.intent.Snapshot.Profile.CNPJ || input["valor_total"] != "15.99" || ref == "" {
				t.Error("invalid simulated submission")
				w.WriteHeader(400)
				return
			}
			if _, exists := ledger[ref]; exists {
				t.Error("duplicate POST to simulated Focus")
				w.WriteHeader(409)
				return
			}
			posts++
			record = remoteRecord{key1, xml1}
			if posts == 2 {
				record = remoteRecord{key2, xml2}
			}
			ledger[ref] = record
			if posts == 2 {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				conn.Close()
				return
			}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v2/nfce/"):
			queries++
			ref = strings.TrimPrefix(r.URL.Path, "/v2/nfce/")
			record = ledger[ref]
			if record.key == "" {
				w.WriteHeader(404)
				return
			}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/arquivos/"):
			for _, saved := range ledger {
				if strings.Contains(r.URL.Path, saved.key) {
					downloads++
					w.Header().Set("Content-Type", "application/xml")
					w.Write(saved.xml)
					return
				}
			}
			w.WriteHeader(404)
			return
		default:
			t.Error("unexpected simulated Focus request")
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		}
		json.NewEncoder(w).Encode(map[string]string{"ref": ref, "cnpj_emitente": p.intent.Snapshot.Profile.CNPJ, "status": "autorizado", "chave_nfe": record.key, "caminho_xml_nota_fiscal": "/arquivos/" + record.key[6:20] + "/202611/XMLs/" + record.key + "-nfe.xml"})
	}))
	t.Cleanup(focus.Close)
	// Strictly reroute only the fixed homologation host to a verified local TLS
	// fixture. No InsecureSkipVerify, real DNS or Internet fallback is allowed.
	previous := http.DefaultTransport
	transport := previous.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("test forbids external network")
	}
	roots := x509.NewCertPool()
	roots.AddCert(focus.Certificate())
	u, _ := url.Parse(focus.URL)
	transport.DialTLSContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "homologacao.focusnfe.com.br:443" {
			return nil, errors.New("test forbids external host")
		}
		d := tls.Dialer{Config: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: u.Hostname()}}
		return d.DialContext(ctx, network, u.Host)
	}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	dir := t.TempDir()
	vaultPath, masterKey, archiveKey := filepath.Join(dir, "vault.sqlite"), filepath.Join(dir, "master.key"), filepath.Join(dir, "archive.key")
	if err := fiscalproduction.CreateCustomerFiscalVault(vaultPath, masterKey, io.Discard); err != nil {
		t.Fatal(err)
	}
	vault, err := fiscalproduction.OpenCustomerFiscalVault(vaultPath, masterKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = vault.Put(t.Context(), p.f.owner.TenantID, "token", []byte("SyntheticFocusScreenToken123")); err != nil {
		t.Fatal(err)
	}
	if err = vault.Put(t.Context(), p.f.owner.TenantID, "csc", []byte("SyntheticScreenCSC123")); err != nil {
		t.Fatal(err)
	}
	vault.Close()
	if err = backup.InitKey(archiveKey); err != nil {
		t.Fatal(err)
	}
	config := fiscalproduction.TenantFiscalConfiguration{Provider: "external", Environment: "homologation", CRT: "1", CertificateType: "A1", VaultPath: vaultPath, MasterKeyFile: masterKey, XMLArchiveKeyFile: archiveKey, Binding: fiscalproduction.ProviderBinding{TenantID: p.f.owner.TenantID, StoreID: p.f.owner.StoreID, DeviceID: p.f.device.DeviceID, ProfileID: "issuer-one", ProfileRevision: 3}}
	raw, _ := json.Marshal(map[string]any{"version": 1, "tenants": []any{config}})
	configPath := filepath.Join(dir, "providers.json")
	if err = os.WriteFile(configPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := fiscalproduction.LoadProviderRegistry(configPath)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"test-issuer": p.f.private.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	p.f.app, err = NewWithFiscalProvidersAndGate(p.f.db, p.f.device, verifier, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = p.f.app.Listener(listener) }()
	defer p.f.app.Shutdown()
	defer listener.Close()
	bootstrap, _ := json.Marshal(map[string]any{"url": "http://" + listener.Addr().String(), "token": p.f.token, "product": p.f.product, "location": p.f.shelf, "policy": policy.Rule.ID})
	ctx, cancel := context.WithTimeout(t.Context(), 70*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script)
	cmd.Stdin = bytes.NewReader(bootstrap)
	out, err := cmd.CombinedOutput()
	if err != nil {
		mu.Lock()
		t.Logf("simulation counters: POST=%d GET=%d XML=%d", posts, queries, downloads)
		mu.Unlock()
		rows, e := p.f.db.Query(`SELECT payload_json FROM outbox WHERE event_type='checkout.fiscal_status'`)
		if e == nil {
			defer rows.Close()
			for rows.Next() {
				var raw string
				var status map[string]any
				if rows.Scan(&raw) == nil && json.Unmarshal([]byte(raw), &status) == nil {
					t.Logf("fiscal status only: state=%v code=%v", status["state"], status["error_code"])
				}
			}
		}
		t.Fatalf("rendered checkout Focus simulation: %v\n%s", err, out)
	}
	var observed struct {
		Sales []string `json:"sales"`
	}
	if json.Unmarshal(bytes.TrimSpace(out), &observed) != nil || len(observed.Sales) != 2 {
		t.Fatal("missing browser proof")
	}
	mu.Lock()
	if posts != 2 || queries < 1 || downloads != 2 {
		t.Errorf("remote ledger: posts=%d queries=%d XML downloads=%d", posts, queries, downloads)
	}
	mu.Unlock()
	for _, id := range observed.Sales {
		var count int
		if err = p.f.db.QueryRow(`SELECT count(*) FROM sales WHERE tenant_id=? AND id=?`, p.f.owner.TenantID, id).Scan(&count); err != nil || count != 1 {
			t.Fatal("sale duplication", err)
		}
		if code, _ := request(t, p.f.app, "POST", planPath+"/"+id+"/finalize-cash", `{"operation_id":"production-probe","received_cents":2000,"fiscal_invoice":{"Environment":1}}`, p.f.token); code != 400 {
			t.Fatal("production accepted", code)
		}
	}
	from, until := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	code, export := request(t, p.f.app, "GET", "/local/v1/fiscal/accounting-export?"+url.Values{"from": {from}, "until": {until}, "lang": {"pt-BR"}}.Encode(), "", p.f.token)
	if code != 200 {
		t.Fatal("accounting XML export", code)
	}
	z, err := zip.NewReader(bytes.NewReader(export), int64(len(export)))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, file := range z.File {
		if strings.HasSuffix(file.Name, ".xml") {
			r, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			data, e := io.ReadAll(r)
			r.Close()
			if e != nil || (!bytes.Equal(data, xml1) && !bytes.Equal(data, xml2)) {
				t.Fatal("exported XML changed")
			}
			found++
		}
	}
	if found != 2 {
		t.Fatal("missing archived XML", found)
	}
	var plaintext int
	if err = p.f.db.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_xml_archive' AND (payload_json LIKE '%nfeProc%' OR payload_json LIKE '%<NFe%')`).Scan(&plaintext); err != nil || plaintext != 0 {
		t.Fatal("plaintext XML in archive metadata", err)
	}
	t.Log("PASS actual Checkout96 rendered in Chromium -> HTTP/SQLite -> simulated Focus; two sales, one dropped response, explicit GET recovery, two encrypted XMLs exported unchanged and 80mm coupons; production refused; not official acceptance")
}
