package fiscalproduction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// Local fault/contract evidence only, not Focus/SEFAZ acceptance.
func TestFocusNFeTimeoutConsultsSameReference(t *testing.T) {
	a := focusContext(t)
	ref := focusReference(a.Actor.TenantID, "sale55")
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: "12345678000123", Model: "55", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	var posts, gets atomic.Int32
	p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if r.URL.Path != "/v2/nfe" || r.URL.Query().Get("ref") != ref {
				t.Error("wrong emission resource")
			}
			posts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
			return
		}
		if r.URL.Path != "/v2/nfe/"+ref {
			t.Error("wrong consultation")
		}
		gets.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{"cnpj_emitente": "12345678000123", "ref": ref, "status": "autorizado", "chave_nfe": "NFe" + key, "caminho_danfe": "/arquivos/12345678000123/202610/DANFEs/" + key + ".pdf"})
	})
	intent := fiscal.Intent{ID: "intent55", Snapshot: fiscal.Snapshot{Profile: fiscal.ProfileInput{CNPJ: "12345678000123", Model: "55"}}}
	p.client.Timeout = 30 * time.Millisecond
	first, err := p.submit(context.Background(), a, "sale55", intent, ref, []byte(`{}`))
	if !errors.Is(err, errFocusUncertain) || first.State != "recovery_required" {
		t.Fatal("uncertain submission lost")
	}
	p.client.Timeout = 2 * time.Second
	restarted := *p
	for i := 0; i < 2; i++ {
		out, e := restarted.submit(context.Background(), a, "sale55", intent, ref, []byte(`{}`))
		if e != nil || out.Key != key || out.State != "authorized_homologation" || !strings.Contains(string(out.Evidence), "danfe_path") {
			t.Fatal("consultation mapping failed", e)
		}
	}
	if posts.Load() != 1 || gets.Load() != 2 {
		t.Fatal("duplicate emission")
	}
}

func TestFocusNFeAsyncFailureCancellationAndModelBinding(t *testing.T) {
	for _, scenario := range []string{"pending", "cancelled", "refused", "malformed", "wrong-model"} {
		t.Run(scenario, func(t *testing.T) {
			p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.TrimSuffix(r.URL.Path, "/ref") != "/v2/nfe" {
					t.Error("wrong NF-e resource")
				}
				switch scenario {
				case "pending":
					w.WriteHeader(202)
					_, _ = io.WriteString(w, `{"ref":"ref","cnpj_emitente":"12345678000123","status":"processando_autorizacao"}`)
				case "cancelled":
					_, _ = io.WriteString(w, `{"status":"cancelado"}`)
				case "refused":
					w.WriteHeader(422)
					_, _ = io.WriteString(w, `{"mensagem":"Recusa SyntheticFocusToken123 SyntheticCSC123"}`)
				case "malformed":
					_, _ = io.WriteString(w, `{`)
				case "wrong-model":
					_ = json.NewEncoder(w).Encode(map[string]string{"ref": "ref", "cnpj_emitente": "12345678000123", "status": "autorizado", "chave_nfe": "NFe" + syntheticFocusKey(t)})
				}
			})
			method := http.MethodDelete
			if scenario == "pending" {
				method = http.MethodPost
			}
			if scenario == "wrong-model" {
				method = http.MethodGet
			}
			out, err := p.exchangeModel(context.Background(), method, "ref", "12345678000123", []byte(`{"justificativa":"Cancelamento solicitado pelo operador"}`), "55")
			switch scenario {
			case "pending":
				if err != nil || out.State != "pending" {
					t.Fatal("async response", err)
				}
			case "cancelled":
				if err != nil || out.State != fiscal.CheckoutCancelled {
					t.Fatal("cancellation response", err)
				}
			default:
				if err == nil {
					t.Fatal("failure accepted")
				}
			}
			if scenario == "refused" && (out.State != fiscal.CheckoutCancellationFailed || !strings.Contains(out.Message, "Recusa")) {
				t.Fatal("reason lost")
			}
			if strings.Contains(fmt.Sprint(out, err), "SyntheticFocusToken123") || strings.Contains(fmt.Sprint(out, err), "SyntheticCSC123") {
				t.Fatal("secret exposed")
			}
		})
	}
}

func TestFocusNFeRequiredDataNoDefaults(t *testing.T) {
	p := fiscal.ProfileInput{State: "SP"}
	in := DocumentRequest{Purpose: "1", FreightMode: "9"}
	inv := fiscalcore.Invoice{Nature: "VENDA", Presence: "1", Direction: "1", Destination: "1", FinalConsumer: "1", Recipient: &fiscalcore.Party{CPF: "12345678909", Name: "Synthetic consumer", IEDestination: "9", Address: fiscalcore.Address{Street: "Rua Teste", Number: "1", District: "Centro", City: "Sao Paulo", UF: "SP"}}}
	if err := validateFocusNFeInput(inv, in, p); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*fiscalcore.Invoice, *DocumentRequest){
		func(i *fiscalcore.Invoice, d *DocumentRequest) { d.Purpose = "" }, func(i *fiscalcore.Invoice, d *DocumentRequest) { d.FreightMode = "" },
		func(i *fiscalcore.Invoice, d *DocumentRequest) { i.Recipient = nil }, func(i *fiscalcore.Invoice, d *DocumentRequest) { i.Nature = "" },
		func(i *fiscalcore.Invoice, d *DocumentRequest) { i.Destination = "2" }, func(i *fiscalcore.Invoice, d *DocumentRequest) { i.FinalConsumer = "0" },
	} {
		copy, req := inv, in
		change(&copy, &req)
		if !errors.Is(validateFocusNFeInput(copy, req, p), ErrFocusNFeIncomplete) {
			t.Fatal("missing/unsupported input defaulted")
		}
	}
	if safeFocusDANFEPath("https://evil.invalid/note.pdf", "123", "key") || safeFocusDANFEPath("/arquivos/123/../456/key.pdf", "123", "key") || safeFocusDANFEPath("/arquivos/456/key.pdf", "123", "key") {
		t.Fatal("unsafe download")
	}
}

func TestFocusNFeCustomerVaultAndCheckoutSeparation(t *testing.T) {
	r, a, _ := customerFixture(t)
	ctx := context.Background()
	profile, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, "profile", 0)
	if err != nil {
		t.Fatal(err)
	}
	input := profile.Input
	input.OperationID = "profile55"
	input.ExpectedRevision = 1
	input.Model = "55"
	if _, err = fiscal.Configure(ctx, a.DB, a.License, a.Actor, a.Device, input, a.Token); err != nil {
		t.Fatal(err)
	}
	_, err = r.SaveCustomerCredentials(ctx, a, "profile", CustomerCredentialInput{OperationID: "nfe55-token", ProfileRevision: 2, Token: []byte("SyntheticNFeToken123")})
	if err != nil {
		t.Fatal(err)
	}
	provider, _, err := r.Resolve(a)
	if err != nil {
		t.Fatal(err)
	}
	p := provider.(*FocusProvider)
	if err = p.validateModel(ctx, a, 2, "55"); err != nil {
		t.Fatal(err)
	}
	if err = p.ValidateCheckout(ctx, a, 2); err == nil {
		t.Fatal("checkout NFC-e accepted NF-e profile")
	}
	if _, err = p.Emit(ctx, a, DocumentRequest{Model: "55", Environment: 1, SaleID: "sale"}); !errors.Is(err, ErrCheckoutProduction) {
		t.Fatal("production emission allowed")
	}
	out, err := p.Invalidate(ctx, a, DocumentRequest{Model: "55"})
	if !errors.Is(err, ErrFocusInvalidationUnavailable) || out.State != "unavailable" {
		t.Fatal("invalidation enabled")
	}
}

func TestFocusNFeDANFEDownloadRestricted(t *testing.T) {
	key, err := fiscalcore.AccessKey(fiscalcore.KeyInput{UF: "SP", IssuedAt: time.Now(), CNPJ: "12345678000123", Model: "55", Series: 1, Number: 1, Emission: 1, NumericCode: "12345678"})
	if err != nil {
		t.Fatal(err)
	}
	resource := "/arquivos/12345678000123/202610/DANFEs/" + key + ".pdf"
	for _, scenario := range []string{"pdf", "html", "redirect", "secret"} {
		t.Run(scenario, func(t *testing.T) {
			p := faultFocus(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != resource {
					t.Error("wrong file")
				}
				user, password, ok := r.BasicAuth()
				if !ok || user != "SyntheticFocusToken123" || password != "" {
					t.Error("wrong authentication")
				}
				switch scenario {
				case "pdf":
					w.Header().Set("Content-Type", "application/pdf")
					_, _ = io.WriteString(w, "%PDF-1.4\nsynthetic download fixture")
				case "html":
					_, _ = io.WriteString(w, "<html>not a pdf</html>")
				case "secret":
					_, _ = io.WriteString(w, "%PDF-1.4\nSyntheticFocusToken123")
				case "redirect":
					http.Redirect(w, r, "https://outside.invalid/secret", http.StatusFound)
				}
			})
			data, e := p.downloadDANFE(context.Background(), resource, key)
			if scenario == "pdf" {
				if e != nil || !strings.HasPrefix(string(data), "%PDF-") {
					t.Fatal("PDF download", e)
				}
			} else if e == nil || data != nil {
				t.Fatal("unsafe response accepted")
			}
			if _, e = p.downloadDANFE(context.Background(), "https://outside.invalid/secret", key); e == nil {
				t.Fatal("foreign path accepted")
			}
		})
	}
}
