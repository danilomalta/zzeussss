package fiscalproduction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
)

// API contracts: https://doc.focusnfe.com.br/reference/{emitir,consultar,cancelar}_nfce
// No local signing, XSD, numbering or financial cancellation is performed here.
const focusHomologation = "https://homologacao.focusnfe.com.br"

var ErrFocusA3 = errors.New("Focus NF-e/NFC-e requires A1; A3 is not supported")
var errFocusUncertain = errors.New("Focus fiscal response requires consultation")
var errFocusRejected = errors.New("Focus fiscal request rejected")

type FocusProvider struct {
	configuration TenantFiscalConfiguration
	token         []byte
	csc           []byte
	client        *http.Client
}

func (*FocusProvider) String() string               { return "[Focus fiscal provider]" }
func (*FocusProvider) GoString() string             { return "[Focus fiscal provider]" }
func (*FocusProvider) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }

func newFocusProvider(c TenantFiscalConfiguration, token []byte, csc ...[]byte) (*FocusProvider, error) {
	if c.DocumentModel != "" && c.DocumentModel != "55" && c.DocumentModel != "65" {
		return nil, ErrUnavailable
	}
	if c.CertificateType == "A3" {
		return nil, ErrFocusA3
	}
	if c.Environment != "homologation" {
		return nil, ErrCheckoutProduction
	}
	if c.FocusBaseURL == "" {
		c.FocusBaseURL = focusHomologation
	}
	if c.FocusBaseURL != focusHomologation || c.CertificateType != "A1" || len(token) == 0 || len(token) > 512 {
		return nil, ErrUnavailable
	}
	for _, b := range token {
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			return nil, ErrUnavailable
		}
	}
	var cscCopy []byte
	if len(csc) > 0 {
		cscCopy = append([]byte(nil), csc[0]...)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &FocusProvider{configuration: c, token: append([]byte(nil), token...), csc: cscCopy, client: &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func focusReference(tenant, sale string) string {
	raw, _ := json.Marshal([]string{tenant, sale})
	hash := sha256.Sum256(raw)
	return "titan" + hex.EncodeToString(hash[:])
}
func (p *FocusProvider) scope(ctx context.Context, a RequestContext, permission identity.Permission) error {
	if p == nil || p.configuration.FocusBaseURL != focusHomologation || p.configuration.Environment != "homologation" {
		return ErrCheckoutProduction
	}
	if p.configuration.CertificateType == "A3" {
		return ErrFocusA3
	}
	b := p.configuration.Binding
	if a.DB == nil || a.License == nil || p.configuration.CertificateType != "A1" || len(p.token) == 0 || a.Actor.TenantID != b.TenantID || a.Actor.StoreID != b.StoreID || a.Device.TenantID != b.TenantID || a.Device.StoreID != b.StoreID || a.Device.DeviceID != b.DeviceID {
		return ErrUnavailable
	}
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return ErrUnavailable
	}
	defer tx.Rollback()
	if err = fiscal.ValidateSaleSessionTx(ctx, tx, a.Actor, a.Device, a.Token); err != nil {
		return err
	}
	if err = a.License.RequireTx(ctx, tx, a.Actor, a.Device, permission, modules.Fiscal); err != nil {
		return err
	}
	var barriers int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, a.Actor.TenantID).Scan(&barriers); err != nil {
		return ErrUnavailable
	}
	if barriers != 0 {
		return fiscal.ErrRestored
	}
	if permission == identity.CancelSale {
		var role string
		if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.Actor.TenantID, a.Actor.IdentityID).Scan(&role); err != nil {
			return ErrUnavailable
		}
		if role != "owner" && role != "manager" {
			return identity.ErrDenied
		}
	}
	return tx.Commit()
}
func (p *FocusProvider) ValidateCheckout(ctx context.Context, a RequestContext, environment int) error {
	return p.validateModel(ctx, a, environment, "65")
}

func (p *FocusProvider) validateModel(ctx context.Context, a RequestContext, environment int, model string) error {
	if environment != 2 {
		return ErrCheckoutProduction
	}
	if p == nil || p.documentModel() != model {
		return ErrUnavailable
	}
	if err := p.scope(ctx, a, identity.Sell); err != nil {
		return err
	}
	profile, err := fiscal.ReadProfile(ctx, a.DB, a.Actor, a.Device, p.configuration.Binding.ProfileID, 0)
	if err != nil {
		return err
	}
	if profile.Revision != p.configuration.Binding.ProfileRevision || profile.Input.Environment != "homologation" || profile.Input.Model != model || profile.Input.CRT != p.configuration.CRT {
		return ErrUnavailable
	}
	return nil
}
func (p *FocusProvider) intent(ctx context.Context, a RequestContext, in DocumentRequest) (fiscal.Intent, error) {
	if in.Environment != 2 {
		return fiscal.Intent{}, ErrCheckoutProduction
	}
	if (in.Model != "65" && in.Model != "55") || in.SaleID == "" || len(in.SaleID) > 128 {
		return fiscal.Intent{}, fiscal.ErrInvalid
	}
	if err := p.validateModel(ctx, a, 2, in.Model); err != nil {
		return fiscal.Intent{}, err
	}
	b := p.configuration.Binding
	var other int
	if err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_intents WHERE tenant_id=? AND store_id=? AND json_extract(request_json,'$.sale_id')=? AND operation_id<>?`, a.Actor.TenantID, a.Actor.StoreID, in.SaleID, focusReference(a.Actor.TenantID, in.SaleID)).Scan(&other); err != nil {
		return fiscal.Intent{}, ErrUnavailable
	}
	if other != 0 {
		return fiscal.Intent{}, fiscal.ErrConflict
	}
	return fiscal.Prepare(ctx, a.DB, a.License, a.Actor, a.Device, fiscal.IntentInput{OperationID: focusReference(a.Actor.TenantID, in.SaleID), SaleID: in.SaleID, ProfileID: b.ProfileID, ProfileRevision: b.ProfileRevision}, a.Token)
}

// A unique durable submission marker is committed BEFORE HTTP. A crash, timeout
// or concurrent request can then consult only, never acquire another POST.
func (p *FocusProvider) claim(ctx context.Context, a RequestContext, sale, ref, event string, body []byte) (bool, error) {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err = fiscal.ValidateSaleSessionTx(ctx, tx, a.Actor, a.Device, a.Token); err != nil {
		return false, err
	}
	permission := identity.Sell
	if event == "checkout.focus_cancel" {
		permission = identity.CancelSale
	}
	if err = a.License.RequireTx(ctx, tx, a.Actor, a.Device, permission, modules.Fiscal); err != nil {
		return false, err
	}
	var barriers int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM fiscal_restore_barriers WHERE tenant_id=?`, a.Actor.TenantID).Scan(&barriers); err != nil {
		return false, ErrUnavailable
	}
	if barriers != 0 {
		return false, fiscal.ErrRestored
	}
	if permission == identity.CancelSale {
		var role string
		if err = tx.QueryRowContext(ctx, `SELECT role FROM memberships WHERE tenant_id=? AND identity_id=? AND status='active'`, a.Actor.TenantID, a.Actor.IdentityID).Scan(&role); err != nil {
			return false, ErrUnavailable
		}
		if role != "owner" && role != "manager" {
			return false, identity.ErrDenied
		}
	}

	id, err := localdb.NewID()
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(body)
	payload, _ := json.Marshal(struct {
		Reference string `json:"ref"`
		SHA256    string `json:"request_sha256"`
	}{ref, hex.EncodeToString(sum[:])})
	result, err := tx.ExecContext(ctx, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,?,1,?,?) ON CONFLICT(tenant_id,device_id,operation_id,event_type) DO NOTHING`, id, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, ref, sale, event, string(payload), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, tx.Commit()
}

type focusReply struct {
	CNPJ         string `json:"cnpj_emitente"`
	Reference    string `json:"ref"`
	Status       string `json:"status"`
	StatusSEFAZ  string `json:"status_sefaz"`
	MessageSEFAZ string `json:"mensagem_sefaz"`
	Key          string `json:"chave_nfe"`
	DANFE        string `json:"caminho_danfe"`
	Message      string `json:"mensagem"`
	Code         string `json:"codigo"`
	Errors       []struct {
		Message string `json:"mensagem"`
	} `json:"erros"`
}

func (p *FocusProvider) safeMessage(s string) string {
	// Provider responses may echo credentials. Redact before persistence or HTTP.
	for _, secret := range []string{string(p.token), string(p.csc), base64.StdEncoding.EncodeToString(append(append([]byte{}, p.token...), ':'))} {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[redacted]")
		}
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	runes := []rune(s)
	if len(runes) > 1000 {
		runes = runes[:1000]
	}
	return string(runes)
}
func (p *FocusProvider) exchange(ctx context.Context, method, ref, cnpj string, body []byte) (DocumentResult, error) {
	return p.exchangeModel(ctx, method, ref, cnpj, body, "65")
}

func (p *FocusProvider) exchangeModel(ctx context.Context, method, ref, cnpj string, body []byte, model string) (DocumentResult, error) {
	out := DocumentResult{State: "recovery_required", OperationID: ref, ErrorCode: "focus_consultation_required"}
	if method == http.MethodDelete {
		out.State = fiscal.CheckoutCancellationFailed
		out.Message = "Cancellation response unavailable; consultation required"
	}
	if p == nil || p.configuration.FocusBaseURL != focusHomologation || p.configuration.Environment != "homologation" {
		return out, ErrCheckoutProduction
	}
	resource := "/v2/nfce"
	if model == "55" {
		resource = "/v2/nfe"
	} else if model != "65" {
		return out, fiscal.ErrInvalid
	}
	path := resource + "/" + url.PathEscape(ref)
	if method == http.MethodPost {
		path = resource + "?ref=" + url.QueryEscape(ref)
	}
	req, err := http.NewRequestWithContext(ctx, method, focusHomologation+path, bytes.NewReader(body))
	if err != nil {
		return out, ErrUnavailable
	}
	req.SetBasicAuth(string(p.token), "")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return out, errFocusUncertain
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 {
		return out, errFocusUncertain
	}
	var reply focusReply
	if json.Unmarshal(raw, &reply) != nil {
		return out, errFocusUncertain
	}
	message := reply.MessageSEFAZ
	if message == "" {
		message = reply.Message
	}
	for _, e := range reply.Errors {
		if e.Message != "" {
			message += " " + e.Message
		}
	}
	out.Message = p.safeMessage(message)
	if method == http.MethodDelete {
		out.State = fiscal.CheckoutCancellationFailed
		out.ErrorCode = "focus_cancellation_failed"
		if resp.StatusCode == http.StatusOK && reply.Status == "cancelado" {
			out.State = fiscal.CheckoutCancelled
			out.ErrorCode = ""
			return out, nil
		}
		if resp.StatusCode == http.StatusOK && reply.Status == "erro_cancelamento" || resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return out, errFocusRejected
		}
		return out, errFocusUncertain
	}
	expected := http.StatusOK
	if method == http.MethodPost {
		expected = http.StatusCreated
	}
	if resp.StatusCode != expected && !(model == "55" && method == http.MethodPost && resp.StatusCode == http.StatusAccepted) {
		if method == http.MethodPost {
			switch resp.StatusCode {
			case 400, 401, 403, 415, 422:
				out.State = "failed"
				out.ErrorCode = "focus_request_rejected"
				return out, errFocusRejected
			}
		}
		return out, errFocusUncertain
	}
	if reply.Reference != ref || reply.CNPJ != cnpj {
		return out, errFocusUncertain
	}
	switch reply.Status {
	case "autorizado":
		key := strings.TrimPrefix(reply.Key, "NFe")
		if !fiscalcore.ValidAccessKey(key) || key[6:20] != cnpj || key[20:22] != model {
			return out, errFocusUncertain
		}
		out.State = "authorized_homologation"
		out.Key = key
		if model == "55" && safeFocusDANFEPath(reply.DANFE, cnpj, key) && p.safeMessage(reply.DANFE) == reply.DANFE {
			out.Evidence, _ = json.Marshal(map[string]string{"danfe_path": reply.DANFE})
		}
		out.ErrorCode = ""
		return out, nil
	case "erro_autorizacao", "denegado":
		out.State = "failed"
		out.ErrorCode = "focus_authorization_rejected"
		return out, errFocusRejected
	case "processando_autorizacao":
		out.State = "pending"
		return out, nil
	case "cancelado":
		out.State = fiscal.CheckoutCancelled
		out.ErrorCode = ""
		return out, nil
	default:
		return out, errFocusUncertain
	}
}
func (p *FocusProvider) save(ctx context.Context, a RequestContext, sale, intent string, out DocumentResult, err error) (DocumentResult, error) {
	out.IntentID = intent
	durable, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	if e := fiscal.RecordCheckoutStatus(durable, a.DB, a.Actor, a.Device, sale, fiscal.CheckoutStatus{State: out.State, OperationID: out.OperationID, IntentID: intent, Key: out.Key, ErrorCode: out.ErrorCode, Message: out.Message}); e != nil {
		out.ErrorCode = "fiscal_status_persistence_failed"
		return out, ErrUnavailable
	}
	return out, err
}
func (p *FocusProvider) Emit(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	intent, err := p.intent(ctx, a, in)
	if err != nil {
		return DocumentResult{}, err
	}
	ref := focusReference(a.Actor.TenantID, in.SaleID)
	body, err := focusInvoiceRequest(ctx, a, intent, in)
	if err != nil {
		return p.save(ctx, a, in.SaleID, intent.ID, DocumentResult{State: "failed", OperationID: ref, ErrorCode: "focus_request_unsupported"}, err)
	}
	return p.submit(ctx, a, in.SaleID, intent, ref, body)
}

// submit is shared by the adapter and its fault-injection tests. Durable claim
// and checkout status use the real SQLite store even when HTTP is httptest.
func (p *FocusProvider) submit(ctx context.Context, a RequestContext, sale string, intent fiscal.Intent, ref string, body []byte) (DocumentResult, error) {
	first, err := p.claim(ctx, a, sale, ref, "checkout.focus_submission", body)
	if err != nil {
		return DocumentResult{}, err
	}
	method := http.MethodPost
	if !first {
		method = http.MethodGet
		body = nil
	}
	model := intent.Snapshot.Profile.Model
	out, err := p.exchangeModel(ctx, method, ref, intent.Snapshot.Profile.CNPJ, body, model)
	if method == http.MethodGet {
		out = p.preserveCancellationFailure(ctx, a, sale, out)
	}
	return p.save(ctx, a, sale, intent.ID, out, err)
}

func (p *FocusProvider) Query(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	intent, err := p.intent(ctx, a, in)
	if err != nil {
		return DocumentResult{}, err
	}
	out, err := p.exchangeModel(ctx, http.MethodGet, focusReference(a.Actor.TenantID, in.SaleID), intent.Snapshot.Profile.CNPJ, nil, in.Model)
	out = p.preserveCancellationFailure(ctx, a, in.SaleID, out)
	return p.save(ctx, a, in.SaleID, intent.ID, out, err)
}
func (p *FocusProvider) Reprocess(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	// Reprocessing ONLY consults. Neither a timeout nor even GET 404 proves that
	// an earlier POST was not accepted. Corrected-payload resubmission is separate.
	return p.Query(ctx, a, in)
}
func (p *FocusProvider) Cancel(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	if err := p.scope(ctx, a, identity.CancelSale); err != nil {
		return DocumentResult{}, err
	}
	if utf8.RuneCountInString(in.Reason) < 15 || utf8.RuneCountInString(in.Reason) > 255 || in.OperationID == "" || in.ApprovalReference == "" {
		return DocumentResult{}, fiscal.ErrInvalid
	}
	intent, err := p.intent(ctx, a, in)
	if err != nil {
		return DocumentResult{}, err
	}
	ref := focusReference(a.Actor.TenantID, in.SaleID)
	current, err := p.exchangeModel(ctx, http.MethodGet, ref, intent.Snapshot.Profile.CNPJ, nil, in.Model)
	if err != nil {
		return p.save(ctx, a, in.SaleID, intent.ID, current, err)
	}
	if current.State == fiscal.CheckoutCancelled {
		return p.save(ctx, a, in.SaleID, intent.ID, current, nil)
	}
	if current.State != "authorized_homologation" {
		return DocumentResult{}, fiscal.ErrConflict
	}
	body, _ := json.Marshal(struct {
		Reason string `json:"justificativa"`
	}{in.Reason})
	// One DELETE for this explicit operator operation; a replay only consults.
	opSum := sha256.Sum256([]byte(in.OperationID))
	op := ref + hex.EncodeToString(opSum[:16])
	first, err := p.claim(ctx, a, in.SaleID, op, "checkout.focus_cancel", body)
	if err != nil {
		return DocumentResult{}, err
	}
	if !first {
		return p.save(ctx, a, in.SaleID, intent.ID, p.preserveCancellationFailure(ctx, a, in.SaleID, current), fiscalcore.ErrRecorded)
	}
	out, err := p.exchangeModel(ctx, http.MethodDelete, ref, intent.Snapshot.Profile.CNPJ, body, in.Model)
	out.Key = current.Key
	return p.save(ctx, a, in.SaleID, intent.ID, out, err)
}

// A failed explicit cancellation remains a fiscal cancellation failure after a
// read-only consultation: an authorized note does not mean cancellation passed.
func (p *FocusProvider) preserveCancellationFailure(ctx context.Context, a RequestContext, sale string, out DocumentResult) DocumentResult {
	if out.State != "authorized_homologation" {
		return out
	}
	var raw []byte
	if a.DB.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND aggregate_id=? AND event_type='checkout.fiscal_status' ORDER BY created_at DESC,event_id DESC LIMIT 1`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, sale).Scan(&raw) != nil {
		return out
	}
	var previous fiscal.CheckoutStatus
	if json.Unmarshal(raw, &previous) == nil && previous.State == fiscal.CheckoutCancellationFailed {
		out.State = previous.State
		out.Message = p.safeMessage(previous.Message)
		out.ErrorCode = previous.ErrorCode
	}
	return out
}
