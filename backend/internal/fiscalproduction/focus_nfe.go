package fiscalproduction

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// Contract: https://campos.focusnfe.com.br/nfe_homologacao/NotaFiscalXML.html
// Only the existing frozen cash-sale tax case is mapped. No tax defaults.
var ErrFocusNFeIncomplete = errors.New("NF-e requires explicit nature, direction, destination, final consumer, purpose, freight mode and recipient identity/address; only normal internal consumer cash sales without freight are supported")
var ErrFocusInvalidationUnavailable = errors.New("Focus invalidation unavailable: year contract not established")

func (p *FocusProvider) documentModel() string {
	if p.configuration.DocumentModel == "" {
		return "65"
	}
	return p.configuration.DocumentModel
}

func validateFocusNFeInput(t fiscalcore.Invoice, in DocumentRequest, p fiscal.ProfileInput) error {
	if t.Presence != "1" || in.Purpose != "1" || in.FreightMode != "9" || t.Direction != "1" || t.Destination != "1" || t.FinalConsumer != "1" || strings.TrimSpace(t.Nature) != t.Nature || utf8.RuneCountInString(t.Nature) < 1 || utf8.RuneCountInString(t.Nature) > 60 {
		return ErrFocusNFeIncomplete
	}
	r := t.Recipient
	if r == nil || (r.CNPJ == "") == (r.CPF == "") || r.IEDestination != "9" || r.Address.UF != p.State {
		return ErrFocusNFeIncomplete
	}
	if r.CNPJ != "" && !fiscal.ValidCNPJ(r.CNPJ) {
		return ErrFocusNFeIncomplete
	}
	// Validate identity before transmitting customer data.
	if r.CPF != "" && !validFocusCPF(r.CPF) {
		return ErrFocusNFeIncomplete
	}
	for _, value := range []string{r.Name, r.Address.Street, r.Address.District, r.Address.City} {
		if strings.TrimSpace(value) != value || utf8.RuneCountInString(value) < 2 || utf8.RuneCountInString(value) > 60 {
			return ErrFocusNFeIncomplete
		}
	}
	if r.Address.Number == "" || len(r.Address.Number) > 60 {
		return ErrFocusNFeIncomplete
	}
	return nil
}

func validFocusCPF(s string) bool {
	if len(s) != 11 || s == strings.Repeat(s[:1], 11) {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	for n := 9; n <= 10; n++ {
		sum := 0
		for i := 0; i < n; i++ {
			sum += int(s[i]-'0') * (n + 1 - i)
		}
		d := 11 - sum%11
		if d >= 10 {
			d = 0
		}
		if int(s[n]-'0') != d {
			return false
		}
	}
	return true
}

// No arbitrary URL, query, redirect or cross-issuer download is accepted.
func safeFocusDANFEPath(value, cnpj, key string) bool {
	if value == "" || path.Clean(value) != value || !strings.HasPrefix(value, "/arquivos/"+cnpj+"/") || path.Base(value) != key+".pdf" {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("/_.-", c)) {
			return false
		}
	}
	u, err := url.Parse(value)
	return err == nil && u.Host == "" && u.Scheme == "" && u.RawQuery == "" && u.Fragment == ""
}

// DANFE downloads only the path returned by an authorized model-55 consultation.
// It does not generate a DANFE locally and never returns the provider token.
func (p *FocusProvider) DANFE(ctx context.Context, a RequestContext, in DocumentRequest) ([]byte, error) {
	if in.Model != "55" {
		return nil, ErrUnavailable
	}
	result, err := p.Query(ctx, a, in)
	if err != nil {
		return nil, err
	}
	var evidence struct {
		Path string `json:"danfe_path"`
	}
	if result.State != "authorized_homologation" || json.Unmarshal(result.Evidence, &evidence) != nil || evidence.Path == "" {
		return nil, ErrUnavailable
	}
	return p.downloadDANFE(ctx, evidence.Path, result.Key)
}

func (p *FocusProvider) downloadDANFE(ctx context.Context, resource, key string) ([]byte, error) {
	if p == nil || p.configuration.Environment != "homologation" || p.configuration.FocusBaseURL != focusHomologation {
		return nil, ErrCheckoutProduction
	}
	if !fiscalcore.ValidAccessKey(key) || key[20:22] != "55" || !safeFocusDANFEPath(resource, key[6:20], key) || p.safeMessage(resource) != resource {
		return nil, ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, focusHomologation+resource, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	req.SetBasicAuth(string(p.token), "")
	req.Header.Set("Accept", "application/pdf")
	response, err := p.client.Do(req)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024+1))
	if err != nil || len(data) > 16*1024*1024 || !strings.HasPrefix(string(data), "%PDF-") {
		return nil, ErrUnavailable
	}
	for _, secret := range [][]byte{p.token, p.csc} {
		if len(secret) > 0 && strings.Contains(string(data), string(secret)) {
			return nil, ErrUnavailable
		}
	}
	return data, nil
}

// Invalidation remains explicitly unavailable: no guessed fiscal year.
func (p *FocusProvider) Invalidate(context.Context, RequestContext, DocumentRequest) (DocumentResult, error) {
	return DocumentResult{State: "unavailable", ErrorCode: "focus_invalidation_unavailable"}, ErrFocusInvalidationUnavailable
}
