package sale

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrProofInvalid   = errors.New("comprovante invalido")
	ErrProofNotFound  = errors.New("comprovante inexistente neste contexto")
	ErrProofConflict  = errors.New("comprovante ou estado conflitante")
	ErrProofIntegrity = errors.New("comprovante e historico nao conferem")
	ErrProofRange     = errors.New("relatorio de comprovantes excede limite exato")
	ErrProofText      = errors.New("texto incompativel com CSV de comprovantes")
)

type ProofEvent struct {
	OperationID string `json:"operation_id"`
	ActorID     string `json:"actor_id"`
	Action      string `json:"action"`
	FromState   string `json:"from_state"`
	ToState     string `json:"to_state"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
type PaymentProof struct {
	ID                string       `json:"id"`
	CheckoutID        string       `json:"checkout_id"`
	PartID            string       `json:"part_id"`
	SubmitterID       string       `json:"submitter_id"`
	Method            string       `json:"method"`
	AmountCents       int64        `json:"amount_cents"`
	Provider          string       `json:"provider"`
	Terminal          string       `json:"terminal"`
	Reference         string       `json:"reference"`
	OccurredAt        string       `json:"occurred_at"`
	State             string       `json:"state"`
	ReviewerID        *string      `json:"reviewer_id"`
	ProviderConfirmed bool         `json:"provider_confirmed"`
	Effect            string       `json:"effect"`
	CreatedAt         string       `json:"created_at"`
	UpdatedAt         string       `json:"updated_at"`
	LastOperationID   string       `json:"last_operation_id"`
	History           []ProofEvent `json:"history"`
}
type ProofResult struct {
	OperationID string       `json:"operation_id"`
	Repeated    bool         `json:"repeated"`
	Proof       PaymentProof `json:"proof"`
}
type SubmitProofInput struct {
	OperationID string `json:"operation_id"`
	ProofID     string `json:"proof_id"`
	Provider    string `json:"provider"`
	Terminal    string `json:"terminal"`
	Reference   string `json:"reference"`
	OccurredAt  string `json:"occurred_at"`
}
type ReviewProofInput struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
}
type ProofPage struct {
	Items      []PaymentProof `json:"items"`
	State      string         `json:"state"`
	TotalCount int64          `json:"total_count"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
	HasMore    bool           `json:"has_more"`
}

func proofKey(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for i, c := range []byte(s) {
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alnum && (i == 0 || c != '_' && c != '.' && c != ':' && c != '-') {
			return false
		}
	}
	return true
}
func proofText(s string, n int) bool {
	return s != "" && s == strings.TrimSpace(s) && utf8.ValidString(s) && utf8.RuneCountInString(s) <= n && !strings.ContainsRune(s, 0)
}
func proofChecked(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		var code interface{ Code() int }
		if errors.As(err, &code) && (code.Code() == 1555 || code.Code() == 2067) {
			return ErrProofConflict
		}
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrProofIntegrity
	}
	return nil
}
