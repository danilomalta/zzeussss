// Package cashcount records physical counts without adjusting vault money.
package cashcount

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid      = errors.New("conferencia invalida")
	ErrNotFound     = errors.New("conferencia inexistente neste contexto")
	ErrConflict     = errors.New("estado de conferencia conflitante")
	ErrInconsistent = errors.New("historico de conferencia inconsistente")
	ErrRange        = errors.New("relatorio excede faixa ou limite")
	ErrText         = errors.New("texto incompativel com CSV seguro")
)

const maxSafe int64 = 9007199254740991
const MaxReportVaults = 1000

type Event struct {
	OperationID string `json:"operation_id"`
	ActorID     string `json:"actor_id"`
	Action      string `json:"action"`
	FromState   string `json:"from_state"`
	ToState     string `json:"to_state"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
type Count struct {
	ID               string  `json:"id"`
	VaultID          string  `json:"vault_id"`
	CreatorID        string  `json:"creator_id"`
	CounterID        string  `json:"counter_id"`
	Reason           string  `json:"reason"`
	State            string  `json:"state"`
	ExpectedCents    *int64  `json:"expected_cents"`
	CountedCents     *int64  `json:"counted_cents"`
	DifferenceCents  *int64  `json:"difference_cents"`
	VaultOperationID string  `json:"vault_operation_id"`
	ReviewerID       *string `json:"reviewer_id"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	LastOperationID  string  `json:"last_operation_id"`
	History          []Event `json:"history"`
}
type Result struct {
	OperationID string `json:"operation_id"`
	Repeated    bool   `json:"repeated"`
	Count       Count  `json:"count"`
}
type CreateInput struct {
	OperationID string `json:"operation_id"`
	CountID     string `json:"count_id"`
	VaultID     string `json:"vault_id"`
	CounterID   string `json:"counter_id"`
	Reason      string `json:"reason"`
}
type ActionInput struct {
	OperationID  string `json:"operation_id"`
	CountedCents int64  `json:"counted_cents"`
	Reason       string `json:"reason"`
}
type Page struct {
	Items      []Count `json:"items"`
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
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alnum && (i == 0 || c != '_' && c != '.' && c != ':' && c != '-') {
			return false
		}
	}
	return true
}
func text(s string, n int) bool {
	return s != "" && s == strings.TrimSpace(s) && utf8.ValidString(s) && utf8.RuneCountInString(s) <= n && !strings.ContainsRune(s, 0)
}
func visible(c Count) Count {
	if c.CountedCents == nil {
		c.ExpectedCents = nil
	}
	return c
}
func checked(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		var code interface{ Code() int }
		if errors.As(err, &code) && (code.Code() == 1555 || code.Code() == 2067) {
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
