package fiscalproduction

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/localsetup"
)

// CATEGORY 1: synthetic failures only. None proves acceptance by Focus/SEFAZ.
type focusFaultTransport struct{ target *url.URL }

func (t focusFaultTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.Host != "homologacao.focusnfe.com.br" {
		return nil, errors.New("unexpected test destination")
	}
	clone := r.Clone(r.Context())
	u := *r.URL
	clone.URL = &u
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}
func faultFocus(t *testing.T, handler http.HandlerFunc) *FocusProvider {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	vault, _ := testVault(t)
	if err := vault.Put(context.Background(), "test-tenant", "token", []byte("SyntheticFocusToken123")); err != nil {
		t.Fatal(err)
	}
	token, err := vault.read(context.Background(), "test-tenant", "token")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(token)
	p, err := newFocusProvider(TenantFiscalConfiguration{Provider: "external", Environment: "homologation", CertificateType: "A1"}, token, []byte("SyntheticCSC123"))
	if err != nil {
		t.Fatal(err)
	}
	p.client.Transport = focusFaultTransport{target: target}
	return p
}
func focusContext(t *testing.T) RequestContext {
	t.Helper()
	ctx := context.Background()
	db, err := localdb.Open(ctx, filepath.Join(t.TempDir(), "focus-fault.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := localsetup.Initialize(ctx, db, localsetup.Input{TenantName: "Synthetic", StoreName: "Test", OwnerName: "Owner", DeviceName: "Test", Password: "synthetic-test-password", PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	device := identity.DeviceContext{TenantID: owner.TenantID, StoreID: owner.StoreID, DeviceID: owner.DeviceID}
	actor := identity.Scope{TenantID: owner.TenantID, StoreID: owner.StoreID, IdentityID: owner.OwnerID}
	issuer, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := entitlements.NewVerifier(map[string]ed25519.PublicKey{"fault-test": issuer})
	if err != nil {
		t.Fatal(err)
	}
	license, err := entitlementstore.New(db, verifier, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	raw, _ := json.Marshal(entitlements.Claims{Version: 1, TenantID: owner.TenantID, Revision: 1, IssuedAt: now - 60, NotBefore: now - 60, ExpiresAt: now + 3600, Modules: []modules.ID{modules.Core, modules.Fiscal}})
	if _, err = license.Install(ctx, actor, device, entitlements.Envelope{KeyID: "fault-test", Payload: raw, Signature: ed25519.Sign(private, entitlements.SigningMessage("fault-test", raw))}); err != nil {
		t.Fatal(err)
	}
	session, err := localauth.Login(ctx, db, device, owner.OwnerID, "synthetic-test-password")
	if err != nil {
		t.Fatal(err)
	}
	return RequestContext{DB: db, License: license, Actor: actor, Device: device, Token: session.Token}
}
func syntheticFocusKey(t *testing.T) string {
	t.Helper()
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: "12345678000123", Model: "65", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	return key
}
func TestFocusFaultTimeoutThenConsultationNeverDuplicates(t *testing.T) {
	for _, remote := range []string{"autorizado", "cancelado"} {
		t.Run(remote, func(t *testing.T) {
			a := focusContext(t)
			ref := focusReference(a.Actor.TenantID, "sale")
			var posts, gets atomic.Int32
			key := syntheticFocusKey(t)
			p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
				user, password, ok := r.BasicAuth()
				if !ok || user != "SyntheticFocusToken123" || password != "" {
					t.Error("wrong basic auth")
				}
				if r.Method == http.MethodPost {
					posts.Add(1)
					if r.URL.Path != "/v2/nfce" || r.URL.Query().Get("ref") != ref {
						t.Error("wrong emission path")
					}
					_, _ = io.Copy(io.Discard, r.Body)
					select {
					case <-r.Context().Done():
					case <-time.After(time.Second):
					}
					return
				}
				gets.Add(1)
				if r.URL.Path != "/v2/nfce/"+ref {
					t.Error("wrong consultation reference")
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"cnpj_emitente": "12345678000123", "ref": ref, "status": remote, "chave_nfe": "NFe" + key})
			})
			p.client.Timeout = 30 * time.Millisecond
			intent := fiscal.Intent{ID: "synthetic-intent", Snapshot: fiscal.Snapshot{Profile: fiscal.ProfileInput{CNPJ: "12345678000123"}}}
			first, err := p.submit(context.Background(), a, "sale", intent, ref, []byte(`{"synthetic":"timeout test only"}`))
			if !errors.Is(err, errFocusUncertain) || first.State != "recovery_required" {
				t.Fatal("timeout must remain uncertain")
			}
			p.client.Timeout = time.Second
			// A fresh adapter simulates a restart; the real SQLite claim remains durable.
			copy := *p
			for i := 0; i < 2; i++ {
				out, e := copy.submit(context.Background(), a, "sale", intent, ref, []byte(`{"synthetic":"same logical sale"}`))
				expected := "authorized_homologation"
				if remote == "cancelado" {
					expected = fiscal.CheckoutCancelled
				}
				if e != nil || out.State != expected || (remote == "autorizado" && out.Key != key) {
					t.Fatal("fault recovery did not consult existing reference")
				}
			}
			if posts.Load() != 1 || gets.Load() != 2 {
				t.Fatal("duplicate submission", posts.Load(), gets.Load())
			}
			var count int
			if err = a.DB.QueryRow(`SELECT count(*) FROM outbox WHERE event_type='checkout.focus_submission'`).Scan(&count); err != nil || count != 1 {
				t.Fatal("duplicate durable marker")
			}
		})
	}
}
func TestFocusFaultRejectedMalformedAndCancellationReasons(t *testing.T) {
	for _, scenario := range []string{"rejected", "malformed", "cancellation-rejected", "cancellation-malformed"} {
		t.Run(scenario, func(t *testing.T) {
			p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
				switch scenario {
				case "rejected":
					w.WriteHeader(http.StatusCreated)
					_, _ = io.WriteString(w, `{"cnpj_emitente":"12345678000123","ref":"ref","status":"erro_autorizacao","mensagem_sefaz":"Rejeicao sintetica"}`)
				case "malformed", "cancellation-malformed":
					_, _ = io.WriteString(w, `{"status":`)
				default:
					w.WriteHeader(422)
					_, _ = io.WriteString(w, `{"codigo":"requisicao_invalida","mensagem":"Recusa sintetica SyntheticFocusToken123 SyntheticCSC123"}`)
				}
			})
			method := http.MethodPost
			if strings.HasPrefix(scenario, "cancellation") {
				method = http.MethodDelete
			}
			result, err := p.exchange(context.Background(), method, "ref", "12345678000123", []byte(`{}`))
			if err == nil {
				t.Fatal("synthetic failure accepted")
			}
			if strings.Contains(scenario, "malformed") && !errors.Is(err, errFocusUncertain) {
				t.Fatal("malformed response is uncertain")
			}
			if scenario == "rejected" && result.State != "failed" {
				t.Fatal("rejection mapping")
			}
			if method == http.MethodDelete && result.State != fiscal.CheckoutCancellationFailed {
				t.Fatal("fiscal cancellation error mapping")
			}
			text := fmt.Sprint(err, result, p)
			if strings.Contains(text, "SyntheticFocusToken123") || strings.Contains(text, "SyntheticCSC123") {
				t.Fatal("token disclosure")
			}
			if scenario == "cancellation-rejected" {
				a := focusContext(t)
				// Persistence is fiscal-only, even when the authority refuses cancellation.
				if _, e := p.save(context.Background(), a, "sale", "intent", result, err); !errors.Is(e, errFocusRejected) {
					t.Fatal("failure not persisted")
				}
				var raw string
				if e := a.DB.QueryRow(`SELECT payload_json FROM outbox WHERE event_type='checkout.fiscal_status'`).Scan(&raw); e != nil {
					t.Fatal(e)
				}
				if !strings.Contains(raw, "Recusa sintetica") || (strings.Contains(raw, "SyntheticFocusToken123") || strings.Contains(raw, "SyntheticCSC123")) {
					t.Fatal("reason lost or secret persisted")
				}
				visible := p.preserveCancellationFailure(context.Background(), a, "sale", DocumentResult{State: "authorized_homologation"})
				if visible.State != fiscal.CheckoutCancellationFailed || !strings.Contains(visible.Message, "Recusa sintetica") {
					t.Fatal("consultation hid cancellation failure")
				}
				recorder := httptest.NewRecorder()
				_ = json.NewEncoder(recorder).Encode(visible)
				if strings.Contains(recorder.Body.String(), "SyntheticFocusToken123") {
					t.Fatal("secret in response")
				}

				for _, table := range []string{"sales", "sale_payments", "stock_movements", "cash_sessions"} {
					var n int
					if e := a.DB.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); e != nil || n != 0 {
						t.Fatal("fiscal cancellation changed commercial data", table)
					}
				}
			}
		})
	}
}
func TestFocusFaultA3AndProductionAddressRefused(t *testing.T) {
	c := TenantFiscalConfiguration{Environment: "homologation", CertificateType: "A3"}
	if _, err := newFocusProvider(c, []byte("SyntheticToken")); !errors.Is(err, ErrFocusA3) {
		t.Fatal("A3 accepted")
	}
	c.CertificateType = "A1"
	for _, address := range []string{"https://api.focusnfe.com.br", "http://homologacao.focusnfe.com.br", focusHomologation + "/", focusHomologation + "@example.com"} {
		c.FocusBaseURL = address
		if _, err := newFocusProvider(c, []byte("SyntheticToken")); err == nil {
			t.Fatal("non-homologation address accepted")
		}
	}
	c.FocusBaseURL = focusHomologation
	c.Environment = "production"
	if _, err := newFocusProvider(c, []byte("SyntheticToken")); !errors.Is(err, ErrCheckoutProduction) {
		t.Fatal("production accepted")
	}
}

func TestFocusFaultVaultTenantProviderIsolation(t *testing.T) {
	vault, path := testVault(t)
	configurations := []TenantFiscalConfiguration{}
	for _, tenant := range []string{"tenant-one", "tenant-two"} {
		if err := vault.Put(context.Background(), tenant, "token", []byte("SyntheticToken"+strings.ReplaceAll(tenant, "-", ""))); err != nil {
			t.Fatal(err)
		}
		configurations = append(configurations, TenantFiscalConfiguration{Binding: ProviderBinding{TenantID: tenant, StoreID: "store", DeviceID: "device", ProfileID: "profile", ProfileRevision: 1}, Provider: "external", Environment: "homologation", CRT: "1", CertificateType: "A1"})
	}
	config := struct {
		Version   int                         `json:"version"`
		VaultPath string                      `json:"vault_path"`
		Tenants   []TenantFiscalConfiguration `json:"tenants"`
	}{1, path, configurations}
	raw, _ := json.Marshal(config)
	file := filepath.Join(t.TempDir(), "providers.json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := LoadProviderRegistry(file)
	if err != nil {
		t.Fatal(err)
	}
	var previous *FocusProvider
	for _, c := range configurations {
		a := RequestContext{Actor: identity.Scope{TenantID: c.Binding.TenantID, StoreID: "store"}, Device: identity.DeviceContext{TenantID: c.Binding.TenantID, StoreID: "store", DeviceID: "device"}}
		selected, _, err := registry.Resolve(a)
		if err != nil {
			t.Fatal(err)
		}
		focus, ok := selected.(*FocusProvider)
		if !ok || focus == previous {
			t.Fatal("wrong tenant provider")
		}
		if string(focus.token) != "SyntheticToken"+strings.ReplaceAll(c.Binding.TenantID, "-", "") {
			t.Fatal("wrong tenant vault token")
		}
		encoded, _ := json.Marshal(focus)
		if string(encoded) != "{}" {
			t.Fatal("provider serialized credentials")
		}
		a.Actor.TenantID = "foreign"
		if _, _, err := registry.Resolve(a); err == nil {
			t.Fatal("foreign tenant resolved provider")
		}
		previous = focus
	}
}
