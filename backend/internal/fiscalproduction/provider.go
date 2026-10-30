package fiscalproduction

import (
	"context"
	"encoding/json"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/localdb/fiscal"
)

// DocumentRequest carries business identifiers and an existing invoice contract,
// never credentials, provider URLs or provider-specific transport objects.
// With SaleID empty, Emit uses the already prepared/approved fiscal intent.
type DocumentRequest struct {
	Model             string
	Environment       int
	SaleID            string
	Invoice           json.RawMessage
	OperationID       string
	Revision          int
	ReviewSHA256      string
	QueryOperationID  string
	Reason            string
	ApprovalReference string
}
type DocumentResult struct {
	Message     string
	State       string
	OperationID string
	IntentID    string
	Key         string
	ErrorCode   string
	Evidence    json.RawMessage
}
type ProviderBinding struct {
	TenantID        string `json:"tenant_id"`
	StoreID         string `json:"store_id"`
	DeviceID        string `json:"device_id"`
	ProfileID       string `json:"profile_id"`
	ProfileRevision int    `json:"profile_revision"`
}

type FiscalProvider interface {
	ValidateCheckout(context.Context, RequestContext, int) error
	Emit(context.Context, RequestContext, DocumentRequest) (DocumentResult, error)
	Query(context.Context, RequestContext, DocumentRequest) (DocumentResult, error)
	Cancel(context.Context, RequestContext, DocumentRequest) (DocumentResult, error)
	Reprocess(context.Context, RequestContext, DocumentRequest) (DocumentResult, error)
}

type OwnProvider struct{ runtime *Runtime }

func NewOwnProvider(r *Runtime) *OwnProvider { return &OwnProvider{runtime: r} }
func (p *OwnProvider) ValidateCheckout(ctx context.Context, a RequestContext, environment int) error {
	if p == nil {
		return ErrUnavailable
	}
	return p.runtime.ValidateCheckoutHomologation(ctx, a, environment)
}
func (p *OwnProvider) Emit(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	if p == nil || p.runtime == nil {
		return DocumentResult{}, ErrUnavailable
	}
	if in.Model != "55" && in.Model != "65" {
		return DocumentResult{}, fiscal.ErrInvalid
	}
	if in.SaleID != "" {
		if in.Model != "65" || in.Environment != 2 {
			return DocumentResult{}, ErrCheckoutProduction
		}
		var invoice fiscalcore.Invoice
		if fiscalcore.DecodeInput(in.Invoice, &invoice) != nil || invoice.Environment != in.Environment {
			return DocumentResult{}, fiscal.ErrInvalid
		}
		state, err := p.runtime.EmitCheckoutHomologation(ctx, a, in.SaleID, invoice)
		return DocumentResult{State: state.State, OperationID: state.OperationID, IntentID: state.IntentID, Key: state.Key, ErrorCode: state.ErrorCode}, err
	}
	// Prepared NF-e/NFC-e retain the existing review, approval and journal checks.
	if in.Model != p.runtime.Binding().Model || in.Environment != 1 {
		return DocumentResult{}, fiscal.ErrInvalid
	}
	evidence, err := p.runtime.Emit(ctx, a, in.OperationID, in.Revision, in.ReviewSHA256)
	raw, _ := json.Marshal(evidence)
	return DocumentResult{State: evidence.State, OperationID: in.OperationID, Evidence: raw}, err
}
func (p *OwnProvider) Query(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	if p == nil || p.runtime == nil {
		return DocumentResult{}, ErrUnavailable
	}
	status, err := p.runtime.Query(ctx, a, in.OperationID, in.Revision)
	raw, _ := json.Marshal(status)
	return DocumentResult{State: status.Evidence.State, OperationID: in.OperationID, Evidence: raw}, err
}
func (p *OwnProvider) Cancel(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	if p == nil || p.runtime == nil {
		return DocumentResult{}, ErrUnavailable
	}
	evidence, err := p.runtime.Cancel(ctx, a, in.OperationID, in.QueryOperationID, in.Reason, in.ApprovalReference)
	raw, _ := json.Marshal(evidence)
	return DocumentResult{OperationID: in.OperationID, Evidence: raw}, err
}
func (p *OwnProvider) Reprocess(ctx context.Context, a RequestContext, in DocumentRequest) (DocumentResult, error) {
	// Existing Emit performs durable recovery/idempotent replay before any send.
	return p.Emit(ctx, a, in)
}
