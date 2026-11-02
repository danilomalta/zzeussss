package fiscalproduction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"
	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/identity"
	"unicode/utf8"
)

type CorrectionInput struct {
	Text      string `json:"correction"`
	Confirmed bool   `json:"confirmed_allowed_fields"`
}
type CorrectionResult struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	Number  int    `json:"number,omitempty"`
}

// Official contract: /reference/emitir_carta_correcao. One durable attempt per
// sale; an uncertain response never permits another POST or a new reference.
// Multiple successive CCes require a reconciled remote event history first.
func (p *FocusProvider) Correct(ctx context.Context, a RequestContext, in DocumentRequest, correction CorrectionInput) (CorrectionResult, error) {
	if in.Model != "55" || in.Environment != 2 {
		return CorrectionResult{}, ErrCheckoutProduction
	}
	if err := authorizeCustomerCredentials(ctx, a); err != nil {
		return CorrectionResult{}, err
	}
	if err := p.scope(ctx, a, identity.CancelSale); err != nil {
		return CorrectionResult{}, err
	}
	if !correction.Confirmed || !transportText(correction.Text, 15, 1000) || utf8.RuneCountInString(correction.Text) > 1000 {
		return CorrectionResult{}, fiscal.ErrInvalid
	}
	if p.safeMessage(correction.Text) != correction.Text {
		return CorrectionResult{}, fiscal.ErrInvalid
	}
	current, err := p.Query(ctx, a, in)
	if err != nil {
		return CorrectionResult{}, err
	}
	if current.State != "authorized_homologation" {
		return CorrectionResult{}, fiscal.ErrConflict
	}
	ref := focusReference(a.Actor.TenantID, in.SaleID)
	body, _ := json.Marshal(map[string]string{"correcao": correction.Text})
	won, err := p.claim(ctx, a, in.SaleID, ref+"-cce", "checkout.focus_correction", body)
	if err != nil {
		return CorrectionResult{}, err
	}
	if !won {
		var marker string
		if a.DB.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND event_type='checkout.focus_correction'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, ref+"-cce").Scan(&marker) != nil {
			return CorrectionResult{}, ErrUnavailable
		}
		var saved struct {
			SHA256 string `json:"request_sha256"`
		}
		sum := sha256.Sum256(body)
		if json.Unmarshal([]byte(marker), &saved) != nil || saved.SHA256 != hex.EncodeToString(sum[:]) {
			return CorrectionResult{}, fiscal.ErrConflict
		}
		var raw string
		err := a.DB.QueryRowContext(ctx, `SELECT payload_json FROM outbox WHERE tenant_id=? AND store_id=? AND device_id=? AND operation_id=? AND event_type='checkout.focus_correction_result'`, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, ref+"-cce-result").Scan(&raw)
		if err == sql.ErrNoRows {
			return CorrectionResult{State: "correction_reconciliation_required"}, nil
		}
		var result CorrectionResult
		if err != nil || json.Unmarshal([]byte(raw), &result) != nil {
			return CorrectionResult{}, ErrUnavailable
		}
		return result, nil
	}
	result := p.sendCorrection(ctx, ref, body)
	raw, _ := json.Marshal(result)
	id, err := localdb.NewID()
	if err != nil {
		return CorrectionResult{State: "correction_reconciliation_required"}, ErrUnavailable
	}
	durable, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	_, err = a.DB.ExecContext(durable, `INSERT INTO outbox(event_id,tenant_id,store_id,device_id,operation_id,aggregate_id,event_type,schema_version,payload_json,created_at) VALUES(?,?,?,?,?,?,'checkout.focus_correction_result',1,?,?)`, id, a.Actor.TenantID, a.Actor.StoreID, a.Device.DeviceID, ref+"-cce-result", in.SaleID, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return CorrectionResult{State: "correction_reconciliation_required"}, ErrUnavailable
	}
	return result, nil
}
func (p *FocusProvider) sendCorrection(ctx context.Context, ref string, body []byte) CorrectionResult {
	if p == nil || p.configuration.Environment != "homologation" || p.configuration.FocusBaseURL != focusHomologation || p.configuration.CertificateType != "A1" {
		return CorrectionResult{State: "correction_failed"}
	}
	result := CorrectionResult{State: "correction_reconciliation_required"}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, focusHomologation+"/v2/nfe/"+url.PathEscape(ref)+"/carta_correcao", bytes.NewReader(body))
	if err != nil {
		return result
	}
	req.SetBasicAuth(string(p.token), "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	response, err := p.client.Do(req)
	if err == nil {
		defer response.Body.Close()
		raw, e := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
		var reply struct {
			Status  string `json:"status"`
			Message string `json:"mensagem_sefaz"`
			Number  int    `json:"numero_carta_correcao"`
		}
		if e == nil && len(raw) <= 1024*1024 && json.Unmarshal(raw, &reply) == nil {
			result.Message = p.safeMessage(reply.Message)
			switch {
			case response.StatusCode == 200 && reply.Status == "autorizado" && reply.Number >= 1 && reply.Number <= 20:
				result.State = "correction_authorized_homologation"
				result.Number = reply.Number
			case reply.Status == "erro_autorizacao" && (response.StatusCode == 200 || response.StatusCode == 400):
				result.State = "correction_failed"
			case response.StatusCode == 400 || response.StatusCode == 401 || response.StatusCode == 404 || response.StatusCode == 415:
				result.State = "correction_failed"
			}
		}
	}
	return result
}
