// Package payables records local supplier obligations, never payment execution.
package payables

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/identity"
	"unicode/utf8"
)

var (
	ErrInvalid      = errors.New("conta a pagar invalida")
	ErrNotFound     = errors.New("conta nao encontrada neste contexto")
	ErrConflict     = errors.New("documento ou estado conflitante")
	ErrInconsistent = errors.New("historico de conta inconsistente")
	ErrRange        = errors.New("agenda excede faixa ou limite")
	ErrText         = errors.New("texto incompativel com CSV seguro")
)

const MaxSafe int64 = 9007199254740991
const MaxDocuments = 1000

type Installment struct {
	ID          string `json:"id"`
	DueOn       string `json:"due_on"`
	AmountCents int64  `json:"amount_cents"`
}
type Event struct {
	OperationID string `json:"operation_id"`
	ActorID     string `json:"actor_id"`
	Action      string `json:"action"`
	FromState   string `json:"from_state"`
	ToState     string `json:"to_state"`
	Reason      string `json:"reason"`
	CreatedAt   string `json:"created_at"`
}
type Payable struct {
	ID                        string        `json:"id"`
	SupplierID                string        `json:"supplier_id"`
	SupplierName              string        `json:"supplier_name"`
	DocumentReference         string        `json:"document_reference"`
	Description               string        `json:"description"`
	IssuedOn                  string        `json:"issued_on"`
	Currency                  string        `json:"currency"`
	TotalCents                int64         `json:"total_cents"`
	CreatorID                 string        `json:"creator_id"`
	State                     string        `json:"state"`
	CreatedAt                 string        `json:"created_at"`
	UpdatedAt                 string        `json:"updated_at"`
	LastOperationID           string        `json:"last_operation_id"`
	Installments              []Installment `json:"installments"`
	History                   []Event       `json:"history"`
	PaymentExecutionAvailable bool          `json:"payment_execution_available"`
	SettledCents              *int64        `json:"settled_cents"`
}
type Result struct {
	OperationID string  `json:"operation_id"`
	Repeated    bool    `json:"repeated"`
	Payable     Payable `json:"payable"`
}
type CreateInput struct {
	OperationID       string        `json:"operation_id"`
	PayableID         string        `json:"payable_id"`
	SupplierID        string        `json:"supplier_id"`
	DocumentReference string        `json:"document_reference"`
	Description       string        `json:"description"`
	IssuedOn          string        `json:"issued_on"`
	Currency          string        `json:"currency"`
	TotalCents        int64         `json:"total_cents"`
	Installments      []Installment `json:"installments"`
}
type ActionInput struct {
	OperationID string `json:"operation_id"`
	Reason      string `json:"reason"`
}
type Page struct {
	Items      []Payable `json:"items"`
	TotalCount int       `json:"total_count"`
	Limit      int       `json:"limit"`
	Offset     int       `json:"offset"`
	HasMore    bool      `json:"has_more"`
}

func key(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for i, b := range []byte(s) {
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') && (i == 0 || b != '_' && b != '.' && b != ':' && b != '-') {
			return false
		}
	}
	return true
}
func text(s string, n int) bool {
	return s != "" && s == strings.TrimSpace(s) && utf8.ValidString(s) && utf8.RuneCountInString(s) <= n && !strings.ContainsRune(s, 0)
}
func date(s string) bool {
	t, e := time.Parse(time.DateOnly, s)
	return e == nil && len(s) == 10 && t.Format(time.DateOnly) == s && t.Year() >= 1900
}
func schedule(parts []Installment, total int64, issued string) bool {
	if total < 1 || total > MaxSafe || !date(issued) || len(parts) < 1 || len(parts) > 120 {
		return false
	}
	sum := int64(0)
	seen := map[string]bool{}
	previous := issued
	for _, p := range parts {
		if !key(p.ID) || seen[p.ID] || !date(p.DueOn) || p.DueOn < previous || p.AmountCents < 1 || p.AmountCents > MaxSafe-sum {
			return false
		}
		seen[p.ID] = true
		sum += p.AmountCents
		previous = p.DueOn
	}
	return sum == total
}
func checked(ctx context.Context, tx *sql.Tx, q string, args ...any) error {
	r, e := tx.ExecContext(ctx, q, args...)
	if e != nil {
		var c interface{ Code() int }
		if errors.As(e, &c) && (c.Code() == 1555 || c.Code() == 2067) {
			return ErrConflict
		}
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrInconsistent
	}
	return nil
}
func begin(ctx context.Context, db *sql.DB, a identity.Scope, d identity.DeviceContext) (*sql.Tx, error) {
	if db == nil {
		return nil, ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	if e = identity.CanOperateTx(ctx, tx, a, d, identity.ViewAccounting); e != nil {
		tx.Rollback()
		return nil, e
	}
	return tx, nil
}
