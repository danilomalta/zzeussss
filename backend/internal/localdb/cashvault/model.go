// Package cashvault manages local physical-cash destinations, not bank accounts.
package cashvault

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid      = errors.New("dados de cofre invalidos")
	ErrNotFound     = errors.New("cofre ou recibo nao encontrado neste escopo")
	ErrConflict     = errors.New("operacao de cofre conflitante")
	ErrInconsistent = errors.New("saldo e origem do cofre nao conferem")
	ErrRange        = errors.New("saldo de cofre fora da faixa exata")
)

const maxSafe int64 = 9007199254740991

type Vault struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	State               string `json:"state"`
	BalanceCents        int64  `json:"balance_cents"`
	CreatedBy           string `json:"created_by"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	CreationOperationID string `json:"creation_operation_id"`
	LastOperationID     string `json:"last_operation_id"`
}
type Entry struct {
	ID             string  `json:"id"`
	VaultID        string  `json:"vault_id"`
	OperationID    string  `json:"operation_id"`
	Kind           string  `json:"kind"`
	DeltaCents     int64   `json:"delta_cents"`
	BagID          *string `json:"bag_id"`
	CashSessionID  *string `json:"cash_session_id"`
	CashMovementID *string `json:"cash_movement_id"`
	ActorID        string  `json:"actor_id"`
	Reason         string  `json:"reason"`
	CreatedAt      string  `json:"created_at"`
}
type Result struct {
	OperationID    string  `json:"operation_id"`
	Kind           string  `json:"kind"`
	Reason         string  `json:"reason"`
	CreatedAt      string  `json:"created_at"`
	Repeated       bool    `json:"repeated"`
	Vaults         []Vault `json:"vaults"`
	Entries        []Entry `json:"entries"`
	BagID          *string `json:"bag_id"`
	CashMovementID *string `json:"cash_movement_id"`
}
type CreateInput struct {
	OperationID string `json:"operation_id"`
	VaultID     string `json:"vault_id"`
	Name        string `json:"name"`
}
type Input struct {
	OperationID   string `json:"operation_id"`
	BagID         string `json:"bag_id"`
	DestinationID string `json:"destination_id"`
	CashSessionID string `json:"cash_session_id"`
	AmountCents   int64  `json:"amount_cents"`
	Reason        string `json:"reason"`
}
type Statement struct {
	Vault      Vault   `json:"vault"`
	Items      []Entry `json:"items"`
	TotalCount int64   `json:"total_count"`
	Limit      int     `json:"limit"`
	Offset     int     `json:"offset"`
	HasMore    bool    `json:"has_more"`
}
type Page struct {
	Items      []Vault `json:"items"`
	TotalCount int64   `json:"total_count"`
	Limit      int     `json:"limit"`
	Offset     int     `json:"offset"`
	HasMore    bool    `json:"has_more"`
}

func key(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for i, c := range []byte(s) {
		alpha := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alpha && (i == 0 || c != '_' && c != '.' && c != ':' && c != '-') {
			return false
		}
	}
	return true
}
func text(s string, n int) bool {
	return s != "" && s == strings.TrimSpace(s) && utf8.ValidString(s) && utf8.RuneCountInString(s) <= n && !strings.ContainsRune(s, 0)
}
func checked(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		var c interface{ Code() int }
		if errors.As(err, &c) && (c.Code() == 1555 || c.Code() == 2067) {
			return ErrConflict
		}
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrInconsistent
	}
	return nil
}
