// Package cashaccounting exposes local closed-drawer operational records to
// authorized accounting readers. It does not implement a statutory ledger.
package cashaccounting

import (
	"errors"
	"time"
	"titansystem-backend/internal/localdb/cashjournal"
	"titansystem-backend/internal/localdb/register"
)

const MaxScopeSessions = 10000
const MaxPeriodSessions = 500
const MaxEntries = 10000
const MaxCSVBytes = 16 << 20
const maxSafe int64 = 9007199254740991

var (
	ErrInvalid      = errors.New("periodo contabil invalido")
	ErrInconsistent = errors.New("fontes do periodo nao conferem")
	ErrRange        = errors.New("periodo excede limite ou precisao exata")
	ErrText         = errors.New("texto incompativel com CSV contabil seguro")
)

type Window struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func parseWindow(w Window) (time.Time, time.Time, error) {
	if len(w.From) > 64 || len(w.To) > 64 {
		return time.Time{}, time.Time{}, ErrInvalid
	}
	from, err := time.Parse(time.RFC3339Nano, w.From)
	if err != nil {
		return from, time.Time{}, ErrInvalid
	}
	to, err := time.Parse(time.RFC3339Nano, w.To)
	if err != nil || !from.Before(to) || to.Sub(from) > 31*24*time.Hour {
		return from, to, ErrInvalid
	}
	return from.UTC(), to.UTC(), nil
}

type Session struct {
	OperatorID string                `json:"operator_id"`
	Report     register.ClosedReport `json:"report"`
}
type Posting struct {
	SessionID  string            `json:"session_id"`
	OperatorID string            `json:"operator_id"`
	Entry      cashjournal.Entry `json:"entry"`
}
type Report struct {
	Scope                    string                    `json:"scope"`
	Window                   Window                    `json:"window"`
	GeneratedAt              string                    `json:"generated_at"`
	Complete                 bool                      `json:"complete"`
	BankSettlementAvailable  bool                      `json:"bank_settlement_available"`
	StatutoryLedgerAvailable bool                      `json:"statutory_ledger_available"`
	Summary                  map[string]int64          `json:"summary"`
	Payments                 []register.PaymentSummary `json:"payments"`
	Sessions                 []Session                 `json:"sessions"`
	Entries                  []Posting                 `json:"entries"`
}
type SessionPage struct {
	Scope      string                    `json:"scope"`
	Window     Window                    `json:"window"`
	Complete   bool                      `json:"complete"`
	Summary    map[string]int64          `json:"summary"`
	Payments   []register.PaymentSummary `json:"payments"`
	Items      []Session                 `json:"items"`
	TotalCount int                       `json:"total_count"`
	Limit      int                       `json:"limit"`
	Offset     int                       `json:"offset"`
	HasMore    bool                      `json:"has_more"`
}
type PostingPage struct {
	Scope      string           `json:"scope"`
	Window     Window           `json:"window"`
	Complete   bool             `json:"complete"`
	Summary    map[string]int64 `json:"summary"`
	Items      []Posting        `json:"items"`
	TotalCount int              `json:"total_count"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
	HasMore    bool             `json:"has_more"`
}

var metricNames = []string{"session_count", "sale_count", "gross_sales_cents", "cancelled_sales_cents", "returned_sales_cents", "net_sales_cents", "sum_openings_cents", "sum_expected_at_closures_cents", "sum_declared_at_closures_cents", "sum_closure_differences_cents", "supply_cents", "withdrawal_cents", "journal_entry_count", "journal_debit_cents", "journal_credit_cents", "unclassified_count"}
